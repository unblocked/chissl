package chserver

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/NextChapterSoftware/chissl/share/auth"
	"github.com/NextChapterSoftware/chissl/share/database"
)

// SSOConfigRequest represents SSO configuration request
type SSOConfigRequest struct {
	Provider database.SSOProvider `json:"provider"`
	Enabled  bool                 `json:"enabled"`
	Config   json.RawMessage      `json:"config"`
}

// SSOConfigResponse represents SSO configuration response
type SSOConfigResponse struct {
	*database.SSOConfig
	TestURL string `json:"test_url,omitempty"`
}

// handleListSSOConfigs returns all SSO configurations (admin only)
func (s *Server) handleListSSOConfigs(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}

	configs, err := s.db.ListSSOConfigs()
	if err != nil {
		s.Debugf("Failed to list SSO configs: %v", err)
		http.Error(w, "Failed to list SSO configurations", http.StatusInternalServerError)
		return
	}

	// Convert to response format
	var responses []SSOConfigResponse
	for _, config := range configs {
		response := SSOConfigResponse{
			SSOConfig: config,
		}

		// Add test URL for SCIM providers
		if config.Provider == database.SSOProviderSCIM || config.Provider == database.SSOProviderOkta {
			if _, ok := config.Config.(database.SCIMConfig); ok {
				response.TestURL = "/auth/scim/login"
			}
		}

		responses = append(responses, response)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responses)
}

// handleGetSSOConfig returns SSO configuration for a specific provider (admin only)
func (s *Server) handleGetSSOConfig(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}

	// Extract provider from URL path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	provider := database.SSOProvider(parts[4])

	config, err := s.db.GetSSOConfig(provider)
	if err != nil {
		s.Debugf("Failed to get SSO config for %s: %v", provider, err)
		http.Error(w, "Failed to get SSO configuration", http.StatusInternalServerError)
		return
	}

	if config == nil {
		http.Error(w, "SSO configuration not found", http.StatusNotFound)
		return
	}

	response := SSOConfigResponse{
		SSOConfig: config,
	}

	// Add test URL for SCIM providers
	if config.Provider == database.SSOProviderSCIM || config.Provider == database.SSOProviderOkta {
		response.TestURL = "/auth/scim/login"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleCreateSSOConfig creates or updates SSO configuration (admin only)
func (s *Server) handleCreateSSOConfig(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}

	var req SSOConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate provider
	if req.Provider == "" {
		http.Error(w, "Provider is required", http.StatusBadRequest)
		return
	}

	// Parse config based on provider
	var config interface{}
	switch req.Provider {
	case database.SSOProviderSCIM, database.SSOProviderOkta:
		var scimConfig database.SCIMConfig
		if err := json.Unmarshal(req.Config, &scimConfig); err != nil {
			http.Error(w, "Invalid SCIM configuration", http.StatusBadRequest)
			return
		}

		// Validate required fields
		if scimConfig.ClientID == "" || scimConfig.ClientSecret == "" || scimConfig.AuthURL == "" || scimConfig.TokenURL == "" {
			http.Error(w, "Missing required SCIM configuration fields", http.StatusBadRequest)
			return
		}

		config = scimConfig

	case database.SSOProviderAuth0:
		var auth0Config database.Auth0Config
		if err := json.Unmarshal(req.Config, &auth0Config); err != nil {
			http.Error(w, "Invalid Auth0 configuration", http.StatusBadRequest)
			return
		}

		// Validate required fields
		if auth0Config.Domain == "" || auth0Config.ClientID == "" || auth0Config.ClientSecret == "" {
			http.Error(w, "Missing required Auth0 configuration fields", http.StatusBadRequest)
			return
		}

		config = auth0Config

	default:
		http.Error(w, "Unsupported provider", http.StatusBadRequest)
		return
	}

	// Create SSO config
	ssoConfig := &database.SSOConfig{
		Provider: req.Provider,
		Enabled:  req.Enabled,
		Config:   config,
	}

	if err := s.db.CreateSSOConfig(ssoConfig); err != nil {
		s.Debugf("Failed to create SSO config: %v", err)
		http.Error(w, "Failed to save SSO configuration", http.StatusInternalServerError)
		return
	}

	// Reload SSO middleware if this config was enabled
	if req.Enabled {
		s.reloadSSOMiddleware(req.Provider)
	}

	response := SSOConfigResponse{
		SSOConfig: ssoConfig,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// handleDeleteSSOConfig deletes SSO configuration (admin only)
func (s *Server) handleDeleteSSOConfig(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}

	// Extract provider from URL path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	provider := database.SSOProvider(parts[4])

	if err := s.db.DeleteSSOConfig(provider); err != nil {
		s.Debugf("Failed to delete SSO config for %s: %v", provider, err)
		http.Error(w, "Failed to delete SSO configuration", http.StatusInternalServerError)
		return
	}

	// Disable SSO middleware for this provider
	s.disableSSOMiddleware(provider)

	w.WriteHeader(http.StatusNoContent)
}

// handleTestSSOConfig tests SSO configuration (admin only)
func (s *Server) handleTestSSOConfig(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}

	// Extract provider from URL path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 6 {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	provider := database.SSOProvider(parts[4])

	config, err := s.db.GetSSOConfig(provider)
	if err != nil || config == nil {
		http.Error(w, "SSO configuration not found", http.StatusNotFound)
		return
	}

	if !config.Enabled {
		http.Error(w, "SSO configuration is disabled", http.StatusBadRequest)
		return
	}

	// For SCIM providers, redirect to auth URL
	if config.Provider == database.SSOProviderSCIM || config.Provider == database.SSOProviderOkta {
		if _, ok := config.Config.(database.SCIMConfig); ok {
			// Redirect to SCIM login
			http.Redirect(w, r, "/auth/scim/login", http.StatusFound)
			return
		}
	}

	http.Error(w, "Test not supported for this provider", http.StatusBadRequest)
}

// reloadSSOMiddleware reloads SSO middleware for a provider
func (s *Server) reloadSSOMiddleware(provider database.SSOProvider) {
	// This would reload the middleware - implementation depends on your architecture
	s.Infof("Reloading SSO middleware for provider: %s", provider)
	// TODO: Implement middleware reloading
}

// disableSSOMiddleware disables SSO middleware for a provider
func (s *Server) disableSSOMiddleware(provider database.SSOProvider) {
	// This would disable the middleware - implementation depends on your architecture
	s.Infof("Disabling SSO middleware for provider: %s", provider)
	// TODO: Implement middleware disabling
}

// handleListEnabledSSOConfigs returns only enabled SSO configurations (public endpoint for login page)
func (s *Server) handleListEnabledSSOConfigs(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]SSOConfigResponse{})
		return
	}

	configs, err := s.db.ListSSOConfigs()
	if err != nil {
		s.Debugf("Failed to list SSO configs: %v", err)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]SSOConfigResponse{})
		return
	}

	// Filter only enabled configs and remove sensitive information
	var responses []SSOConfigResponse
	for _, config := range configs {
		if config.Enabled {
			response := SSOConfigResponse{
				SSOConfig: &database.SSOConfig{
					ID:        config.ID,
					Provider:  config.Provider,
					Enabled:   config.Enabled,
					CreatedAt: config.CreatedAt,
					UpdatedAt: config.UpdatedAt,
					// Don't include Config to avoid exposing secrets
				},
			}
			responses = append(responses, response)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responses)
}

// handleSCIMLogin handles SCIM OAuth login initiation
func (s *Server) handleSCIMLogin(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}

	// Get enabled SCIM or Okta configuration
	var config *database.SSOConfig
	var err error

	// Try Okta first, then SCIM
	config, err = s.db.GetSSOConfig(database.SSOProviderOkta)
	if err != nil || config == nil || !config.Enabled {
		config, err = s.db.GetSSOConfig(database.SSOProviderSCIM)
		if err != nil || config == nil || !config.Enabled {
			http.Error(w, "No enabled SCIM/Okta configuration found", http.StatusNotFound)
			return
		}
	}

	scimConfig, ok := config.Config.(database.SCIMConfig)
	if !ok {
		http.Error(w, "Invalid SCIM configuration", http.StatusInternalServerError)
		return
	}

	// Create SCIM middleware and handle login
	scimMiddleware := auth.NewSCIMMiddleware(s.db, &scimConfig)
	scimMiddleware.HandleLogin(w, r)
}

// handleSCIMCallback handles SCIM OAuth callback
func (s *Server) handleSCIMCallback(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}

	// Get enabled SCIM or Okta configuration
	var config *database.SSOConfig
	var err error

	// Try Okta first, then SCIM
	config, err = s.db.GetSSOConfig(database.SSOProviderOkta)
	if err != nil || config == nil || !config.Enabled {
		config, err = s.db.GetSSOConfig(database.SSOProviderSCIM)
		if err != nil || config == nil || !config.Enabled {
			http.Error(w, "No enabled SCIM/Okta configuration found", http.StatusNotFound)
			return
		}
	}

	scimConfig, ok := config.Config.(database.SCIMConfig)
	if !ok {
		http.Error(w, "Invalid SCIM configuration", http.StatusInternalServerError)
		return
	}

	// Create SCIM middleware and handle callback
	scimMiddleware := auth.NewSCIMMiddleware(s.db, &scimConfig)
	scimMiddleware.HandleCallback(w, r)
}

// handleAuth0Login handles Auth0 login initiation
func (s *Server) handleAuth0Login(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}

	config, err := s.db.GetSSOConfig(database.SSOProviderAuth0)
	if err != nil || config == nil || !config.Enabled {
		http.Error(w, "Auth0 configuration not found or disabled", http.StatusNotFound)
		return
	}

	_, ok := config.Config.(database.Auth0Config)
	if !ok {
		http.Error(w, "Invalid Auth0 configuration", http.StatusInternalServerError)
		return
	}

	// TODO: Implement Auth0 login initiation
	http.Error(w, "Auth0 login not yet implemented", http.StatusNotImplemented)
}

// handleAuth0Callback handles Auth0 callback
func (s *Server) handleAuth0Callback(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}

	config, err := s.db.GetSSOConfig(database.SSOProviderAuth0)
	if err != nil || config == nil || !config.Enabled {
		http.Error(w, "Auth0 configuration not found or disabled", http.StatusNotFound)
		return
	}

	_, ok := config.Config.(database.Auth0Config)
	if !ok {
		http.Error(w, "Invalid Auth0 configuration", http.StatusInternalServerError)
		return
	}

	// TODO: Implement Auth0 callback handling
	http.Error(w, "Auth0 callback not yet implemented", http.StatusNotImplemented)
}

// handleListUserAuthSources returns user authentication sources (admin only)
func (s *Server) handleListUserAuthSources(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}

	sources, err := s.db.ListUserAuthSources()
	if err != nil {
		s.Debugf("Failed to list user auth sources: %v", err)
		http.Error(w, "Failed to list user authentication sources", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sources)
}
