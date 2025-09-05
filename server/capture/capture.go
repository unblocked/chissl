package capture

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/NextChapterSoftware/chissl/share/settings"
)

// EventType represents the type of capture event.
type EventType string

const (
	ConnOpen   EventType = "conn_open"
	ConnClose  EventType = "conn_close"
	ReqHeaders EventType = "req_headers"
	ReqBody    EventType = "req_body"
	ResHeaders EventType = "res_headers"
	ResBody    EventType = "res_body"
	Metric     EventType = "metric"
)

// Event represents a captured event for a tunnel/connection.
type Event struct {
	Time      time.Time `json:"time"`
	TunnelID  string    `json:"tunnel_id"`
	User      string    `json:"user"`
	ConnID    string    `json:"conn_id"`
	Type      EventType `json:"type"`
	Meta      any       `json:"meta,omitempty"`
	Data      []byte    `json:"data,omitempty"`
	Truncated bool      `json:"truncated,omitempty"`
}

// Ring is a fixed-size ring buffer of events.
type Ring struct {
	mu    sync.RWMutex
	buf   []Event
	start int
	len   int
}

func NewRing(cap int) *Ring { return &Ring{buf: make([]Event, cap)} }

func (r *Ring) Add(e Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.buf) == 0 {
		return
	}
	pos := (r.start + r.len) % len(r.buf)
	r.buf[pos] = e
	if r.len < len(r.buf) {
		r.len++
	} else {
		r.start = (r.start + 1) % len(r.buf)
	}
}

func (r *Ring) Snapshot() []Event {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Event, r.len)
	for i := 0; i < r.len; i++ {
		out[i] = r.buf[(r.start+i)%len(r.buf)]
	}
	return out
}

// Service manages capture buffers per tunnel and persistence.
type Service struct {
	mu            sync.RWMutex
	buffers       map[string]*Ring // tunnelID -> ring
	maxBytes      int              // per event body cap
	persist       bool
	dir           string
	rotateBytes   int64
	retentionDays int
	cleanEvery    time.Duration
	// subscribers per tunnel for live streaming
	subs map[string]map[chan Event]struct{}
	// optional hooks for DB updates
	onMetric    func(tunnelID string, sent, received int64)
	onConnDelta func(tunnelID string, delta int)
}

func NewService(maxEvents, maxBytes int) *Service {
	s := &Service{
		buffers:       make(map[string]*Ring),
		maxBytes:      maxBytes,
		dir:           settings.Env("CAPTURE_DIR"),
		rotateBytes:   int64(settings.EnvInt("CAPTURE_ROTATE_BYTES", 10*1024*1024)),
		retentionDays: settings.EnvInt("CAPTURE_RETENTION_DAYS", 7),
		cleanEvery:    settings.EnvDuration("CAPTURE_CLEAN_INTERVAL", 24*time.Hour),
	}
	// default to persist ON unless explicitly disabled
	if v := settings.Env("CAPTURE_PERSIST"); v == "" {
		s.persist = true
	} else {
		s.persist = settings.EnvBool("CAPTURE_PERSIST")
	}
	if s.dir == "" {
		s.dir = "./data/capture"
	}
	// initialize subscribers map
	s.subs = make(map[string]map[chan Event]struct{})
	if s.persist {
		_ = os.MkdirAll(s.dir, 0o755)
		go s.cleanupLoop()
	}
	// Load existing persisted data on startup if persistence is enabled
	if s.persist {
		s.loadPersistedData()
	}
	return s
}

// SetOnMetric sets a hook to be called when a connection closes with byte counts.
func (s *Service) SetOnMetric(fn func(tunnelID string, sent, received int64)) { s.onMetric = fn }

// SetOnConnDelta sets a hook to be called when a connection opens (+1) or closes (-1).
func (s *Service) SetOnConnDelta(fn func(tunnelID string, delta int)) { s.onConnDelta = fn }

// AddEvent adds to ring and optionally persists
func (s *Service) AddEvent(tunnelID string, e Event, maxEvents int) {
	if len(e.Data) > s.maxBytes {
		e.Data = e.Data[:s.maxBytes]
		e.Truncated = true
	}

	// fan out live
	if subs := s.subs[tunnelID]; subs != nil {
		for ch := range subs {
			select {
			case ch <- e:
			default:
				// drop if subscriber is slow
			}
		}
	}

	s.ring(tunnelID, maxEvents).Add(e)
	if s.persist {
		_ = s.appendJSONL(tunnelID, e)
	}

}

// Subscribe returns a channel that will receive future events for a tunnel.
func (s *Service) Subscribe(tunnelID string) chan Event {
	ch := make(chan Event, 64)
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.subs[tunnelID]
	if m == nil {
		m = make(map[chan Event]struct{})
		s.subs[tunnelID] = m
	}
	m[ch] = struct{}{}
	return ch
}

// Unsubscribe removes a subscriber channel.
func (s *Service) Unsubscribe(tunnelID string, ch chan Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m := s.subs[tunnelID]; m != nil {
		delete(m, ch)
		close(ch)
		if len(m) == 0 {
			delete(s.subs, tunnelID)
		}
	}
}

func (s *Service) GetRecent(tunnelID string, maxEvents int) []Event {
	return s.ring(tunnelID, maxEvents).Snapshot()
}

func (s *Service) ring(tunnelID string, maxEvents int) *Ring {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.buffers[tunnelID]
	if !ok {
		r = NewRing(maxEvents)
		s.buffers[tunnelID] = r
	}
	return r
}

// appendJSONL appends event as a JSON line into per-connection rotating file
func (s *Service) appendJSONL(tunnelID string, e Event) error {
	if e.ConnID == "" {
		return errors.New("missing connID")
	}
	dir := filepath.Join(s.dir, tunnelID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	// current file
	path := filepath.Join(dir, fmt.Sprintf("%s.log", e.ConnID))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	// rotate if needed
	if st, err := f.Stat(); err == nil {
		if st.Size() >= s.rotateBytes {
			_ = f.Close()
			// find next suffix
			for i := 1; ; i++ {
				rot := filepath.Join(dir, fmt.Sprintf("%s.log.%d", e.ConnID, i))
				if _, err := os.Stat(rot); os.IsNotExist(err) {
					_ = os.Rename(path, rot)
					break
				}
			}
			// reopen fresh file
			f, err = os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
			if err != nil {
				return err
			}
			defer f.Close()
		}
	}
	// write JSONL
	b, _ := json.Marshal(e)
	b = append(b, '\n')
	_, _ = f.Write(b)
	return nil
}

// ListConnections returns known connection IDs for a tunnel based on persisted files
func (s *Service) ListConnections(tunnelID string) ([]string, error) {
	dir := filepath.Join(s.dir, tunnelID)
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	m := map[string]struct{}{}
	for _, e := range ents {
		name := e.Name()
		if strings.HasSuffix(name, ".log") || strings.Contains(name, ".log.") {
			base := name
			if i := strings.Index(base, ".log"); i >= 0 {
				base = base[:i]
			}
			if base != "" {
				m[base] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out, nil
}

// LatestLogFile returns the latest file path for a given connection (base or highest rotated)
func (s *Service) LatestLogFile(tunnelID, connID string) (string, error) {
	dir := filepath.Join(s.dir, tunnelID)
	base := filepath.Join(dir, connID+".log")
	if _, err := os.Stat(base); err == nil {
		return base, nil
	}
	// find highest suffix
	maxI := -1
	var best string
	ents, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	prefix := connID + ".log."
	for _, e := range ents {
		name := e.Name()
		if strings.HasPrefix(name, prefix) {
			var i int
			if _, err := fmt.Sscanf(name, connID+".log.%d", &i); err == nil {
				if i > maxI {
					maxI, best = i, filepath.Join(dir, name)
				}
			}
		}
	}
	if best != "" {
		return best, nil
	}
	return base, os.ErrNotExist
}

func (s *Service) cleanupLoop() {
	t := time.NewTicker(s.cleanEvery)
	defer t.Stop()
	for range t.C {
		s.cleanup()
	}
}

func (s *Service) cleanup() {
	if s.retentionDays <= 0 {
		return
	}
	cutoff := time.Now().Add(-time.Duration(s.retentionDays) * 24 * time.Hour)
	_ = filepath.Walk(s.dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		// delete files older than cutoff
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(path)
		}
		return nil
	})
}

// loadPersistedData loads existing capture files from disk into memory rings on startup
func (s *Service) loadPersistedData() {
	if s.dir == "" {
		return
	}

	// Walk through tunnel directories and load recent events
	tunnelDirs, err := os.ReadDir(s.dir)
	if err != nil {
		return // No existing data or can't read
	}

	for _, tunnelDir := range tunnelDirs {
		if !tunnelDir.IsDir() {
			continue
		}

		tunnelID := tunnelDir.Name()
		tunnelPath := filepath.Join(s.dir, tunnelID)

		// Load events from all connection files in this tunnel
		connFiles, err := os.ReadDir(tunnelPath)
		if err != nil {
			continue
		}

		var allEvents []Event
		for _, connFile := range connFiles {
			if !strings.HasSuffix(connFile.Name(), ".log") {
				continue
			}

			filePath := filepath.Join(tunnelPath, connFile.Name())
			events := s.loadEventsFromFile(filePath)
			allEvents = append(allEvents, events...)
		}

		// Sort by time and keep most recent
		if len(allEvents) > 0 {
			sort.Slice(allEvents, func(i, j int) bool {
				return allEvents[i].Time.After(allEvents[j].Time)
			})

			// Initialize ring buffer for this tunnel
			s.mu.Lock()
			if s.buffers[tunnelID] == nil {
				s.buffers[tunnelID] = NewRing(500) // Use default max events
			}

			// Add events to ring (most recent first, so they appear in correct order)
			maxLoad := 500
			if len(allEvents) < maxLoad {
				maxLoad = len(allEvents)
			}
			for i := 0; i < maxLoad; i++ {
				s.buffers[tunnelID].Add(allEvents[i])
			}
			s.mu.Unlock()
		}
	}
}

// loadEventsFromFile reads events from a single log file
func (s *Service) loadEventsFromFile(filePath string) []Event {
	var events []Event

	file, err := os.Open(filePath)
	if err != nil {
		return events
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var event Event
		if err := json.Unmarshal([]byte(line), &event); err == nil {
			events = append(events, event)
		}
	}

	return events
}
