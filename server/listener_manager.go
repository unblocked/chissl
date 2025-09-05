package chserver

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/NextChapterSoftware/chissl/server/capture"
	"github.com/NextChapterSoftware/chissl/share/database"
	"github.com/NextChapterSoftware/chissl/share/settings"
	"github.com/NextChapterSoftware/chissl/share/tunnel"
)

// ListenerManager manages HTTP listeners/proxies
type ListenerManager struct {
	mu        sync.RWMutex
	listeners map[string]*ActiveListener // listenerID -> ActiveListener
	capture   *capture.Service
	db        database.Database
	tlsConfig *tls.Config // TLS configuration from main server
}

// ActiveListener represents a running HTTP listener
type ActiveListener struct {
	ID         string
	Config     *database.Listener
	Server     *http.Server
	Cancel     context.CancelFunc
	TapFactory tunnel.TapFactory
}

// NewListenerManager creates a new listener manager
func NewListenerManager(captureService *capture.Service, db database.Database, tlsConfig *tls.Config) *ListenerManager {
	return &ListenerManager{
		listeners: make(map[string]*ActiveListener),
		capture:   captureService,
		db:        db,
		tlsConfig: tlsConfig,
	}
}

// StartListener starts a new HTTP listener
func (lm *ListenerManager) StartListener(config *database.Listener, tapFactory tunnel.TapFactory) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	// Check if port is already in use
	for _, existing := range lm.listeners {
		if existing.Config.Port == config.Port {
			return fmt.Errorf("port %d already in use by listener %s", config.Port, existing.ID)
		}
	}

	// Create HTTP handler based on mode
	var handler http.Handler
	switch config.Mode {
	case "sink":
		handler = lm.createSinkHandler(config, tapFactory)
	case "proxy":
		handler = lm.createProxyHandler(config, tapFactory)
	default:
		return fmt.Errorf("unsupported listener mode: %s", config.Mode)
	}

	// Create HTTP server
	ctx, cancel := context.WithCancel(context.Background())
	server := &http.Server{
		Addr:      fmt.Sprintf(":%d", config.Port),
		Handler:   handler,
		TLSConfig: lm.tlsConfig, // Use the main server's TLS config
	}

	activeListener := &ActiveListener{
		ID:         config.ID,
		Config:     config,
		Server:     server,
		Cancel:     cancel,
		TapFactory: tapFactory,
	}

	// Start server in goroutine
	go func() {
		defer cancel()

		fmt.Printf("Starting listener %s on port %d (UseTLS: %v, TLS config available: %v)\n",
			config.ID, config.Port, config.UseTLS, lm.tlsConfig != nil)

		listener, err := net.Listen("tcp", server.Addr)
		if err != nil {
			// Update status to error
			config.Status = "error"
			if lm.db != nil {
				_ = lm.db.UpdateListener(config)
			}
			return
		}

		// Wrap with TLS if configured and requested
		if lm.tlsConfig != nil && config.UseTLS {
			// Clone the TLS config to avoid race conditions
			tlsConfigCopy := lm.tlsConfig.Clone()
			listener = tls.NewListener(listener, tlsConfigCopy)
			// Log TLS configuration details for debugging
			certCount := len(tlsConfigCopy.Certificates)
			if tlsConfigCopy.GetCertificate != nil {
				certCount = -1 // Dynamic certificate (Let's Encrypt)
			}
			fmt.Printf("Listener %s on port %d: TLS enabled with %d certificates (dynamic: %v)\n",
				config.ID, config.Port, certCount, tlsConfigCopy.GetCertificate != nil)
		} else if config.UseTLS {
			// Log when TLS is requested but not available
			fmt.Printf("Listener %s on port %d: TLS requested but no TLS config available\n",
				config.ID, config.Port)
		} else {
			fmt.Printf("Listener %s on port %d: Plain HTTP (TLS disabled)\n",
				config.ID, config.Port)
		}

		// Update status to open
		config.Status = "open"
		if lm.db != nil {
			_ = lm.db.UpdateListener(config)
		}

		// Serve until context is cancelled
		go func() {
			<-ctx.Done()
			server.Close()
		}()

		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			// Update status to error
			config.Status = "error"
			if lm.db != nil {
				_ = lm.db.UpdateListener(config)
			}
		}
	}()

	lm.listeners[config.ID] = activeListener
	return nil
}

// StopListener stops a running listener
func (lm *ListenerManager) StopListener(listenerID string) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	activeListener, exists := lm.listeners[listenerID]
	if !exists {
		return fmt.Errorf("listener %s not found", listenerID)
	}

	// Cancel context and close server
	activeListener.Cancel()

	// Update status to closed
	activeListener.Config.Status = "closed"
	if lm.db != nil {
		_ = lm.db.UpdateListener(activeListener.Config)
	}

	delete(lm.listeners, listenerID)
	return nil
}

// GetActiveListener returns an active listener by ID
func (lm *ListenerManager) GetActiveListener(listenerID string) (*ActiveListener, bool) {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	listener, exists := lm.listeners[listenerID]
	return listener, exists
}

// ListActiveListeners returns all active listeners
func (lm *ListenerManager) ListActiveListeners() []*ActiveListener {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	var result []*ActiveListener
	for _, listener := range lm.listeners {
		result = append(result, listener)
	}
	return result
}

// UpdateTLSConfig updates the TLS configuration for the listener manager
func (lm *ListenerManager) UpdateTLSConfig(tlsConfig *tls.Config) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.tlsConfig = tlsConfig
}

// createSinkHandler creates a handler that logs requests and returns configured responses
func (lm *ListenerManager) createSinkHandler(config *database.Listener, tapFactory tunnel.TapFactory) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connID := fmt.Sprintf("%s-%d", r.RemoteAddr, time.Now().UnixNano())

		// Create tap for this connection
		var tap tunnel.Tap
		if tapFactory != nil {
			meta := tunnel.Meta{
				Username: config.Username,
				Remote:   settings.Remote{LocalHost: "127.0.0.1", LocalPort: strconv.Itoa(config.Port), RemoteHost: "127.0.0.1", RemotePort: strconv.Itoa(config.Port)},
				ConnID:   connID,
			}
			tap = tapFactory(meta)
			if tap != nil {
				tap.OnOpen()
				defer func() {
					// Calculate approximate bytes (headers + body)
					sent := int64(len(config.Response))
					received := int64(r.ContentLength)
					if received < 0 {
						received = 0
					}
					tap.OnClose(sent, received)
				}()
			}
		}

		// Log the request
		if tap != nil {
			// Capture request
			reqData := fmt.Sprintf("%s %s %s\r\n", r.Method, r.URL.String(), r.Proto)
			for name, values := range r.Header {
				for _, value := range values {
					reqData += fmt.Sprintf("%s: %s\r\n", name, value)
				}
			}
			reqData += "\r\n"

			// Read body if present
			if r.Body != nil {
				body, _ := io.ReadAll(r.Body)
				reqData += string(body)
				r.Body.Close()
			}

			tap.DstWriter().Write([]byte(reqData))
		}

		// Send configured response or default
		response := config.Response
		if response == "" {
			response = `{"status": "ok", "message": "Request logged"}`
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))

		// Log the response
		if tap != nil {
			respData := fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: %d\r\n\r\n%s",
				len(response), response)
			tap.SrcWriter().Write([]byte(respData))
		}
	})
}

// createProxyHandler creates a handler that proxies requests to a target URL
func (lm *ListenerManager) createProxyHandler(config *database.Listener, tapFactory tunnel.TapFactory) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connID := fmt.Sprintf("%s-%d", r.RemoteAddr, time.Now().UnixNano())

		// Debug log the config
		fmt.Printf("Proxy handler for listener %s: target='%s', mode='%s'\n", config.ID, config.TargetURL, config.Mode)

		// Create tap for this connection
		var tap tunnel.Tap
		if tapFactory != nil {
			meta := tunnel.Meta{
				Username: config.Username,
				Remote:   settings.Remote{LocalHost: "127.0.0.1", LocalPort: strconv.Itoa(config.Port), RemoteHost: "127.0.0.1", RemotePort: strconv.Itoa(config.Port)},
				ConnID:   connID,
			}
			tap = tapFactory(meta)
			if tap != nil {
				tap.OnOpen()
				defer func() {
					// Bytes will be calculated during proxy
					tap.OnClose(0, 0) // TODO: Calculate actual bytes
				}()
			}
		}

		// Parse target URL (trim whitespace first)
		cleanTargetURL := strings.TrimSpace(config.TargetURL)
		targetURL, err := url.Parse(cleanTargetURL)
		if err != nil {
			errMsg := fmt.Sprintf("Invalid target URL '%s': %v", cleanTargetURL, err)
			fmt.Printf("URL parse error: %s\n", errMsg)
			http.Error(w, errMsg, http.StatusInternalServerError)
			return
		}

		// Debug log the target URL
		fmt.Printf("Parsed target URL: %s (scheme: %s, host: %s)\n", targetURL.String(), targetURL.Scheme, targetURL.Host)

		// Build the full target URL with the original path and query
		fullTargetURL := *targetURL
		fullTargetURL.Path = r.URL.Path
		fullTargetURL.RawQuery = r.URL.RawQuery

		// Create proxy request
		proxyReq, err := http.NewRequest(r.Method, fullTargetURL.String(), r.Body)
		if err != nil {
			http.Error(w, "Failed to create proxy request", http.StatusInternalServerError)
			return
		}

		// Copy headers but rewrite Host header to target host
		for name, values := range r.Header {
			// Skip Host header - we'll set it to the target host
			if strings.ToLower(name) == "host" {
				continue
			}
			for _, value := range values {
				proxyReq.Header.Add(name, value)
			}
		}

		// Set the Host header to the target host
		proxyReq.Host = targetURL.Host
		proxyReq.Header.Set("Host", targetURL.Host)

		// Log the original request (before proxy)
		if tap != nil {
			reqData := fmt.Sprintf("=== ORIGINAL REQUEST ===\r\n")
			reqData += fmt.Sprintf("%s %s %s\r\n", r.Method, r.URL.String(), r.Proto)
			reqData += fmt.Sprintf("Host: %s\r\n", r.Host)
			for name, values := range r.Header {
				for _, value := range values {
					reqData += fmt.Sprintf("%s: %s\r\n", name, value)
				}
			}
			reqData += "\r\n"

			// Also log the proxy request details
			reqData += fmt.Sprintf("=== PROXY REQUEST ===\r\n")
			reqData += fmt.Sprintf("%s %s %s\r\n", proxyReq.Method, proxyReq.URL.String(), proxyReq.Proto)
			reqData += fmt.Sprintf("Host: %s\r\n", proxyReq.Host)
			for name, values := range proxyReq.Header {
				for _, value := range values {
					reqData += fmt.Sprintf("%s: %s\r\n", name, value)
				}
			}
			reqData += "\r\n"

			tap.DstWriter().Write([]byte(reqData))
		}

		// Also log to console for debugging
		fmt.Printf("Proxying: %s %s -> %s\n", r.Method, r.URL.String(), proxyReq.URL.String())

		// Make proxy request with proper TLS configuration
		transport := &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false, // Verify upstream certificates
			},
		}

		client := &http.Client{
			Timeout:   30 * time.Second, // Increased timeout for slow sites
			Transport: transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Allow up to 10 redirects
				if len(via) >= 10 {
					return fmt.Errorf("stopped after 10 redirects")
				}
				// Log redirects for debugging
				fmt.Printf("Following redirect: %s -> %s\n", via[len(via)-1].URL.String(), req.URL.String())
				return nil
			},
		}

		resp, err := client.Do(proxyReq)
		if err != nil {
			errMsg := fmt.Sprintf("Proxy request failed: %v", err)
			fmt.Printf("Proxy error for %s: %v\n", proxyReq.URL.String(), err)
			if tap != nil {
				tap.SrcWriter().Write([]byte(fmt.Sprintf("=== PROXY ERROR ===\r\n%s\r\n", errMsg)))
			}
			http.Error(w, fmt.Sprintf("Proxy request failed: %v", err), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		// Log the upstream response first
		if tap != nil {
			respData := fmt.Sprintf("=== UPSTREAM RESPONSE ===\r\n")
			respData += fmt.Sprintf("HTTP/1.1 %d %s\r\n", resp.StatusCode, resp.Status)
			for name, values := range resp.Header {
				for _, value := range values {
					respData += fmt.Sprintf("%s: %s\r\n", name, value)
				}
			}
			respData += "\r\n"
			tap.SrcWriter().Write([]byte(respData))
		}

		// Copy response headers (potentially modify them here if needed)
		for name, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(name, value)
			}
		}
		w.WriteHeader(resp.StatusCode)

		// Stream response body while capturing it
		if tap != nil {
			// Use a tee reader to capture the response body
			teeReader := io.TeeReader(resp.Body, tap.SrcWriter())
			io.Copy(w, teeReader)
		} else {
			io.Copy(w, resp.Body)
		}
	})
}
