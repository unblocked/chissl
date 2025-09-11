package chserver

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/NextChapterSoftware/chissl/server/capture"
	chshare "github.com/NextChapterSoftware/chissl/share"
	"github.com/NextChapterSoftware/chissl/share/auth"
	"github.com/NextChapterSoftware/chissl/share/ccrypto"
	"github.com/NextChapterSoftware/chissl/share/cio"
	"github.com/NextChapterSoftware/chissl/share/cnet"
	"github.com/NextChapterSoftware/chissl/share/database"
	"github.com/NextChapterSoftware/chissl/share/settings"
	"github.com/NextChapterSoftware/chissl/share/tunnel"
	"github.com/gorilla/websocket"
	"github.com/jpillora/requestlog"
	"golang.org/x/crypto/ssh"
	"net"
	"strings"
)

// Config is the configuration for the chisel service
type Config struct {
	KeySeed   string
	KeyFile   string
	AuthFile  string
	Auth      string
	Proxy     string
	Reverse   bool
	KeepAlive time.Duration
	TLS       TLSConfig
	TlsConf   *tls.Config
	Database  *database.DatabaseConfig
	Auth0     *auth.Auth0Config
	Dashboard DashboardConfig
	LogDir    string
	// Security-related server settings
	Security SecurityConfig
}

// DashboardConfig holds dashboard configuration
type DashboardConfig struct {
	Enabled bool
	Path    string
}

// LoginBackoffConfig controls login rate limiting/backoff behavior
// Durations are expressed as milliseconds/minutes for easy JSON handling.
type LoginBackoffConfig struct {
	BaseDelayMS      int  `json:"base_delay_ms"`
	MaxDelayMS       int  `json:"max_delay_ms"`
	MaxExponent      int  `json:"max_exponent"`
	HardLockFailures int  `json:"hard_lock_failures"`
	HardLockMinutes  int  `json:"hard_lock_minutes"`
	PerIPEnabled     bool `json:"per_ip_enabled"`
}

// IPRateLimitConfig controls IP-level request caps independent of username
type IPRateLimitConfig struct {
	MaxPerMinute int `json:"max_per_minute"`
	BanMinutes   int `json:"ban_minutes"`
}

// SecurityConfig groups security-related settings
type SecurityConfig struct {
	LoginBackoff LoginBackoffConfig `json:"login_backoff"`
	IPRate       IPRateLimitConfig  `json:"ip_rate"`
}

// Server respresent a chisel service
type Server struct {
	*cio.Logger
	config       *Config
	fingerprint  string
	httpServer   *cnet.HTTPServer
	reverseProxy *httputil.ReverseProxy
	sessCount    int32
	sessions     *settings.Users
	sshConfig    *ssh.ServerConfig
	users        *settings.UserIndex
	db           database.Database
	auth0        *auth.Auth0Middleware
	startTime    time.Time
	// capture service
	capture *capture.Service
	// listener manager
	listeners *ListenerManager
	// multicast manager
	multicasts *MulticastManager
	// log manager
	logManager *LogManager
	// in-memory live tunnels when DB is not used
	liveMu      sync.RWMutex
	liveTunnels map[string]*database.Tunnel
	// server-side blocklist of deleted tunnel IDs (visibility override)
	deletedTunnelIDs map[string]struct{}
	// login backoff state (per-username and optional per-IP)
	backoffMu     sync.Mutex
	loginBackoffs map[string]*loginBackoff
	ipBackoffs    map[string]map[string]*loginBackoff // username -> ip -> state
	// IP-only rate limiting state
	ipRateMu sync.Mutex
	ipRates  map[string]*ipRateState
	// security events (in-memory ring buffer)
	securityMu     sync.Mutex
	securityEvents []SecurityEvent
}

// loginBackoff tracks failed login attempts for backoff/lockout
type loginBackoff struct {
	failures    int
	lastAttempt time.Time
	lockedUntil time.Time
}

// ipRateState tracks per-IP rate limiting windows
type ipRateState struct {
	windowStart time.Time
	count       int
	bannedUntil time.Time
}

// SecurityEvent captures notable security actions
type SecurityEvent struct {
	Type     string    `json:"type"`
	Severity string    `json:"severity"`
	Username string    `json:"username"`
	IP       string    `json:"ip"`
	At       time.Time `json:"at"`
	Message  string    `json:"message"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin:     func(r *http.Request) bool { return true },
	ReadBufferSize:  settings.EnvInt("WS_BUFF_SIZE", 0),
	WriteBufferSize: settings.EnvInt("WS_BUFF_SIZE", 0),
}

// NewServer creates and returns a new chisel server
func NewServer(c *Config) (*Server, error) {
	server := &Server{
		config:        c,
		httpServer:    cnet.NewHTTPServer(),
		Logger:        cio.NewLogger("server"),
		sessions:      settings.NewUsers(),
		startTime:     time.Now(),
		loginBackoffs: make(map[string]*loginBackoff),
	}
	// initialize per-IP backoff state map
	server.ipBackoffs = make(map[string]map[string]*loginBackoff)
	// initialize IP rate limiter map
	server.ipRates = make(map[string]*ipRateState)
	// set default security config if not provided
	lb := &server.config.Security.LoginBackoff
	if lb.BaseDelayMS == 0 {
		lb.BaseDelayMS = 250
	}
	if lb.MaxDelayMS == 0 {
		lb.MaxDelayMS = 5000
	}
	if lb.MaxExponent == 0 {
		lb.MaxExponent = 4
	}
	if lb.HardLockFailures == 0 {
		lb.HardLockFailures = 8
	}
	if lb.HardLockMinutes == 0 {
		lb.HardLockMinutes = 10
	}
	// defaults for IP-only rate limiting
	ip := &server.config.Security.IPRate
	if ip.MaxPerMinute == 0 {
		ip.MaxPerMinute = 120
	}
	if ip.BanMinutes == 0 {
		ip.BanMinutes = 10
	}

	// Load persisted security settings if present
	if server.db != nil {
		if v, err := server.db.GetSettingInt("security_backoff_base_ms", 0); err == nil && v > 0 {
			server.config.Security.LoginBackoff.BaseDelayMS = v
		}
		if v, err := server.db.GetSettingInt("security_backoff_max_ms", 0); err == nil && v > 0 {
			server.config.Security.LoginBackoff.MaxDelayMS = v
		}
		if v, err := server.db.GetSettingInt("security_backoff_max_exp", 0); err == nil && v > 0 {
			server.config.Security.LoginBackoff.MaxExponent = v
		}
		if v, err := server.db.GetSettingInt("security_backoff_hard_fail", 0); err == nil && v > 0 {
			server.config.Security.LoginBackoff.HardLockFailures = v
		}
		if v, err := server.db.GetSettingInt("security_backoff_hard_min", 0); err == nil && v > 0 {
			server.config.Security.LoginBackoff.HardLockMinutes = v
		}
		if v, err := server.db.GetSettingBool("security_backoff_per_ip", server.config.Security.LoginBackoff.PerIPEnabled); err == nil {
			server.config.Security.LoginBackoff.PerIPEnabled = v
		}
		if v, err := server.db.GetSettingInt("security_ip_max_per_min", 0); err == nil && v > 0 {
			server.config.Security.IPRate.MaxPerMinute = v
		}
		if v, err := server.db.GetSettingInt("security_ip_ban_minutes", 0); err == nil && v > 0 {
			server.config.Security.IPRate.BanMinutes = v
		}
	}

	server.deletedTunnelIDs = make(map[string]struct{})
	server.Info = true
	server.users = settings.NewUserIndex(server.Logger)
	server.liveTunnels = make(map[string]*database.Tunnel)
	// Initialize capture for dashboard if enabled
	if c.Dashboard.Enabled {
		// defaults: last 500 events, 64KB per event
		server.capture = capture.NewService(500, 64*1024)
		server.Infof("Dashboard capture enabled")
	}

	// Initialize log manager
	logDir := "./logs"
	if c.LogDir != "" {
		logDir = c.LogDir
	}
	if logManager, err := NewLogManager(logDir); err != nil {
		server.Infof("Failed to initialize log manager: %v", err)
	} else {
		server.logManager = logManager
		server.Infof("Log manager initialized, logs will be saved to: %s", logDir)

		// Log the initialization
		logManager.WriteLog("info", "Log manager initialized successfully", "server")
	}

	// Initialize database if configured
	if c.Database != nil {
		server.db = database.NewDatabase(c.Database)
		if err := server.db.Connect(); err != nil {
			return nil, err
		}
		if err := server.db.Migrate(); err != nil {
			return nil, err
		}
		server.Infof("Database initialized: %s", c.Database.Type)
	}
	// On startup, mark any lingering 'open' tunnels as 'closed' since they can't survive restart
	if server.db != nil {
		if err := server.db.MarkStaleTunnelsClosed(0); err != nil {
			server.Debugf("Failed to reset stale tunnels on startup: %v", err)
		}
		// Also mark stale listeners as closed
		if err := server.db.MarkStaleListenersClosed(0); err != nil {
			server.Debugf("Failed to reset stale listeners on startup: %v", err)
		}
	}
	// Initialize capture service early (default persist on)
	if c.Dashboard.Enabled {
		if server.capture == nil {
			server.capture = capture.NewService(500, 64*1024)
		}
		// Wire DB updates from capture metrics/conn deltas if DB is used
		if server.db != nil {
			server.capture.SetOnMetric(func(tunnelID string, sent, received int64) {
				if err := server.db.AddTunnelBytes(tunnelID, sent, received); err != nil {
					server.Debugf("AddTunnelBytes failed: %v", err)
				}
			})
			server.capture.SetOnConnDelta(func(tunnelID string, delta int) {
				if err := server.db.AddTunnelConnections(tunnelID, delta); err != nil {
					server.Debugf("AddTunnelConnections failed: %v", err)
				}
			})
		}
		// Initialize listener manager (TLS config will be set later in StartContext)
		server.listeners = NewListenerManager(server.capture, server.db, nil)
		// Initialize multicast manager (TLS config will be set later in StartContext)
		server.multicasts = NewMulticastManager(server.capture, server.db, nil)
	}
	// Reconciler to mark stale active tunnels as closed if not updated recently
	go func() {
		t := time.NewTicker(60 * time.Second)
		for range t.C {
			if server.db == nil {
				continue
			}
			// Mark tunnels inactive if UpdatedAt older than 2 minutes
			_ = server.db.MarkStaleTunnelsClosed(2 * time.Minute)
		}
	}()

	// Initialize Auth0 if configured
	if c.Auth0 != nil && c.Auth0.Enabled {
		auth0Middleware, err := auth.NewAuth0Middleware(c.Auth0)
		if err != nil {
			return nil, err
		}
		server.auth0 = auth0Middleware
		server.Infof("Auth0 integration enabled")
	}
	if c.AuthFile != "" {
		if err := server.users.LoadUsers(c.AuthFile); err != nil {

			return nil, err
		}
	}
	if c.Auth != "" {
		u := &settings.User{Addrs: []*regexp.Regexp{settings.UserAllowAll}}
		u.IsAdmin = true
		u.Name, u.Pass = settings.ParseAuth(c.Auth)
		if u.Name != "" {
			server.users.AddUser(u)
		}
	}

	var pemBytes []byte
	var err error
	if c.KeyFile != "" {
		var key []byte

		if ccrypto.IsChiselKey([]byte(c.KeyFile)) {
			key = []byte(c.KeyFile)
		} else {
			key, err = os.ReadFile(c.KeyFile)
			if err != nil {
				log.Fatalf("Failed to read key file %s", c.KeyFile)
			}
		}

		pemBytes = key
		if ccrypto.IsChiselKey(key) {
			pemBytes, err = ccrypto.ChiselKey2PEM(key)
			if err != nil {
				log.Fatalf("Invalid key %s", string(key))
			}
		}
	} else {
		//generate private key (optionally using seed)
		pemBytes, err = ccrypto.Seed2PEM(c.KeySeed)
		if err != nil {
			log.Fatal("Failed to generate key")
		}
	}

	//convert into ssh.PrivateKey
	private, err := ssh.ParsePrivateKey(pemBytes)
	if err != nil {
		log.Fatal("Failed to parse key")
	}
	//fingerprint this key
	server.fingerprint = ccrypto.FingerprintKey(private.PublicKey())
	//create ssh config
	server.sshConfig = &ssh.ServerConfig{
		ServerVersion:    "SSH-" + chshare.ProtocolVersion + "-server",
		PasswordCallback: server.authUser,
	}
	server.sshConfig.AddHostKey(private)

	//print when reverse tunnelling is enabled
	if c.Reverse {
		server.Infof("Reverse tunnelling enabled")
	}
	return server, nil
}

// Override logging methods to also write to log manager
func (s *Server) Infof(f string, args ...interface{}) {
	s.Logger.Infof(f, args...)
	if s.logManager != nil {
		s.logManager.WriteLog("info", fmt.Sprintf(f, args...), "server")
	}
}

func (s *Server) Debugf(f string, args ...interface{}) {
	s.Logger.Debugf(f, args...)
	if s.logManager != nil {
		s.logManager.WriteLog("debug", fmt.Sprintf(f, args...), "server")
	}
}

func (s *Server) Errorf(f string, args ...interface{}) error {
	err := s.Logger.Errorf(f, args...)
	if s.logManager != nil {
		s.logManager.WriteLog("error", fmt.Sprintf(f, args...), "server")
	}
	return err
}

// Run is responsible for starting the chisel service.
// Internally this calls Start then Wait.
func (s *Server) Run(host, port string) error {
	if err := s.Start(host, port); err != nil {
		return err
	}
	return s.Wait()
}

// Start is responsible for kicking off the http server
func (s *Server) Start(host, port string) error {
	return s.StartContext(context.Background(), host, port)
}

// StartContext is responsible for kicking off the http server,
// and can be closed by cancelling the provided context
func (s *Server) StartContext(ctx context.Context, host, port string) error {
	s.Infof("Fingerprint %s", s.fingerprint)
	if s.users.Len() > 0 {
		s.Infof("User authentication enabled")
	}
	if s.reverseProxy != nil {
		s.Infof("Reverse proxy enabled")
	}
	l, err := s.listener(host, port)
	if err != nil {
		return err
	}

	// Update listener manager with the actual TLS config now that it's available
	if s.listeners != nil && s.config.TlsConf != nil {
		s.listeners.UpdateTLSConfig(s.config.TlsConf)
		s.Debugf("Updated listener manager with TLS config")
		// Now restore active listeners from database with proper TLS config
		s.restoreListeners()
	}
	// Update multicast manager with TLS and restore enabled multicasts
	if s.multicasts != nil && s.config.TlsConf != nil {
		s.multicasts.UpdateTLSConfig(s.config.TlsConf)
		s.Debugf("Updated multicast manager with TLS config")
		s.restoreMulticasts()

	}
	h := http.Handler(http.HandlerFunc(s.handleClientHandler))
	if s.Debug {
		o := requestlog.DefaultOptions
		o.TrustProxy = true
		h = requestlog.WrapWith(h, o)
	}
	return s.httpServer.GoServe(ctx, l, h)
}

// Wait waits for the http server to close
func (s *Server) Wait() error {
	return s.httpServer.Wait()
}

// Close forcibly closes the http server
func (s *Server) Close() error {
	return s.httpServer.Close()
}

// GetFingerprint is used to access the server fingerprint
func (s *Server) GetFingerprint() string {
	return s.fingerprint
}

// authUser validates SSH username/password against (in order):
// 1) --auth admin user, 2) database users, 3) authfile/in-memory users
func (s *Server) authUser(c ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
	p := string(password)
	n := c.User()

	// 1) Check --auth admin (always honored even if authfile changed later)
	if s.config != nil && s.config.Auth != "" {
		adminUser, adminPass := settings.ParseAuth(s.config.Auth)
		if n == adminUser && p == adminPass {
			u := &settings.User{Name: adminUser, Pass: adminPass, Addrs: []*regexp.Regexp{settings.UserAllowAll}, IsAdmin: true}
			s.sessions.Set(string(c.SessionID()), u)
			return nil, nil
		}
	}

	// 2) Check database (password or token)
	if s.db != nil {
		if dbUser, err := s.db.GetUser(n); err == nil {
			if dbUser != nil && dbUser.Password == p {
				u := &settings.User{Name: dbUser.Username, Pass: dbUser.Password, IsAdmin: dbUser.IsAdmin, Addrs: []*regexp.Regexp{settings.UserAllowAll}}
				s.sessions.Set(string(c.SessionID()), u)
				return nil, nil
			}
		}

		// Try token authentication
		if userToken, err := s.db.ValidateUserToken(p); err == nil {
			if dbUser, err := s.db.GetUser(userToken.Username); err == nil {
				u := &settings.User{Name: dbUser.Username, Pass: "", IsAdmin: dbUser.IsAdmin, Addrs: []*regexp.Regexp{settings.UserAllowAll}}
				s.sessions.Set(string(c.SessionID()), u)
				return nil, nil
			}
		}
	}

	// 3) Fallback to in-memory users (authfile and any others loaded)
	if user, found := s.users.Get(n); found && user.Pass == p {
		// insert the user session map
		s.sessions.Set(string(c.SessionID()), user)
		return nil, nil
	}

	s.Debugf("Login failed for user: %s", n)
	return nil, errors.New("Invalid authentication for username: %s")
}

// AddUser adds a new user into the server user index

// clientIP returns the best-effort client IP, respecting X-Forwarded-For
func (s *Server) clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// nextLoginDelayFor computes lock/delay for a given username and optional client IP
func (s *Server) nextLoginDelayFor(username, ip string) (locked bool, retryAfter time.Duration, delay time.Duration) {
	now := time.Now()
	lb := s.config.Security.LoginBackoff
	s.backoffMu.Lock()
	var st *loginBackoff
	if lb.PerIPEnabled && ip != "" {
		m := s.ipBackoffs[username]
		if m == nil {
			m = make(map[string]*loginBackoff)
			s.ipBackoffs[username] = m
		}
		st = m[ip]
		if st == nil {
			st = &loginBackoff{}
			m[ip] = st
		}
	} else {
		st = s.loginBackoffs[username]
		if st == nil {
			st = &loginBackoff{}
			s.loginBackoffs[username] = st
		}
	}
	// Hard lockout
	if st.lockedUntil.After(now) {
		ra := time.Until(st.lockedUntil)
		s.backoffMu.Unlock()
		return true, ra, 0
	}
	// Soft delay: exponential backoff on consecutive failures
	if st.failures <= 0 {
		s.backoffMu.Unlock()
		return false, 0, 0
	}
	base := time.Duration(lb.BaseDelayMS) * time.Millisecond
	if base <= 0 {
		base = 250 * time.Millisecond
	}
	exp := st.failures - 1
	if exp < 0 {
		exp = 0
	}
	if lb.MaxExponent > 0 && exp > lb.MaxExponent {
		exp = lb.MaxExponent
	}
	d := base << exp
	maxD := time.Duration(lb.MaxDelayMS) * time.Millisecond
	if maxD <= 0 {
		maxD = 5 * time.Second
	}
	if d > maxD {
		d = maxD
	}
	s.backoffMu.Unlock()
	return false, 0, d
}

// recordLoginFailureFor updates state and may set a hard lockout window
func (s *Server) recordLoginFailureFor(username, ip string) {
	now := time.Now()
	lb := s.config.Security.LoginBackoff
	s.backoffMu.Lock()
	var st *loginBackoff
	if lb.PerIPEnabled && ip != "" {
		m := s.ipBackoffs[username]
		if m == nil {
			m = make(map[string]*loginBackoff)
			s.ipBackoffs[username] = m
		}
		st = m[ip]
		if st == nil {
			st = &loginBackoff{}
			m[ip] = st
		}
	} else {
		st = s.loginBackoffs[username]
		if st == nil {
			st = &loginBackoff{}
			s.loginBackoffs[username] = st
		}
	}
	st.failures++
	st.lastAttempt = now
	if lb.HardLockFailures <= 0 {
		lb.HardLockFailures = 8
	}
	if st.failures >= lb.HardLockFailures {
		dur := time.Duration(lb.HardLockMinutes) * time.Minute
		if dur <= 0 {
			dur = 10 * time.Minute
		}
		st.lockedUntil = now.Add(dur)
		// audit log and in-memory event
		if s.logManager != nil {
			s.logManager.WriteLog("warn", fmt.Sprintf("Login lockout applied: user=%s ip=%s for %v", username, ip, dur), "security")
		}
		s.recordSecurityEvent("lockout", "warn", username, ip, "Temporarily locked due to failed logins")
	}
	s.backoffMu.Unlock()
}

// resetLoginBackoffFor clears the backoff state for a username/ip bucket
func (s *Server) resetLoginBackoffFor(username, ip string) {
	lb := s.config.Security.LoginBackoff
	s.backoffMu.Lock()
	defer s.backoffMu.Unlock()
	if lb.PerIPEnabled && ip != "" {
		if m := s.ipBackoffs[username]; m != nil {
			delete(m, ip)
			if len(m) == 0 {
				delete(s.ipBackoffs, username)
			}
		}
		return
	}
	delete(s.loginBackoffs, username)
}

// recordSecurityEvent appends a security event to the in-memory buffer and dispatches webhooks
func (s *Server) recordSecurityEvent(evType, severity, username, ip, msg string) {
	if severity == "" {
		severity = "info"
	}
	ev := SecurityEvent{Type: evType, Severity: severity, Username: username, IP: ip, At: time.Now(), Message: msg}
	// persist to DB if available (best-effort)
	if s.db != nil {
		_ = s.db.InsertSecurityEvent(&database.SecurityEventLog{
			Type: ev.Type, Severity: ev.Severity, Username: ev.Username, IP: ev.IP, Message: ev.Message, At: ev.At,
		})
	}
	// append to in-memory buffer
	s.securityMu.Lock()
	s.securityEvents = append(s.securityEvents, ev)
	if len(s.securityEvents) > 100 {
		s.securityEvents = s.securityEvents[len(s.securityEvents)-100:]
	}
	s.securityMu.Unlock()
	// fire-and-forget webhook dispatch
	go s.dispatchSecurityEvent(ev)
}

// dispatchSecurityEvent posts the event to all enabled webhooks (non-blocking per event)
func (s *Server) dispatchSecurityEvent(ev SecurityEvent) {
	if s.db == nil {
		return
	}
	webhooks, err := s.db.ListSecurityWebhooks(true)
	if err != nil || len(webhooks) == 0 {
		return
	}
	for _, wh := range webhooks {
		wh := wh
		go s.sendSecurityEventToWebhook(wh.URL, wh.Type, ev)
	}
}

// sendSecurityEventToWebhook sends a single event to a target webhook URL with type slack|json
func (s *Server) sendSecurityEventToWebhook(url string, wtype string, ev SecurityEvent) {
	client := &http.Client{Timeout: 3 * time.Second}
	var payload []byte
	contentType := "application/json"
	if wtype == "slack" {
		// Slack attachments with color by severity
		color := getSlackColorBySeverity(ev.Severity)
		att := map[string]any{
			"color": color,
			"title": fmt.Sprintf("[%s] %s", strings.ToUpper(ev.Type), ev.Message),
			"fields": []map[string]string{
				{"title": "User", "value": ev.Username, "short": "true"},
				{"title": "IP", "value": ev.IP, "short": "true"},
			},
			"ts": ev.At.Unix(),
		}
		obj := map[string]any{"attachments": []any{att}}
		payload, _ = json.Marshal(obj)
	} else {
		payload, _ = json.Marshal(ev)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", contentType)
	_, _ = client.Do(req)
}

func getSlackColorBySeverity(sev string) string {
	s := strings.ToLower(sev)
	switch s {
	case "warn", "warning":
		return "#FFA500"
	case "error", "err", "critical", "crit":
		return "#FF0000"
	default:
		return "#36a64f" // info/ok
	}
}

// ipRateCheck returns whether an IP is currently rate-limited and for how long
func (s *Server) ipRateCheck(ip string) (bool, time.Duration) {
	now := time.Now()
	s.ipRateMu.Lock()
	st := s.ipRates[ip]
	if st == nil {
		st = &ipRateState{}
		s.ipRates[ip] = st
	}
	if st.bannedUntil.After(now) {
		d := time.Until(st.bannedUntil)
		s.ipRateMu.Unlock()
		return true, d
	}
	// roll window if needed
	if st.windowStart.IsZero() || now.Sub(st.windowStart) >= time.Minute {
		st.windowStart = now
		st.count = 0
	}
	// not locked
	s.ipRateMu.Unlock()
	return false, 0
}

// ipRateRecord increments the attempt counter for an IP in the current window
func (s *Server) ipRateRecord(ip string) {
	now := time.Now()
	cfg := s.config.Security.IPRate
	s.ipRateMu.Lock()
	st := s.ipRates[ip]
	if st == nil {
		st = &ipRateState{}
		s.ipRates[ip] = st
	}
	if st.windowStart.IsZero() || now.Sub(st.windowStart) >= time.Minute {
		st.windowStart = now
		st.count = 0
	}
	st.count++
	if cfg.MaxPerMinute > 0 && st.count > cfg.MaxPerMinute {
		ban := time.Duration(cfg.BanMinutes) * time.Minute
		if ban <= 0 {
			ban = 10 * time.Minute
		}
		st.bannedUntil = now.Add(ban)
		// audit
		if s.logManager != nil {
			s.logManager.WriteLog("warn", fmt.Sprintf("IP rate limit triggered: ip=%s window=%s count=%d ban=%v", ip, st.windowStart.Format(time.RFC3339), st.count, ban), "security")
		}
		s.recordSecurityEvent("ip_ban", "warn", "", ip, "Temporary ban due to excessive requests")
	}
	s.ipRateMu.Unlock()
}

// Backward-compatible wrappers
func (s *Server) nextLoginDelay(username string) (bool, time.Duration, time.Duration) {
	return s.nextLoginDelayFor(username, "")
}
func (s *Server) recordLoginFailure(username string) { s.recordLoginFailureFor(username, "") }
func (s *Server) resetLoginBackoff(username string)  { s.resetLoginBackoffFor(username, "") }

func (s *Server) AddUser(user, pass string, addrs ...string) error {
	authorizedAddrs := []*regexp.Regexp{}
	for _, addr := range addrs {
		authorizedAddr, err := regexp.Compile(addr)
		if err != nil {
			return err
		}
		authorizedAddrs = append(authorizedAddrs, authorizedAddr)
	}
	s.users.AddUser(&settings.User{
		Name:  user,
		Pass:  pass,
		Addrs: authorizedAddrs,
	})
	return nil
}

// DeleteUser removes a user from the server user index
func (s *Server) DeleteUser(user string) {
	s.users.Del(user)
}

// ResetUsers in the server user index.
// Use nil to remove all.
func (s *Server) ResetUsers(users []*settings.User) {
	s.users.Reset(users)
}

// restoreListeners restarts listeners that were active before server restart
func (s *Server) restoreListeners() {
	if s.db == nil || s.listeners == nil {
		return
	}

	// Get all listeners that should be restored (were previously open)
	listeners, err := s.db.ListListeners()
	if err != nil {
		s.Debugf("Failed to list listeners for restoration: %v", err)
		return
	}

	for _, listener := range listeners {
		// Only restore listeners that were previously open
		if listener.Status == "closed" {
			// Create tap factory for this listener
			var tapFactory tunnel.TapFactory
			if s.capture != nil {
				tapFactory = capture.NewTapFactory(s.capture, listener.ID, listener.Username, 500)
			}

			// Start the listener
			if err := s.listeners.StartListener(listener, tapFactory); err != nil {
				s.Debugf("Failed to restore listener %s on port %d: %v", listener.ID, listener.Port, err)

				// Update status to error
				listener.Status = "error"
				_ = s.db.UpdateListener(listener)
			} else {
				s.Debugf("Restored listener %s on port %d (%s mode)", listener.ID, listener.Port, listener.Mode)
			}
		}
	}
}

// restoreMulticasts restarts multicast tunnels that are enabled
func (s *Server) restoreMulticasts() {
	if s.db == nil || s.multicasts == nil {
		return
	}
	mts, err := s.db.ListMulticastTunnels()
	if err != nil {
		s.Debugf("Failed to list multicast tunnels for restoration: %v", err)
		return
	}
	for _, mt := range mts {
		if mt.Enabled {
			if err := s.multicasts.StartMulticast(mt); err != nil {
				s.Debugf("Failed to start multicast %s on port %d: %v", mt.ID, mt.Port, err)
				mt.Status = "error"
				_ = s.db.UpdateMulticastTunnel(mt)
			} else {
				s.Debugf("Started multicast %s on port %d (mode=%s)", mt.ID, mt.Port, mt.Mode)
			}
		}
	}
}
