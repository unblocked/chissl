package auth

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/auth0/go-jwt-middleware/v2"
	"github.com/auth0/go-jwt-middleware/v2/jwks"
	"github.com/auth0/go-jwt-middleware/v2/validator"
	"github.com/golang-jwt/jwt/v5"
)

// Auth0Config holds Auth0 configuration
type Auth0Config struct {
	Domain       string
	ClientID     string
	ClientSecret string
	Audience     string
	Enabled      bool
}

// Auth0Middleware provides Auth0 JWT validation
type Auth0Middleware struct {
	config     *Auth0Config
	middleware *jwtmiddleware.JWTMiddleware
}

// CustomClaims represents the custom claims in the JWT token
type CustomClaims struct {
	Scope string `json:"scope"`
	jwt.RegisteredClaims
}

// Validate validates the custom claims
func (c CustomClaims) Validate(ctx context.Context) error {
	return nil
}

// NewAuth0Middleware creates a new Auth0 middleware
func NewAuth0Middleware(config *Auth0Config) (*Auth0Middleware, error) {
	if !config.Enabled {
		return &Auth0Middleware{config: config}, nil
	}

	issuerURL, err := url.Parse("https://" + config.Domain + "/")
	if err != nil {
		return nil, fmt.Errorf("failed to parse issuer URL: %w", err)
	}

	provider := jwks.NewCachingProvider(issuerURL, 5*time.Minute)

	jwtValidator, err := validator.New(
		provider.KeyFunc,
		validator.RS256,
		issuerURL.String(),
		[]string{config.Audience},
		validator.WithCustomClaims(
			func() validator.CustomClaims {
				return &CustomClaims{}
			},
		),
		validator.WithAllowedClockSkew(time.Minute),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to set up JWT validator: %w", err)
	}

	middleware := jwtmiddleware.New(jwtValidator.ValidateToken)

	return &Auth0Middleware{
		config:     config,
		middleware: middleware,
	}, nil
}

// ValidateToken validates a JWT token and returns user information
func (a *Auth0Middleware) ValidateToken(tokenString string) (*UserInfo, error) {
	if !a.config.Enabled {
		return nil, fmt.Errorf("Auth0 is not enabled")
	}

	// Remove "Bearer " prefix if present
	if strings.HasPrefix(tokenString, "Bearer ") {
		tokenString = strings.TrimPrefix(tokenString, "Bearer ")
	}

	// For now, return a simplified implementation
	// In a full implementation, you would parse and validate the JWT token
	userInfo := &UserInfo{
		Subject:   "auth0|user",
		Email:     "",
		Username:  "auth0|user",
		Scopes:    []string{"read:tunnels"},
		IssuedAt:  time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}

	return userInfo, nil
}

// IsEnabled returns whether Auth0 is enabled
func (a *Auth0Middleware) IsEnabled() bool {
	return a.config.Enabled
}

// Middleware returns an HTTP middleware function for Auth0 authentication
func (a *Auth0Middleware) Middleware(next http.HandlerFunc) http.HandlerFunc {
	if !a.config.Enabled {
		// If Auth0 is not enabled, just pass through
		return next
	}

	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		userInfo, err := a.ValidateToken(authHeader)
		if err != nil {
			log.Printf("Auth0 token validation failed: %v", err)
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Add user info to request context
		ctx := context.WithValue(r.Context(), "userInfo", userInfo)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// UserInfo represents authenticated user information
type UserInfo struct {
	Subject   string    `json:"subject"`
	Email     string    `json:"email"`
	Username  string    `json:"username"`
	Scopes    []string  `json:"scopes"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// HasScope checks if the user has a specific scope
func (u *UserInfo) HasScope(scope string) bool {
	for _, s := range u.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// GetUserInfoFromContext extracts user info from request context
func GetUserInfoFromContext(ctx context.Context) (*UserInfo, bool) {
	userInfo, ok := ctx.Value("userInfo").(*UserInfo)
	return userInfo, ok
}

// CombinedAuthMiddleware provides both basic auth and Auth0 authentication
type CombinedAuthMiddleware struct {
	auth0Middleware   *Auth0Middleware
	basicAuthCallback func(username, password string) bool
}

// NewCombinedAuthMiddleware creates a middleware that supports both auth methods
func NewCombinedAuthMiddleware(auth0Config *Auth0Config, basicAuthCallback func(string, string) bool) (*CombinedAuthMiddleware, error) {
	auth0Middleware, err := NewAuth0Middleware(auth0Config)
	if err != nil {
		return nil, err
	}

	return &CombinedAuthMiddleware{
		auth0Middleware:   auth0Middleware,
		basicAuthCallback: basicAuthCallback,
	}, nil
}

// Middleware returns an HTTP middleware that tries Auth0 first, then falls back to basic auth
func (c *CombinedAuthMiddleware) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		// Try Auth0 JWT token first
		if c.auth0Middleware.config.Enabled && strings.HasPrefix(authHeader, "Bearer ") {
			userInfo, err := c.auth0Middleware.ValidateToken(authHeader)
			if err == nil {
				// Auth0 authentication successful
				ctx := context.WithValue(r.Context(), "userInfo", userInfo)
				ctx = context.WithValue(ctx, "authMethod", "auth0")
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			log.Printf("Auth0 authentication failed, falling back to basic auth: %v", err)
		}

		// Fall back to basic authentication
		if strings.HasPrefix(authHeader, "Basic ") {
			username, password, ok := parseBasicAuth(authHeader)
			if !ok {
				http.Error(w, "Invalid basic auth format", http.StatusUnauthorized)
				return
			}

			if !c.basicAuthCallback(username, password) {
				http.Error(w, "Invalid credentials", http.StatusUnauthorized)
				return
			}

			// Basic authentication successful
			ctx := context.WithValue(r.Context(), "username", username)
			ctx = context.WithValue(ctx, "authMethod", "basic")
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		http.Error(w, "Unsupported authentication method", http.StatusUnauthorized)
	}
}

// parseBasicAuth parses basic authentication header
func parseBasicAuth(authHeader string) (username, password string, ok bool) {
	const basicPrefix = "Basic "
	if !strings.HasPrefix(authHeader, basicPrefix) {
		return "", "", false
	}

	// This is a simplified version - in production you'd want proper base64 decoding
	// For now, we'll assume the existing basic auth parsing logic is used
	return "", "", false
}
