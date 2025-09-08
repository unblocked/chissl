package chserver

import (
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
				s.combinedAuthMiddleware(s.handleListCaptureConnections)(w, r)
				return
			}
			if strings.HasSuffix(path, "/download") {
				s.combinedAuthMiddleware(s.handleDownloadCaptureLog)(w, r)
				return
			}
			if strings.HasSuffix(path, "/recent") {
				s.combinedAuthMiddleware(s.handleGetRecentEvents)(w, r)
				return
			}
			if strings.HasSuffix(path, "/stream") {
				s.combinedAuthMiddleware(s.handleSSEStream)(w, r)
				return
			}
		}
		return
	case strings.HasPrefix(path, "/api/capture/listeners/"):
		switch r.Method {
		case http.MethodGet:
			if strings.HasSuffix(path, "/recent") {
				s.combinedAuthMiddleware(s.handleGetRecentEvents)(w, r)
				return
			}
			if strings.HasSuffix(path, "/stream") {
				s.combinedAuthMiddleware(s.handleSSEStream)(w, r)
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
	case strings.HasPrefix(path, "/api/ai-providers"):
		pathParts := strings.Split(path, "/")
		if len(pathParts) >= 4 && pathParts[3] != "" {
			// Individual provider routes: /api/ai-providers/{id} or /api/ai-providers/{id}/test
			if len(pathParts) >= 5 && pathParts[4] == "test" {
				s.handleTestAIProvider(w, r)
			} else {
				s.handleAIProvider(w, r)
			}
		} else {
			// Collection routes: /api/ai-providers
			s.handleAIProviders(w, r)
		}
		return
	case strings.HasPrefix(path, "/api/ai-listeners"):
		if strings.Contains(path, "/chat") {
			s.handleAIListenerChat(w, r)
		} else if strings.Contains(path, "/preview") {
			s.handleAIListenerPreview(w, r)
		} else if strings.Contains(path, "/refine") {
			s.handleAIListenerRefine(w, r)
		} else if strings.Contains(path, "/force-regenerate") {
			s.handleForceRegenerateAIListener(w, r)
		} else if strings.Contains(path, "/debug") {
			s.handleAIListenerDebug(w, r)
		} else if strings.Contains(path, "/versions") || strings.Contains(path, "/activate") {
			s.handleAIListenerVersions(w, r)
		} else {
			s.handleAIListeners(w, r)
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
				s.combinedAuthMiddleware(s.handleSSEStream)(w, r)
				return
			}
			if strings.Contains(path, "/recent") {
				s.combinedAuthMiddleware(s.handleGetRecentEvents)(w, r)
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
		if !r.CanListen() {

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
		tapFactory = capture.NewTapFactory(s.capture, tunnelID, func() string {
			if user != nil {
				return user.Name
			}
			return ""
		}(), 500)
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

	// Upsert active tunnel into DB so it appears in dashboard
	if s.db != nil {
		// Only record the first remote for now to fill ports/hosts
		var localPort, remotePort int
		if len(c.Remotes) > 0 {
			lp, _ := strconv.Atoi(c.Remotes[0].LocalPort)
			rp, _ := strconv.Atoi(c.Remotes[0].RemotePort)
			localPort, remotePort = lp, rp
		}
		t := &database.Tunnel{ID: tunnelID, Username: tun.Username, LocalPort: localPort, LocalHost: c.Remotes[0].LocalHost, RemotePort: remotePort, RemoteHost: c.Remotes[0].RemoteHost, Status: "open"}
		if e := s.db.CreateTunnel(t); e != nil {
			_ = s.db.UpdateTunnel(t)
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
					// Touch updated_at without changing other fields
					if err := s.db.AddTunnelConnections(tunnelID, 0); err != nil {
						s.Debugf("tunnel heartbeat failed for %s: %v", tunnelID, err)
					}
				}
			}
		})
	}

	eg.Go(func() error {

		// Track live tunnel in memory (for dashboard when DB is off)
		if s.db == nil {
			var localPort, remotePort int
			var localHost, remoteHost string
			if len(c.Remotes) > 0 {
				localHost = c.Remotes[0].LocalHost
				remoteHost = c.Remotes[0].RemoteHost
				lp, _ := strconv.Atoi(c.Remotes[0].LocalPort)
				rp, _ := strconv.Atoi(c.Remotes[0].RemotePort)
				localPort, remotePort = lp, rp
			}
			s.liveMu.Lock()
			s.liveTunnels[tunnelID] = &database.Tunnel{ID: tunnelID, Username: tun.Username, LocalPort: localPort, LocalHost: localHost, RemotePort: remotePort, RemoteHost: remoteHost, Status: "open", CreatedAt: time.Now(), UpdatedAt: time.Now()}
			s.liveMu.Unlock()
			defer func() {
				s.liveMu.Lock()
				if t, ok := s.liveTunnels[tunnelID]; ok {
					t.Status = "inactive"
					t.UpdatedAt = time.Now()
				}
				s.liveMu.Unlock()
			}()
		}

		//connected, handover ssh connection for tunnel to use, and block
		return tun.BindSSH(ctx, sshConn, reqs, chans)
	})
	eg.Go(func() error {
		//connected, setup reversed-remotes?
		serverInbound := c.Remotes.Reversed(true)
		if len(serverInbound) == 0 {
			return nil
		}
		//block
		return tun.BindRemotes(ctx, serverInbound)
	})
	err = eg.Wait()
	// Persist/update active tunnel info in DB if configured
	if s.db != nil {
		// Update status only; creation happens on open
		status := "closed"
		if err != nil && !strings.HasSuffix(err.Error(), "EOF") {
			status = "error"
		}
		t := &database.Tunnel{ID: tunnelID, Username: tun.Username, Status: status}
		_ = s.db.UpdateTunnel(t)
	}
	if err != nil && !strings.HasSuffix(err.Error(), "EOF") {
		l.Debugf("Closed connection (%s)", err)
	} else {
		l.Debugf("Closed connection")
	}
}
