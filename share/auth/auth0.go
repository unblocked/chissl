package auth

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/auth0/go-jwt-middleware/v2/jwks"
	"github.com/auth0/go-jwt-middleware/v2/validator"
)

// Auth0Config holds Auth0 configuration
type Auth0Config struct {
	Domain       string
	ClientID     string
	ClientSecret string
	Audience     string
	Enabled      bool
}

// Auth0Middleware validates Auth0-issued JWT access tokens.
type Auth0Middleware struct {
	config    *Auth0Config
	validator *validator.Validator
}

// CustomClaims represents the custom claims in the JWT token
type CustomClaims struct {
	Scope string `json:"scope"`
	Email string `json:"email"`
}

// Validate validates the custom claims
func (c *CustomClaims) Validate(ctx context.Context) error {
	return nil
}

// NewAuth0Middleware creates a new Auth0 middleware
func NewAuth0Middleware(config *Auth0Config) (*Auth0Middleware, error) {
	if !config.Enabled {
		return &Auth0Middleware{config: config}, nil
	}
	if config.Domain == "" || config.Audience == "" {
		return nil, errors.New("auth0: domain and audience are required")
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
		validator.WithCustomClaims(func() validator.CustomClaims { return &CustomClaims{} }),
		validator.WithAllowedClockSkew(time.Minute),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to set up JWT validator: %w", err)
	}

	return &Auth0Middleware{config: config, validator: jwtValidator}, nil
}

// ValidateToken verifies the token's signature (via the tenant JWKS), issuer,
// audience and expiry, and returns the authenticated user's information.
func (a *Auth0Middleware) ValidateToken(tokenString string) (*UserInfo, error) {
	if a == nil || a.config == nil || !a.config.Enabled || a.validator == nil {
		return nil, errors.New("auth0 is not enabled")
	}
	tokenString = strings.TrimSpace(strings.TrimPrefix(tokenString, "Bearer "))
	if tokenString == "" {
		return nil, errors.New("empty token")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	res, err := a.validator.ValidateToken(ctx, tokenString)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	claims, ok := res.(*validator.ValidatedClaims)
	if !ok || claims.RegisteredClaims.Subject == "" {
		return nil, errors.New("invalid token claims")
	}

	info := &UserInfo{
		Subject:  claims.RegisteredClaims.Subject,
		Username: claims.RegisteredClaims.Subject,
	}
	if custom, ok := claims.CustomClaims.(*CustomClaims); ok && custom != nil {
		info.Email = custom.Email
		info.Scopes = strings.Fields(custom.Scope)
		if custom.Email != "" {
			info.Username = custom.Email
		}
	}
	if claims.RegisteredClaims.IssuedAt != 0 {
		info.IssuedAt = time.Unix(claims.RegisteredClaims.IssuedAt, 0)
	}
	if claims.RegisteredClaims.Expiry != 0 {
		info.ExpiresAt = time.Unix(claims.RegisteredClaims.Expiry, 0)
	}
	return info, nil
}

// IsEnabled returns whether Auth0 is enabled
func (a *Auth0Middleware) IsEnabled() bool {
	return a != nil && a.config != nil && a.config.Enabled
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
