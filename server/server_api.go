package chserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
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

	// Server-side visibility override: remove any IDs flagged as deleted
	if s.deletedTunnelIDs != nil {
		filtered := make([]*database.Tunnel, 0, len(tunnels))
		for _, t := range tunnels {
			if _, hidden := s.deletedTunnelIDs[t.ID]; hidden {
				continue
			}
			filtered = append(filtered, t)
		}
		tunnels = filtered
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

	// Soft-delete: mark as 'deleted' to preserve history but hide from listings
	// Always hard-delete to ensure inactive/stale tunnels are removed
	err := s.db.DeleteTunnel(tunnelID)
	if err != nil {
		http.Error(w, "Failed to delete tunnel", http.StatusInternalServerError)
		return
	}
	// Also hide at server-level in case another source reintroduces it
	if s.deletedTunnelIDs != nil {
		s.deletedTunnelIDs[tunnelID] = struct{}{}
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
		entityType := parts[2] // "tunnels", "listeners", or "multicast(-tunnels)"
		entityID := parts[3]
		switch entityType {
		case "tunnels", "listeners":
			return entityID, entityType[:len(entityType)-1]
		case "multicast", "multicasts", "multicast-tunnels":
			return entityID, "multicast"
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
	decode := func(s string) string {
		if u, err := url.PathUnescape(s); err == nil {
			return u
		}
		return s
	}
	if len(parts) >= 3 && parts[1] == "tunnels" {
		return decode(parts[2])
	}
	if len(parts) >= 4 && parts[1] == "capture" && parts[2] == "tunnels" {
		return decode(parts[3])
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
	} else if entityType == "multicast" {
		return s.userHasMulticastAccess(r, entityID)
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

// userHasMulticastAccess checks if user can see a multicast tunnel
func (s *Server) userHasMulticastAccess(r *http.Request, id string) bool {
	if s.db == nil {
		return false
	}
	username := s.getCurrentUsername(r)
	if username == "" {
		return false
	}
	// Admins have access to all
	if s.isUserAdmin(r.Context()) {
		return true
	}
	mt, err := s.db.GetMulticastTunnel(id)
	if err != nil {
		return false
	}
	// Visible to all users if visible flag set
	if mt.Visible {
		return true
	}
	// Otherwise only owner can access
	return mt.Owner == username
}

// ---- Settings: Experimental Features ----
// GET /api/settings/feature/ai-mock-visible
func (s *Server) handleGetAIMockVisibility(w http.ResponseWriter, r *http.Request) {
	// no DB → default hidden
	enabled := false
	if s.db != nil {
		if val, err := s.db.GetSettingBool("feature_ai_mock_visible", false); err == nil {
			enabled = val
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"enabled": enabled})
}

// PUT /api/settings/feature/ai-mock-visible {"enabled": bool}
func (s *Server) handleSetAIMockVisibility(w http.ResponseWriter, r *http.Request) {
	if !s.isUserAdmin(r.Context()) {
		http.Error(w, "Admin privileges required", http.StatusForbidden)
		return
	}
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	val := "0"
	if req.Enabled {
		val = "1"
	}
	if err := s.db.SetSettingString("feature_ai_mock_visible", val); err != nil {
		http.Error(w, "Failed to persist setting", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/tunnels/closed — delete current user's closed/inactive tunnels
// Admins may delete all by passing ?all=true
func (s *Server) handleDeleteClosedTunnels(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}
	q := r.URL.Query()
	all := q.Get("all") == "true"
	daysStr := q.Get("days")
	var cutoff *time.Time
	if daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
			c := time.Now().Add(-time.Duration(d) * 24 * time.Hour)
			cutoff = &c
		}
	}
	if all {
		if !s.isUserAdmin(r.Context()) {
			http.Error(w, "Admin privileges required", http.StatusForbidden)
			return
		}
		var err error
		if cutoff != nil {
			err = s.db.DeleteClosedTunnelsOlderThan(*cutoff)
		} else {
			err = s.db.DeleteClosedTunnels()
		}
		if err != nil {
			http.Error(w, "Failed to delete closed tunnels", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	// User-scoped delete
	username := s.getCurrentUsername(r)
	if username == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	var err error
	if cutoff != nil {
		err = s.db.DeleteClosedTunnelsByUserOlderThan(username, *cutoff)
	} else {
		err = s.db.DeleteClosedTunnelsByUser(username)
	}
	if err != nil {
		http.Error(w, "Failed to delete closed tunnels", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/sessions/closed — delete current user's closed/inactive sessions (base session rows)
func (s *Server) handleDeleteClosedSessions(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}
	username := s.getCurrentUsername(r)
	if username == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	q := r.URL.Query()
	daysStr := q.Get("days")
	var err error
	if daysStr != "" {
		if d, e := strconv.Atoi(daysStr); e == nil && d > 0 {
			cutoff := time.Now().Add(-time.Duration(d) * 24 * time.Hour)
			err = s.db.DeleteClosedSessionsByUserOlderThan(username, cutoff)
		} else {
			http.Error(w, "Invalid days parameter", http.StatusBadRequest)
			return
		}
	} else {
		err = s.db.DeleteClosedSessionsByUser(username)
	}
	if err != nil {
		http.Error(w, "Failed to delete closed sessions", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
