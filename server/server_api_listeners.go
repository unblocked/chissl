package chserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/NextChapterSoftware/chissl/server/capture"
	"github.com/NextChapterSoftware/chissl/share/database"
	"github.com/NextChapterSoftware/chissl/share/tunnel"
)

// handleListListeners returns all listeners
func (s *Server) handleListListeners(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}

	listeners, err := s.db.ListListeners()
	if err != nil {
		s.Debugf("Failed to list listeners: %v", err)
		http.Error(w, "Failed to list listeners", http.StatusInternalServerError)
		return
	}

	// Filter listeners by user permissions
	filteredListeners := s.filterListenersByUser(r, listeners)

	// Enhance listeners with AI information
	enhancedListeners := s.enhanceListenersWithAIInfo(filteredListeners)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(enhancedListeners)
}

// handleCreateListener creates a new listener
func (s *Server) handleCreateListener(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Name      string `json:"name"` // human-friendly name
		Port      int    `json:"port"`
		Mode      string `json:"mode"` // "sink" or "proxy"
		TargetURL string `json:"target_url,omitempty"`
		Response  string `json:"response,omitempty"`
		UseTLS    bool   `json:"use_tls"` // whether to use TLS
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate input
	if req.Port <= 0 || req.Port > 65535 {
		http.Error(w, "Invalid port number", http.StatusBadRequest)
		return
	}

	if req.Mode != "sink" && req.Mode != "proxy" {
		http.Error(w, "Mode must be 'sink' or 'proxy'", http.StatusBadRequest)
		return
	}

	if req.Mode == "proxy" && req.TargetURL == "" {
		http.Error(w, "Target URL required for proxy mode", http.StatusBadRequest)
		return
	}

	// Get username from auth
	username := s.getAuthenticatedUsername(r)
	if username == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// Check if user can use this port
	if available, errMsg := s.isPortAvailableForUser(req.Port, username); !available {
		http.Error(w, errMsg, http.StatusForbidden)
		return
	}

	// Create listener
	listener := &database.Listener{
		ID:        fmt.Sprintf("listener-%d-%d", req.Port, time.Now().UnixNano()),
		Name:      strings.TrimSpace(req.Name), // Trim whitespace
		Username:  username,
		Port:      req.Port,
		Mode:      req.Mode,
		TargetURL: strings.TrimSpace(req.TargetURL), // Trim whitespace
		Response:  strings.TrimSpace(req.Response),  // Trim whitespace
		UseTLS:    req.UseTLS,
		Status:    "closed", // Will be set to "open" when started
	}

	if err := s.db.CreateListener(listener); err != nil {
		s.Debugf("Failed to create listener: %v", err)
		if strings.Contains(err.Error(), "UNIQUE constraint failed") || strings.Contains(err.Error(), "duplicate key") {
			http.Error(w, "Port already in use", http.StatusConflict)
			return
		}
		http.Error(w, "Failed to create listener", http.StatusInternalServerError)
		return
	}

	// Start the listener
	if s.listeners != nil {
		var tapFactory tunnel.TapFactory
		if s.capture != nil {
			tapFactory = capture.NewTapFactory(s.capture, listener.ID, username, 500)
		}

		if err := s.listeners.StartListener(listener, tapFactory); err != nil {
			s.Debugf("Failed to start listener: %v", err)
			// Update status to error
			listener.Status = "error"
			_ = s.db.UpdateListener(listener)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(listener)
}

// handleGetListener returns a specific listener
func (s *Server) handleGetListener(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}

	// Extract listener ID from path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		http.Error(w, "Invalid listener ID", http.StatusBadRequest)
		return
	}
	listenerID := parts[3]

	listener, err := s.db.GetListener(listenerID)
	if err != nil {
		s.Debugf("Failed to get listener: %v", err)
		http.Error(w, "Listener not found", http.StatusNotFound)
		return
	}

	// Check user permissions
	if !s.canUserAccessListener(r, listener) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(listener)
}

// handleUpdateListener updates a listener
func (s *Server) handleUpdateListener(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}

	// Extract listener ID from path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		http.Error(w, "Invalid listener ID", http.StatusBadRequest)
		return
	}
	listenerID := parts[3]

	var req struct {
		Name      string `json:"name,omitempty"` // human-friendly name
		TargetURL string `json:"target_url,omitempty"`
		Response  string `json:"response,omitempty"`
		Status    string `json:"status,omitempty"`
		UseTLS    *bool  `json:"use_tls,omitempty"` // pointer to distinguish between false and not set
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Get existing listener
	listener, err := s.db.GetListener(listenerID)
	if err != nil {
		http.Error(w, "Listener not found", http.StatusNotFound)
		return
	}

	// Check user permissions
	if !s.canUserAccessListener(r, listener) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Update fields
	if req.Name != "" {
		listener.Name = strings.TrimSpace(req.Name) // Trim whitespace
	}
	if req.TargetURL != "" {
		listener.TargetURL = strings.TrimSpace(req.TargetURL) // Trim whitespace
	}
	if req.Response != "" {
		listener.Response = strings.TrimSpace(req.Response) // Trim whitespace
	}
	if req.Status != "" {
		listener.Status = req.Status
	}
	if req.UseTLS != nil {
		listener.UseTLS = *req.UseTLS
	}

	if err := s.db.UpdateListener(listener); err != nil {
		s.Debugf("Failed to update listener: %v", err)
		http.Error(w, "Failed to update listener", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(listener)
}

// handleDeleteListener deletes a listener
func (s *Server) handleDeleteListener(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}

	// Extract listener ID from path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		http.Error(w, "Invalid listener ID", http.StatusBadRequest)
		return
	}
	listenerID := parts[3]

	// Get existing listener to check permissions
	listener, err := s.db.GetListener(listenerID)
	if err != nil {
		http.Error(w, "Listener not found", http.StatusNotFound)
		return
	}

	// Check user permissions
	if !s.canUserAccessListener(r, listener) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Stop the listener if it's running
	if s.listeners != nil {
		if err := s.listeners.StopListener(listenerID); err != nil {
			s.Debugf("Failed to stop listener: %v", err)
		}
	}

	// Delete from database
	if err := s.db.DeleteListener(listenerID); err != nil {
		s.Debugf("Failed to delete listener: %v", err)
		http.Error(w, "Failed to delete listener", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// filterListenersByUser filters listeners based on user permissions
func (s *Server) filterListenersByUser(r *http.Request, listeners []*database.Listener) []*database.Listener {
	username := s.getCurrentUsername(r)
	if username == "" {
		return []*database.Listener{} // No access if not authenticated
	}

	// Check if user is admin
	isAdmin := s.isUserAdmin(r.Context())
	if isAdmin {
		return listeners // Admins see all listeners
	}

	// Non-admin users only see their own listeners
	var filteredListeners []*database.Listener
	for _, listener := range listeners {
		if listener.Username == username {
			filteredListeners = append(filteredListeners, listener)
		}
	}

	return filteredListeners
}

// canUserAccessListener checks if the current user can access a specific listener
func (s *Server) canUserAccessListener(r *http.Request, listener *database.Listener) bool {
	username := s.getCurrentUsername(r)
	if username == "" {
		return false // No access if not authenticated
	}

	// Check if user is admin
	isAdmin := s.isUserAdmin(r.Context())
	if isAdmin {
		return true // Admins can access all listeners
	}

	// Non-admin users can only access their own listeners
	return listener.Username == username
}

// getAuthenticatedUsername extracts username from request authentication
func (s *Server) getAuthenticatedUsername(r *http.Request) string {
	// Check session cookie first
	if cookie, err := r.Cookie("chissl_session"); err == nil && cookie.Value != "" {
		return cookie.Value
	}

	// Check basic auth
	username, _, ok := s.decodeBasicAuthHeader(r.Header)
	if ok {
		return username
	}

	return ""
}

// EnhancedListener represents a listener with AI information
type EnhancedListener struct {
	*database.Listener
	Active             bool   `json:"active"` // Computed from Status field
	AIGenerationStatus string `json:"ai_generation_status,omitempty"`
	AIProviderName     string `json:"ai_provider_name,omitempty"`
	AIProviderType     string `json:"ai_provider_type,omitempty"`
	AIGenerationError  string `json:"ai_generation_error,omitempty"`
}

// enhanceListenersWithAIInfo adds AI information to listeners
func (s *Server) enhanceListenersWithAIInfo(listeners []*database.Listener) []*EnhancedListener {
	enhanced := make([]*EnhancedListener, len(listeners))

	for i, listener := range listeners {
		enhancedListener := &EnhancedListener{
			Listener: listener,
			Active:   listener.Status == "open", // Convert status to boolean
		}

		// Check if this is an AI listener
		if listener.Mode == "ai-mock" {
			if aiListener, err := s.db.GetAIListenerByListenerID(listener.ID); err == nil && aiListener != nil {
				enhancedListener.AIGenerationStatus = aiListener.GenerationStatus
				enhancedListener.AIGenerationError = aiListener.GenerationError

				// Get AI provider information
				if provider, err := s.db.GetAIProvider(aiListener.AIProviderID); err == nil {
					enhancedListener.AIProviderName = provider.Name
					enhancedListener.AIProviderType = provider.ProviderType
				}
			}
		}

		enhanced[i] = enhancedListener
	}

	return enhanced
}
