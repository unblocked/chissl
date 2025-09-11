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
