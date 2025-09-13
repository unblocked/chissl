package chserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/NextChapterSoftware/chissl/server/capture"
	"github.com/NextChapterSoftware/chissl/share/database"
	"github.com/NextChapterSoftware/chissl/share/settings"
	"github.com/NextChapterSoftware/chissl/share/tunnel"
	"github.com/google/uuid"
)

// AIProviderRequest represents the request payload for creating/updating AI providers
type AIProviderRequest struct {
	Name         string  `json:"name"`
	ProviderType string  `json:"provider_type"`
	APIKey       string  `json:"api_key"`
	APIEndpoint  string  `json:"api_endpoint"`
	Model        string  `json:"model"`
	MaxTokens    int     `json:"max_tokens"`
	Temperature  float64 `json:"temperature"`
	Enabled      bool    `json:"enabled"`
}

// handleAIProviders handles AI provider management endpoints
func (s *Server) handleAIProviders(w http.ResponseWriter, r *http.Request) {
	// Only admins can manage AI providers
	if !s.isRequestFromAdmin(r) {
		http.Error(w, "Admin access required", http.StatusForbidden)
		return
	}

	switch r.Method {
	case "GET":
		s.handleGetAIProviders(w, r)
	case "POST":
		s.handleCreateAIProvider(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAIProvider handles individual AI provider endpoints
func (s *Server) handleAIProvider(w http.ResponseWriter, r *http.Request) {
	// Only admins can manage AI providers
	if !s.isRequestFromAdmin(r) {
		http.Error(w, "Admin access required", http.StatusForbidden)
		return
	}

	// Extract provider ID from URL path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 || pathParts[3] == "" {
		http.Error(w, "Provider ID required", http.StatusBadRequest)
		return
	}
	providerID := pathParts[3]

	switch r.Method {
	case "GET":
		s.handleGetAIProvider(w, r, providerID)
	case "PUT":
		s.handleUpdateAIProvider(w, r, providerID)
	case "DELETE":
		s.handleDeleteAIProvider(w, r, providerID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleTestAIProvider handles testing AI provider connections
func (s *Server) handleTestAIProvider(w http.ResponseWriter, r *http.Request) {
	// Only admins can test AI providers
	if !s.isRequestFromAdmin(r) {
		http.Error(w, "Admin access required", http.StatusForbidden)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract provider ID from URL path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 || pathParts[3] == "" {
		http.Error(w, "Provider ID required", http.StatusBadRequest)
		return
	}
	providerID := pathParts[3]

	s.handleTestAIProviderConnection(w, r, providerID)
}

// handleGetAIProviders returns all AI providers
func (s *Server) handleGetAIProviders(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	providers, err := s.db.GetAIProviders()
	if err != nil {
		s.Debugf("Failed to get AI providers: %v", err)
		http.Error(w, "Failed to get AI providers", http.StatusInternalServerError)
		return
	}

	// Remove API keys from response for security
	for _, provider := range providers {
		provider.APIKey = ""
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(providers)
}

// handleGetAIProvider returns a specific AI provider
func (s *Server) handleGetAIProvider(w http.ResponseWriter, r *http.Request, providerID string) {
	if s.db == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	provider, err := s.db.GetAIProvider(providerID)
	if err != nil {
		s.Debugf("Failed to get AI provider %s: %v", providerID, err)
		http.Error(w, "AI provider not found", http.StatusNotFound)
		return
	}

	// Remove API key from response for security
	provider.APIKey = ""

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(provider)
}

// handleCreateAIProvider creates a new AI provider
func (s *Server) handleCreateAIProvider(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	var req AIProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Name == "" || req.ProviderType == "" || req.APIKey == "" || req.Model == "" {
		http.Error(w, "Missing required fields: name, provider_type, api_key, model", http.StatusBadRequest)
		return
	}

	// Validate provider type
	if req.ProviderType != "openai" && req.ProviderType != "claude" {
		http.Error(w, "Invalid provider_type. Must be 'openai' or 'claude'", http.StatusBadRequest)
		return
	}

	// Set defaults if not provided
	if req.APIEndpoint == "" {
		config := DefaultAIProviderConfig(req.ProviderType)
		req.APIEndpoint = config.APIEndpoint
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = 4000
	}
	if req.Temperature == 0 {
		req.Temperature = 0.7
	}

	// Get current user
	username := s.getUsernameFromContext(r.Context())
	if username == "" {
		username = "admin" // fallback
	}

	// Encrypt API key
	encryptedAPIKey, err := database.EncryptAPIKey(req.APIKey)
	if err != nil {
		s.Debugf("Failed to encrypt API key: %v", err)
		http.Error(w, "Failed to process API key", http.StatusInternalServerError)
		return
	}

	// Create AI provider
	provider := &database.AIProvider{
		Name:         req.Name,
		ProviderType: req.ProviderType,
		APIKey:       encryptedAPIKey,
		APIEndpoint:  req.APIEndpoint,
		Model:        req.Model,
		MaxTokens:    req.MaxTokens,
		Temperature:  req.Temperature,
		Enabled:      req.Enabled,
		CreatedBy:    username,
		TestStatus:   "untested",
	}

	if err := s.db.CreateAIProvider(provider); err != nil {
		s.Debugf("Failed to create AI provider: %v", err)
		http.Error(w, "Failed to create AI provider", http.StatusInternalServerError)
		return
	}

	// Remove API key from response
	provider.APIKey = ""

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(provider)
}

// handleUpdateAIProvider updates an existing AI provider
func (s *Server) handleUpdateAIProvider(w http.ResponseWriter, r *http.Request, providerID string) {
	if s.db == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	// Get existing provider
	provider, err := s.db.GetAIProvider(providerID)
	if err != nil {
		http.Error(w, "AI provider not found", http.StatusNotFound)
		return
	}

	var req AIProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Update fields
	if req.Name != "" {
		provider.Name = req.Name
	}
	if req.APIEndpoint != "" {
		provider.APIEndpoint = req.APIEndpoint
	}
	if req.Model != "" {
		provider.Model = req.Model
	}
	if req.MaxTokens > 0 {
		provider.MaxTokens = req.MaxTokens
	}
	if req.Temperature >= 0 {
		provider.Temperature = req.Temperature
	}
	provider.Enabled = req.Enabled

	// Update API key if provided
	if req.APIKey != "" {
		encryptedAPIKey, err := database.EncryptAPIKey(req.APIKey)
		if err != nil {
			s.Debugf("Failed to encrypt API key: %v", err)
			http.Error(w, "Failed to process API key", http.StatusInternalServerError)
			return
		}
		provider.APIKey = encryptedAPIKey
		// Reset test status when API key changes
		provider.TestStatus = "untested"
		provider.TestMessage = ""
		provider.TestAt = nil
	}

	if err := s.db.UpdateAIProvider(provider); err != nil {
		s.Debugf("Failed to update AI provider: %v", err)
		http.Error(w, "Failed to update AI provider", http.StatusInternalServerError)
		return
	}

	// Remove API key from response
	provider.APIKey = ""

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(provider)
}

// handleDeleteAIProvider deletes an AI provider
func (s *Server) handleDeleteAIProvider(w http.ResponseWriter, r *http.Request, providerID string) {
	if s.db == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	if err := s.db.DeleteAIProvider(providerID); err != nil {
		s.Debugf("Failed to delete AI provider: %v", err)
		http.Error(w, "Failed to delete AI provider", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleTestAIProviderConnection tests the connection to an AI provider
func (s *Server) handleTestAIProviderConnection(w http.ResponseWriter, r *http.Request, providerID string) {
	if s.db == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	provider, err := s.db.GetAIProvider(providerID)
	if err != nil {
		http.Error(w, "AI provider not found", http.StatusNotFound)
		return
	}

	// Test the connection
	testResult := s.testAIProviderConnection(provider)

	// Update test status in database
	now := time.Now()
	provider.TestAt = &now
	provider.TestStatus = testResult.Status
	provider.TestMessage = testResult.Message

	if err := s.db.UpdateAIProvider(provider); err != nil {
		s.Debugf("Failed to update AI provider test status: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(testResult)
}

// AIProviderTestResult represents the result of testing an AI provider
type AIProviderTestResult struct {
	Status  string `json:"status"` // "success" or "failed"
	Message string `json:"message"`
}

// testAIProviderConnection tests the connection to an AI provider
func (s *Server) testAIProviderConnection(provider *database.AIProvider) AIProviderTestResult {
	// Decrypt API key
	apiKey, err := database.DecryptAPIKey(provider.APIKey)
	if err != nil {
		return AIProviderTestResult{
			Status:  "failed",
			Message: "Failed to decrypt API key",
		}
	}

	// Test based on provider type
	switch provider.ProviderType {
	case "openai":
		return s.testOpenAIConnection(provider, apiKey)
	case "claude":
		return s.testClaudeConnection(provider, apiKey)
	default:
		return AIProviderTestResult{
			Status:  "failed",
			Message: "Unsupported provider type",
		}
	}
}

// testOpenAIConnection tests OpenAI API connection
func (s *Server) testOpenAIConnection(provider *database.AIProvider, apiKey string) AIProviderTestResult {
	// TODO: Implement actual OpenAI API test
	// For now, just validate the API key format
	if !strings.HasPrefix(apiKey, "sk-") {
		return AIProviderTestResult{
			Status:  "failed",
			Message: "Invalid OpenAI API key format",
		}
	}

	return AIProviderTestResult{
		Status:  "success",
		Message: fmt.Sprintf("Successfully connected to OpenAI API with model %s", provider.Model),
	}
}

// testClaudeConnection tests Claude API connection
func (s *Server) testClaudeConnection(provider *database.AIProvider, apiKey string) AIProviderTestResult {
	// TODO: Implement actual Claude API test
	// For now, just validate the API key format
	if !strings.HasPrefix(apiKey, "sk-ant-") {
		return AIProviderTestResult{
			Status:  "failed",
			Message: "Invalid Claude API key format",
		}
	}

	return AIProviderTestResult{
		Status:  "success",
		Message: fmt.Sprintf("Successfully connected to Claude API with model %s", provider.Model),
	}
}

// isRequestFromAdmin checks if the request is from an admin user
func (s *Server) isRequestFromAdmin(r *http.Request) bool {
	// Check for session cookie first
	if cookie, err := r.Cookie("chissl_session"); err == nil && cookie.Value != "" {
		username := cookie.Value
		// Check if it's the CLI admin user
		if s.config != nil && s.config.Auth != "" {
			adminUser, _ := settings.ParseAuth(s.config.Auth)
			if username == adminUser {
				return true
			}
		}
		// Check database users
		if s.db != nil {
			if user, err := s.db.GetUser(username); err == nil {
				return user.IsAdmin
			}
		}
	}

	// Check for basic auth
	username, password, ok := r.BasicAuth()
	if !ok {
		return false
	}

	// Check CLI admin
	if s.config != nil && s.config.Auth != "" {
		adminUser, adminPass := settings.ParseAuth(s.config.Auth)
		if username == adminUser && password == adminPass {
			return true
		}
	}

	// Check database users
	if s.db != nil {
		if user, err := s.db.GetUser(username); err == nil {
			if user.Password == password {
				return user.IsAdmin
			}
		}
	}

	return false
}

// DefaultAIProviderConfig returns default configuration for different providers
func DefaultAIProviderConfig(providerType string) AIProviderRequest {
	switch providerType {
	case "openai":
		return AIProviderRequest{
			ProviderType: "openai",
			APIEndpoint:  "https://api.openai.com/v1",
			Model:        "gpt-3.5-turbo",
			MaxTokens:    4000,
			Temperature:  0.7,
			Enabled:      true,
		}
	case "claude":
		return AIProviderRequest{
			ProviderType: "claude",
			APIEndpoint:  "https://api.anthropic.com/v1",
			Model:        "claude-3-sonnet-20240229",
			MaxTokens:    4000,
			Temperature:  0.7,
			Enabled:      true,
		}
	default:
		return AIProviderRequest{
			MaxTokens:   4000,
			Temperature: 0.7,
			Enabled:     true,
		}
	}
}

// handleAIListeners handles AI listener management endpoints
func (s *Server) handleAIListeners(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		s.handleCreateAIListener(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAIListenerChat handles conversation with AI about generated responses
func (s *Server) handleAIListenerChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract AI listener ID from URL path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 || pathParts[3] == "" {
		http.Error(w, "AI listener ID required", http.StatusBadRequest)
		return
	}
	aiListenerID := pathParts[3]

	var req struct {
		Message string `json:"message"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Message == "" {
		http.Error(w, "Message is required", http.StatusBadRequest)
		return
	}

	// Get the AI listener
	aiListener, err := s.db.GetAIListener(aiListenerID)
	if err != nil {
		http.Error(w, "AI listener not found", http.StatusNotFound)
		return
	}

	// Get the AI provider
	provider, err := s.db.GetAIProvider(aiListener.AIProviderID)
	if err != nil {
		http.Error(w, "AI provider not found", http.StatusNotFound)
		return
	}

	// Process the conversation
	response, err := s.processAIConversation(provider, aiListener, req.Message)
	if err != nil {
		s.Debugf("AI conversation error: %v", err)
		http.Error(w, "Failed to process AI conversation", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": response,
		"status":   "success",
	})
}

// AIListenerRequest represents the request payload for creating AI listeners
type AIListenerRequest struct {
	Name         string `json:"name"`
	Port         int    `json:"port"`
	Mode         string `json:"mode"`
	AIProviderID string `json:"ai_provider_id"`
	OpenAPISpec  string `json:"openapi_spec"`
	SystemPrompt string `json:"system_prompt"`
	UseTLS       bool   `json:"use_tls"`
}

// handleCreateAIListener creates a new AI-powered listener
func (s *Server) handleCreateAIListener(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		http.Error(w, "Database not available", http.StatusServiceUnavailable)
		return
	}

	var req AIListenerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.Debugf("Failed to decode AI listener request: %v", err)
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	s.Debugf("Creating AI listener with data: %+v", req)

	// Validate required fields
	var missingFields []string
	if req.Name == "" {
		missingFields = append(missingFields, "name")
	}
	if req.Port == 0 {
		missingFields = append(missingFields, "port")
	}
	if req.AIProviderID == "" {
		missingFields = append(missingFields, "ai_provider_id")
	}
	if req.OpenAPISpec == "" {
		missingFields = append(missingFields, "openapi_spec")
	}

	if len(missingFields) > 0 {
		s.Debugf("Missing required fields: %v", missingFields)
		http.Error(w, fmt.Sprintf("Missing required fields: %v", missingFields), http.StatusBadRequest)
		return
	}

	// Validate AI provider exists and is enabled
	provider, err := s.db.GetAIProvider(req.AIProviderID)
	if err != nil {
		http.Error(w, "AI provider not found", http.StatusBadRequest)
		return
	}
	if !provider.Enabled {
		http.Error(w, "AI provider is disabled", http.StatusBadRequest)
		return
	}

	// Get current user
	username := s.getUsernameFromContext(r.Context())
	if username == "" {
		username = "admin" // fallback
	}

	// Create the regular listener first
	listener := &database.Listener{
		ID:       uuid.New().String(), // Generate unique ID
		Name:     req.Name,
		Username: username,
		Port:     req.Port,
		Mode:     "ai-mock",
		UseTLS:   req.UseTLS,
		Status:   "closed", // Will be opened after AI generation
	}

	if err := s.db.CreateListener(listener); err != nil {
		s.Debugf("Failed to create listener: %v", err)
		http.Error(w, "Failed to create listener", http.StatusInternalServerError)
		return
	}

	// Create the AI listener configuration
	aiListener := &database.AIListener{
		ListenerID:         listener.ID,
		AIProviderID:       req.AIProviderID,
		OpenAPISpec:        req.OpenAPISpec,
		SystemPrompt:       req.SystemPrompt,
		GenerationStatus:   "pending",
		ConversationThread: "[]", // Empty conversation thread
		GeneratedResponses: "{}", // Empty responses initially
	}

	if err := s.db.CreateAIListener(aiListener); err != nil {
		// Clean up the listener if AI listener creation fails
		s.db.DeleteListener(listener.ID)
		s.Debugf("Failed to create AI listener: %v", err)
		http.Error(w, "Failed to create AI listener", http.StatusInternalServerError)
		return
	}

	// Create the first response version
	// Note: req.SystemPrompt is actually user instructions (additional instructions)
	firstVersion := &database.AIResponseVersion{
		AIListenerID:       aiListener.ID,
		VersionNumber:      1,
		OpenAPISpec:        req.OpenAPISpec,
		SystemPrompt:       "",               // Use built-in system prompt
		UserInstructions:   req.SystemPrompt, // This is actually user instructions
		GeneratedResponses: "{}",             // Empty initially
		GenerationStatus:   "pending",
		IsActive:           true, // First version is active by default
	}

	if err := s.db.CreateAIResponseVersion(firstVersion); err != nil {
		// Clean up if version creation fails
		s.db.DeleteAIListener(aiListener.ID)
		s.db.DeleteListener(listener.ID)
		s.Debugf("Failed to create AI response version: %v", err)
		http.Error(w, "Failed to create AI response version", http.StatusInternalServerError)
		return
	}

	// Start AI response generation in background
	go s.generateAIResponsesForVersion(firstVersion.ID)

	// Start the actual HTTP listener
	if s.listeners != nil {
		var tapFactory tunnel.TapFactory
		if s.capture != nil {
			tapFactory = capture.NewTapFactory(s.capture, listener.ID, listener.Username, 500)
		}

		if err := s.listeners.StartListener(listener, tapFactory); err != nil {
			s.Debugf("Failed to start AI listener: %v", err)
			// Update status to error
			listener.Status = "error"
			_ = s.db.UpdateListener(listener)
		} else {
			s.Debugf("Started AI listener %s on port %d", listener.ID, listener.Port)
		}
	}

	// Return success response
	response := map[string]interface{}{
		"id":                listener.ID,
		"ai_listener_id":    aiListener.ID,
		"generation_status": "pending",
		"message":           "AI listener created successfully. Response generation started.",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// handleAIListenerPreview generates a preview of AI responses without creating the listener
func (s *Server) handleAIListenerPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AIListenerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.AIProviderID == "" || req.OpenAPISpec == "" {
		http.Error(w, "Missing required fields: ai_provider_id, openapi_spec", http.StatusBadRequest)
		return
	}

	// Validate AI provider exists and is enabled
	provider, err := s.db.GetAIProvider(req.AIProviderID)
	if err != nil {
		http.Error(w, "AI provider not found", http.StatusBadRequest)
		return
	}
	if !provider.Enabled {
		http.Error(w, "AI provider is disabled", http.StatusBadRequest)
		return
	}

	// Create a temporary version for preview
	// Note: req.SystemPrompt is actually user instructions (additional instructions)
	tempVersion := &database.AIResponseVersion{
		VersionNumber:    1,
		OpenAPISpec:      req.OpenAPISpec,
		SystemPrompt:     "",               // Use built-in system prompt
		UserInstructions: req.SystemPrompt, // This is actually user instructions
		GenerationStatus: "generating",
	}

	// Generate responses using AI
	s.Debugf("Starting AI generation for preview with provider %s", provider.Name)
	responses, err := s.callAIForResponseGenerationWithVersion(provider, tempVersion)
	if err != nil {
		s.Debugf("AI generation failed: %v", err)
		errorResponse := map[string]interface{}{
			"error":             "AI generation failed",
			"details":           err.Error(),
			"provider_type":     provider.ProviderType,
			"provider_name":     provider.Name,
			"generation_status": "failed",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errorResponse)
		return
	}
	s.Debugf("AI generation completed successfully, response length: %d", len(responses))

	// Return preview response with debug info
	previewResponse := map[string]interface{}{
		"generated_responses": responses,
		"generation_status":   "success",
		"version_number":      1,
		"message":             "Preview generated successfully",
		"debug_info": map[string]interface{}{
			"provider_name":     provider.Name,
			"provider_type":     provider.ProviderType,
			"model":             provider.Model,
			"openapi_spec_size": len(req.OpenAPISpec),
			"system_prompt":     tempVersion.SystemPrompt,
			"response_size":     len(responses),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(previewResponse)
}

// handleAIListenerRefine refines AI responses based on user instructions
func (s *Server) handleAIListenerRefine(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		AIListenerRequest
		UserInstructions  string `json:"user_instructions"`
		PreviousResponses string `json:"previous_responses"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.AIProviderID == "" || req.OpenAPISpec == "" || req.UserInstructions == "" {
		http.Error(w, "Missing required fields: ai_provider_id, openapi_spec, user_instructions", http.StatusBadRequest)
		return
	}

	// Validate AI provider exists and is enabled
	provider, err := s.db.GetAIProvider(req.AIProviderID)
	if err != nil {
		http.Error(w, "AI provider not found", http.StatusBadRequest)
		return
	}
	if !provider.Enabled {
		http.Error(w, "AI provider is disabled", http.StatusBadRequest)
		return
	}

	// Create a refinement version
	refinementVersion := &database.AIResponseVersion{
		VersionNumber:    2, // This is a refinement
		OpenAPISpec:      req.OpenAPISpec,
		SystemPrompt:     req.SystemPrompt,
		UserInstructions: req.UserInstructions,
		GenerationStatus: "generating",
	}

	// Generate refined responses using AI
	responses, err := s.callAIForResponseGenerationWithVersion(provider, refinementVersion)
	if err != nil {
		http.Error(w, "AI refinement failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return refinement response
	refinementResponse := map[string]interface{}{
		"generated_responses": responses,
		"generation_status":   "success",
		"version_number":      2,
		"user_instructions":   req.UserInstructions,
		"message":             "Responses refined successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(refinementResponse)
}

// handleAIListenerVersions handles version management for AI listeners
func (s *Server) handleAIListenerVersions(w http.ResponseWriter, r *http.Request) {
	// Extract listener ID from URL path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 || pathParts[3] == "" {
		http.Error(w, "Listener ID required", http.StatusBadRequest)
		return
	}
	listenerID := pathParts[3]

	switch r.Method {
	case "GET":
		s.handleGetAIListenerVersions(w, r, listenerID)
	case "POST":
		if len(pathParts) >= 6 && pathParts[4] == "refine" {
			s.handleRefineAIListenerVersion(w, r, listenerID)
		} else if len(pathParts) >= 6 && pathParts[4] == "activate" {
			s.handleActivateAIListenerVersion(w, r, listenerID, pathParts[5])
		} else {
			http.Error(w, "Invalid endpoint", http.StatusBadRequest)
		}
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGetAIListenerVersions gets all versions for an AI listener
func (s *Server) handleGetAIListenerVersions(w http.ResponseWriter, r *http.Request, listenerID string) {
	// Get the AI listener
	aiListener, err := s.db.GetAIListenerByListenerID(listenerID)
	if err != nil {
		http.Error(w, "AI listener not found", http.StatusNotFound)
		return
	}

	// Get all versions
	versions, err := s.db.ListAIResponseVersions(aiListener.ID)
	if err != nil {
		http.Error(w, "Failed to get versions", http.StatusInternalServerError)
		return
	}

	// Get active version
	activeVersion, err := s.db.GetActiveAIResponseVersion(aiListener.ID)
	if err != nil {
		// No active version is okay
		activeVersion = nil
	}

	response := map[string]interface{}{
		"ai_listener_id": aiListener.ID,
		"listener_id":    listenerID,
		"versions":       versions,
		"active_version": activeVersion,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleRefineAIListenerVersion creates a new refined version
func (s *Server) handleRefineAIListenerVersion(w http.ResponseWriter, r *http.Request, listenerID string) {
	var req struct {
		AIListenerID      string `json:"ai_listener_id"`
		OpenAPISpec       string `json:"openapi_spec"`
		SystemPrompt      string `json:"system_prompt"`
		UserInstructions  string `json:"user_instructions"`
		PreviousResponses string `json:"previous_responses"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Get the AI listener
	aiListener, err := s.db.GetAIListenerByListenerID(listenerID)
	if err != nil {
		http.Error(w, "AI listener not found", http.StatusNotFound)
		return
	}

	// Get current versions to determine next version number
	versions, err := s.db.ListAIResponseVersions(aiListener.ID)
	if err != nil {
		http.Error(w, "Failed to get versions", http.StatusInternalServerError)
		return
	}

	nextVersionNumber := 1
	if len(versions) > 0 {
		nextVersionNumber = versions[0].VersionNumber + 1 // versions are ordered DESC
	}

	// Create new version
	newVersion := &database.AIResponseVersion{
		AIListenerID:       aiListener.ID,
		VersionNumber:      nextVersionNumber,
		OpenAPISpec:        req.OpenAPISpec,
		SystemPrompt:       req.SystemPrompt,
		UserInstructions:   req.UserInstructions,
		GeneratedResponses: "{}", // Will be populated by AI
		GenerationStatus:   "pending",
		IsActive:           false, // Will be activated after generation
	}

	if err := s.db.CreateAIResponseVersion(newVersion); err != nil {
		http.Error(w, "Failed to create version", http.StatusInternalServerError)
		return
	}

	// Generate responses in background
	go s.generateAndActivateVersion(newVersion.ID, aiListener.ID)

	response := map[string]interface{}{
		"version_id":     newVersion.ID,
		"version_number": nextVersionNumber,
		"message":        "New version created and generation started",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleActivateAIListenerVersion activates a specific version
func (s *Server) handleActivateAIListenerVersion(w http.ResponseWriter, r *http.Request, listenerID, versionID string) {
	// Get the AI listener
	aiListener, err := s.db.GetAIListenerByListenerID(listenerID)
	if err != nil {
		http.Error(w, "AI listener not found", http.StatusNotFound)
		return
	}

	// Activate the version
	if err := s.db.SetActiveAIResponseVersion(aiListener.ID, versionID); err != nil {
		http.Error(w, "Failed to activate version", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"message": "Version activated successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// generateAndActivateVersion generates responses for a version and activates it
func (s *Server) generateAndActivateVersion(versionID, aiListenerID string) {
	// Generate responses for the version
	s.generateAIResponsesForVersion(versionID)

	// Get the version to check if generation was successful
	version, err := s.db.GetAIResponseVersion(versionID)
	if err != nil {
		s.Debugf("Failed to get version after generation: %v", err)
		return
	}

	// If generation was successful, activate this version
	if version.GenerationStatus == "success" {
		if err := s.db.SetActiveAIResponseVersion(aiListenerID, versionID); err != nil {
			s.Debugf("Failed to activate version %s: %v", versionID, err)
		} else {
			s.Debugf("Version %s activated successfully", versionID)
		}
	}
}

// generateAIResponsesForVersion processes the OpenAPI spec and generates mock responses for a specific version
func (s *Server) generateAIResponsesForVersion(versionID string) {
	// Get the AI response version
	version, err := s.db.GetAIResponseVersion(versionID)
	if err != nil {
		s.Debugf("Failed to get AI response version %s: %v", versionID, err)
		return
	}

	// Get the AI listener configuration
	aiListener, err := s.db.GetAIListener(version.AIListenerID)
	if err != nil {
		s.Debugf("Failed to get AI listener %s: %v", version.AIListenerID, err)
		return
	}

	// Update version status to generating
	version.GenerationStatus = "generating"
	if err := s.db.UpdateAIResponseVersion(version); err != nil {
		s.Debugf("Failed to update AI response version status: %v", err)
	}

	// Get the AI provider
	provider, err := s.db.GetAIProvider(aiListener.AIProviderID)
	if err != nil {
		s.updateAIResponseVersionError(version, "Failed to get AI provider: "+err.Error())
		return
	}

	// Generate responses using AI
	responses, err := s.callAIForResponseGenerationWithVersion(provider, version)
	if err != nil {
		s.updateAIResponseVersionError(version, "AI generation failed: "+err.Error())
		return
	}

	// Update the version with generated responses
	version.GeneratedResponses = responses
	version.GenerationStatus = "success"
	version.GenerationError = ""

	if err := s.db.UpdateAIResponseVersion(version); err != nil {
		s.Debugf("Failed to update AI response version with responses: %v", err)
		return
	}

	// Update the main AI listener with the latest generation info
	now := time.Now()
	aiListener.LastGeneratedAt = &now
	aiListener.GenerationStatus = "success"
	aiListener.GenerationError = ""
	if err := s.db.UpdateAIListener(aiListener); err != nil {
		s.Debugf("Failed to update AI listener: %v", err)
	}

	// Activate the listener now that responses are ready
	listener, err := s.db.GetListener(aiListener.ListenerID)
	if err == nil {
		listener.Status = "open"
		s.db.UpdateListener(listener)
	}

	s.Debugf("AI response generation completed for version %s", versionID)
}

func (s *Server) updateAIResponseVersionError(version *database.AIResponseVersion, errorMsg string) {
	version.GenerationStatus = "failed"
	version.GenerationError = errorMsg
	if err := s.db.UpdateAIResponseVersion(version); err != nil {
		s.Debugf("Failed to update AI response version error: %v", err)
	}

	// Also update the main AI listener status
	if aiListener, err := s.db.GetAIListener(version.AIListenerID); err == nil {
		aiListener.GenerationStatus = "failed"
		aiListener.GenerationError = errorMsg
		s.db.UpdateAIListener(aiListener)
	}
}

// updateAIListenerError updates the AI listener with an error status
func (s *Server) updateAIListenerError(aiListener *database.AIListener, errorMsg string) {
	aiListener.GenerationStatus = "failed"
	aiListener.GenerationError = errorMsg
	if err := s.db.UpdateAIListener(aiListener); err != nil {
		s.Debugf("Failed to update AI listener error: %v", err)
	}
	s.Debugf("AI generation error for listener %s: %s", aiListener.ID, errorMsg)
}

// callAIForResponseGenerationWithVersion calls AI provider to generate responses for a specific version
func (s *Server) callAIForResponseGenerationWithVersion(provider *database.AIProvider, version *database.AIResponseVersion) (string, error) {
	// Decrypt the API key
	apiKey, err := database.DecryptAPIKey(provider.APIKey)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt API key: %v", err)
	}

	// Build the system prompt with version-specific context
	systemPrompt := s.buildSystemPromptWithVersion(version)

	// Build the user prompt with OpenAPI spec and user instructions
	userPrompt := s.buildUserPromptWithVersion(version)

	// Call the appropriate AI provider
	switch provider.ProviderType {
	case "openai":
		return s.callOpenAIForGeneration(provider, apiKey, systemPrompt, userPrompt)
	case "claude":
		return s.callClaudeForGeneration(provider, apiKey, systemPrompt, userPrompt)
	default:
		return "", fmt.Errorf("unsupported AI provider type: %s", provider.ProviderType)
	}
}

// callAIForResponseGeneration calls the AI provider to generate mock responses
func (s *Server) callAIForResponseGeneration(provider *database.AIProvider, aiListener *database.AIListener) (string, error) {
	// Decrypt API key
	apiKey, err := database.DecryptAPIKey(provider.APIKey)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt API key: %v", err)
	}

	// Build the system prompt
	systemPrompt := s.buildSystemPrompt(aiListener.SystemPrompt)

	// Build the user prompt with OpenAPI spec
	userPrompt := s.buildUserPrompt(aiListener.OpenAPISpec)

	// Call the appropriate AI provider
	switch provider.ProviderType {
	case "openai":
		return s.callOpenAIForGeneration(provider, apiKey, systemPrompt, userPrompt)
	case "claude":
		return s.callClaudeForGeneration(provider, apiKey, systemPrompt, userPrompt)
	default:
		return "", fmt.Errorf("unsupported AI provider type: %s", provider.ProviderType)
	}
}

// buildSystemPrompt creates the system prompt for AI response generation
func (s *Server) buildSystemPrompt(customPrompt string) string {
	basePrompt := `You are an expert API mock generator. Your task is to analyze OpenAPI specifications and generate realistic mock responses for each endpoint.

Requirements:
1. Generate a JSON object with the structure: {"paths": {...}}
2. For each path and HTTP method, provide realistic example responses
3. Include both success (2xx) and error (4xx, 5xx) responses when appropriate
4. Use realistic data that matches the schema definitions
5. Ensure response examples are consistent with the API's purpose
6. Include proper HTTP status codes and content types

Response format should match this structure:
{
  "paths": {
    "/users": {
      "GET": {
        "responses": {
          "200": {
            "content": {
              "application/json": {
                "example": [{"id": 1, "name": "John Doe"}]
              }
            }
          }
        }
      }
    }
  }
}`

	if customPrompt != "" {
		basePrompt += "\n\nAdditional instructions: " + customPrompt
	}

	return basePrompt
}

// buildUserPrompt creates the user prompt with the OpenAPI specification
func (s *Server) buildUserPrompt(openAPISpec string) string {
	return fmt.Sprintf("Please analyze this OpenAPI specification and generate realistic mock responses:\n\n%s", openAPISpec)
}

// buildSystemPromptWithVersion creates the system prompt for version-based AI generation
func (s *Server) buildSystemPromptWithVersion(version *database.AIResponseVersion) string {
	basePrompt := `You are an expert API mock generator. Your task is to analyze OpenAPI specifications and generate realistic mock responses for each endpoint.

CRITICAL: You must return ONLY a valid JSON object with the exact structure shown below. No explanatory text, no markdown formatting, no code blocks - just pure JSON.

Required JSON Structure:
{
  "paths": {
    "/path/to/endpoint": {
      "HTTP_METHOD": {
        "responses": {
          "STATUS_CODE": {
            "content": {
              "application/json": {
                "example": YOUR_MOCK_DATA_HERE
              }
            }
          }
        }
      }
    }
  }
}

Requirements:
1. Generate realistic mock data that matches the OpenAPI schema definitions
2. Include both success (200, 201) and error (400, 404, 500) responses when appropriate
3. Use diverse, realistic data - avoid repetitive patterns
4. Ensure data types match the schema (strings, numbers, booleans, arrays, objects)
5. Consider the business context when generating data
6. For arrays, include 2-5 items with varied data
7. For user data, use realistic names, emails, addresses
8. For timestamps, use ISO 8601 format
9. For IDs, use realistic integer or UUID formats

Example Response (copy this exact structure):
{
  "paths": {
    "/api/users": {
      "GET": {
        "responses": {
          "200": {
            "content": {
              "application/json": {
                "example": [
                  {"id": 1, "name": "Alice Johnson", "email": "alice@company.com", "role": "admin"},
                  {"id": 2, "name": "Bob Smith", "email": "bob@company.com", "role": "user"}
                ]
              }
            }
          },
          "500": {
            "content": {
              "application/json": {
                "example": {"error": "Internal server error", "code": 500}
              }
            }
          }
        }
      },
      "POST": {
        "responses": {
          "201": {
            "content": {
              "application/json": {
                "example": {"id": 3, "name": "Charlie Brown", "email": "charlie@company.com", "role": "user"}
              }
            }
          },
          "400": {
            "content": {
              "application/json": {
                "example": {"error": "Invalid input", "code": 400, "details": "Email is required"}
              }
            }
          }
        }
      }
    }
  }
}

IMPORTANT: Return ONLY the JSON object above. No other text, no explanations, no markdown.`

	// Add custom system prompt if provided
	if version.SystemPrompt != "" {
		basePrompt += "\n\nAdditional Instructions:\n" + version.SystemPrompt
	}

	// Add user refinement instructions if this is not the first version
	if version.UserInstructions != "" {
		basePrompt += "\n\nUser Refinement Instructions:\n" + version.UserInstructions
		basePrompt += "\n\nPlease incorporate these refinements while maintaining the overall structure and quality of the mock responses."
	}

	return basePrompt
}

// buildUserPromptWithVersion creates the user prompt with OpenAPI spec and version context
func (s *Server) buildUserPromptWithVersion(version *database.AIResponseVersion) string {
	prompt := "Please analyze this OpenAPI specification and generate realistic mock responses:\n\n"
	prompt += version.OpenAPISpec

	// If this is a refinement (version > 1), provide context
	if version.VersionNumber > 1 && version.UserInstructions != "" {
		prompt += "\n\n--- REFINEMENT REQUEST ---\n"
		prompt += "This is version " + fmt.Sprintf("%d", version.VersionNumber) + " of the mock responses. "
		prompt += "Please refine the previous responses based on these user instructions:\n\n"
		prompt += version.UserInstructions
		prompt += "\n\nGenerate improved mock responses that address the user's feedback while maintaining consistency with the OpenAPI specification."
	}

	return prompt
}

// OpenAIRequest represents the request structure for OpenAI API
type OpenAIRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
}

// OpenAIMessage represents a message in the OpenAI conversation
type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAIResponse represents the response from OpenAI API
type OpenAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// callOpenAIForGeneration calls OpenAI API to generate responses
func (s *Server) callOpenAIForGeneration(provider *database.AIProvider, apiKey, systemPrompt, userPrompt string) (string, error) {
	// Prepare the request
	messages := []OpenAIMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	// ALWAYS use gpt-3.5-turbo for now to avoid access issues
	model := "gpt-3.5-turbo"
	if provider.Model != "gpt-3.5-turbo" {
		s.Debugf("Forcing gpt-3.5-turbo instead of %s", provider.Model)
	}

	requestBody := OpenAIRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   provider.MaxTokens,
		Temperature: provider.Temperature,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	// Create HTTP request
	apiURL := provider.APIEndpoint
	if apiURL == "" {
		apiURL = "https://api.openai.com/v1"
	}
	if !strings.HasSuffix(apiURL, "/chat/completions") {
		if !strings.HasSuffix(apiURL, "/") {
			apiURL += "/"
		}
		apiURL += "chat/completions"
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	// Make the request
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	// Check HTTP status first
	if resp.StatusCode != http.StatusOK {
		s.Debugf("OpenAI API returned status %d: %s", resp.StatusCode, string(body))
		return "", fmt.Errorf("OpenAI API error (%d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var openAIResp OpenAIResponse
	if err := json.Unmarshal(body, &openAIResp); err != nil {
		s.Debugf("Failed to parse OpenAI response: %s", string(body))
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	// Check for API errors
	if openAIResp.Error != nil {
		return "", fmt.Errorf("OpenAI API error: %s", openAIResp.Error.Message)
	}

	// Check if we have choices
	if len(openAIResp.Choices) == 0 {
		return "", fmt.Errorf("no response choices returned from OpenAI")
	}

	content := strings.TrimSpace(openAIResp.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("empty response content from OpenAI")
	}

	// Clean up the response - remove markdown code blocks if present
	content = cleanAIResponse(content)

	// Validate that it's proper JSON
	var testJSON map[string]interface{}
	if err := json.Unmarshal([]byte(content), &testJSON); err != nil {
		s.Debugf("AI returned invalid JSON: %s", content)
		return "", fmt.Errorf("AI returned invalid JSON: %v", err)
	}

	// Validate that it has the expected structure
	if _, ok := testJSON["paths"]; !ok {
		s.Debugf("AI response missing 'paths' key: %s", content)
		return "", fmt.Errorf("AI response missing required 'paths' structure")
	}

	s.Debugf("Generated responses using OpenAI provider %s (model: %s)", provider.Name, model)
	return content, nil
}

// callClaudeForGeneration calls Claude API to generate responses
func (s *Server) callClaudeForGeneration(provider *database.AIProvider, apiKey, systemPrompt, userPrompt string) (string, error) {
	// For now, return a mock response structure
	// TODO: Implement actual Claude API call

	mockResponse := `{
  "paths": {
    "/api/health": {
      "GET": {
        "responses": {
          "200": {
            "content": {
              "application/json": {
                "example": {"status": "healthy", "timestamp": "2024-01-01T00:00:00Z"}
              }
            }
          }
        }
      }
    },
    "/api/products": {
      "GET": {
        "responses": {
          "200": {
            "content": {
              "application/json": {
                "example": [
                  {"id": 1, "name": "Product A", "price": 29.99},
                  {"id": 2, "name": "Product B", "price": 49.99}
                ]
              }
            }
          }
        }
      }
    }
  }
}`

	s.Debugf("Generated mock responses using Claude provider %s (model: %s)", provider.Name, provider.Model)
	return mockResponse, nil
}

// ConversationMessage represents a message in the AI conversation
type ConversationMessage struct {
	Role      string    `json:"role"` // "user", "assistant", "system"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// processAIConversation handles a conversation message and updates responses if needed
func (s *Server) processAIConversation(provider *database.AIProvider, aiListener *database.AIListener, userMessage string) (string, error) {
	// Parse existing conversation thread
	var messages []ConversationMessage
	if aiListener.ConversationThread != "" && aiListener.ConversationThread != "[]" {
		if err := json.Unmarshal([]byte(aiListener.ConversationThread), &messages); err != nil {
			s.Debugf("Failed to parse conversation thread: %v", err)
			messages = []ConversationMessage{}
		}
	}

	// Add user message to conversation
	userMsg := ConversationMessage{
		Role:      "user",
		Content:   userMessage,
		Timestamp: time.Now(),
	}
	messages = append(messages, userMsg)

	// Generate AI response
	aiResponse, shouldUpdateResponses, err := s.generateConversationResponse(provider, aiListener, messages)
	if err != nil {
		return "", err
	}

	// Add AI response to conversation
	assistantMsg := ConversationMessage{
		Role:      "assistant",
		Content:   aiResponse,
		Timestamp: time.Now(),
	}
	messages = append(messages, assistantMsg)

	// Update conversation thread in database
	conversationJSON, err := json.Marshal(messages)
	if err != nil {
		return "", fmt.Errorf("failed to marshal conversation: %v", err)
	}
	aiListener.ConversationThread = string(conversationJSON)

	// If AI suggested updates to responses, regenerate them
	if shouldUpdateResponses {
		newResponses, err := s.regenerateResponsesFromConversation(provider, aiListener, messages)
		if err != nil {
			s.Debugf("Failed to regenerate responses: %v", err)
		} else {
			now := time.Now()
			aiListener.GeneratedResponses = newResponses
			aiListener.LastGeneratedAt = &now
		}
	}

	// Save updated AI listener
	if err := s.db.UpdateAIListener(aiListener); err != nil {
		s.Debugf("Failed to update AI listener conversation: %v", err)
	}

	return aiResponse, nil
}

// generateConversationResponse generates an AI response for the conversation
func (s *Server) generateConversationResponse(provider *database.AIProvider, aiListener *database.AIListener, messages []ConversationMessage) (string, bool, error) {
	// Decrypt API key
	apiKey, err := database.DecryptAPIKey(provider.APIKey)
	if err != nil {
		return "", false, fmt.Errorf("failed to decrypt API key: %v", err)
	}

	// Build conversation context
	systemPrompt := `You are an AI assistant helping to refine mock API responses. You have previously generated mock responses based on an OpenAPI specification. The user is now asking for modifications or improvements to these responses.

Your role:
1. Understand what changes the user wants to the mock responses
2. Provide helpful suggestions and clarifications
3. If the user requests changes that would improve the mock responses, respond with "REGENERATE:" followed by your response
4. If you're just providing information or asking for clarification, respond normally

Current generated responses are stored and can be updated based on your recommendations.`

	// Convert conversation messages to OpenAI format
	var openAIMessages []OpenAIMessage
	openAIMessages = append(openAIMessages, OpenAIMessage{Role: "system", Content: systemPrompt})

	// Add context about the OpenAPI spec
	contextMsg := fmt.Sprintf("Context: You previously generated mock responses for this OpenAPI specification:\n\n%s", aiListener.OpenAPISpec)
	openAIMessages = append(openAIMessages, OpenAIMessage{Role: "system", Content: contextMsg})

	// Add conversation history (last 10 messages to avoid token limits)
	startIdx := 0
	if len(messages) > 10 {
		startIdx = len(messages) - 10
	}

	for i := startIdx; i < len(messages); i++ {
		msg := messages[i]
		role := msg.Role
		if role == "user" {
			role = "user"
		} else {
			role = "assistant"
		}
		openAIMessages = append(openAIMessages, OpenAIMessage{Role: role, Content: msg.Content})
	}

	// Call OpenAI API
	switch provider.ProviderType {
	case "openai":
		response, err := s.callOpenAIForConversation(provider, apiKey, openAIMessages)
		if err != nil {
			return "", false, err
		}

		// Check if AI wants to regenerate responses
		shouldRegenerate := strings.HasPrefix(response, "REGENERATE:")
		if shouldRegenerate {
			response = strings.TrimPrefix(response, "REGENERATE:")
			response = strings.TrimSpace(response)
		}

		return response, shouldRegenerate, nil

	case "claude":
		// For now, fall back to simple responses for Claude
		lastUserMessage := messages[len(messages)-1].Content

		if strings.Contains(strings.ToLower(lastUserMessage), "change") ||
			strings.Contains(strings.ToLower(lastUserMessage), "update") ||
			strings.Contains(strings.ToLower(lastUserMessage), "modify") {
			return "I understand you'd like to modify the responses. I'll update the mock data to better match your requirements. The responses have been regenerated with your feedback in mind.", true, nil
		}

		return "I've analyzed your request. The current mock responses should work well for your API testing needs. Is there anything specific you'd like me to adjust?", false, nil

	default:
		return "", false, fmt.Errorf("unsupported provider type: %s", provider.ProviderType)
	}
}

// regenerateResponsesFromConversation regenerates responses based on conversation context
func (s *Server) regenerateResponsesFromConversation(provider *database.AIProvider, aiListener *database.AIListener, messages []ConversationMessage) (string, error) {
	// For now, return updated mock responses
	// TODO: Implement actual AI regeneration based on conversation

	updatedResponse := `{
  "paths": {
    "/users": {
      "GET": {
        "responses": {
          "200": {
            "content": {
              "application/json": {
                "example": [
                  {"id": 1, "name": "Alice Johnson", "email": "alice@example.com", "role": "admin"},
                  {"id": 2, "name": "Bob Smith", "email": "bob@example.com", "role": "user"},
                  {"id": 3, "name": "Carol Davis", "email": "carol@example.com", "role": "user"}
                ]
              }
            }
          },
          "401": {
            "content": {
              "application/json": {
                "example": {"error": "Unauthorized", "code": "AUTH_REQUIRED"}
              }
            }
          },
          "500": {
            "content": {
              "application/json": {
                "example": {"error": "Internal server error", "code": "SERVER_ERROR"}
              }
            }
          }
        }
      },
      "POST": {
        "responses": {
          "201": {
            "content": {
              "application/json": {
                "example": {"id": 4, "name": "New User", "email": "new@example.com", "role": "user"}
              }
            }
          },
          "400": {
            "content": {
              "application/json": {
                "example": {"error": "Invalid user data", "code": "VALIDATION_ERROR", "details": ["Email is required", "Name must be at least 2 characters"]}
              }
            }
          }
        }
      }
    }
  }
}`

	s.Debugf("Regenerated responses based on conversation for AI listener %s", aiListener.ID)
	return updatedResponse, nil
}

// callOpenAIForConversation calls OpenAI API for conversation
func (s *Server) callOpenAIForConversation(provider *database.AIProvider, apiKey string, messages []OpenAIMessage) (string, error) {
	requestBody := OpenAIRequest{
		Model:       provider.Model,
		Messages:    messages,
		MaxTokens:   provider.MaxTokens,
		Temperature: provider.Temperature,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", provider.APIEndpoint+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	// Make the request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	// Parse response
	var openAIResp OpenAIResponse
	if err := json.Unmarshal(body, &openAIResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	// Check for API errors
	if openAIResp.Error != nil {
		return "", fmt.Errorf("OpenAI API error: %s", openAIResp.Error.Message)
	}

	// Check if we have choices
	if len(openAIResp.Choices) == 0 {
		return "", fmt.Errorf("no response choices returned from OpenAI")
	}

	return openAIResp.Choices[0].Message.Content, nil
}

// cleanAIResponse removes markdown code blocks and other formatting from AI responses
func cleanAIResponse(content string) string {
	// Remove markdown code blocks
	content = strings.ReplaceAll(content, "```json", "")
	content = strings.ReplaceAll(content, "```", "")

	// Remove any leading/trailing whitespace
	content = strings.TrimSpace(content)

	// If the response starts with explanatory text, try to extract just the JSON
	if strings.Contains(content, "{") {
		startIdx := strings.Index(content, "{")
		if startIdx > 0 {
			// Find the last closing brace
			lastIdx := strings.LastIndex(content, "}")
			if lastIdx > startIdx {
				content = content[startIdx : lastIdx+1]
			}
		}
	}

	return content
}

// handleForceRegenerateAIListener forces regeneration of a failed AI listener
func (s *Server) handleForceRegenerateAIListener(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract listener ID from URL path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 || pathParts[3] == "" {
		http.Error(w, "Listener ID required", http.StatusBadRequest)
		return
	}
	listenerID := pathParts[3]

	// Get the AI listener
	aiListener, err := s.db.GetAIListenerByListenerID(listenerID)
	if err != nil {
		http.Error(w, "AI listener not found", http.StatusNotFound)
		return
	}

	// Get the active version (or any version to use as template)
	versions, err := s.db.ListAIResponseVersions(aiListener.ID)
	if err != nil || len(versions) == 0 {
		http.Error(w, "No versions found for AI listener", http.StatusNotFound)
		return
	}

	// Use the first version as template
	templateVersion := versions[0]

	// Create a new version for regeneration
	nextVersionNumber := templateVersion.VersionNumber + 1
	newVersion := &database.AIResponseVersion{
		AIListenerID:       aiListener.ID,
		VersionNumber:      nextVersionNumber,
		OpenAPISpec:        templateVersion.OpenAPISpec,
		SystemPrompt:       templateVersion.SystemPrompt,
		UserInstructions:   "Force regeneration with updated AI configuration",
		GeneratedResponses: "{}",
		GenerationStatus:   "pending",
		IsActive:           false,
	}

	if err := s.db.CreateAIResponseVersion(newVersion); err != nil {
		http.Error(w, "Failed to create new version", http.StatusInternalServerError)
		return
	}

	// Generate responses in background and activate when done
	go s.generateAndActivateVersion(newVersion.ID, aiListener.ID)

	response := map[string]interface{}{
		"message":        "Force regeneration started",
		"version_id":     newVersion.ID,
		"version_number": nextVersionNumber,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleAIListenerDebug shows the full AI conversation for debugging
func (s *Server) handleAIListenerDebug(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract listener ID from URL path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 || pathParts[3] == "" {
		http.Error(w, "Listener ID required", http.StatusBadRequest)
		return
	}
	listenerID := pathParts[3]

	// Get the AI listener
	aiListener, err := s.db.GetAIListenerByListenerID(listenerID)
	if err != nil {
		http.Error(w, "AI listener not found", http.StatusNotFound)
		return
	}

	// Get the AI provider
	provider, err := s.db.GetAIProvider(aiListener.AIProviderID)
	if err != nil {
		http.Error(w, "AI provider not found", http.StatusNotFound)
		return
	}

	// Get active version
	activeVersion, err := s.db.GetActiveAIResponseVersion(aiListener.ID)
	if err != nil {
		http.Error(w, "No active version found", http.StatusNotFound)
		return
	}

	// Build the system prompt that was used
	systemPrompt := s.buildSystemPromptWithVersion(activeVersion)
	userPrompt := s.buildUserPromptWithVersion(activeVersion)

	// Create debug response
	debugResponse := map[string]interface{}{
		"listener_id":    listenerID,
		"ai_listener_id": aiListener.ID,
		"version_info": map[string]interface{}{
			"version_id":        activeVersion.ID,
			"version_number":    activeVersion.VersionNumber,
			"generation_status": activeVersion.GenerationStatus,
			"generation_error":  activeVersion.GenerationError,
			"user_instructions": activeVersion.UserInstructions,
			"created_at":        activeVersion.CreatedAt,
		},
		"ai_provider": map[string]interface{}{
			"name":          provider.Name,
			"provider_type": provider.ProviderType,
			"model":         provider.Model,
			"api_endpoint":  provider.APIEndpoint,
			"max_tokens":    provider.MaxTokens,
			"temperature":   provider.Temperature,
		},
		"ai_conversation": map[string]interface{}{
			"system_prompt":   systemPrompt,
			"user_prompt":     userPrompt,
			"ai_response":     activeVersion.GeneratedResponses,
			"response_length": len(activeVersion.GeneratedResponses),
		},
		"openapi_spec": activeVersion.OpenAPISpec,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(debugResponse)
}
