package chserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/NextChapterSoftware/chissl/share/database"
	"github.com/NextChapterSoftware/chissl/share/settings"
)

// UserInfo represents user information for the frontend
type UserInfo struct {
	Username       string `json:"username"`
	Email          string `json:"email,omitempty"`
	DisplayName    string `json:"display_name,omitempty"`
	IsAdmin        bool   `json:"admin"`
	SSOEnabled     bool   `json:"sso_enabled"`
	CanEditProfile bool   `json:"can_edit_profile"`
	AuthMethod     string `json:"auth_method,omitempty"`
}

// UserToken represents an API token
type UserToken struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Token     string     `json:"token,omitempty"` // Only included when creating
	CreatedAt time.Time  `json:"created_at"`
	LastUsed  *time.Time `json:"last_used,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// handleGetUserInfo returns current user information
func (s *Server) handleGetUserInfo(w http.ResponseWriter, r *http.Request) {
	username := s.getUsernameFromContext(r.Context())
	if username == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Check if this is a CLI admin user (from --auth flag)
	isCLIAdmin := false
	if s.config != nil && s.config.Auth != "" {
		au, _ := settings.ParseAuth(s.config.Auth)
		if username == au {
			isCLIAdmin = true
		}
	}

	// Get auth method from context
	authMethod := ""
	if method := r.Context().Value("authMethod"); method != nil {
		if methodStr, ok := method.(string); ok {
			authMethod = methodStr
		}
	}

	userInfo := UserInfo{
		Username:       username,
		IsAdmin:        s.isUserAdmin(r.Context()),
		CanEditProfile: !isCLIAdmin, // CLI admin users can't edit profile
		AuthMethod:     authMethod,
		SSOEnabled:     s.auth0 != nil && s.auth0.IsEnabled(),
	}

	// Try to get additional user info from database
	if s.db != nil && !isCLIAdmin {
		if dbUser, err := s.db.GetUser(username); err == nil && dbUser != nil {
			userInfo.Email = dbUser.Email
			userInfo.DisplayName = dbUser.DisplayName
		}
	}

	// Check if user is using SSO
	if authMethodValue := r.Context().Value("authMethod"); authMethodValue != nil {
		if authMethodStr, ok := authMethodValue.(string); ok && authMethodStr == "auth0" {
			userInfo.SSOEnabled = true
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userInfo)
}

// handleListUserTokens returns user's API tokens
func (s *Server) handleListUserTokens(w http.ResponseWriter, r *http.Request) {
	username := s.getUsernameFromContext(r.Context())
	if username == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}

	tokens, err := s.db.ListUserTokens(username)
	if err != nil {
		s.Debugf("Failed to list user tokens: %v", err)
		http.Error(w, "Failed to list tokens", http.StatusInternalServerError)
		return
	}

	// Convert to API format (without actual token values)
	var apiTokens []UserToken
	for _, token := range tokens {
		apiTokens = append(apiTokens, UserToken{
			ID:        token.ID,
			Name:      token.Name,
			CreatedAt: token.CreatedAt,
			LastUsed:  token.LastUsed,
			ExpiresAt: token.ExpiresAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apiTokens)
}

// handleCreateUserToken creates a new API token
func (s *Server) handleCreateUserToken(w http.ResponseWriter, r *http.Request) {
	username := s.getUsernameFromContext(r.Context())
	if username == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Name       string `json:"name"`
		ExpiryDays *int   `json:"expiry_days,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		http.Error(w, "Token name is required", http.StatusBadRequest)
		return
	}

	// Generate a shorter, more user-friendly token (16 bytes = 32 hex chars)
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}
	tokenValue := hex.EncodeToString(tokenBytes)

	// Calculate expiry date if specified
	var expiresAt *time.Time
	if req.ExpiryDays != nil && *req.ExpiryDays > 0 {
		expiry := time.Now().AddDate(0, 0, *req.ExpiryDays)
		expiresAt = &expiry
	}

	// Create token in database
	token := &database.UserToken{
		ID:        fmt.Sprintf("token-%d-%d", time.Now().Unix(), time.Now().UnixNano()),
		Username:  username,
		Name:      strings.TrimSpace(req.Name),
		Token:     tokenValue,
		CreatedAt: time.Now(),
		ExpiresAt: expiresAt,
	}

	if err := s.db.CreateUserToken(token); err != nil {
		s.Debugf("Failed to create user token: %v", err)
		http.Error(w, "Failed to create token", http.StatusInternalServerError)
		return
	}

	// Return the token (only time it's shown)
	response := UserToken{
		ID:        token.ID,
		Name:      token.Name,
		Token:     tokenValue,
		CreatedAt: token.CreatedAt,
		ExpiresAt: token.ExpiresAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// handleRevokeUserToken revokes an API token
func (s *Server) handleRevokeUserToken(w http.ResponseWriter, r *http.Request) {
	username := s.getUsernameFromContext(r.Context())
	if username == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}

	// Extract token ID from path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		http.Error(w, "Invalid token ID", http.StatusBadRequest)
		return
	}
	tokenID := parts[4]

	// Verify token belongs to user
	token, err := s.db.GetUserToken(tokenID)
	if err != nil {
		http.Error(w, "Token not found", http.StatusNotFound)
		return
	}

	if token.Username != username {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Delete the token
	if err := s.db.DeleteUserToken(tokenID); err != nil {
		s.Debugf("Failed to delete user token: %v", err)
		http.Error(w, "Failed to revoke token", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleUpdateUserProfile updates user profile information
func (s *Server) handleUpdateUserProfile(w http.ResponseWriter, r *http.Request) {
	username := s.getUsernameFromContext(r.Context())
	if username == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}

	// Check if this is a CLI admin user (can't edit profile)
	if s.config != nil && s.config.Auth != "" {
		au, _ := settings.ParseAuth(s.config.Auth)
		if username == au {
			http.Error(w, "CLI admin users cannot edit profile", http.StatusBadRequest)
			return
		}
	}

	var req struct {
		DisplayName     string `json:"display_name,omitempty"`
		Email           string `json:"email,omitempty"`
		CurrentPassword string `json:"current_password,omitempty"`
		NewPassword     string `json:"new_password,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Get current user from database
	dbUser, err := s.db.GetUser(username)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Check if user is using SSO (can't change email)
	authMethod := r.Context().Value("authMethod")
	if authMethod == "auth0" && req.Email != "" && req.Email != dbUser.Email {
		http.Error(w, "Cannot change email when using SSO", http.StatusBadRequest)
		return
	}

	// If changing password, verify current password
	if req.NewPassword != "" {
		if req.CurrentPassword == "" {
			http.Error(w, "Current password required to change password", http.StatusBadRequest)
			return
		}
		if dbUser.Password != req.CurrentPassword {
			http.Error(w, "Current password is incorrect", http.StatusBadRequest)
			return
		}
		dbUser.Password = req.NewPassword
	}

	// Update fields
	if req.DisplayName != "" {
		dbUser.DisplayName = strings.TrimSpace(req.DisplayName)
	}
	if req.Email != "" && authMethod != "auth0" {
		dbUser.Email = strings.TrimSpace(req.Email)
	}

	// Update in database
	if err := s.db.UpdateUser(dbUser); err != nil {
		s.Debugf("Failed to update user profile: %v", err)
		http.Error(w, "Failed to update profile", http.StatusInternalServerError)
		return
	}

	// Return success
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// Helper functions
func (s *Server) getUsernameFromContext(ctx context.Context) string {
	if username, ok := ctx.Value("username").(string); ok {
		return username
	}
	return ""
}

func (s *Server) isUserAdmin(ctx context.Context) bool {
	username := s.getUsernameFromContext(ctx)
	if username == "" {
		return false
	}

	// Check --auth admin
	if s.config != nil && s.config.Auth != "" {
		au, _ := settings.ParseAuth(s.config.Auth)
		if username == au {
			return true
		}
	}

	// Check database user
	if s.db != nil {
		if dbUser, err := s.db.GetUser(username); err == nil && dbUser != nil {
			return dbUser.IsAdmin
		}
	}

	return false
}
