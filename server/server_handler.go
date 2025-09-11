package chserver

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/NextChapterSoftware/chissl/server/capture"
	chshare "github.com/NextChapterSoftware/chissl/share"
	"github.com/NextChapterSoftware/chissl/share/cnet"
	"github.com/NextChapterSoftware/chissl/share/database"
	"github.com/NextChapterSoftware/chissl/share/settings"
	"github.com/NextChapterSoftware/chissl/share/tunnel"
	"golang.org/x/crypto/ssh"
	"golang.org/x/sync/errgroup"
)

// handleClientHandler is the main http websocket handler for the chisel server
func (s *Server) handleClientHandler(w http.ResponseWriter, r *http.Request) {
	//websockets upgrade AND has chisel prefix
	upgrade := strings.ToLower(r.Header.Get("Upgrade"))
	protocol := r.Header.Get("Sec-WebSocket-Protocol")
	if upgrade == "websocket" {
		if protocol == chshare.ProtocolVersion {
			s.handleWebsocket(w, r)
			return
		}
		//print into server logs and silently fall-through
		s.Infof("ignored client connection using protocol '%s', expected '%s'",
			protocol, chshare.ProtocolVersion)
	}

	//proxy target was provided
	if s.reverseProxy != nil {
		s.reverseProxy.ServeHTTP(w, r)
		return
	}

	//no proxy defined, provide access to health/version checks
	path := r.URL.Path
	// Redirect root to dashboard for browsers; avoid surprising APIs/CLIs
	if path == "/" || path == "" {
		accept := r.Header.Get("Accept")
		if strings.Contains(accept, "text/html") || accept == "" || accept == "*/*" {
			// Use 302 to satisfy some clients that do not auto-follow 303
			http.Redirect(w, r, "/dashboard", http.StatusFound)
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
		return
	}

	switch {
	case strings.HasPrefix(path, "/health"):
		w.Write([]byte("OK\n"))
		return
	case strings.HasPrefix(path, "/version"):
		w.Write([]byte(chshare.BuildVersion))
		return
	case strings.HasPrefix(path, "/authfile"):
		switch r.Method {
		case http.MethodPost:
			s.basicAuthMiddleware(s.handleAuthfile)(w, r) // Protecting with Basic Auth
			return
		}
	case strings.HasPrefix(path, "/users"):
		switch r.Method {
		case http.MethodGet:
			s.combinedAuthMiddleware(s.handleGetUsers)(w, r) // Protecting with Combined Auth
			return
		case http.MethodPost:
			s.combinedAuthMiddleware(s.handleCreateUser)(w, r) // Create user
			return
		}
	case strings.HasPrefix(path, "/api/users"):
		switch r.Method {
		case http.MethodGet:
			s.combinedAuthMiddleware(s.handleGetUsers)(w, r)
			return
		case http.MethodPost:
			s.combinedAuthMiddleware(s.handleCreateUser)(w, r)
			return
		case http.MethodPut:
			s.combinedAuthMiddleware(s.handleUpdateUserAPI)(w, r)
			return
		case http.MethodDelete:
			s.combinedAuthMiddleware(s.handleDeleteUserAPI)(w, r)
			return
		}
	case strings.HasPrefix(path, "/user"):
		switch r.Method {
		case http.MethodGet:
			s.combinedAuthMiddleware(s.handleGetUser)(w, r) // Protecting with Combined Auth
			return
		case http.MethodDelete:
			s.combinedAuthMiddleware(s.handleDeleteUser)(w, r) // Protecting with Combined Auth
			return
		case http.MethodPost:
			s.combinedAuthMiddleware(s.handleAddUser)(w, r) // Protecting with Combined Auth
			return
		case http.MethodPut:
			s.combinedAuthMiddleware(s.handleUpdateUser)(w, r) // Protecting with Combined Auth
			return
		}
	case strings.HasPrefix(path, "/api/tunnels"):
		switch r.Method {
		case http.MethodGet:
			if strings.Contains(path, "/active") {
				s.userAuthMiddleware(s.handleGetActiveTunnels)(w, r)
			} else if strings.Contains(path, "/payloads") {
				s.userAuthMiddleware(s.handleGetTunnelPayloads)(w, r)
			} else if getTunnelIDFromPath(path) != "" {
				s.userAuthMiddleware(s.handleGetTunnel)(w, r)
			} else {
				s.userAuthMiddleware(s.handleGetTunnels)(w, r)
			}
			return
		case http.MethodDelete:
			if strings.HasSuffix(path, "/closed") {
				s.userAuthMiddleware(s.handleDeleteClosedTunnels)(w, r)
				return
			}
			s.userAuthMiddleware(s.handleDeleteTunnel)(w, r)
			return
		}
	case strings.HasPrefix(path, "/api/connections"):
		switch r.Method {
		case http.MethodGet:
			s.combinedAuthMiddleware(s.handleGetConnections)(w, r)
			return
		}
	case strings.HasPrefix(path, "/api/capture/tunnels/"):
		switch r.Method {
		case http.MethodGet:
			if strings.HasSuffix(path, "/connections") {
				s.userAuthMiddleware(s.handleListCaptureConnections)(w, r)
				return
			}
			if strings.HasSuffix(path, "/download") {
				s.userAuthMiddleware(s.handleDownloadCaptureLog)(w, r)
				return
			}
			if strings.HasSuffix(path, "/recent") {
				s.userAuthMiddleware(s.handleGetRecentEvents)(w, r)
				return
			}
			if strings.HasSuffix(path, "/stream") {
				s.userAuthMiddleware(s.handleSSEStream)(w, r)
				return
			}
		}
		return
	case strings.HasPrefix(path, "/api/capture/listeners/"):
		switch r.Method {
		case http.MethodGet:
			if strings.HasSuffix(path, "/recent") {
				s.userAuthMiddleware(s.handleGetRecentEvents)(w, r)
				return
			}
			if strings.HasSuffix(path, "/stream") {
				s.userAuthMiddleware(s.handleSSEStream)(w, r)
				return
			}
		}
		return
	case strings.HasPrefix(path, "/api/listeners"):
		switch r.Method {
		case http.MethodGet:
			s.userAuthMiddleware(s.handleListListeners)(w, r)
			return
		case http.MethodPost:
			s.userAuthMiddleware(s.handleCreateListener)(w, r)
			return
		}
		return
	case strings.HasPrefix(path, "/api/multicast-tunnels"):
		// Public listing endpoint (authenticated user, not admin-only)
		if strings.HasSuffix(path, "/public") && r.Method == http.MethodGet {
			s.userAuthMiddleware(s.handleListPublicMulticastTunnels)(w, r)
			return
		}
		// Admin CRUD endpoints
		switch r.Method {
		case http.MethodGet:
			s.combinedAuthMiddleware(s.handleListMulticastTunnels)(w, r)
			return
		case http.MethodPost:
			s.combinedAuthMiddleware(s.handleCreateMulticastTunnel)(w, r)
			return
		case http.MethodPut, http.MethodPatch:
			s.combinedAuthMiddleware(s.handleUpdateMulticastTunnel)(w, r)
			return
		case http.MethodDelete:
			s.combinedAuthMiddleware(s.handleDeleteMulticastTunnel)(w, r)
			return
		}
		return
	case strings.HasPrefix(path, "/api/ai-providers"):
		pathParts := strings.Split(path, "/")
		if len(pathParts) >= 4 && pathParts[3] != "" {
			// Individual provider routes: /api/ai-providers/{id} or /api/ai-providers/{id}/test
			if len(pathParts) >= 5 && pathParts[4] == "test" {
				s.combinedAuthMiddleware(s.handleTestAIProvider)(w, r)
			} else {
				s.combinedAuthMiddleware(s.handleAIProvider)(w, r)
			}
		} else {
			// Collection routes: /api/ai-providers
			s.combinedAuthMiddleware(s.handleAIProviders)(w, r)
		}
		return
	case strings.HasPrefix(path, "/api/ai-listeners"):
		if strings.Contains(path, "/chat") {
			s.userAuthMiddleware(s.handleAIListenerChat)(w, r)
		} else if strings.Contains(path, "/preview") {
			s.userAuthMiddleware(s.handleAIListenerPreview)(w, r)
		} else if strings.Contains(path, "/refine") {
			s.userAuthMiddleware(s.handleAIListenerRefine)(w, r)
		} else if strings.Contains(path, "/force-regenerate") {
			s.userAuthMiddleware(s.handleForceRegenerateAIListener)(w, r)
		} else if strings.Contains(path, "/debug") {
			s.userAuthMiddleware(s.handleAIListenerDebug)(w, r)
		} else if strings.Contains(path, "/versions") || strings.Contains(path, "/activate") {
			s.userAuthMiddleware(s.handleAIListenerVersions)(w, r)
		} else {
			s.userAuthMiddleware(s.handleAIListeners)(w, r)
		}
		return
	case strings.HasPrefix(path, "/api/listener/"):
		switch r.Method {
		case http.MethodGet:
			s.userAuthMiddleware(s.handleGetListener)(w, r)
			return
		case http.MethodPut:
			s.userAuthMiddleware(s.handleUpdateListener)(w, r)
			return
		case http.MethodDelete:
			s.userAuthMiddleware(s.handleDeleteListener)(w, r)
			return
		}
		return
	case strings.HasPrefix(path, "/api/capture"):
		switch r.Method {
		case http.MethodGet:
			if strings.Contains(path, "/stream") {
				s.userAuthMiddleware(s.handleSSEStream)(w, r)
				return
			}
			if strings.Contains(path, "/recent") {
				s.userAuthMiddleware(s.handleGetRecentEvents)(w, r)
				return
			}
		}
	case strings.HasPrefix(path, "/api/stats"):
		switch r.Method {
		case http.MethodGet:
			s.userAuthMiddleware(s.handleGetStats)(w, r)
			return
		}
	case strings.HasPrefix(path, "/api/sessions"):
		switch r.Method {
		case http.MethodGet:
			s.combinedAuthMiddleware(s.handleGetSessions)(w, r)
			return
		case http.MethodDelete:
			if strings.HasSuffix(path, "/closed") {
				s.userAuthMiddleware(s.handleDeleteClosedSessions)(w, r)
				return
			}
		}
	case strings.HasPrefix(path, "/api/system"):
		switch r.Method {
		case http.MethodGet:
			s.userAuthMiddleware(s.handleGetSystemInfo)(w, r)
			return
		}
	case strings.HasPrefix(path, "/api/settings/feature/ai-mock-visible"):
		switch r.Method {
		case http.MethodGet:
			s.userAuthMiddleware(s.handleGetAIMockVisibility)(w, r)
			return
		case http.MethodPut, http.MethodPost:
			s.userAuthMiddleware(s.handleSetAIMockVisibility)(w, r)
			return
		}
		return

	case strings.HasPrefix(path, "/api/logs"):
		switch r.Method {
		case http.MethodGet:
			s.combinedAuthMiddleware(s.handleGetLogs)(w, r)
			return
		}
	case strings.HasPrefix(path, "/api/user/"):
		switch r.Method {
		case http.MethodGet:
			if strings.HasSuffix(path, "/info") {
				s.userAuthMiddleware(s.handleGetUserInfo)(w, r)
				return
			}
			if strings.Contains(path, "/tokens") {
				s.userAuthMiddleware(s.handleListUserTokens)(w, r)
				return
			}
			if strings.HasSuffix(path, "/port-reservations") {
				s.userAuthMiddleware(s.handleListUserPortReservations)(w, r)
				return
			}
			if strings.HasSuffix(path, "/reserved-ports-threshold") {
				s.userAuthMiddleware(s.handleGetReservedPortsThreshold)(w, r)
				return
			}
			if strings.Contains(path, "/preferences/") {
				s.userAuthMiddleware(s.handleGetUserPreference)(w, r)
				return
			}
		case http.MethodPost:
			if strings.Contains(path, "/tokens") {
				s.userAuthMiddleware(s.handleCreateUserToken)(w, r)
				return
			}
		case http.MethodPut:
			if strings.HasSuffix(path, "/profile") {
				s.userAuthMiddleware(s.handleUpdateUserProfile)(w, r)
				return
			}
			if strings.Contains(path, "/preferences/") {
				s.userAuthMiddleware(s.handleSetUserPreference)(w, r)
				return
			}
		case http.MethodDelete:
			if strings.Contains(path, "/tokens/") {
				s.userAuthMiddleware(s.handleRevokeUserToken)(w, r)
				return
			}
		}
		return
	case strings.HasPrefix(path, "/api/port-reservations"):
		switch r.Method {
		case http.MethodGet:
			s.combinedAuthMiddleware(s.handleListPortReservations)(w, r)
			return
		case http.MethodPost:
			s.combinedAuthMiddleware(s.handleCreatePortReservation)(w, r)
			return
		case http.MethodDelete:
			s.combinedAuthMiddleware(s.handleDeletePortReservation)(w, r)
			return
		}
		return
	case strings.HasPrefix(path, "/api/settings/reserved-ports-threshold"):
		switch r.Method {
		case http.MethodGet:
			s.combinedAuthMiddleware(s.handleGetReservedPortsThreshold)(w, r)
			return
		case http.MethodPost:
			s.combinedAuthMiddleware(s.handleSetReservedPortsThreshold)(w, r)
			return
		}
		return
	case strings.HasPrefix(path, "/api/settings/login-backoff"):
		switch r.Method {
		case http.MethodGet:
			s.combinedAuthMiddleware(s.handleGetLoginBackoffSettings)(w, r)
			return
		case http.MethodPut, http.MethodPost:
			s.combinedAuthMiddleware(s.handleUpdateLoginBackoffSettings)(w, r)
			return
		}
		return
	case strings.HasPrefix(path, "/api/settings/ip-rate"):
		switch r.Method {
		case http.MethodGet:
			s.combinedAuthMiddleware(s.handleGetIPRateSettings)(w, r)
			return
		case http.MethodPut, http.MethodPost:
			s.combinedAuthMiddleware(s.handleUpdateIPRateSettings)(w, r)
			return
		}
		return

	case strings.HasPrefix(path, "/api/security/events"):
		if r.Method == http.MethodGet {
			s.combinedAuthMiddleware(s.handleGetSecurityEvents)(w, r)
			return
		}
		return
	case strings.HasPrefix(path, "/api/security/webhooks"):
		// Special: test endpoint
		if strings.HasSuffix(path, "/test") && r.Method == http.MethodPost {
			s.combinedAuthMiddleware(s.handleTestSecurityWebhook)(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
			s.combinedAuthMiddleware(s.handleListSecurityWebhooks)(w, r)
			return
		case http.MethodPost:
			s.combinedAuthMiddleware(s.handleCreateSecurityWebhook)(w, r)
			return
		case http.MethodPut:
			s.combinedAuthMiddleware(s.handleUpdateSecurityWebhook)(w, r)
			return
		case http.MethodDelete:
			s.combinedAuthMiddleware(s.handleDeleteSecurityWebhook)(w, r)
			return
		}
		return

	case strings.HasPrefix(path, "/api/logs"):
		if strings.HasPrefix(path, "/api/logs/files/") {
			if strings.HasSuffix(path, "/download") {
				s.combinedAuthMiddleware(s.handleDownloadLogFile)(w, r)
				return
			} else {
				s.combinedAuthMiddleware(s.handleGetLogFileContent)(w, r)
				return
			}
		} else if strings.HasPrefix(path, "/api/logs/files") {
			s.combinedAuthMiddleware(s.handleGetLogFiles)(w, r)
			return
		} else if strings.HasSuffix(path, "/clear") {
			if r.Method == http.MethodPost {
				s.combinedAuthMiddleware(s.handleClearLogs)(w, r)
				return
			}
		} else {
			s.combinedAuthMiddleware(s.handleGetLogs)(w, r)
			return
		}
		return
	case strings.HasPrefix(path, "/auth/"):
		// Handle SSO authentication routes
		if strings.HasPrefix(path, "/auth/scim/") {
			if path == "/auth/scim/login" {
				s.handleSCIMLogin(w, r)
				return
			} else if path == "/auth/scim/callback" {
				s.handleSCIMCallback(w, r)
				return
			}
		} else if strings.HasPrefix(path, "/auth/auth0/") {
			if path == "/auth/auth0/login" {
				s.handleAuth0Login(w, r)
				return
			} else if path == "/auth/auth0/callback" {
				s.handleAuth0Callback(w, r)
				return
			}
		}
		http.NotFound(w, r)
		return
	case strings.HasPrefix(path, "/api/sso"):
		if strings.HasPrefix(path, "/api/sso/configs") {
			if strings.Contains(path, "/test") {
				s.combinedAuthMiddleware(s.handleTestSSOConfig)(w, r)
				return
			}
			switch r.Method {
			case http.MethodGet:
				if len(strings.Split(path, "/")) > 4 {
					s.combinedAuthMiddleware(s.handleGetSSOConfig)(w, r)
				} else {
					s.combinedAuthMiddleware(s.handleListSSOConfigs)(w, r)
				}
				return
			case http.MethodPost:
				s.combinedAuthMiddleware(s.handleCreateSSOConfig)(w, r)
				return
			case http.MethodDelete:
				s.combinedAuthMiddleware(s.handleDeleteSSOConfig)(w, r)
				return
			}
		} else if strings.HasPrefix(path, "/api/sso/user-sources") {
			s.combinedAuthMiddleware(s.handleListUserAuthSources)(w, r)
			return
		} else if path == "/api/sso/enabled" {
			// Public endpoint for enabled SSO configurations (for login page)
			s.handleListEnabledSSOConfigs(w, r)
			return
		}
		return
	case strings.HasPrefix(path, "/dashboard"):
		if s.config.Dashboard.Enabled {
			s.handleDashboard(w, r)
			return
		}
	}

	w.WriteHeader(404)
	w.Write([]byte("Not found"))
}

// handleWebsocket is responsible for handling the websocket connection
func (s *Server) handleWebsocket(w http.ResponseWriter, req *http.Request) {
	id := atomic.AddInt32(&s.sessCount, 1)
	l := s.Fork("session#%d", id)
	wsConn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		l.Debugf("Failed to upgrade (%s)", err)
		return
	}
	conn := cnet.NewWebSocketConn(wsConn)
	// perform SSH handshake on net.Conn
	l.Debugf("Handshaking with %s...", req.RemoteAddr)
	sshConn, chans, reqs, err := ssh.NewServerConn(conn, s.sshConfig)
	if err != nil {
		s.Debugf("Failed to handshake (%s)", err)
		return
	}
	// pull the users from the session map
	var user *settings.User
	if s.users.Len() > 0 {
		sid := string(sshConn.SessionID())
		u, ok := s.sessions.Get(sid)
		if !ok {
			panic("bug in ssh auth handler")
		}
		user = u
		s.sessions.Del(sid)
	}
	// chisel server handshake (reverse of client handshake)
	// verify configuration
	l.Debugf("Verifying configuration")
	// wait for request, with timeout
	var r *ssh.Request
	select {
	case r = <-reqs:
	case <-time.After(settings.EnvDuration("CONFIG_TIMEOUT", 10*time.Second)):
		l.Debugf("Timeout waiting for configuration")
		sshConn.Close()
		return
	}
	failed := func(err error) {
		l.Debugf("Failed: %s", err)
		r.Reply(false, []byte(err.Error()))
	}
	if r.Type != "config" {
		failed(s.Errorf("expecting config request"))
		return
	}
	c, err := settings.DecodeConfig(r.Payload)
	if err != nil {
		failed(s.Errorf("invalid config"))
		return
	}
	//print if client and server  versions dont match
	if c.Version != chshare.BuildVersion {
		v := c.Version
		if v == "" {
			v = "<unknown>"
		}
		l.Infof("Client version (%s) differs from server version (%s)",
			v, chshare.BuildVersion)
	}

	//validate remotes
	for _, r := range c.Remotes {
		//if user is provided, ensure they have
		//access to the desired remotes
		if user != nil {
			addr := r.UserAddr()
			if !user.HasAccess(addr) {
				failed(s.Errorf("access to '%s' denied", addr))
				return
			}

			// Check port reservations for the local port
			if localPort, err := strconv.Atoi(r.LocalPort); err == nil {
				if available, errMsg := s.isPortAvailableForUser(localPort, user.Name); !available {
					failed(s.Errorf("port reservation error: %s", errMsg))
					return
				}
			}
		}
		//confirm reverse tunnels are allowed
		if !s.config.Reverse {
			l.Debugf("Denied reverse port forwarding request, please enable --reverse")
			failed(s.Errorf("Reverse port forwaring not enabled on server"))
			return
		}
		//confirm reverse tunnel is available
		allowed := false
		if s.multicasts != nil {
			if lp, err := strconv.Atoi(r.LocalPort); err == nil {
				if am := s.multicasts.getActiveByPort(lp); am != nil && am.Config.Enabled {
					allowed = true
				}
			}
		}
		if !allowed && !r.CanListen() {
			// initialize capture service if not set
			if s.config.Dashboard.Enabled && s.capture == nil {
				// defaults: last 500 events, 64KB per event
				s.capture = capture.NewService(500, 64*1024)
			}
			failed(s.Errorf("Server cannot listen on %s", r.String()))
			return
		}
	}
	//successfuly validated config!
	r.Reply(true, nil)
	//tunnel per ssh connection
	// URL-safe tunnel ID
	tunnelID := fmt.Sprintf("sess-%d", id)
	var tapFactory tunnel.TapFactory
	if s.config.Dashboard.Enabled && s.capture != nil {
		// Build mapping so each remote has distinct capture id (sessionID-r{index}), AND also emit to base session id and canonical per-user+ports ID
		portToIndex := map[string]int{}
		for i, r := range c.Remotes {
			portToIndex[r.LocalPort] = i
		}
		uname := ""
		if user != nil {
			uname = user.Name
		}
		unameEnc := base64.RawURLEncoding.EncodeToString([]byte(uname))
		tapFactory = capture.NewTripleTapFactoryWithCanonical(s.capture, tunnelID, uname, 500, portToIndex, unameEnc)
	}
	tun := tunnel.New(tunnel.Config{
		Logger:     l,
		Inbound:    s.config.Reverse,
		Outbound:   true, //server always accepts outbound
		KeepAlive:  s.config.KeepAlive,
		TlsConf:    s.config.TlsConf,
		TapFactory: tapFactory,
		Username: func() string {
			if user != nil {
				return user.Name
			}
			return ""
		}(),
	})

	// Upsert active tunnels into DB so they appear in dashboard (base session + one row per remote)
	// Canonical per-remote tunnel IDs use URL-safe base64 of username for isolation between users
	unameEnc := base64.RawURLEncoding.EncodeToString([]byte(tun.Username))
	if s.db != nil {
		// base session row (no specific ports)
		{
			t := &database.Tunnel{ID: tunnelID, Username: tun.Username, Status: "open"}
			if e := s.db.CreateTunnel(t); e != nil {
				_ = s.db.UpdateTunnel(t)
			}
		}
		for _, rmt := range c.Remotes {
			lp, _ := strconv.Atoi(rmt.LocalPort)
			// Skip DB row for multicast subscriber remotes so they don't appear as regular tunnels
			if s.multicasts != nil {
				if am := s.multicasts.getActiveByPort(lp); am != nil && am.Config.Enabled {
					continue
				}
			}
			rp, _ := strconv.Atoi(rmt.RemotePort)
			// Ensure only one open row per user/local/remote combo by closing any previous open rows
			_ = s.db.CloseActiveTunnelsByUserPorts(tun.Username, lp, rp)
			id := fmt.Sprintf("tun-%s-%d-%d", unameEnc, lp, rp)
			t := &database.Tunnel{ID: id, Username: tun.Username, LocalPort: lp, LocalHost: rmt.LocalHost, RemotePort: rp, RemoteHost: rmt.RemoteHost, Status: "open"}
			if e := s.db.CreateTunnel(t); e != nil {
				_ = s.db.UpdateTunnel(t)
			}
		}
	}

	//bind
	eg, ctx := errgroup.WithContext(req.Context())

	// Periodic DB heartbeat to keep tunnel status fresh while connection is alive
	if s.db != nil {
		eg.Go(func() error {
			ticker := time.NewTicker(60 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return nil
				case <-ticker.C:
					// Touch updated_at without changing other fields for base and each remote row
					_ = s.db.AddTunnelConnections(tunnelID, 0)
					for _, rmt := range c.Remotes {
						lp, _ := strconv.Atoi(rmt.LocalPort)
						if s.multicasts != nil {
							if am := s.multicasts.getActiveByPort(lp); am != nil && am.Config.Enabled {
								continue
							}
						}
						rp, _ := strconv.Atoi(rmt.RemotePort)
						id := fmt.Sprintf("tun-%s-%d-%d", unameEnc, lp, rp)
						_ = s.db.AddTunnelConnections(id, 0)
					}
				}
			}
		})
	}

	eg.Go(func() error {

		// Track live tunnels in memory (for dashboard when DB is off)
		if s.db == nil {
			s.liveMu.Lock()
			// base session entry
			s.liveTunnels[tunnelID] = &database.Tunnel{ID: tunnelID, Username: tun.Username, Status: "open", CreatedAt: time.Now(), UpdatedAt: time.Now()}
			for _, rmt := range c.Remotes {
				lp, _ := strconv.Atoi(rmt.LocalPort)
				if s.multicasts != nil {
					if am := s.multicasts.getActiveByPort(lp); am != nil && am.Config.Enabled {
						continue
					}
				}
				rp, _ := strconv.Atoi(rmt.RemotePort)
				id := fmt.Sprintf("tun-%s-%d-%d", unameEnc, lp, rp)
				s.liveTunnels[id] = &database.Tunnel{ID: id, Username: tun.Username, LocalPort: lp, LocalHost: rmt.LocalHost, RemotePort: rp, RemoteHost: rmt.RemoteHost, Status: "open", CreatedAt: time.Now(), UpdatedAt: time.Now()}
			}
			s.liveMu.Unlock()
			defer func() {
				s.liveMu.Lock()
				// mark base and remotes inactive
				if t, ok := s.liveTunnels[tunnelID]; ok {
					t.Status = "inactive"
					t.UpdatedAt = time.Now()
				}
				for _, rmt := range c.Remotes {
					lp, _ := strconv.Atoi(rmt.LocalPort)
					if s.multicasts != nil {
						if am := s.multicasts.getActiveByPort(lp); am != nil && am.Config.Enabled {
							continue
						}
					}
					rp, _ := strconv.Atoi(rmt.RemotePort)
					id := fmt.Sprintf("tun-%s-%d-%d", unameEnc, lp, rp)
					if t, ok := s.liveTunnels[id]; ok {
						t.Status = "inactive"
						t.UpdatedAt = time.Now()
					}
				}
				s.liveMu.Unlock()
			}()
		}

		//connected, handover ssh connection for tunnel to use, and block
		return tun.BindSSH(ctx, sshConn, reqs, chans)
	})
	eg.Go(func() error {
		// connected, setup reversed-remotes? For multicast ports, register as subscribers instead of binding listeners
		serverInbound := c.Remotes.Reversed(true)
		inboundFiltered := make([]*settings.Remote, 0, len(serverInbound))
		// Track subs to unregister on session close
		type subKey struct {
			port int
			id   string
		}
		var subs []subKey
		if s.multicasts != nil {
			for i, rmt := range serverInbound {
				lp, _ := strconv.Atoi(rmt.LocalPort)
				if am := s.multicasts.getActiveByPort(lp); am != nil && am.Config.Enabled {
					subID := fmt.Sprintf("%s-r%d", tunnelID, i)
					_ = s.multicasts.AddSubscriber(lp, &subscriber{ID: subID, Tun: tun, Remote: *rmt, Username: tun.Username})
					subs = append(subs, subKey{port: lp, id: subID})
					continue
				}
				inboundFiltered = append(inboundFiltered, rmt)
			}
			if len(subs) > 0 {
				go func(keys []subKey) {
					<-ctx.Done()
					for _, k := range keys {
						s.multicasts.RemoveSubscriber(k.port, k.id)
					}
				}(subs)
			}
		} else {
			inboundFiltered = serverInbound
		}
		if len(inboundFiltered) == 0 {
			return nil
		}
		// block on non-multicast remotes
		return tun.BindRemotes(ctx, inboundFiltered)
	})
	err = eg.Wait()
	// Persist/update active tunnel info in DB if configured
	if s.db != nil {
		// Update status only; creation happens on open
		status := "closed"
		if err != nil && !strings.HasSuffix(err.Error(), "EOF") {
			status = "error"
		}
		// base session row
		_ = s.db.UpdateTunnel(&database.Tunnel{ID: tunnelID, Username: tun.Username, Status: status})
		for _, rmt := range c.Remotes {
			lp, _ := strconv.Atoi(rmt.LocalPort)
			if s.multicasts != nil {
				if am := s.multicasts.getActiveByPort(lp); am != nil && am.Config.Enabled {
					continue
				}
			}
			rp, _ := strconv.Atoi(rmt.RemotePort)
			id := fmt.Sprintf("tun-%s-%d-%d", unameEnc, lp, rp)
			_ = s.db.UpdateTunnel(&database.Tunnel{ID: id, Username: tun.Username, Status: status})
		}
	}
	if err != nil && !strings.HasSuffix(err.Error(), "EOF") {
		l.Debugf("Closed connection (%s)", err)
	} else {
		l.Debugf("Closed connection")
	}
}
