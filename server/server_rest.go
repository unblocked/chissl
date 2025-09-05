package chserver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/NextChapterSoftware/chissl/share/auth"
	"github.com/NextChapterSoftware/chissl/share/database"
	"github.com/NextChapterSoftware/chissl/share/settings"
)

type UpdateUserRequest struct {
	Name    string           `json:"username"`
	Pass    string           `json:"password,omitempty"`
	Addrs   []*regexp.Regexp `json:"addresses,omitempty"`
	IsAdmin bool             `json:"is_admin"`
}

// decodeBasicAuthHeader extracts the username and password from auth headers
func (s *Server) decodeBasicAuthHeader(headers http.Header) (username, password string, ok bool) {
	authHeader := headers.Get("Authorization")
	if authHeader == "" {
		return "", "", false
	}
	const basicAuthPrefix = "Basic "
	if !strings.HasPrefix(authHeader, basicAuthPrefix) {
		return "", "", false
	}

	// Decode the base64 encoded username:password
	decoded, err := base64.StdEncoding.DecodeString(authHeader[len(basicAuthPrefix):])
	if err != nil {
		return "", "", false
	}

	// Split the username and password
	credentials := strings.SplitN(string(decoded), ":", 2)
	if len(credentials) != 2 {
		return "", "", false
	}

	return credentials[0], credentials[1], true
}

// getCurrentUsername extracts the current username from the request context or headers
func (s *Server) getCurrentUsername(r *http.Request) string {
	// Try to get from context first (Auth0)
	if userInfo, ok := r.Context().Value("userInfo").(*auth.UserInfo); ok {
		return userInfo.Username
	}

	// Try to get from context (set by combinedAuthMiddleware)
	if username, ok := r.Context().Value("username").(string); ok {
		return username
	}

	// Try session cookie
	if cookie, err := r.Cookie("chissl_session"); err == nil && cookie.Value != "" {
		return cookie.Value
	}

	// Fall back to basic auth
	username, _, _ := s.decodeBasicAuthHeader(r.Header)
	return username
}

// UserAuthMiddleware validates authentication for any user (not just admins)
func (s *Server) userAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check for session cookie first (for dashboard)
		if cookie, err := r.Cookie("chissl_session"); err == nil && cookie.Value != "" {
			username := cookie.Value
			// Check --auth admin
			if s.config != nil && s.config.Auth != "" {
				au, _ := settings.ParseAuth(s.config.Auth)
				if username == au {
					ctx := r.Context()
					ctx = context.WithValue(ctx, "username", username)
					ctx = context.WithValue(ctx, "authMethod", "session")
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}
			// Check DB user
			if s.db != nil {
				if _, err := s.db.GetUser(username); err == nil {
					ctx := r.Context()
					ctx = context.WithValue(ctx, "username", username)
					ctx = context.WithValue(ctx, "authMethod", "session")
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}
			// Check authfile user
			if _, found := s.users.Get(username); found {
				ctx := r.Context()
				ctx = context.WithValue(ctx, "username", username)
				ctx = context.WithValue(ctx, "authMethod", "session")
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Try Auth0 JWT token first
		if s.auth0 != nil && strings.HasPrefix(authHeader, "Bearer ") {
			userInfo, err := s.auth0.ValidateToken(authHeader)
			if err == nil {
				// Auth0 authentication successful
				ctx := r.Context()
				ctx = context.WithValue(ctx, "userInfo", userInfo)
				ctx = context.WithValue(ctx, "authMethod", "auth0")
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		// Fall back to basic authentication
		if strings.HasPrefix(authHeader, "Basic ") {
			username, password, ok := s.decodeBasicAuthHeader(r.Header)
			if !ok {
				http.Error(w, "Invalid basic auth format", http.StatusUnauthorized)
				return
			}

			var user *settings.User
			var found bool

			// Check in-memory users first
			user, found = s.users.Get(username)
			if found && (username != user.Name || password != user.Pass) {
				found = false
			}

			// If not found in memory and database is available, check database
			if !found && s.db != nil {
				dbUser, err := s.db.GetUser(username)
				if err == nil && dbUser.Password == password {
					// Convert database user to settings user for compatibility
					user = &settings.User{
						Name:    dbUser.Username,
						Pass:    dbUser.Password,
						IsAdmin: dbUser.IsAdmin,
					}
					found = true
				}
			}

			if !found {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Basic authentication successful (any user, not just admin)
			ctx := r.Context()
			ctx = context.WithValue(ctx, "username", username)
			ctx = context.WithValue(ctx, "user", user)
			ctx = context.WithValue(ctx, "authMethod", "basic")
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}
}

// BasicAuthMiddleware validates the username and password
func (s *Server) basicAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := s.decodeBasicAuthHeader(r.Header)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Accept --auth admin
		if s.config != nil && s.config.Auth != "" {
			au, ap := settings.ParseAuth(s.config.Auth)
			if username == au && password == ap {
				next.ServeHTTP(w, r)
				return
			}
		}
		// Then DB
		if s.db != nil {
			if dbUser, err := s.db.GetUser(username); err == nil {
				if dbUser != nil && dbUser.Password == password && dbUser.IsAdmin {
					next.ServeHTTP(w, r)
					return
				}
			}
		}
		// Finally authfile/in-memory
		if u, found := s.users.Get(username); found && u.Pass == password && u.IsAdmin {
			next.ServeHTTP(w, r)
			return
		}
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}
}

// CombinedAuthMiddleware validates both Auth0 JWT tokens and basic auth
func (s *Server) combinedAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check for session cookie first (for dashboard) — session user can be admin from --auth or DB
		if cookie, err := r.Cookie("chissl_session"); err == nil && cookie.Value != "" {
			username := cookie.Value
			// --auth admin
			if s.config != nil && s.config.Auth != "" {
				au, _ := settings.ParseAuth(s.config.Auth)
				if username == au {
					ctx := r.Context()
					ctx = context.WithValue(ctx, "username", username)
					ctx = context.WithValue(ctx, "authMethod", "session")
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}
			// DB admin
			if s.db != nil {
				if dbUser, err := s.db.GetUser(username); err == nil && dbUser.IsAdmin {
					ctx := r.Context()
					ctx = context.WithValue(ctx, "username", username)
					ctx = context.WithValue(ctx, "authMethod", "session")
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}
			// Authfile admin
			if user, found := s.users.Get(username); found && user.IsAdmin {
				ctx := r.Context()
				ctx = context.WithValue(ctx, "username", username)
				ctx = context.WithValue(ctx, "user", user)
				ctx = context.WithValue(ctx, "authMethod", "session")
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Try Auth0 JWT token first if enabled
		if s.auth0 != nil && strings.HasPrefix(authHeader, "Bearer ") {
			userInfo, err := s.auth0.ValidateToken(authHeader)
			if err == nil {
				// Auth0 authentication successful
				// Check if user exists in our system or create them
				if s.db != nil {
					dbUser, err := s.db.GetUser(userInfo.Username)
					if err != nil {
						// User doesn't exist, create them (if auto-provisioning is enabled)
						dbUser = &database.User{
							Username:  userInfo.Username,
							Password:  "",    // No password for Auth0 users
							IsAdmin:   false, // Default to non-admin
							Addresses: "",
						}
						if err := s.db.CreateUser(dbUser); err != nil {
							http.Error(w, "Failed to create user", http.StatusInternalServerError)
							return
						}
					}

					// Add user info to context
					ctx := r.Context()
					ctx = context.WithValue(ctx, "userInfo", userInfo)
					ctx = context.WithValue(ctx, "dbUser", dbUser)
					ctx = context.WithValue(ctx, "authMethod", "auth0")
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}
		}

		// Fall back to basic authentication
		if strings.HasPrefix(authHeader, "Basic ") {
			username, password, ok := s.decodeBasicAuthHeader(r.Header)
			if !ok {
				http.Error(w, "Invalid basic auth format", http.StatusUnauthorized)
				return
			}

			var user *settings.User
			var found bool

			// Check in-memory users first
			user, found = s.users.Get(username)
			if found && (username != user.Name || password != user.Pass) {
				found = false
			}

			// If not found in memory and database is available, check database
			if !found && s.db != nil {
				dbUser, err := s.db.GetUser(username)
				if err == nil && dbUser.Password == password {
					// Convert database user to settings user for compatibility
					user = &settings.User{
						Name:    dbUser.Username,
						Pass:    dbUser.Password,
						IsAdmin: dbUser.IsAdmin,
					}
					found = true
				}
			}

			if !found || !user.IsAdmin {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Basic authentication successful
			ctx := r.Context()
			ctx = context.WithValue(ctx, "username", username)
			ctx = context.WithValue(ctx, "user", user)
			ctx = context.WithValue(ctx, "authMethod", "basic")
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		http.Error(w, "Unsupported authentication method", http.StatusUnauthorized)
	}
}

func getUsernameFromPath(path string) (string, error) {
	// Define the expected URL pattern
	pattern := `^/user/([^/]+)$`
	re := regexp.MustCompile(pattern)

	// Check if the path matches the pattern
	matches := re.FindStringSubmatch(path)
	if matches == nil {
		return "", fmt.Errorf("invalid URL format: %s", path)
	}

	// URL decode the username to handle special characters
	username, err := url.QueryUnescape(matches[1])
	if err != nil {
		return "", fmt.Errorf("failed to decode username: %w", err)
	}

	return username, nil
}

func (s *Server) handleGetUsers(w http.ResponseWriter, r *http.Request) {
	if s.db != nil {
		// Use database
		users, err := s.db.ListUsers()
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(users)
		return
	}

	// Fall back to file-based users
	data, err := s.users.ToJSON()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Write([]byte(data))
}

func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
	username, err := getUsernameFromPath(r.URL.Path)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if s.db != nil {
		// Use database
		user, err := s.db.GetUser(username)
		if err != nil {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(user)
		return
	}

	// Fall back to file-based users
	u, found := s.users.Get(username)
	if !found {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	responseJson, err := u.ToJSON()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Write([]byte(responseJson))
}

func (s *Server) handleAddUser(w http.ResponseWriter, r *http.Request) {
	if s.db != nil {
		// Use database
		var newUser database.User
		if err := json.NewDecoder(r.Body).Decode(&newUser); err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}

		// Basic validation
		if newUser.Username == "" || newUser.Password == "" {
			http.Error(w, "Username and password are required", http.StatusBadRequest)
			return
		}

		// Check if user already exists
		_, err := s.db.GetUser(newUser.Username)
		if err == nil {
			http.Error(w, "User already exists", http.StatusConflict)
			return
		}

		// Create the user
		if err := s.db.CreateUser(&newUser); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		return
	}

	// Fall back to file-based users
	var newUser settings.User
	if err := json.NewDecoder(r.Body).Decode(&newUser); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Validate the user input
	if err := newUser.ValidateUser(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, found := s.users.Get(newUser.Name)
	if found {
		http.Error(w, "User already exists", http.StatusConflict)
		return
	}

	// Add the user to the server's user collection
	s.users.Set(newUser.Name, &newUser)
	err := s.users.WriteUsers()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Respond with a status indicating success
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	if s.db != nil {
		// Use database
		var targetUser database.User
		if err := json.NewDecoder(r.Body).Decode(&targetUser); err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}

		// Check if user exists
		existingUser, err := s.db.GetUser(targetUser.Username)
		if err != nil {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}

		// Get current user making this request
		requestingUser := s.getCurrentUsername(r)

		// Admins cannot revoke admin permission from themselves
		if !targetUser.IsAdmin && targetUser.Username == requestingUser {
			http.Error(w, "Cannot revoke admin from yourself", http.StatusBadRequest)
			return
		}

		// Preserve existing values if not provided
		if targetUser.Password == "" {
			targetUser.Password = existingUser.Password
		}
		if targetUser.Addresses == "" {
			targetUser.Addresses = existingUser.Addresses
		}

		// Update the user
		if err := s.db.UpdateUser(&targetUser); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusAccepted)
		return
	}

	// Fall back to file-based users
	var targetUser settings.User
	if err := json.NewDecoder(r.Body).Decode(&targetUser); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	targetUserFromLookup, found := s.users.Get(targetUser.Name)
	if !found {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Get current user making this request
	requestingUser, _, _ := s.decodeBasicAuthHeader(r.Header)

	// Admins cannot revoke admin permission from themselves
	if !targetUser.IsAdmin && targetUser.Name == requestingUser {
		http.Error(w, "Cannot revoke admin from yourself", http.StatusBadRequest)
		return
	}

	if targetUser.Pass == "" {
		targetUser.Pass = targetUserFromLookup.Pass
	}

	if len(targetUser.Addrs) == 0 {
		targetUser.Addrs = targetUserFromLookup.Addrs
	}

	s.users.Set(targetUser.Name, &targetUser)
	err := s.users.WriteUsers()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	username, err := getUsernameFromPath(r.URL.Path)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if s.db != nil {
		// Use database
		_, err := s.db.GetUser(username)
		if err != nil {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}

		// Get current user making this request
		requestingUser := s.getCurrentUsername(r)
		if requestingUser == username {
			http.Error(w, "Cannot delete your own user", http.StatusBadRequest)
			return
		}

		if err := s.db.DeleteUser(username); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusAccepted)
		return
	}

	// Fall back to file-based users
	u, found := s.users.Get(username)
	if !found {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Get current user making this request
	requestingUser, _, _ := s.decodeBasicAuthHeader(r.Header)
	if requestingUser == u.Name {
		http.Error(w, "Cannot delete your own user", http.StatusBadRequest)
		return
	}

	s.users.Del(u.Name)
	err = s.users.WriteUsers()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleAuthfile(w http.ResponseWriter, r *http.Request) {
	// Parse the request body to create a User object
	var users []*settings.User
	if err := json.NewDecoder(r.Body).Decode(&users); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	if len(users) == 0 {
		http.Error(w, "No users found in file", http.StatusBadRequest)
	}

	// Get current user making this request
	requestingUser, _, _ := s.decodeBasicAuthHeader(r.Header)
	u, _ := s.users.Get(requestingUser)

	requestingUserFromPayload := &settings.User{}
	for _, user := range users {
		err := user.ValidateUser()
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid user setting for %s: %v", user.Name, err), http.StatusBadRequest)
		}
		if user.Name == u.Name {
			requestingUserFromPayload = user
		}
	}
	if requestingUserFromPayload == nil || !requestingUserFromPayload.IsAdmin {
		http.Error(w, "file must include the current requesting user with admin permission", http.StatusBadRequest)
		return
	}

	s.users.Reset(users)
	err := s.users.WriteUsers()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	// Implement user update logic here
	w.WriteHeader(http.StatusAccepted)
}

// handleCreateUser creates a new user (API version)
func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	// Check if user is admin
	if !s.isUserAdmin(r.Context()) {
		http.Error(w, "Admin privileges required", http.StatusForbidden)
		return
	}

	s.handleAddUser(w, r) // Reuse existing logic
}

// handleUpdateUserAPI updates a user (API version)
func (s *Server) handleUpdateUserAPI(w http.ResponseWriter, r *http.Request) {
	// Check if user is admin
	if !s.isUserAdmin(r.Context()) {
		http.Error(w, "Admin privileges required", http.StatusForbidden)
		return
	}

	// Parse the new API format
	var apiUser struct {
		Username string `json:"username"`
		Password string `json:"password,omitempty"`
		IsAdmin  bool   `json:"is_admin"`
	}

	if err := json.NewDecoder(r.Body).Decode(&apiUser); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	s.Debugf("handleUpdateUserAPI: updating user %s, isAdmin=%v", apiUser.Username, apiUser.IsAdmin)

	if s.db != nil {
		// Use database
		dbUser := &database.User{
			Username: apiUser.Username,
			IsAdmin:  apiUser.IsAdmin,
		}

		// Get existing user to preserve password if not provided
		existingUser, err := s.db.GetUser(apiUser.Username)
		if err != nil {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}

		// Set password
		if apiUser.Password != "" {
			dbUser.Password = apiUser.Password
		} else {
			dbUser.Password = existingUser.Password
		}

		// Preserve addresses
		dbUser.Addresses = existingUser.Addresses

		// Get current user making this request
		requestingUser := s.getCurrentUsername(r)

		// Admins cannot revoke admin permission from themselves
		if !apiUser.IsAdmin && apiUser.Username == requestingUser {
			http.Error(w, "Cannot revoke admin from yourself", http.StatusBadRequest)
			return
		}

		// Update the user
		if err := s.db.UpdateUser(dbUser); err != nil {
			s.Debugf("handleUpdateUserAPI: database update failed: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		s.Debugf("handleUpdateUserAPI: user updated successfully")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Fall back to in-memory users (convert to old format)
	targetUser := settings.User{
		Name:    apiUser.Username,
		IsAdmin: apiUser.IsAdmin,
	}

	// Get existing user
	existingUser, found := s.users.Get(apiUser.Username)
	if !found {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Set password
	if apiUser.Password != "" {
		targetUser.Pass = apiUser.Password
	} else {
		targetUser.Pass = existingUser.Pass
	}

	// Preserve addresses
	targetUser.Addrs = existingUser.Addrs

	// Get current user making this request
	requestingUser := s.getCurrentUsername(r)

	// Admins cannot revoke admin permission from themselves
	if !apiUser.IsAdmin && apiUser.Username == requestingUser {
		http.Error(w, "Cannot revoke admin from yourself", http.StatusBadRequest)
		return
	}

	s.users.Set(targetUser.Name, &targetUser)
	err := s.users.WriteUsers()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// handleDeleteUserAPI deletes a user (API version)
func (s *Server) handleDeleteUserAPI(w http.ResponseWriter, r *http.Request) {
	// Check if user is admin
	if !s.isUserAdmin(r.Context()) {
		http.Error(w, "Admin privileges required", http.StatusForbidden)
		return
	}

	s.handleDeleteUser(w, r) // Reuse existing logic
}
