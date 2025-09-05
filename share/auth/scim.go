package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/NextChapterSoftware/chissl/share/database"
)

// SCIMMiddleware handles SCIM-based authentication
type SCIMMiddleware struct {
	db     database.Database
	config *database.SCIMConfig
}

// NewSCIMMiddleware creates a new SCIM middleware
func NewSCIMMiddleware(db database.Database, config *database.SCIMConfig) *SCIMMiddleware {
	return &SCIMMiddleware{
		db:     db,
		config: config,
	}
}

// SCIMUserInfo represents user information from SCIM provider
type SCIMUserInfo struct {
	ID          string `json:"sub"`                // Okta uses "sub" for user ID
	Username    string `json:"preferred_username"` // Okta uses "preferred_username"
	Email       string `json:"email"`
	DisplayName string `json:"name"`        // Okta uses "name" for display name
	FirstName   string `json:"given_name"`  // Okta uses "given_name"
	LastName    string `json:"family_name"` // Okta uses "family_name"
	Active      bool   `json:"active"`
}

// OAuthTokenResponse represents OAuth token response
type OAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// HandleLogin initiates SCIM OAuth flow
func (s *SCIMMiddleware) HandleLogin(w http.ResponseWriter, r *http.Request) {
	// Generate state parameter for CSRF protection
	state := s.generateState()

	// Store state in session/cookie for validation
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		MaxAge:   600, // 10 minutes
	})

	// Build authorization URL
	authURL, err := url.Parse(s.config.AuthURL)
	if err != nil {
		http.Error(w, "Invalid auth URL configuration", http.StatusInternalServerError)
		return
	}

	params := url.Values{}
	params.Add("client_id", s.config.ClientID)
	params.Add("response_type", "code")
	params.Add("redirect_uri", s.config.RedirectURL)
	params.Add("scope", strings.Join(s.config.Scopes, " "))
	params.Add("state", state)

	if s.config.TenantID != "" {
		params.Add("tenant", s.config.TenantID)
	}

	authURL.RawQuery = params.Encode()

	// Redirect to authorization server
	http.Redirect(w, r, authURL.String(), http.StatusFound)
}

// HandleCallback handles OAuth callback
func (s *SCIMMiddleware) HandleCallback(w http.ResponseWriter, r *http.Request) {
	// Validate state parameter
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return
	}

	// Clear state cookie
	http.SetCookie(w, &http.Cookie{
		Name:   "oauth_state",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	// Get authorization code
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Authorization code not provided", http.StatusBadRequest)
		return
	}

	// Exchange code for token
	token, err := s.exchangeCodeForToken(code)
	if err != nil {
		http.Error(w, "Failed to exchange code for token", http.StatusInternalServerError)
		return
	}

	// Get user info
	userInfo, err := s.getUserInfo(token.AccessToken)
	if err != nil {
		fmt.Printf("Failed to get user info: %v\n", err)
		http.Error(w, "Failed to get user information", http.StatusInternalServerError)
		return
	}

	// Debug: log the user info we received
	fmt.Printf("Received user info: %+v\n", userInfo)

	// Fallback: use email as username if preferred_username is empty
	if userInfo.Username == "" && userInfo.Email != "" {
		userInfo.Username = userInfo.Email
		fmt.Printf("Using email as username: %s\n", userInfo.Username)
	}

	// Validate that we have required fields
	if userInfo.Username == "" {
		fmt.Printf("Error: No username found in user info\n")
		http.Error(w, "No username found in user information", http.StatusInternalServerError)
		return
	}

	// Create or update user in database
	user, err := s.createOrUpdateUser(userInfo)
	if err != nil {
		// Log the actual error for debugging
		fmt.Printf("Failed to create/update user %s: %v\n", userInfo.Username, err)
		http.Error(w, "Failed to create/update user", http.StatusInternalServerError)
		return
	}

	// Create session using the same cookie name as the dashboard
	http.SetCookie(w, &http.Cookie{
		Name:     "chissl_session",
		Value:    user.Username,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		MaxAge:   86400, // 24 hours
	})

	// Store user info in context for session
	ctx := context.WithValue(r.Context(), "user", user)
	ctx = context.WithValue(ctx, "authMethod", "scim")

	// Redirect to dashboard
	http.Redirect(w, r.WithContext(ctx), "/dashboard", http.StatusFound)
}

// exchangeCodeForToken exchanges authorization code for access token
func (s *SCIMMiddleware) exchangeCodeForToken(code string) (*OAuthTokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("client_id", s.config.ClientID)
	data.Set("client_secret", s.config.ClientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", s.config.RedirectURL)

	req, err := http.NewRequest("POST", s.config.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed: %s", string(body))
	}

	var token OAuthTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, err
	}

	return &token, nil
}

// getUserInfo retrieves user information from SCIM provider
func (s *SCIMMiddleware) getUserInfo(accessToken string) (*SCIMUserInfo, error) {
	req, err := http.NewRequest("GET", s.config.UserInfoURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("user info request failed: %s", string(body))
	}

	// Debug: read and log the raw response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	fmt.Printf("Raw user info response: %s\n", string(body))

	var userInfo SCIMUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, fmt.Errorf("failed to parse user info JSON: %w", err)
	}

	return &userInfo, nil
}

// createOrUpdateUser creates or updates user in database
func (s *SCIMMiddleware) createOrUpdateUser(userInfo *SCIMUserInfo) (*database.User, error) {
	// Check if user exists
	existingUser, err := s.db.GetUser(userInfo.Username)
	if err != nil && !strings.Contains(err.Error(), "user not found") {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}

	var user *database.User
	if existingUser != nil {
		// Update existing user
		user = existingUser
		user.Email = userInfo.Email
		user.DisplayName = userInfo.DisplayName
		if err := s.db.UpdateUser(user); err != nil {
			return nil, fmt.Errorf("failed to update user in database: %w", err)
		}
	} else {
		// Create new user
		user = &database.User{
			Username:    userInfo.Username,
			Password:    "SSO_USER", // Placeholder password for SSO users
			Email:       userInfo.Email,
			DisplayName: userInfo.DisplayName,
			IsAdmin:     false, // SSO users are not admin by default
			Addresses:   "",    // Default empty addresses
		}
		if err := s.db.CreateUser(user); err != nil {
			return nil, fmt.Errorf("failed to create user in database: %w", err)
		}
	}

	// Create or update auth source
	authSource := &database.UserAuthSource{
		Username:   userInfo.Username,
		AuthSource: "scim",
		ExternalID: &userInfo.ID,
	}

	// Store provider data
	providerData, _ := json.Marshal(userInfo)
	providerDataStr := string(providerData)
	authSource.ProviderData = &providerDataStr

	existingAuthSource, _ := s.db.GetUserAuthSource(userInfo.Username)
	if existingAuthSource != nil {
		authSource.ID = existingAuthSource.ID
		if err := s.db.UpdateUserAuthSource(authSource); err != nil {
			return nil, fmt.Errorf("failed to update user auth source: %w", err)
		}
	} else {
		if err := s.db.CreateUserAuthSource(authSource); err != nil {
			return nil, fmt.Errorf("failed to create user auth source: %w", err)
		}
	}

	return user, nil
}

// generateState generates a random state parameter
func (s *SCIMMiddleware) generateState() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

// generateSessionToken generates a random session token
func (s *SCIMMiddleware) generateSessionToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

// IsEnabled returns whether SCIM is enabled
func (s *SCIMMiddleware) IsEnabled() bool {
	return s.config != nil
}
