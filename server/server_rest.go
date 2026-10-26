package chserver

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/NextChapterSoftware/chissl/share/ccrypto"
	"github.com/NextChapterSoftware/chissl/share/database"
	"github.com/NextChapterSoftware/chissl/share/settings"
)

// dbUserRequest is the JSON body accepted when creating or updating a database user.
// Passwords are accepted here and hashed before storage; they are never serialized back.
type dbUserRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password,omitempty"`
	Email       string `json:"email,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	IsAdmin     bool   `json:"is_admin"`
	Addresses   string `json:"addresses"`
}

func (r dbUserRequest) toUser() *database.User {
	return &database.User{
		Username:    r.Username,
		Password:    r.Password,
		Email:       r.Email,
		DisplayName: r.DisplayName,
		IsAdmin:     r.IsAdmin,
		Addresses:   r.Addresses,
	}
}

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

// cleanupUserResources performs comprehensive cleanup of all user resources
func (s *Server) cleanupUserResources(username string) error {
	s.Infof("Starting cleanup for user: %s", username)

	// 1. Close all active SSH sessions for this user
	if err := s.closeUserSessions(username); err != nil {
		s.Debugf("Failed to close user sessions: %v", err)
	}

	// 2. Stop and delete all user's listeners
	if err := s.cleanupUserListeners(username); err != nil {
		s.Debugf("Failed to cleanup user listeners: %v", err)
	}

	// 3. Close all active tunnels for this user
	if err := s.cleanupUserTunnels(username); err != nil {
		s.Debugf("Failed to cleanup user tunnels: %v", err)
	}

	// 4. Delete all user's port reservations
	if err := s.cleanupUserPortReservations(username); err != nil {
		s.Debugf("Failed to cleanup user port reservations: %v", err)
	}

	// 5. Delete all user's API tokens
	if err := s.cleanupUserTokens(username); err != nil {
		s.Debugf("Failed to cleanup user tokens: %v", err)
	}

	// 6. Delete user's auth sources
	if err := s.cleanupUserAuthSources(username); err != nil {
		s.Debugf("Failed to cleanup user auth sources: %v", err)
	}

	s.Infof("Completed cleanup for user: %s", username)
	return nil
}

// closeUserSessions closes all active SSH sessions for a user
func (s *Server) closeUserSessions(username string) error {
	if s.sessions == nil {
		return nil
	}

	// Get all sessions and close ones belonging to this user
	sessionIDs := s.sessions.GetSessionIDs()
	for _, sessionID := range sessionIDs {
		if user, found := s.sessions.Get(sessionID); found && user.Name == username {
			s.sessions.Del(sessionID)
			s.Debugf("Closed session %s for user %s", sessionID, username)
		}
	}

	return nil
}

// cleanupUserListeners stops and deletes all listeners owned by a user
func (s *Server) cleanupUserListeners(username string) error {
	if s.db == nil {
		return nil
	}

	// Get all listeners for this user
	listeners, err := s.db.ListListeners()
	if err != nil {
		return fmt.Errorf("failed to list listeners: %w", err)
	}

	for _, listener := range listeners {
		if listener.Username == username {
			// Stop the listener if it's running
			if s.listeners != nil {
				if err := s.listeners.StopListener(listener.ID); err != nil {
					s.Debugf("Failed to stop listener %s: %v", listener.ID, err)
				}
			}

			// Delete from database
			if err := s.db.DeleteListener(listener.ID); err != nil {
				s.Debugf("Failed to delete listener %s: %v", listener.ID, err)
			} else {
				s.Debugf("Deleted listener %s for user %s", listener.ID, username)
			}
		}
	}

	return nil
}

// cleanupUserTunnels closes and deletes all tunnels owned by a user
func (s *Server) cleanupUserTunnels(username string) error {
	if s.db == nil {
		return nil
	}

	// Get all tunnels for this user
	tunnels, err := s.db.ListTunnels()
	if err != nil {
		return fmt.Errorf("failed to list tunnels: %w", err)
	}

	for _, tunnel := range tunnels {
		if tunnel.Username == username {
			// Mark tunnel as closed (this will trigger cleanup in active connections)
			tunnel.Status = "closed"
			if err := s.db.UpdateTunnel(tunnel); err != nil {
				s.Debugf("Failed to update tunnel status %s: %v", tunnel.ID, err)
			}

			// Delete from database
			if err := s.db.DeleteTunnel(tunnel.ID); err != nil {
				s.Debugf("Failed to delete tunnel %s: %v", tunnel.ID, err)
			} else {
				s.Debugf("Deleted tunnel %s for user %s", tunnel.ID, username)
			}
		}
	}

	// Also clean up from in-memory live tunnels if not using database
	if s.liveTunnels != nil {
		s.liveMu.Lock()
		for tunnelID, tunnel := range s.liveTunnels {
			if tunnel.Username == username {
				delete(s.liveTunnels, tunnelID)
				s.Debugf("Removed live tunnel %s for user %s", tunnelID, username)
			}
		}
		s.liveMu.Unlock()
	}

	return nil
}

// cleanupUserPortReservations deletes all port reservations for a user
func (s *Server) cleanupUserPortReservations(username string) error {
	if s.db == nil {
		return nil
	}

	// Get all port reservations for this user
	reservations, err := s.db.ListUserPortReservations(username)
	if err != nil {
		return fmt.Errorf("failed to list user port reservations: %w", err)
	}

	for _, reservation := range reservations {
		if err := s.db.DeletePortReservation(reservation.ID); err != nil {
			s.Debugf("Failed to delete port reservation %s: %v", reservation.ID, err)
		} else {
			s.Debugf("Deleted port reservation %s (ports %d-%d) for user %s",
				reservation.ID, reservation.StartPort, reservation.EndPort, username)
		}
	}

	return nil
}

// cleanupUserTokens deletes all API tokens for a user
func (s *Server) cleanupUserTokens(username string) error {
	if s.db == nil {
		return nil
	}

	// Get all tokens for this user
	tokens, err := s.db.ListUserTokens(username)
	if err != nil {
		return fmt.Errorf("failed to list user tokens: %w", err)
	}

	for _, token := range tokens {
		if err := s.db.DeleteUserToken(token.ID); err != nil {
			s.Debugf("Failed to delete user token %s: %v", token.ID, err)
		} else {
			s.Debugf("Deleted API token %s for user %s", token.ID, username)
		}
	}

	return nil
}

// cleanupUserAuthSources deletes all auth sources for a user
func (s *Server) cleanupUserAuthSources(username string) error {
	if s.db == nil {
		return nil
	}

	// Get all auth sources for this user
	authSources, err := s.db.ListUserAuthSourcesByUsername(username)
	if err != nil {
		return fmt.Errorf("failed to list user auth sources: %w", err)
	}

	for _, authSource := range authSources {
		if err := s.db.DeleteUserAuthSource(authSource.ID); err != nil {
			s.Debugf("Failed to delete user auth source %d: %v", authSource.ID, err)
		} else {
			s.Debugf("Deleted auth source %d (%s) for user %s",
				authSource.ID, authSource.AuthSource, username)
		}
	}

	return nil
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

	// Fall back to file-based users (never expose passwords)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.users.Redacted())
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
	json.NewEncoder(w).Encode(u.Redacted())
}

func (s *Server) handleAddUser(w http.ResponseWriter, r *http.Request) {
	if s.db != nil {
		// Use database
		var req dbUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}

		// Basic validation
		if req.Username == "" || req.Password == "" {
			http.Error(w, "Username and password are required", http.StatusBadRequest)
			return
		}
		newUser := req.toUser()
		hash, err := ccrypto.HashPassword(req.Password)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		newUser.Password = hash

		// Check if user already exists
		if _, err := s.db.GetUser(newUser.Username); err == nil {
			http.Error(w, "User already exists", http.StatusConflict)
			return
		}

		// Create the user
		if err := s.db.CreateUser(newUser); err != nil {
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
		var req dbUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}
		targetUser := *req.toUser()

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

		// Preserve existing values if not provided; hash any new password
		if req.Password == "" {
			targetUser.Password = existingUser.Password
		} else {
			hash, err := ccrypto.HashPassword(req.Password)
			if err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			targetUser.Password = hash
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
	requestingUser := s.getCurrentUsername(r)

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

		// Perform comprehensive user cleanup
		if err := s.cleanupUserResources(username); err != nil {
			s.Debugf("Failed to cleanup user resources: %v", err)
			http.Error(w, "Failed to cleanup user resources", http.StatusInternalServerError)
			return
		}

		// Finally delete the user from database
		if err := s.db.DeleteUser(username); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		s.Infof("User %s deleted and all resources cleaned up", username)
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
	requestingUser := s.getCurrentUsername(r)
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
	requestingUser := s.getCurrentUsername(r)
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

		// Set password (hashed) or preserve the existing one
		if apiUser.Password != "" {
			hash, err := ccrypto.HashPassword(apiUser.Password)
			if err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			dbUser.Password = hash
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

// Exported middleware methods for testing

// CombinedAuthMiddleware is an exported version of combinedAuthMiddleware for testing
func (s *Server) CombinedAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return s.combinedAuthMiddleware(next)
}

// UserAuthMiddleware is an exported version of userAuthMiddleware for testing
func (s *Server) UserAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return s.userAuthMiddleware(next)
}

// BasicAuthMiddleware is an exported version of basicAuthMiddleware for testing
func (s *Server) BasicAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return s.basicAuthMiddleware(next)
}

// HandleDeleteUser is an exported version of handleDeleteUser for testing
func (s *Server) HandleDeleteUser(w http.ResponseWriter, r *http.Request) {
	s.handleDeleteUser(w, r)
}

// HTTPHandler exposes the main HTTP handler for testing
func (s *Server) HTTPHandler() http.Handler {
	return http.HandlerFunc(s.handleClientHandler)
}
