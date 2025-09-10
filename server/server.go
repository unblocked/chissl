package chserver

import (
	"context"
	"crypto/tls"
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
}

// DashboardConfig holds dashboard configuration
type DashboardConfig struct {
	Enabled bool
	Path    string
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
}

var upgrader = websocket.Upgrader{
	CheckOrigin:     func(r *http.Request) bool { return true },
	ReadBufferSize:  settings.EnvInt("WS_BUFF_SIZE", 0),
	WriteBufferSize: settings.EnvInt("WS_BUFF_SIZE", 0),
}

// NewServer creates and returns a new chisel server
func NewServer(c *Config) (*Server, error) {
	server := &Server{
		config:     c,
		httpServer: cnet.NewHTTPServer(),
		Logger:     cio.NewLogger("server"),
		sessions:   settings.NewUsers(),
		startTime:  time.Now(),
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
