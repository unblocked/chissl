package chserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/NextChapterSoftware/chissl/share/database"
)

// SSE: live traffic for a given tunnel or listener
func (s *Server) handleSSEStream(w http.ResponseWriter, r *http.Request) {
	entityID, entityType := getEntityIDFromPath(r.URL.Path)
	if entityID == "" {
		http.Error(w, "Invalid entity ID", http.StatusBadRequest)
		return
	}
	if !s.userHasEntityAccess(r, entityID, entityType) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable proxy buffering
	w.WriteHeader(http.StatusOK)
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	ch := s.capture.Subscribe(entityID)
	defer s.capture.Unsubscribe(entityID, ch)
	// Send a comment to keep the connection alive
	fmt.Fprintf(w, ": heartbeat\n\n")
	flusher.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case e := <-ch:
			b, _ := json.Marshal(e)
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		case <-time.After(15 * time.Second):
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

// Recent events
func (s *Server) handleGetRecentEvents(w http.ResponseWriter, r *http.Request) {
	entityID, entityType := getEntityIDFromPath(r.URL.Path)
	if entityID == "" {
		http.Error(w, "Invalid entity ID", http.StatusBadRequest)
		return
	}
	if !s.userHasEntityAccess(r, entityID, entityType) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}
	events := s.capture.GetRecent(entityID, 500)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(events)
}

// List persisted connection IDs for a tunnel
func (s *Server) handleListCaptureConnections(w http.ResponseWriter, r *http.Request) {
	tunnelID := getTunnelIDFromPath(r.URL.Path)
	if tunnelID == "" {
		http.Error(w, "Invalid tunnel ID", http.StatusBadRequest)
		return
	}
	if !s.userHasTunnelAccess(r, tunnelID) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}
	ids, err := s.capture.ListConnections(tunnelID)
	if err != nil {
		http.Error(w, "Failed to list connections", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"connections": ids})
}

// Download latest persisted log file for a connection
func (s *Server) handleDownloadCaptureLog(w http.ResponseWriter, r *http.Request) {
	parts := splitPath(r.URL.Path)
	// Expect: /api/capture/tunnels/{id}/connections/{connID}/download
	if len(parts) < 7 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	tunnelID := parts[2]
	connID := parts[4]
	if !s.userHasTunnelAccess(r, tunnelID) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}
	p, err := s.capture.LatestLogFile(tunnelID, connID)
	if err != nil {
		http.Error(w, "Log not found", http.StatusNotFound)
		return
	}
	http.ServeFile(w, r, p)
}

// Extended API endpoints for dashboard and monitoring

// handleGetTunnels returns all tunnels (filtered by user permissions)
func (s *Server) handleGetTunnels(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		// Fallback when no DB: list from in-memory liveTunnels
		s.liveMu.RLock()
		defer s.liveMu.RUnlock()
		list := make([]*database.Tunnel, 0, len(s.liveTunnels))
		for _, t := range s.liveTunnels {
			list = append(list, t)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(list)
		return
	}

	tunnels, err := s.db.ListTunnels()
	if err != nil {
		http.Error(w, "Failed to get tunnels", http.StatusInternalServerError)
		return
	}

	// Filter tunnels based on user permissions
	filteredTunnels := s.filterTunnelsByUser(r, tunnels)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filteredTunnels)
}

// handleGetActiveTunnels returns only active tunnels (filtered by user permissions)
func (s *Server) handleGetActiveTunnels(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		// Fallback when no DB: only active from in-memory list
		s.liveMu.RLock()
		defer s.liveMu.RUnlock()
		list := make([]*database.Tunnel, 0, len(s.liveTunnels))
		for _, t := range s.liveTunnels {
			if t.Status == "active" {
				list = append(list, t)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(list)
		return
	}

	tunnels, err := s.db.ListActiveTunnels()
	if err != nil {
		http.Error(w, "Failed to get active tunnels", http.StatusInternalServerError)
		return
	}

	// Filter tunnels based on user permissions
	filteredTunnels := s.filterTunnelsByUser(r, tunnels)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filteredTunnels)
}

// handleGetTunnel returns a specific tunnel
func (s *Server) handleGetTunnel(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}

	tunnelID := getTunnelIDFromPath(r.URL.Path)
	if tunnelID == "" {
		http.Error(w, "Invalid tunnel ID", http.StatusBadRequest)
		return
	}

	tunnel, err := s.db.GetTunnel(tunnelID)
	if err != nil {
		http.Error(w, "Tunnel not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tunnel)
}

// handleGetTunnelPayloads returns payload logs for a specific tunnel
func (s *Server) handleGetTunnelPayloads(w http.ResponseWriter, r *http.Request) {
	tunnelID := getTunnelIDFromPath(r.URL.Path)
	if tunnelID == "" {
		http.Error(w, "Invalid tunnel ID", http.StatusBadRequest)
		return
	}

	// Check if user has access to this tunnel
	if !s.userHasTunnelAccess(r, tunnelID) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	// For now, return mock payload data
	// In a real implementation, you'd store and retrieve actual payload logs
	payloads := []map[string]interface{}{
		{
			"id":        "payload_1",
			"timestamp": time.Now().Add(-time.Hour).Format(time.RFC3339),
			"method":    "GET",
			"path":      "/api/users",
			"headers":   map[string]string{"User-Agent": "curl/7.68.0", "Accept": "*/*"},
			"body":      "",
			"response":  `{"users": [{"username": "admin", "is_admin": true}]}`,
			"status":    200,
		},
		{
			"id":        "payload_2",
			"timestamp": time.Now().Add(-30 * time.Minute).Format(time.RFC3339),
			"method":    "POST",
			"path":      "/api/data",
			"headers":   map[string]string{"Content-Type": "application/json"},
			"body":      `{"name": "test", "value": 123}`,
			"response":  `{"success": true, "id": "abc123"}`,
			"status":    201,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payloads)
}

// userHasTunnelAccess checks if user has access to a specific tunnel
func (s *Server) userHasTunnelAccess(r *http.Request, tunnelID string) bool {
	username := s.getCurrentUsername(r)
	if username == "" {
		return false
	}

	// Admins have access to all tunnels
	if s.isUserAdmin(r.Context()) {
		return true
	}

	// Check if tunnel belongs to user
	if s.db != nil {
		tunnel, err := s.db.GetTunnel(tunnelID)
		if err != nil {
			return false
		}
		return tunnel.Username == username
	}

	return false
}

// handleDeleteTunnel deletes a tunnel
func (s *Server) handleDeleteTunnel(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}

	tunnelID := getTunnelIDFromPath(r.URL.Path)
	if tunnelID == "" {
		http.Error(w, "Invalid tunnel ID", http.StatusBadRequest)
		return
	}

	err := s.db.DeleteTunnel(tunnelID)
	if err != nil {
		http.Error(w, "Failed to delete tunnel", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleGetConnections returns connections for a tunnel
func (s *Server) handleGetConnections(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}

	tunnelID := r.URL.Query().Get("tunnel_id")
	if tunnelID == "" {
		http.Error(w, "tunnel_id parameter required", http.StatusBadRequest)
		return
	}

	connections, err := s.db.ListConnections(tunnelID)
	if err != nil {
		http.Error(w, "Failed to get connections", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(connections)
}

// handleGetStats returns system statistics
func (s *Server) handleGetStats(w http.ResponseWriter, r *http.Request) {
	var stats *database.Stats
	var err error

	// Check if user-specific stats are requested
	username := s.getUsernameFromContext(r.Context())
	userSpecific := r.URL.Query().Get("user") == "true"

	if s.db != nil {
		if userSpecific && username != "" {
			stats, err = s.db.GetUserStats(username)
		} else {
			stats, err = s.db.GetStats()
		}
		if err != nil {
			http.Error(w, "Failed to get stats", http.StatusInternalServerError)
			return
		}
	} else {
		// Fallback to basic stats without database
		active := 0
		total := 0
		s.liveMu.RLock()
		for _, t := range s.liveTunnels {
			total++
			if t.Status == "active" {
				active++
			}
		}
		s.liveMu.RUnlock()

		// Count listeners from listener manager
		totalListeners := 0
		activeListeners := 0
		if s.listeners != nil {
			activeListenersList := s.listeners.ListActiveListeners()
			activeListeners = len(activeListenersList)
			// For total listeners, we'd need to query the database or keep a separate count
			// For now, just use active listeners as total (fallback mode limitation)
			totalListeners = activeListeners
		}

		stats = &database.Stats{
			TotalTunnels:     total,
			ActiveTunnels:    active,
			TotalListeners:   totalListeners,
			ActiveListeners:  activeListeners,
			TotalUsers:       s.users.Len(),
			ActiveSessions:   s.sessions.Len(),
			TotalConnections: 0,
			TotalBytesSent:   0,
			TotalBytesRecv:   0,
		}
	}

	// Calculate uptime
	stats.UptimeSeconds = int64(time.Since(s.startTime).Seconds())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// handleGetSessions returns active sessions
func (s *Server) handleGetSessions(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		// Fallback to in-memory sessions - simplified implementation
		sessions := make(map[string]interface{})
		sessions["active_count"] = s.sessions.Len()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sessions)
		return
	}

	// Database-based sessions would require additional queries
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"message": "Database sessions not implemented yet"})
}

// handleGetSystemInfo returns system information
func (s *Server) handleGetSystemInfo(w http.ResponseWriter, r *http.Request) {
	info := map[string]interface{}{
		"version":     "1.0.0", // Would come from build info
		"uptime":      time.Since(s.startTime).Seconds(),
		"fingerprint": s.fingerprint,
		"database":    s.db != nil,
		"auth0":       s.auth0 != nil,
		"dashboard":   s.config.Dashboard.Enabled,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

// Extract entity ID and type from URL path like /api/capture/{tunnels|listeners}/{entityID}/stream
func getEntityIDFromPath(path string) (string, string) {
	parts := splitPath(path)
	if len(parts) >= 4 && parts[1] == "capture" {
		entityType := parts[2] // "tunnels" or "listeners"
		entityID := parts[3]
		if entityType == "tunnels" || entityType == "listeners" {
			return entityID, entityType[:len(entityType)-1] // remove 's' to get "tunnel" or "listener"
		}
	}
	return "", ""
}

// Helper function to extract tunnel ID from URL path (backward compatibility)
func getTunnelIDFromPath(path string) string {
	// Extract tunnel ID from supported paths:
	// - /api/tunnels/{id}
	// - /api/capture/tunnels/{id}/... (e.g., /stream, /recent)
	parts := splitPath(path)
	if len(parts) >= 3 && parts[1] == "tunnels" {
		return parts[2]
	}
	if len(parts) >= 4 && parts[1] == "capture" && parts[2] == "tunnels" {
		return parts[3]
	}
	return ""
}

// Helper function to split URL path
func splitPath(path string) []string {
	var parts []string
	current := ""
	for _, char := range path {
		if char == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// filterTunnelsByUser filters tunnels based on user permissions
func (s *Server) filterTunnelsByUser(r *http.Request, tunnels []*database.Tunnel) []*database.Tunnel {
	username := s.getCurrentUsername(r)
	if username == "" {
		return []*database.Tunnel{} // No access if not authenticated
	}

	// Check if user is admin
	isAdmin := s.isUserAdmin(r.Context())
	if isAdmin {
		return tunnels // Admins see all tunnels
	}

	// Non-admin users only see their own tunnels
	var filteredTunnels []*database.Tunnel
	for _, tunnel := range tunnels {
		if tunnel.Username == username {
			filteredTunnels = append(filteredTunnels, tunnel)
		}
	}

	return filteredTunnels
}

// userHasEntityAccess checks if user has access to a tunnel or listener
func (s *Server) userHasEntityAccess(r *http.Request, entityID, entityType string) bool {
	if entityType == "tunnel" {
		return s.userHasTunnelAccess(r, entityID)
	} else if entityType == "listener" {
		return s.userHasListenerAccess(r, entityID)
	}
	return false
}

// userHasListenerAccess checks if user has access to a specific listener
func (s *Server) userHasListenerAccess(r *http.Request, listenerID string) bool {
	if s.db == nil {
		return false
	}

	username := s.getCurrentUsername(r)
	if username == "" {
		return false
	}

	// Check if user is admin
	if s.isUserAdmin(r.Context()) {
		return true
	}

	// Check if user owns the listener
	listener, err := s.db.GetListener(listenerID)
	if err != nil {
		return false
	}

	return listener.Username == username
}
