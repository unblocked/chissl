package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	chserver "github.com/NextChapterSoftware/chissl/server"
	"github.com/NextChapterSoftware/chissl/share/database"
)

// TestAdminMiddleware tests that admin middleware properly rejects non-admin users
func TestAdminMiddleware(t *testing.T) {
	// Create temporary database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	// Create database
	dbConfig := &database.DatabaseConfig{
		Type:     "sqlite",
		FilePath: dbPath,
	}
	db := database.NewDatabase(dbConfig)
	if err := db.Connect(); err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	// Create server with database
	config := &chserver.Config{
		Database: dbConfig,
	}
	srv, err := chserver.NewServer(config)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer srv.Shutdown()

	// Create test users
	adminUser := &database.User{
		Username: "admin",
		Password: "adminpass",
		Email:    "admin@test.com",
		IsAdmin:  true,
	}
	regularUser := &database.User{
		Username: "user",
		Password: "userpass",
		Email:    "user@test.com",
		IsAdmin:  false,
	}

	if err := db.CreateUser(adminUser); err != nil {
		t.Fatalf("Failed to create admin user: %v", err)
	}
	if err := db.CreateUser(regularUser); err != nil {
		t.Fatalf("Failed to create regular user: %v", err)
	}

	// Create a test handler that should only be accessible to admins
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("admin access granted"))
	})

	// Test regular user access (should be denied)
	t.Run("Regular User Should Be Denied", func(t *testing.T) {
		req := createAuthenticatedRequest(t, "GET", "/test", nil, "user", "userpass")
		rr := httptest.NewRecorder()

		// Use the admin middleware to protect the handler
		protectedHandler := srv.CombinedAuthMiddleware(testHandler)
		protectedHandler.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized && rr.Code != http.StatusForbidden {
			t.Errorf("Expected 401 or 403 for regular user, got %d. Response: %s",
				rr.Code, rr.Body.String())
		}
	})

	// Test admin user access (should be allowed)
	t.Run("Admin User Should Be Allowed", func(t *testing.T) {
		req := createAuthenticatedRequest(t, "GET", "/test", nil, "admin", "adminpass")
		rr := httptest.NewRecorder()

		// Use the admin middleware to protect the handler
		protectedHandler := srv.CombinedAuthMiddleware(testHandler)
		protectedHandler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200 for admin user, got %d. Response: %s",
				rr.Code, rr.Body.String())
		}

		if rr.Body.String() != "admin access granted" {
			t.Errorf("Expected 'admin access granted', got '%s'", rr.Body.String())
		}
	})

	// Test unauthenticated access (should be denied)
	t.Run("Unauthenticated User Should Be Denied", func(t *testing.T) {
		req := createUnauthenticatedRequest(t, "GET", "/test", nil)
		rr := httptest.NewRecorder()

		// Use the admin middleware to protect the handler
		protectedHandler := srv.CombinedAuthMiddleware(testHandler)
		protectedHandler.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401 for unauthenticated user, got %d. Response: %s",
				rr.Code, rr.Body.String())
		}
	})

	// Cleanup
	db.Close()
}

// TestUserAuthMiddleware tests that user auth middleware properly authenticates users
func TestUserAuthMiddleware(t *testing.T) {
	// Create temporary database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	// Create database
	dbConfig := &database.DatabaseConfig{
		Type:     "sqlite",
		FilePath: dbPath,
	}
	db := database.NewDatabase(dbConfig)
	if err := db.Connect(); err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	// Create server with database
	config := &chserver.Config{
		Database: dbConfig,
	}
	srv, err := chserver.NewServer(config)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer srv.Shutdown()

	// Create test user
	user := &database.User{
		Username: "testuser",
		Password: "testpass",
		Email:    "test@test.com",
		IsAdmin:  false,
	}

	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create a test handler that should be accessible to authenticated users
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username := r.Context().Value("username").(string)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("user access granted: " + username))
	})

	// Test valid user access (should be allowed)
	t.Run("Valid User Should Be Allowed", func(t *testing.T) {
		req := createAuthenticatedRequest(t, "GET", "/test", nil, "testuser", "testpass")
		rr := httptest.NewRecorder()

		// Use the user auth middleware to protect the handler
		protectedHandler := srv.UserAuthMiddleware(testHandler)
		protectedHandler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200 for valid user, got %d. Response: %s",
				rr.Code, rr.Body.String())
		}

		if rr.Body.String() != "user access granted: testuser" {
			t.Errorf("Expected 'user access granted: testuser', got '%s'", rr.Body.String())
		}
	})

	// Test invalid credentials (should be denied)
	t.Run("Invalid Credentials Should Be Denied", func(t *testing.T) {
		req := createAuthenticatedRequest(t, "GET", "/test", nil, "testuser", "wrongpass")
		rr := httptest.NewRecorder()

		// Use the user auth middleware to protect the handler
		protectedHandler := srv.UserAuthMiddleware(testHandler)
		protectedHandler.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401 for invalid credentials, got %d. Response: %s",
				rr.Code, rr.Body.String())
		}
	})

	// Test unauthenticated access (should be denied)
	t.Run("Unauthenticated User Should Be Denied", func(t *testing.T) {
		req := createUnauthenticatedRequest(t, "GET", "/test", nil)
		rr := httptest.NewRecorder()

		// Use the user auth middleware to protect the handler
		protectedHandler := srv.UserAuthMiddleware(testHandler)
		protectedHandler.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401 for unauthenticated user, got %d. Response: %s",
				rr.Code, rr.Body.String())
		}
	})

	// Cleanup
	db.Close()
}

// Helper function to create authenticated HTTP request
func createAuthenticatedRequest(t *testing.T, method, path string, body interface{}, username, password string) *http.Request {
	var req *http.Request
	var err error

	if body != nil {
		jsonBody, _ := json.Marshal(body)
		req, err = http.NewRequest(method, path, bytes.NewBuffer(jsonBody))
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequest(method, path, nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
	}

	// Add basic auth
	req.SetBasicAuth(username, password)
	return req
}

// Helper function to create unauthenticated HTTP request
func createUnauthenticatedRequest(t *testing.T, method, path string, body interface{}) *http.Request {
	var req *http.Request
	var err error

	if body != nil {
		jsonBody, _ := json.Marshal(body)
		req, err = http.NewRequest(method, path, bytes.NewBuffer(jsonBody))
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequest(method, path, nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
	}

	return req
}

// TestAPITokenAuthentication tests API token-based authentication
func TestAPITokenAuthentication(t *testing.T) {
	// Create temporary database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	// Create database
	dbConfig := &database.DatabaseConfig{
		Type:     "sqlite",
		FilePath: dbPath,
	}
	db := database.NewDatabase(dbConfig)
	if err := db.Connect(); err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	// Create server with database
	config := &chserver.Config{
		Database: dbConfig,
	}
	srv, err := chserver.NewServer(config)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer srv.Shutdown()

	// Create test user
	user := &database.User{
		Username: "tokenuser",
		Password: "tokenpass",
		Email:    "token@test.com",
		IsAdmin:  false,
	}

	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create API token
	expiresAt := time.Now().Add(24 * time.Hour)
	token := &database.UserToken{
		ID:        "test-token-1",
		Username:  "tokenuser",
		Name:      "Test token",
		Token:     "test_api_token_12345",
		CreatedAt: time.Now(),
		ExpiresAt: &expiresAt,
	}

	if err := db.CreateUserToken(token); err != nil {
		t.Fatalf("Failed to create API token: %v", err)
	}

	// Create a test handler that should be accessible to authenticated users
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username := r.Context().Value("username").(string)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("token access granted: " + username))
	})

	// Test with valid API token
	t.Run("Valid API Token Should Be Accepted", func(t *testing.T) {
		req := createUnauthenticatedRequest(t, "GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer test_api_token_12345")
		rr := httptest.NewRecorder()

		// Use the user auth middleware to protect the handler
		protectedHandler := srv.UserAuthMiddleware(testHandler)
		protectedHandler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200 for valid token, got %d. Response: %s",
				rr.Code, rr.Body.String())
		}

		if rr.Body.String() != "token access granted: tokenuser" {
			t.Errorf("Expected 'token access granted: tokenuser', got '%s'", rr.Body.String())
		}
	})

	// Test with invalid API token
	t.Run("Invalid API Token Should Be Rejected", func(t *testing.T) {
		req := createUnauthenticatedRequest(t, "GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer invalid_token")
		rr := httptest.NewRecorder()

		// Use the user auth middleware to protect the handler
		protectedHandler := srv.UserAuthMiddleware(testHandler)
		protectedHandler.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401 for invalid token, got %d. Response: %s",
				rr.Code, rr.Body.String())
		}
	})

	// Cleanup
	db.Close()
}

// TestUserDeletionCleanup tests that user deletion properly cleans up all resources
func TestUserDeletionCleanup(t *testing.T) {
	// Create temporary database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	// Create database
	dbConfig := &database.DatabaseConfig{
		Type:     "sqlite",
		FilePath: dbPath,
	}
	db := database.NewDatabase(dbConfig)
	if err := db.Connect(); err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	// Create server with database
	config := &chserver.Config{
		Database: dbConfig,
	}
	srv, err := chserver.NewServer(config)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer srv.Shutdown()

	// Create test user
	user := &database.User{
		Username: "testuser",
		Password: "testpass",
		Email:    "test@test.com",
		IsAdmin:  false,
	}

	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create some resources for the user

	// 1. Create API token
	expiresAt := time.Now().Add(24 * time.Hour)
	token := &database.UserToken{
		ID:        "test-token-1",
		Username:  "testuser",
		Name:      "Test token",
		Token:     "test_api_token_12345",
		CreatedAt: time.Now(),
		ExpiresAt: &expiresAt,
	}

	if err := db.CreateUserToken(token); err != nil {
		t.Fatalf("Failed to create API token: %v", err)
	}

	// 2. Create port reservation
	reservation := &database.PortReservation{
		ID:          "test-reservation-1",
		Username:    "testuser",
		StartPort:   8000,
		EndPort:     8010,
		Description: "Test reservation",
		CreatedAt:   time.Now(),
	}

	if err := db.CreatePortReservation(reservation); err != nil {
		t.Fatalf("Failed to create port reservation: %v", err)
	}

	// Verify resources exist before deletion
	tokens, err := db.ListUserTokens("testuser")
	if err != nil || len(tokens) == 0 {
		t.Fatalf("Expected user tokens to exist before deletion")
	}

	reservations, err := db.ListUserPortReservations("testuser")
	if err != nil || len(reservations) == 0 {
		t.Fatalf("Expected port reservations to exist before deletion")
	}

	// Test user deletion via API
	req := createAuthenticatedRequest(t, "DELETE", "/user/testuser", nil, "admin", "adminpass")
	rr := httptest.NewRecorder()

	// Create admin user for deletion
	adminUser := &database.User{
		Username: "admin",
		Password: "adminpass",
		Email:    "admin@test.com",
		IsAdmin:  true,
	}

	if err := db.CreateUser(adminUser); err != nil {
		t.Fatalf("Failed to create admin user: %v", err)
	}

	// Use the admin middleware to protect the handler
	deleteHandler := srv.CombinedAuthMiddleware(srv.HandleDeleteUser)
	deleteHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Errorf("Expected 202 for user deletion, got %d. Response: %s",
			rr.Code, rr.Body.String())
	}

	// Verify user is deleted
	_, err = db.GetUser("testuser")
	if err == nil {
		t.Errorf("Expected user to be deleted")
	}

	// Verify resources are cleaned up
	tokens, err = db.ListUserTokens("testuser")
	if err == nil && len(tokens) > 0 {
		t.Errorf("Expected user tokens to be cleaned up, found %d tokens", len(tokens))
	}

	reservations, err = db.ListUserPortReservations("testuser")
	if err == nil && len(reservations) > 0 {
		t.Errorf("Expected port reservations to be cleaned up, found %d reservations", len(reservations))
	}

	// Cleanup
	db.Close()
}

// TestProtectedEndpointsRequireAuth verifies protected endpoints return 401 without auth
func TestProtectedEndpointsRequireAuth(t *testing.T) {
	// Setup temporary DB and server
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	dbConfig := &database.DatabaseConfig{Type: "sqlite", FilePath: dbPath}
	db := database.NewDatabase(dbConfig)
	if err := db.Connect(); err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	srv, err := chserver.NewServer(&chserver.Config{Database: dbConfig})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer srv.Shutdown()
	h := srv.HTTPHandler()

	// Endpoints that must be protected (require auth)
	protected := []struct{ method, path string }{
		{"GET", "/api/users"},
		{"GET", "/api/listeners"},
		{"GET", "/api/sessions"},
		{"GET", "/api/system"},
		{"GET", "/api/stats"},
		{"GET", "/api/ai-providers"}, // admin-only
	}

	for _, ep := range protected {
		req := httptest.NewRequest(ep.method, ep.path, nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: expected 401 when unauthenticated, got %d (body: %s)", ep.method, ep.path, rr.Code, rr.Body.String())
		}
	}
}

// TestProtectedEndpointsWithBasicAuth verifies protected endpoints succeed with valid auth
func TestProtectedEndpointsWithBasicAuth(t *testing.T) {
	// Setup temporary DB and server
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	dbConfig := &database.DatabaseConfig{Type: "sqlite", FilePath: dbPath}
	db := database.NewDatabase(dbConfig)
	if err := db.Connect(); err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	// Create admin user
	adminUser := &database.User{Username: "admin", Password: "adminpass", IsAdmin: true}
	if err := db.CreateUser(adminUser); err != nil {
		t.Fatalf("Failed to create admin user: %v", err)
	}

	srv, err := chserver.NewServer(&chserver.Config{Database: dbConfig})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer srv.Shutdown()
	h := srv.HTTPHandler()

	protected := []struct{ method, path string }{
		{"GET", "/api/users"},
		{"GET", "/api/listeners"},
		{"GET", "/api/sessions"},
		{"GET", "/api/system"},
		{"GET", "/api/stats"},
		{"GET", "/api/ai-providers"}, // admin-only
	}

	for _, ep := range protected {
		req := httptest.NewRequest(ep.method, ep.path, nil)
		req.SetBasicAuth("admin", "adminpass")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code == http.StatusUnauthorized {
			t.Errorf("%s %s: expected non-401 with valid auth, got 401 (body: %s)", ep.method, ep.path, rr.Body.String())
		}

	}
}

// TestAdminAPIsDeniedToRegularUser ensures admin-only APIs are not accessible to regular users
func TestAdminAPIsDeniedToRegularUser(t *testing.T) {
	// Setup temporary DB and server
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	dbConfig := &database.DatabaseConfig{Type: "sqlite", FilePath: dbPath}
	db := database.NewDatabase(dbConfig)
	if err := db.Connect(); err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}
	defer db.Close()

	// Create users: one admin and one regular user
	adminUser := &database.User{Username: "admin", Password: "adminpass", IsAdmin: true}
	if err := db.CreateUser(adminUser); err != nil {
		t.Fatalf("Failed to create admin user: %v", err)
	}
	regularUser := &database.User{Username: "user", Password: "userpass", IsAdmin: false}
	if err := db.CreateUser(regularUser); err != nil {
		t.Fatalf("Failed to create regular user: %v", err)
	}

	srv, err := chserver.NewServer(&chserver.Config{Database: dbConfig})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer srv.Shutdown()
	h := srv.HTTPHandler()

	adminOnly := []struct{ method, path string }{
		{"GET", "/api/users"},
		{"GET", "/api/connections"},
		{"GET", "/api/sessions"},
		{"GET", "/api/logs"},
		{"GET", "/api/port-reservations"},
		{"GET", "/api/settings/login-backoff"},
	}

	for _, ep := range adminOnly {
		req := httptest.NewRequest(ep.method, ep.path, nil)
		req.SetBasicAuth("user", "userpass") // regular user
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if !(rr.Code == http.StatusUnauthorized || rr.Code == http.StatusForbidden) {
			t.Errorf("%s %s: expected 401/403 for regular user, got %d (body: %s)", ep.method, ep.path, rr.Code, rr.Body.String())
		}
	}
}

// TestAdminAPIsAllowedForAdminUser ensures admin-only APIs are accessible to an admin user
func TestAdminAPIsAllowedForAdminUser(t *testing.T) {
	// Setup temporary DB and server
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	dbConfig := &database.DatabaseConfig{Type: "sqlite", FilePath: dbPath}
	db := database.NewDatabase(dbConfig)
	if err := db.Connect(); err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}
	defer db.Close()

	// Create admin user
	adminUser := &database.User{Username: "admin", Password: "adminpass", IsAdmin: true}
	if err := db.CreateUser(adminUser); err != nil {
		t.Fatalf("Failed to create admin user: %v", err)
	}

	srv, err := chserver.NewServer(&chserver.Config{Database: dbConfig})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer srv.Shutdown()
	h := srv.HTTPHandler()

	adminOnly := []struct{ method, path string }{
		{"GET", "/api/users"},
		{"GET", "/api/connections"},
		{"GET", "/api/sessions"},
		{"GET", "/api/logs"},
		{"GET", "/api/port-reservations"},
		{"GET", "/api/settings/login-backoff"},
	}

	for _, ep := range adminOnly {
		req := httptest.NewRequest(ep.method, ep.path, nil)
		req.SetBasicAuth("admin", "adminpass")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code == http.StatusUnauthorized || rr.Code == http.StatusForbidden {
			t.Errorf("%s %s: expected success for admin, got %d (body: %s)", ep.method, ep.path, rr.Code, rr.Body.String())
		}
	}
}

// TestAdminWriteAPIsAllowedForAdmin verifies admin can perform POST/PUT/DELETE on admin APIs
func TestAdminWriteAPIsAllowedForAdmin(t *testing.T) {
	// Setup temporary DB and server
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	dbConfig := &database.DatabaseConfig{Type: "sqlite", FilePath: dbPath}
	db := database.NewDatabase(dbConfig)
	if err := db.Connect(); err != nil {
		t.Fatalf("DB connect: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("DB migrate: %v", err)
	}
	defer db.Close()

	// Create admin user
	if err := db.CreateUser(&database.User{Username: "admin", Password: "adminpass", IsAdmin: true}); err != nil {
		t.Fatalf("create admin: %v", err)
	}

	srv, err := chserver.NewServer(&chserver.Config{Database: dbConfig})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	defer srv.Shutdown()
	h := srv.HTTPHandler()

	// 1) Admin creates a new user via API
	createUserBody := map[string]any{"username": "user2", "password": "pass2", "is_admin": false}
	b1, _ := json.Marshal(createUserBody)
	req := httptest.NewRequest("POST", "/api/users", bytes.NewBuffer(b1))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("admin", "adminpass")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create user: expected 201, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	// 2) Admin creates a port reservation for that user
	resBody := map[string]any{"username": "user2", "start_port": 8000, "end_port": 8001, "description": "test"}
	b2, _ := json.Marshal(resBody)
	req = httptest.NewRequest("POST", "/api/port-reservations", bytes.NewBuffer(b2))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("admin", "adminpass")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create reservation: expected 201, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var createdRes struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &createdRes)
	if createdRes.ID == "" {
		t.Fatalf("create reservation: missing id in response")
	}

	// 3) Admin can delete that reservation
	req = httptest.NewRequest("DELETE", "/api/port-reservations/"+createdRes.ID, nil)
	req.SetBasicAuth("admin", "adminpass")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("delete reservation: expected 204, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	// 4) Admin can update user via API (promote to admin)
	updateBody := map[string]any{"username": "user2", "is_admin": true}
	b3, _ := json.Marshal(updateBody)
	req = httptest.NewRequest("PUT", "/api/users", bytes.NewBuffer(b3))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("admin", "adminpass")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK && rr.Code != http.StatusAccepted {
		t.Fatalf("update user: expected 200/202, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	// 5) Admin can set reserved ports threshold
	thrBody := map[string]any{"threshold": 12000}
	b4, _ := json.Marshal(thrBody)
	req = httptest.NewRequest("POST", "/api/settings/reserved-ports-threshold", bytes.NewBuffer(b4))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("admin", "adminpass")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("set threshold: expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	// 6) Admin can delete user via legacy route
	req = httptest.NewRequest("DELETE", "/user/user2", nil)
	req.SetBasicAuth("admin", "adminpass")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("delete user: expected 202, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

// TestAdminWriteAPIsDeniedToRegular ensures regular users cannot hit admin write endpoints
func TestAdminWriteAPIsDeniedToRegular(t *testing.T) {
	// Setup temporary DB and server
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	dbConfig := &database.DatabaseConfig{Type: "sqlite", FilePath: dbPath}
	db := database.NewDatabase(dbConfig)
	if err := db.Connect(); err != nil {
		t.Fatalf("DB connect: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("DB migrate: %v", err)
	}
	defer db.Close()

	// Create admin and regular users
	_ = db.CreateUser(&database.User{Username: "admin", Password: "adminpass", IsAdmin: true})
	_ = db.CreateUser(&database.User{Username: "user", Password: "userpass", IsAdmin: false})

	srv, err := chserver.NewServer(&chserver.Config{Database: dbConfig})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	defer srv.Shutdown()
	h := srv.HTTPHandler()

	denyCases := []struct {
		method, path string
		body         any
	}{
		{"POST", "/api/users", map[string]any{"username": "x", "password": "y"}},
		{"PUT", "/api/users", map[string]any{"username": "user", "is_admin": true}},
		{"POST", "/api/port-reservations", map[string]any{"username": "user", "start_port": 8000, "end_port": 8001}},
		{"POST", "/api/settings/reserved-ports-threshold", map[string]any{"threshold": 12000}},
	}

	for _, tc := range denyCases {
		var buf *bytes.Buffer
		if tc.body != nil {
			b, _ := json.Marshal(tc.body)
			buf = bytes.NewBuffer(b)
		} else {
			buf = bytes.NewBuffer(nil)
		}
		req := httptest.NewRequest(tc.method, tc.path, buf)
		req.Header.Set("Content-Type", "application/json")
		req.SetBasicAuth("user", "userpass")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if !(rr.Code == http.StatusUnauthorized || rr.Code == http.StatusForbidden) {
			t.Errorf("%s %s: expected 401/403 for regular, got %d (body: %s)", tc.method, tc.path, rr.Code, rr.Body.String())
		}
	}

	// Legacy delete should also be denied to regular user
	req := httptest.NewRequest("DELETE", "/user/user", nil)
	req.SetBasicAuth("user", "userpass")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if !(rr.Code == http.StatusUnauthorized || rr.Code == http.StatusForbidden) {
		t.Errorf("DELETE /user/user: expected 401/403 for regular, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}
