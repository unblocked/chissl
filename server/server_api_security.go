package chserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/NextChapterSoftware/chissl/share/database"
)

// handleGetLoginBackoffSettings returns current login backoff configuration
func (s *Server) handleGetLoginBackoffSettings(w http.ResponseWriter, r *http.Request) {
	resp := s.config.Security.LoginBackoff
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleUpdateLoginBackoffSettings updates login backoff configuration at runtime (admin only)
func (s *Server) handleUpdateLoginBackoffSettings(w http.ResponseWriter, r *http.Request) {
	if !s.isUserAdmin(r.Context()) {
		http.Error(w, "Admin privileges required", http.StatusForbidden)
		return
	}
	var req LoginBackoffConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Basic validation with sensible bounds
	if req.BaseDelayMS < 0 || req.BaseDelayMS > 60000 {
		http.Error(w, "base_delay_ms must be between 0 and 60000", http.StatusBadRequest)
		return
	}
	if req.MaxDelayMS < 0 || req.MaxDelayMS > 600000 {
		http.Error(w, "max_delay_ms must be between 0 and 600000", http.StatusBadRequest)
		return
	}
	if req.MaxExponent < 0 || req.MaxExponent > 10 {
		http.Error(w, "max_exponent must be between 0 and 10", http.StatusBadRequest)
		return
	}
	if req.HardLockFailures < 0 || req.HardLockFailures > 50 {
		http.Error(w, "hard_lock_failures must be between 0 and 50", http.StatusBadRequest)
		return
	}
	if req.HardLockMinutes < 0 || req.HardLockMinutes > 1440 {
		http.Error(w, "hard_lock_minutes must be between 0 and 1440", http.StatusBadRequest)
		return
	}

	// Apply updates
	s.config.Security.LoginBackoff = req

	// Persist to DB if available
	if s.db != nil {
		// store as scalar keys for robust startup load
		_ = s.db.SetSettingString("security_backoff_base_ms", fmt.Sprintf("%d", req.BaseDelayMS))
		_ = s.db.SetSettingString("security_backoff_max_ms", fmt.Sprintf("%d", req.MaxDelayMS))
		_ = s.db.SetSettingString("security_backoff_max_exp", fmt.Sprintf("%d", req.MaxExponent))
		_ = s.db.SetSettingString("security_backoff_hard_fail", fmt.Sprintf("%d", req.HardLockFailures))
		_ = s.db.SetSettingString("security_backoff_hard_min", fmt.Sprintf("%d", req.HardLockMinutes))
		_ = s.db.SetSettingString("security_backoff_per_ip", map[bool]string{true: "1", false: "0"}[req.PerIPEnabled])
	}

	// Respond
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":        "ok",
		"login_backoff": req,
	})
}

// GET /api/settings/ip-rate
func (s *Server) handleGetIPRateSettings(w http.ResponseWriter, r *http.Request) {
	resp := s.config.Security.IPRate
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// PUT /api/settings/ip-rate {"max_per_minute": int, "ban_minutes": int}
func (s *Server) handleUpdateIPRateSettings(w http.ResponseWriter, r *http.Request) {
	if !s.isUserAdmin(r.Context()) {
		http.Error(w, "Admin privileges required", http.StatusForbidden)
		return
	}
	var req IPRateLimitConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	// validation
	if req.MaxPerMinute < 0 || req.MaxPerMinute > 100000 {
		http.Error(w, "max_per_minute must be between 0 and 100000", http.StatusBadRequest)
		return
	}
	if req.BanMinutes < 0 || req.BanMinutes > 10080 { // up to 7 days
		http.Error(w, "ban_minutes must be between 0 and 10080", http.StatusBadRequest)
		return
	}
	// apply
	s.config.Security.IPRate = req
	// persist
	if s.db != nil {
		_ = s.db.SetSettingString("security_ip_max_per_min", fmt.Sprintf("%d", req.MaxPerMinute))
		_ = s.db.SetSettingString("security_ip_ban_minutes", fmt.Sprintf("%d", req.BanMinutes))

	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"status": "ok", "ip_rate": req})

}

// GET /api/settings/session
func (s *Server) handleGetSessionSettings(w http.ResponseWriter, r *http.Request) {
	resp := map[string]int{"session_ttl_minutes": s.config.Security.SessionTTLMinutes}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// PUT /api/settings/session {session_ttl_minutes:int}
func (s *Server) handleUpdateSessionSettings(w http.ResponseWriter, r *http.Request) {
	if !s.isUserAdmin(r.Context()) {
		http.Error(w, "Admin privileges required", http.StatusForbidden)
		return
	}
	var req struct {
		SessionTTLMinutes int `json:"session_ttl_minutes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if req.SessionTTLMinutes <= 0 || req.SessionTTLMinutes > 10080 { // up to 7 days
		http.Error(w, "session_ttl_minutes must be between 1 and 10080", http.StatusBadRequest)
		return
	}
	s.config.Security.SessionTTLMinutes = req.SessionTTLMinutes
	if s.db != nil {
		_ = s.db.SetSettingString("security_session_ttl_minutes", fmt.Sprintf("%d", req.SessionTTLMinutes))
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"status": "ok", "session_ttl_minutes": req.SessionTTLMinutes})

}

// GET /api/security/events?limit=20
func (s *Server) handleGetSecurityEvents(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if q := r.URL.Query().Get("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}
	w.Header().Set("Content-Type", "application/json")
	// Prefer DB-backed history if available
	if s.db != nil {
		rows, err := s.db.ListSecurityEvents(limit)
		if err == nil {
			json.NewEncoder(w).Encode(rows)
			return
		}
	}
	// Fallback to in-memory buffer
	s.securityMu.Lock()
	n := len(s.securityEvents)
	start := 0
	if n > limit {
		start = n - limit
	}
	copySlice := append([]SecurityEvent(nil), s.securityEvents[start:]...)
	s.securityMu.Unlock()
	for i, j := 0, len(copySlice)-1; i < j; i, j = i+1, j-1 {
		copySlice[i], copySlice[j] = copySlice[j], copySlice[i]
	}
	json.NewEncoder(w).Encode(copySlice)
}

// Webhook helpers
func parseWebhookIDFromPath(path string) (int, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 4 { // api security webhooks {id}
		return 0, fmt.Errorf("missing id")
	}
	return strconv.Atoi(parts[3])
}

// GET /api/security/webhooks
func (s *Server) handleListSecurityWebhooks(w http.ResponseWriter, r *http.Request) {
	if !s.isUserAdmin(r.Context()) {
		http.Error(w, "Admin privileges required", http.StatusForbidden)
		return
	}
	onlyEnabled := false
	if r.URL.Query().Get("enabled") == "1" {
		onlyEnabled = true
	}
	if s.db == nil {
		json.NewEncoder(w).Encode([]any{})
		return
	}
	rows, err := s.db.ListSecurityWebhooks(onlyEnabled)
	if err != nil {
		http.Error(w, "Failed to list webhooks", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rows)
}

// POST /api/security/webhooks {url,type,enabled,description}
func (s *Server) handleCreateSecurityWebhook(w http.ResponseWriter, r *http.Request) {
	if !s.isUserAdmin(r.Context()) {
		http.Error(w, "Admin privileges required", http.StatusForbidden)
		return
	}
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}
	var req struct {
		URL         string `json:"url"`
		Type        string `json:"type"`
		Enabled     bool   `json:"enabled"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if req.URL == "" || (req.Type != "slack" && req.Type != "json") {
		http.Error(w, "url and type (slack|json) are required", http.StatusBadRequest)
		return
	}
	wbh := &database.SecurityWebhook{URL: req.URL, Type: req.Type, Enabled: req.Enabled, Description: req.Description}
	if err := s.db.CreateSecurityWebhook(wbh); err != nil {
		http.Error(w, "Failed to create webhook", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(wbh)
}

// PUT /api/security/webhooks/{id}
func (s *Server) handleUpdateSecurityWebhook(w http.ResponseWriter, r *http.Request) {
	if !s.isUserAdmin(r.Context()) {
		http.Error(w, "Admin privileges required", http.StatusForbidden)
		return
	}
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}
	id, err := parseWebhookIDFromPath(r.URL.Path)
	if err != nil {
		http.Error(w, "Invalid webhook id", http.StatusBadRequest)
		return
	}
	var req database.SecurityWebhook
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	// merge with existing so partial updates are allowed
	cur, err := s.db.GetSecurityWebhook(id)
	if err != nil {
		http.Error(w, "Webhook not found", http.StatusNotFound)
		return
	}
	if req.URL == "" {
		req.URL = cur.URL
	}
	if req.Type == "" {
		req.Type = cur.Type
	}
	if req.Description == "" {
		req.Description = cur.Description
	}
	// Enabled boolean: if not specified, zero-value false could unintentionally disable, so we detect via string? For simplicity, if URL/Type provided, we keep Enabled as-is; otherwise flip if provided.
	// Here, keep Enabled as provided; UI should send explicit value on toggle.
	if req.Type != "slack" && req.Type != "json" {
		http.Error(w, "type must be slack or json", http.StatusBadRequest)
		return
	}
	req.ID = id
	if err := s.db.UpdateSecurityWebhook(&req); err != nil {
		http.Error(w, "Failed to update webhook", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/security/webhooks/{id}/test
func (s *Server) handleTestSecurityWebhook(w http.ResponseWriter, r *http.Request) {
	if !s.isUserAdmin(r.Context()) {
		http.Error(w, "Admin privileges required", http.StatusForbidden)
		return
	}
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}
	id, err := parseWebhookIDFromPath(r.URL.Path)
	if err != nil {
		http.Error(w, "Invalid webhook id", http.StatusBadRequest)
		return
	}
	wh, err := s.db.GetSecurityWebhook(id)
	if err != nil {
		http.Error(w, "Webhook not found", http.StatusNotFound)
		return
	}
	// Send a test event
	ev := SecurityEvent{Type: "test", Severity: "info", Message: "Test security webhook", At: time.Now()}
	go s.sendSecurityEventToWebhook(wh.URL, wh.Type, ev)
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /api/security/webhooks/{id}
func (s *Server) handleDeleteSecurityWebhook(w http.ResponseWriter, r *http.Request) {
	if !s.isUserAdmin(r.Context()) {
		http.Error(w, "Admin privileges required", http.StatusForbidden)
		return
	}
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}
	id, err := parseWebhookIDFromPath(r.URL.Path)
	if err != nil {
		http.Error(w, "Invalid webhook id", http.StatusBadRequest)
		return
	}
	if err := s.db.DeleteSecurityWebhook(id); err != nil {
		http.Error(w, "Failed to delete webhook", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
