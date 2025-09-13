package chserver

import (
	"bufio"
	"bytes"
	"context"

	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/NextChapterSoftware/chissl/server/capture"
	"github.com/NextChapterSoftware/chissl/share/database"
	"github.com/NextChapterSoftware/chissl/share/settings"
	"github.com/NextChapterSoftware/chissl/share/tunnel"
)

// MulticastManager manages persistent HTTPS endpoints that broadcast inbound requests to all subscribers via capture SSE.
type MulticastManager struct {
	mu        sync.RWMutex
	items     map[string]*ActiveMulticast // id -> active instance
	capture   *capture.Service
	db        database.Database
	tlsConfig *tls.Config
}

type ActiveMulticast struct {
	ID     string
	Config *database.MulticastTunnel
	Server *http.Server
	Cancel context.CancelFunc
	subsMu sync.RWMutex
	subs   map[string]*subscriber
}

// subscriber represents a connected client that wants to receive multicast payloads
// via its declared remote (e.g., 8090->5050).
type subscriber struct {
	ID       string
	Tun      *tunnel.Tunnel
	Remote   settings.Remote
	Username string
}

func NewMulticastManager(captureSvc *capture.Service, db database.Database, tlsConf *tls.Config) *MulticastManager {
	return &MulticastManager{
		items:     make(map[string]*ActiveMulticast),
		capture:   captureSvc,
		db:        db,
		tlsConfig: tlsConf,
	}
}

func (m *MulticastManager) getActiveByPort(port int) *ActiveMulticast {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, a := range m.items {
		if a.Config != nil && a.Config.Port == port && a.Config.Enabled {
			return a
		}
	}
	return nil
}

func (m *MulticastManager) AddSubscriber(port int, sub *subscriber) error {
	a := m.getActiveByPort(port)
	if a == nil {
		return fmt.Errorf("no active multicast on port %d", port)
	}
	a.subsMu.Lock()
	a.subs[sub.ID] = sub
	a.subsMu.Unlock()
	return nil
}

func (m *MulticastManager) RemoveSubscriber(port int, subID string) {
	if a := m.getActiveByPort(port); a != nil {
		a.subsMu.Lock()
		delete(a.subs, subID)
		a.subsMu.Unlock()
	}
}

func (m *MulticastManager) UpdateTLSConfig(tlsConf *tls.Config) { m.tlsConfig = tlsConf }

// StartMulticast starts an HTTPS server for the given config (webhook mode only in phase 1)
func (m *MulticastManager) StartMulticast(cfg *database.MulticastTunnel) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check port collision
	for _, a := range m.items {
		if a.Config.Port == cfg.Port {
			return fmt.Errorf("port %d already in use by multicast %s", cfg.Port, a.ID)
		}
	}

	_, cancel := context.WithCancel(context.Background())
	var h http.Handler
	if cfg.Mode == "bidirectional" {
		h = m.createBidirectionalHandler(cfg)
	} else {
		h = m.createWebhookHandler(cfg)
	}
	srv := &http.Server{Addr: fmt.Sprintf(":%d", cfg.Port), Handler: h, TLSConfig: m.tlsConfig}
	active := &ActiveMulticast{ID: cfg.ID, Config: cfg, Server: srv, Cancel: cancel, subs: make(map[string]*subscriber)}

	// Start server goroutine
	go func() {
		defer cancel()
		ln, err := net.Listen("tcp", srv.Addr)
		if err != nil {
			cfg.Status = "error"
			if m.db != nil {
				_ = m.db.UpdateMulticastTunnel(cfg)
			}
			return
		}
		cfg.Status = "open"
		if m.db != nil {
			_ = m.db.UpdateMulticastTunnel(cfg)
		}
		// Serve with or without TLS (always TLS expected but guard nil)
		if m.tlsConfig != nil && cfg.UseTLS {
			if err := srv.ServeTLS(ln, "", ""); err != nil && err != http.ErrServerClosed {
				cfg.Status = "error"
				if m.db != nil {
					_ = m.db.UpdateMulticastTunnel(cfg)
				}
			}
		} else {
			if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
				cfg.Status = "error"
				if m.db != nil {
					_ = m.db.UpdateMulticastTunnel(cfg)
				}
			}
		}
	}()

	m.items[cfg.ID] = active
	return nil
}

func (m *MulticastManager) StopMulticast(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	active, ok := m.items[id]
	if !ok {
		return fmt.Errorf("multicast %s not found", id)
	}
	active.Cancel()
	_ = active.Server.Close()
	// Ensure DB reflects disabled + closed to avoid re-enabling via stale in-memory copy
	active.Config.Enabled = false
	active.Config.Status = "closed"
	if m.db != nil {
		_ = m.db.UpdateMulticastTunnel(active.Config)
	}
	delete(m.items, id)
	return nil
}

// createWebhookHandler implements uni-directional mode: immediately 200 OK and broadcast request via capture
func (m *MulticastManager) createWebhookHandler(cfg *database.MulticastTunnel) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connID := fmt.Sprintf("mc-%d", time.Now().UnixNano())
		// Build tap so capture gets nice HTTP events
		var tap tunnel.Tap
		if m.capture != nil {
			meta := tunnel.Meta{
				Username: cfg.Owner,
				Remote:   settings.Remote{LocalHost: "127.0.0.1", LocalPort: strconv.Itoa(cfg.Port), RemoteHost: "127.0.0.1", RemotePort: strconv.Itoa(cfg.Port)},
				ConnID:   connID,
			}
			tap = capture.NewTapFactory(m.capture, cfg.ID, cfg.Owner, 500)(meta)
		}
		if tap != nil {
			tap.OnOpen()
		}

		// Read body
		var reqBody []byte
		if r.Body != nil {
			reqBody, _ = io.ReadAll(io.LimitReader(r.Body, 10*1024*1024))
		}

		// Emit request headers + body into capture via tap SrcWriter
		if tap != nil {
			// Emit request line/headers
			hdr := map[string]any{"method": r.Method, "path": r.URL.String(), "header": r.Header}
			if b, _ := json.Marshal(hdr); len(b) > 0 {
				_, _ = tap.SrcWriter().Write(b)
			}
			// Emit body
			if len(reqBody) > 0 {
				_, _ = tap.SrcWriter().Write(reqBody)
			}
		}

		// Uni-directional webhook: ack 200 immediately
		w.Header().Set("Content-Type", "application/json")
		ack := map[string]any{"status": "ok", "received_at": time.Now().Format(time.RFC3339)}
		ackBytes, _ := json.Marshal(ack)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(ackBytes)

		if tap != nil {
			// Emit response headers/body via DstWriter
			resHdr := map[string]any{"status": "200 OK", "code": 200, "header": w.Header()}
			if b, _ := json.Marshal(resHdr); len(b) > 0 {
				_, _ = tap.DstWriter().Write(b)
			}
			_, _ = tap.DstWriter().Write(ackBytes)
			// close with rough metrics
			sent := int64(len(ackBytes))
			recv := int64(len(reqBody))
			tap.OnClose(sent, recv)
		}

		// Fan-out to subscribers (fire-and-forget)
		if a := m.getActiveByPort(cfg.Port); a != nil {
			// Reconstruct HTTP request bytes
			reqCopy := &http.Request{
				Method:        r.Method,
				URL:           r.URL,
				Proto:         "HTTP/1.1",
				ProtoMajor:    1,
				ProtoMinor:    1,
				Header:        r.Header.Clone(),
				Host:          r.Host,
				Body:          io.NopCloser(bytes.NewReader(reqBody)),
				ContentLength: int64(len(reqBody)),
			}
			var buf bytes.Buffer
			_ = reqCopy.Write(&buf)
			payload := buf.Bytes()

			a.subsMu.RLock()
			for _, sub := range a.subs {
				su := sub
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					_ = su.Tun.DeliverToRemote(ctx, &su.Remote, payload)
				}()
			}
			a.subsMu.RUnlock()
		}

		// Update DB counters
		if m.db != nil {
			_ = m.db.AddMulticastConnections(cfg.ID, +1)
			defer m.db.AddMulticastConnections(cfg.ID, -1)
		}
	})
}

// createBidirectionalHandler forwards the HTTP request to all subscribers and returns the first response.
func (m *MulticastManager) createBidirectionalHandler(cfg *database.MulticastTunnel) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connID := fmt.Sprintf("mc-%d", time.Now().UnixNano())
		// Build tap so capture gets nice HTTP events
		var tap tunnel.Tap
		if m.capture != nil {
			meta := tunnel.Meta{
				Username: cfg.Owner,
				Remote:   settings.Remote{LocalHost: "127.0.0.1", LocalPort: strconv.Itoa(cfg.Port), RemoteHost: "127.0.0.1", RemotePort: strconv.Itoa(cfg.Port)},
				ConnID:   connID,
			}
			tap = capture.NewTapFactory(m.capture, cfg.ID, cfg.Owner, 500)(meta)
		}
		if tap != nil {
			tap.OnOpen()
		}

		// Read body
		var reqBody []byte
		if r.Body != nil {
			reqBody, _ = io.ReadAll(io.LimitReader(r.Body, 10*1024*1024))
		}

		// Emit request headers + body into capture via tap SrcWriter
		if tap != nil {
			hdr := map[string]any{"method": r.Method, "path": r.URL.String(), "header": r.Header}
			if b, _ := json.Marshal(hdr); len(b) > 0 {
				_, _ = tap.SrcWriter().Write(b)
			}
			if len(reqBody) > 0 {
				_, _ = tap.SrcWriter().Write(reqBody)
			}
		}

		// Serialize request to HTTP/1.1 bytes for delivery
		reqCopy := &http.Request{Method: r.Method, URL: r.URL, Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1, Header: r.Header.Clone(), Host: r.Host, Body: io.NopCloser(bytes.NewReader(reqBody)), ContentLength: int64(len(reqBody))}
		var buf bytes.Buffer
		// Encourage upstream to close promptly so we can finish reading one response
		reqCopy.Close = true
		reqCopy.Header.Set("Connection", "close")

		_ = reqCopy.Write(&buf)
		payload := buf.Bytes()

		// Check subscribers
		a := m.getActiveByPort(cfg.Port)
		if a == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			msg := []byte(`{"error":"no subscribers"}`)
			_, _ = w.Write(msg)
			if tap != nil {
				resHdr := map[string]any{"status": "503 Service Unavailable", "code": 503, "header": w.Header()}
				if b, _ := json.Marshal(resHdr); len(b) > 0 {
					_, _ = tap.DstWriter().Write(b)
				}
				_, _ = tap.DstWriter().Write(msg)
				tap.OnClose(int64(len(msg)), int64(len(reqBody)))
			}
			return
		}

		// Fan-out request and wait for first response
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		respCh := make(chan []byte, 1)
		a.subsMu.RLock()
		for _, sub := range a.subs {
			su := sub
			go func() {
				b, err := su.Tun.DeliverToRemoteWithResponse(ctx, &su.Remote, payload)
				if err == nil && len(b) > 0 {
					select {
					case respCh <- b:
					default:
					}
				}
			}()
		}
		a.subsMu.RUnlock()

		select {
		case <-ctx.Done():
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusGatewayTimeout)
			msg := []byte(`{"error":"timeout waiting for subscriber response"}`)
			_, _ = w.Write(msg)
			if tap != nil {
				resHdr := map[string]any{"status": "504 Gateway Timeout", "code": 504, "header": w.Header()}
				if b, _ := json.Marshal(resHdr); len(b) > 0 {
					_, _ = tap.DstWriter().Write(b)
				}
				_, _ = tap.DstWriter().Write(msg)
				tap.OnClose(int64(len(msg)), int64(len(reqBody)))
			}
			if m.db != nil {
				_ = m.db.AddMulticastConnections(cfg.ID, +1)
				defer m.db.AddMulticastConnections(cfg.ID, -1)
			}
			return
		case respBytes := <-respCh:
			// Parse upstream HTTP response
			br := bufio.NewReader(bytes.NewReader(respBytes))
			upResp, err := http.ReadResponse(br, r)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadGateway)
				msg := []byte(`{"error":"invalid upstream response"}`)
				_, _ = w.Write(msg)
				if tap != nil {
					resHdr := map[string]any{"status": "502 Bad Gateway", "code": 502, "header": w.Header()}
					if b, _ := json.Marshal(resHdr); len(b) > 0 {
						_, _ = tap.DstWriter().Write(b)
					}
					_, _ = tap.DstWriter().Write(msg)
					tap.OnClose(int64(len(msg)), int64(len(reqBody)))
				}
				if m.db != nil {
					_ = m.db.AddMulticastConnections(cfg.ID, +1)
					defer m.db.AddMulticastConnections(cfg.ID, -1)
					_ = m.db.AddMulticastBytes(cfg.ID, int64(len(respBytes)), int64(len(reqBody)))
				}
				return
			}
			defer upResp.Body.Close()
			// Copy headers (skip hop-by-hop)
			for k, vs := range upResp.Header {
				if strings.EqualFold(k, "Connection") || strings.EqualFold(k, "Transfer-Encoding") {
					continue
				}
				for _, v := range vs {
					w.Header().Add(k, v)
				}
			}
			w.WriteHeader(upResp.StatusCode)
			bodyBytes, _ := io.ReadAll(upResp.Body)
			_, _ = w.Write(bodyBytes)
			if tap != nil {
				resHdr := map[string]any{"status": upResp.Status, "code": upResp.StatusCode, "header": upResp.Header}
				if b, _ := json.Marshal(resHdr); len(b) > 0 {
					_, _ = tap.DstWriter().Write(b)
				}
				if len(bodyBytes) > 0 {
					_, _ = tap.DstWriter().Write(bodyBytes)
				}
				tap.OnClose(int64(len(bodyBytes)), int64(len(reqBody)))
			}
			if m.db != nil {
				_ = m.db.AddMulticastConnections(cfg.ID, +1)
				defer m.db.AddMulticastConnections(cfg.ID, -1)
				_ = m.db.AddMulticastBytes(cfg.ID, int64(len(bodyBytes)), int64(len(reqBody)))
			}
		}
	})
}
