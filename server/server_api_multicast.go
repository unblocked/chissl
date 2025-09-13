package chserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/NextChapterSoftware/chissl/share/database"
)

// POST /api/multicast-tunnels (admin)
func (s *Server) handleCreateMulticastTunnel(w http.ResponseWriter, r *http.Request) {
	if !s.isUserAdmin(r.Context()) {
		http.Error(w, "Admin privileges required", http.StatusForbidden)
		return
	}
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Name    string `json:"name"`
		Port    int    `json:"port"`
		Mode    string `json:"mode"`    // default webhook
		Enabled bool   `json:"enabled"` // start immediately if true
		Visible bool   `json:"visible"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if req.Port <= 0 || req.Port > 65535 {
		http.Error(w, "Invalid port number", http.StatusBadRequest)
		return
	}
	if req.Mode == "" {
		req.Mode = "webhook"
	}
	if req.Mode != "webhook" && req.Mode != "bidirectional" {
		http.Error(w, "Unsupported mode", http.StatusBadRequest)
		return
	}
	owner := s.getAuthenticatedUsername(r)
	if owner == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	id := fmt.Sprintf("multicast-%d-%d", req.Port, time.Now().UnixNano())
	mt := &database.MulticastTunnel{
		ID:      id,
		Name:    strings.TrimSpace(req.Name),
		Owner:   owner,
		Port:    req.Port,
		Mode:    req.Mode,
		Enabled: req.Enabled,
		Visible: req.Visible,
		UseTLS:  true,
		Status:  "closed",
	}
	if err := s.db.CreateMulticastTunnel(mt); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "duplicate key") {
			http.Error(w, "Port already in use", http.StatusConflict)
			return
		}
		http.Error(w, "Failed to create multicast tunnel", http.StatusInternalServerError)
		return
	}
	// Start if enabled
	if mt.Enabled && s.multicasts != nil {
		if err := s.multicasts.StartMulticast(mt); err != nil {
			mt.Status = "error"
			_ = s.db.UpdateMulticastTunnel(mt)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mt)
}

// GET /api/multicast-tunnels (admin)
func (s *Server) handleListMulticastTunnels(w http.ResponseWriter, r *http.Request) {
	if !s.isUserAdmin(r.Context()) {
		http.Error(w, "Admin privileges required", http.StatusForbidden)
		return
	}
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}
	mts, err := s.db.ListMulticastTunnels()
	if err != nil {
		http.Error(w, "Failed to list", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mts)
}

// GET /api/multicast-tunnels/public (any user)
func (s *Server) handleListPublicMulticastTunnels(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}
	// Must be authenticated (matches tunnels/listeners patterns)
	if s.getCurrentUsername(r) == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	mts, err := s.db.ListPublicMulticastTunnels()
	if err != nil {
		http.Error(w, "Failed to list", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mts)
}

// PUT /api/multicast-tunnels/{id}
func (s *Server) handleUpdateMulticastTunnel(w http.ResponseWriter, r *http.Request) {
	if !s.isUserAdmin(r.Context()) {
		http.Error(w, "Admin privileges required", http.StatusForbidden)
		return
	}
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}
	id := getIDFromPath(r.URL.Path)
	if id == "" {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	current, err := s.db.GetMulticastTunnel(id)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	var req struct {
		Name    *string `json:"name"`
		Port    *int    `json:"port"`
		Mode    *string `json:"mode"`
		Enabled *bool   `json:"enabled"`
		Visible *bool   `json:"visible"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	// Apply changes
	if req.Name != nil {
		current.Name = strings.TrimSpace(*req.Name)
	}
	if req.Port != nil {
		current.Port = *req.Port
	}
	modeChanged := false
	if req.Mode != nil {
		if *req.Mode != "webhook" && *req.Mode != "bidirectional" {
			http.Error(w, "Unsupported mode", http.StatusBadRequest)
			return
		}
		if current.Mode != *req.Mode {
			modeChanged = true
		}
		current.Mode = *req.Mode
	}
	if req.Visible != nil {
		current.Visible = *req.Visible
	}
	if req.Enabled != nil {
		current.Enabled = *req.Enabled
	}
	if err := s.db.UpdateMulticastTunnel(current); err != nil {
		http.Error(w, "Update failed", http.StatusInternalServerError)
		return
	}
	// Start/stop/restart runtime as needed
	if s.multicasts != nil {
		if req.Enabled != nil {
			if *req.Enabled {
				_ = s.multicasts.StartMulticast(current)
			} else {
				_ = s.multicasts.StopMulticast(current.ID)
			}
		} else if modeChanged && current.Enabled {
			// Restart to apply mode change
			_ = s.multicasts.StopMulticast(current.ID)
			_ = s.multicasts.StartMulticast(current)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(current)
}

// DELETE /api/multicast-tunnels/{id}
func (s *Server) handleDeleteMulticastTunnel(w http.ResponseWriter, r *http.Request) {
	if !s.isUserAdmin(r.Context()) {
		http.Error(w, "Admin privileges required", http.StatusForbidden)
		return
	}
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}
	id := getIDFromPath(r.URL.Path)
	if id == "" {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	// Stop if running
	if s.multicasts != nil {
		_ = s.multicasts.StopMulticast(id)
	}
	if err := s.db.DeleteMulticastTunnel(id); err != nil {
		http.Error(w, "Delete failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Helper to extract last path component
func getIDFromPath(path string) string {
	parts := splitPath(path)
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}
