package chserver

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/NextChapterSoftware/chissl/share/database"
)

// handleListPortReservations returns all port reservations (admin only)
func (s *Server) handleListPortReservations(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}

	reservations, err := s.db.ListPortReservations()
	if err != nil {
		s.Debugf("Failed to list port reservations: %v", err)
		http.Error(w, "Failed to list port reservations", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(reservations)
}

// handleListUserPortReservations returns port reservations for the current user
func (s *Server) handleListUserPortReservations(w http.ResponseWriter, r *http.Request) {
	username := s.getUsernameFromContext(r.Context())
	if username == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}

	reservations, err := s.db.ListUserPortReservations(username)
	if err != nil {
		s.Debugf("Failed to list user port reservations: %v", err)
		http.Error(w, "Failed to list port reservations", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(reservations)
}

// handleCreatePortReservation creates a new port reservation (admin only)
func (s *Server) handleCreatePortReservation(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Username    string `json:"username"`
		StartPort   int    `json:"start_port"`
		EndPort     int    `json:"end_port"`
		Description string `json:"description"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate input
	if req.Username == "" {
		http.Error(w, "Username is required", http.StatusBadRequest)
		return
	}
	if req.StartPort <= 0 || req.EndPort <= 0 {
		http.Error(w, "Invalid port range", http.StatusBadRequest)
		return
	}
	if req.StartPort > req.EndPort {
		http.Error(w, "Start port must be less than or equal to end port", http.StatusBadRequest)
		return
	}

	// Check if user exists
	_, err := s.db.GetUser(req.Username)
	if err != nil {
		http.Error(w, "User not found", http.StatusBadRequest)
		return
	}

	// Check for overlapping reservations
	existingReservations, err := s.db.ListPortReservations()
	if err != nil {
		http.Error(w, "Failed to check existing reservations", http.StatusInternalServerError)
		return
	}

	for _, existing := range existingReservations {
		// Check for overlap: new range overlaps if start <= existing.end && end >= existing.start
		if req.StartPort <= existing.EndPort && req.EndPort >= existing.StartPort {
			http.Error(w, "Port range overlaps with existing reservation", http.StatusConflict)
			return
		}
	}

	reservation := &database.PortReservation{
		Username:    req.Username,
		StartPort:   req.StartPort,
		EndPort:     req.EndPort,
		Description: strings.TrimSpace(req.Description),
	}

	if err := s.db.CreatePortReservation(reservation); err != nil {
		s.Debugf("Failed to create port reservation: %v", err)
		http.Error(w, "Failed to create port reservation", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(reservation)
}

// handleDeletePortReservation deletes a port reservation (admin only)
func (s *Server) handleDeletePortReservation(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}

	// Extract reservation ID from URL path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	reservationID := parts[3]

	if err := s.db.DeletePortReservation(reservationID); err != nil {
		s.Debugf("Failed to delete port reservation: %v", err)
		http.Error(w, "Failed to delete port reservation", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleGetReservedPortsThreshold gets the reserved ports threshold (admin only)
func (s *Server) handleGetReservedPortsThreshold(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}

	threshold, err := s.db.GetReservedPortsThreshold()
	if err != nil {
		s.Debugf("Failed to get reserved ports threshold: %v", err)
		http.Error(w, "Failed to get threshold", http.StatusInternalServerError)
		return
	}

	response := map[string]int{"threshold": threshold}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleSetReservedPortsThreshold sets the reserved ports threshold (admin only)
func (s *Server) handleSetReservedPortsThreshold(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Threshold int `json:"threshold"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Threshold < 1 || req.Threshold > 65535 {
		http.Error(w, "Threshold must be between 1 and 65535", http.StatusBadRequest)
		return
	}

	if err := s.db.SetReservedPortsThreshold(req.Threshold); err != nil {
		s.Debugf("Failed to set reserved ports threshold: %v", err)
		http.Error(w, "Failed to set threshold", http.StatusInternalServerError)
		return
	}

	response := map[string]string{"status": "success"}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// isPortAvailableForUser checks if a user can use a specific port
func (s *Server) isPortAvailableForUser(port int, username string) (bool, string) {
	if s.db == nil {
		return true, "" // No restrictions without database
	}

	reserved, err := s.db.IsPortReserved(port, username)
	if err != nil {
		s.Debugf("Failed to check port reservation: %v", err)
		return false, "Failed to check port availability"
	}

	if reserved {
		threshold, _ := s.db.GetReservedPortsThreshold()
		return false, "Port " + strconv.Itoa(port) + " is reserved. Use ports >= " + strconv.Itoa(threshold) + " or contact admin for port assignment."
	}

	return true, ""
}
