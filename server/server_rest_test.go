package chserver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/NextChapterSoftware/chissl/share/settings"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

const debug = true

// test layout configuration
type testLayout struct {
	server     *Config
	serverPort string
}

func (tl *testLayout) GetServerPort() string { return tl.serverPort }
func (tl *testLayout) setup(t *testing.T) (server *Server, teardown func()) {
	//start of the world
	ctx, cancel := context.WithCancel(context.Background())

	//server
	server, err := NewServer(tl.server)
	if err != nil {
		t.Fatal(err)
	}
	server.Debug = debug
	port := "8080" //availablePort()
	tl.serverPort = port
	if err := server.StartContext(ctx, "127.0.0.1", port); err != nil {
		t.Fatal(err)
	}
	go func() {
		server.Wait()
		server.Infof("Closed")
		cancel()
	}()

	//cancel context tree, and wait for both client and server to stop
	teardown = func() {
		cancel()
		server.Wait()
		//confirm goroutines have been cleaned up
		// time.Sleep(500 * time.Millisecond)
		// TODO remove sleep
		// d := runtime.NumGoroutine() - goroutines
		// if d != 0 {
		// 	pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
		// 	t.Fatalf("goroutines left %d", d)
		// }
	}
	//wait a bit...
	//TODO: client signal API, similar to os.Notify(signal)
	//      wait for client setup
	time.Sleep(50 * time.Millisecond)
	//ready
	return server, teardown
}

func simpleSetup(t *testing.T, s *Config) (context.CancelFunc, *testLayout) {
	conf := testLayout{
		server: s,
	}
	_, teardown := conf.setup(t)
	return teardown, &conf
}

func createTempAuthFile(t *testing.T) string {
	t.Helper()
	authFileContent := `[
				 {"username":"root","password":"toor1234","addresses":[".*"],"is_admin":true},
				 {"username":"foo","password":"bar12345","addresses":["^9001","^9002","^9003"],"is_admin":false},
				 {"username":"ping","password":"pong1234","addresses":["^80[0-9]{2}"],"is_admin":false}
				]`
	tmpFile, err := os.CreateTemp("", "auth-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	if _, err := tmpFile.Write([]byte(authFileContent)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}

	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	return tmpFile.Name()
}

func httpRequestWithBodyWithBasicAuth(method, url, body, username, password string) (string, error) {
	// Create a new request with the given URL and body
	req, err := http.NewRequest(strings.ToUpper(method), url, strings.NewReader(body))
	if err != nil {
		return "", err
	}

	// Set the Basic Auth header
	auth := username + ":" + password
	encodedAuth := base64.StdEncoding.EncodeToString([]byte(auth))
	req.Header.Set("Authorization", "Basic "+encodedAuth)

	// Create a custom http.Client with the Basic Auth header
	client := &http.Client{}

	// Perform the request using the custom client
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Read the response body
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

func httpRequestNoBodyWithBasicAuth(method, url, username, password string) (string, error) {
	// Create a new request
	req, err := http.NewRequest(strings.ToUpper(method), url, nil)
	if err != nil {
		return "", err
	}

	// Set the Basic Auth header
	auth := username + ":" + password
	encodedAuth := base64.StdEncoding.EncodeToString([]byte(auth))
	req.Header.Set("Authorization", "Basic "+encodedAuth)

	// Perform the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Read the response body
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func get(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func TestHttpRequestNoAuth(t *testing.T) {

	teardown, tl := simpleSetup(t, &Config{})
	defer teardown()

	result, err := get("http://127.0.0.1:" + tl.GetServerPort() + "/health")
	if err != nil {
		t.Fatal(err)
	}
	if result != "OK\n" {
		t.Fatalf("vailid path with no auth - expected OK\\n but got '%s'", result)
	}

	result, err = get("http://127.0.0.1:" + tl.GetServerPort() + "/healthIgnoreExtraPathComponents/123")
	if err != nil {
		t.Fatal(err)
	}
	if result != "OK\n" {
		t.Fatalf("path with valid prefix and no auth - expected OK\\n but got '%s'", result)
	}

	result, err = get("http://127.0.0.1:" + tl.GetServerPort() + "/ThishealthEndpointDoesntexist")
	if err != nil {
		t.Fatal(err)
	}
	if result != "Not found" {
		t.Fatalf("invalid path - expected 'Not found' but got '%s'", result)
	}
}

func TestGetUserWithAuth(t *testing.T) {
	teardown, tl := simpleSetup(t, &Config{
		Auth: "admin:password",
	})

	// No auth file configured - Must fail
	result, err := get("http://127.0.0.1:" + tl.GetServerPort() + "/user/admin")
	if err != nil {
		t.Fatal(err)
	}
	if result != "Unauthorized\n" {
		t.Fatalf("no auth info - expected 'Unauthorized' but got '%s'", result)
	}
	teardown()

	authFilePath := createTempAuthFile(t)
	defer os.Remove(authFilePath)
	teardown, tl = simpleSetup(t, &Config{
		AuthFile: authFilePath,
	})
	defer teardown()

	// No auth info provided - Must fail
	result, err = get("http://127.0.0.1:" + tl.GetServerPort() + "/user/admin")
	if err != nil {
		t.Fatal(err)
	}
	if result != "Unauthorized\n" {
		t.Fatalf("no auth info - expected 'Unauthorized' but got '%s'", result)
	}

	// Correct auth info provided - Must pass
	result, err = httpRequestNoBodyWithBasicAuth(
		http.MethodGet,
		"http://127.0.0.1:"+tl.GetServerPort()+"/user/ping",
		"root",
		"toor1234",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result != "{\"username\":\"ping\",\"password\":\"pong1234\",\"addresses\":[\"^80[0-9]{2}\"],\"is_admin\":false}" {
		t.Fatalf("valid auth info - expected user info json but got '%s'", result)
	}

	// Wrong auth info provided - Must fail
	result, err = httpRequestNoBodyWithBasicAuth(
		http.MethodGet,
		"http://127.0.0.1:"+tl.GetServerPort()+"/user/admin",
		"root",
		"WrongPassword",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result != "Unauthorized\n" {
		t.Fatalf("invalid password - expected 'Unauthorized' but got '%s'", result)
	}

	// Wrong auth info provided - Must fail
	result, err = httpRequestNoBodyWithBasicAuth(
		http.MethodGet,
		"http://127.0.0.1:"+tl.GetServerPort()+"/user/admin",
		"userthatdoesntexist",
		"password",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result != "Unauthorized\n" {
		t.Fatalf("invalid username - expected 'Unauthorized' but got '%s'", result)
	}
}

func TestAddUserWithAuth(t *testing.T) {

	authFilePath := createTempAuthFile(t)
	defer os.Remove(authFilePath)
	teardown, tl := simpleSetup(t, &Config{
		AuthFile: authFilePath,
	})
	defer teardown()

	u := &settings.User{
		Name:    "nonAdminUser",
		Pass:    "password1",
		Addrs:   []*regexp.Regexp{settings.UserAllowAll},
		IsAdmin: false,
	}

	userJson, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}

	result, err := httpRequestWithBodyWithBasicAuth(
		http.MethodPost,
		"http://127.0.0.1:"+tl.GetServerPort()+"/user",
		string(userJson),
		"root",
		"toor1234",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result != "" {
		t.Fatalf("create valid user - expected to '' but got '%s'", result)
	}

	// Correct auth info provided - Must pass
	result, err = httpRequestNoBodyWithBasicAuth(
		http.MethodGet,
		"http://127.0.0.1:"+tl.GetServerPort()+"/user/nonAdminUser",
		"root",
		"toor1234",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result != "{\"username\":\"nonAdminUser\",\"password\":\"password1\",\"addresses\":[\".*\"],\"is_admin\":false}" {
		t.Fatalf("get new userinfo - expected user info json but got '%s'", result)
	}

	result, err = httpRequestWithBodyWithBasicAuth(
		http.MethodPost,
		"http://127.0.0.1:"+tl.GetServerPort()+"/user",
		string(userJson),
		"root",
		"toor1234",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result != "User already exists\n" {
		t.Fatalf("created duplicate user - expected to 'ser already exists\\n' but got '%s'", result)
	}
}

func TestUpdateUserWithAuth(t *testing.T) {

	authFilePath := createTempAuthFile(t)
	defer os.Remove(authFilePath)
	teardown, tl := simpleSetup(t, &Config{
		AuthFile: authFilePath,
	})
	defer teardown()
	u := &UpdateUserRequest{
		Name: "foo",
		//Pass:    "password1",
		Addrs:   []*regexp.Regexp{settings.UserAllowAll},
		IsAdmin: true,
	}

	userJson, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}

	result, err := httpRequestWithBodyWithBasicAuth(
		http.MethodPut,
		"http://127.0.0.1:"+tl.GetServerPort()+"/user",
		string(userJson),
		"root",
		"toor1234",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result != "" {
		t.Fatalf("update valid user - expected to '' but got '%s'", result)
	}

	// Correct auth info provided - Must pass
	result, err = httpRequestNoBodyWithBasicAuth(
		http.MethodGet,
		"http://127.0.0.1:"+tl.GetServerPort()+"/user/"+u.Name,
		"root",
		"toor1234",
	)
	if err != nil {
		t.Fatal(err)
	}
	updatedUser := &settings.User{}
	err = json.Unmarshal(userJson, updatedUser)
	if err != nil {
		t.Fatal(err)
	}

	if !updatedUser.IsAdmin {
		t.Fatalf("failed to update admin flag for user")
	}

	if len(updatedUser.Addrs) != 1 {
		t.Fatalf("failed to update addresses user")
	}
}

func TestDeleteUserWithAuth(t *testing.T) {

	authFilePath := createTempAuthFile(t)
	defer os.Remove(authFilePath)
	teardown, tl := simpleSetup(t, &Config{
		AuthFile: authFilePath,
	})
	defer teardown()

	// Invalid auth info provided - Must fail
	result, err := httpRequestNoBodyWithBasicAuth(
		http.MethodDelete,
		"http://127.0.0.1:"+tl.GetServerPort()+"/user/ping",
		"root",
		"WrongPassword",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result != "Unauthorized\n" {
		t.Fatalf("no auth info - expected 'Unauthorized' but got '%s'", result)
	}

	// Correct auth but user doesn't exist - Must fail
	result, err = httpRequestNoBodyWithBasicAuth(
		http.MethodDelete,
		"http://127.0.0.1:"+tl.GetServerPort()+"/user/admin",
		"root",
		"toor1234",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(result, "User not found") {
		t.Fatalf("user does not exist - expected 'User not found' but got '%s'", result)
	}

	// Deleting requester (self) user - Must fail
	result, err = httpRequestNoBodyWithBasicAuth(
		http.MethodDelete,
		"http://127.0.0.1:"+tl.GetServerPort()+"/user/root",
		"root",
		"toor1234",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(result, "Cannot delete your own user") {
		t.Fatalf("no deleting self - expected user 'Cannot delete your own user' but got '%s'", result)
	}

	// Valid delete request requester (self) user - Must pass
	result, err = httpRequestNoBodyWithBasicAuth(
		http.MethodDelete,
		"http://127.0.0.1:"+tl.GetServerPort()+"/user/ping",
		"root",
		"toor1234",
	)
	if err != nil {
		t.Fatal(err)
	}
	if result != "" {
		t.Fatalf("no deleting self - expected user to pass but got '%s'", result)
	}

	// Verify user has been deleted - Must pass
	result, err = httpRequestNoBodyWithBasicAuth(
		http.MethodDelete,
		"http://127.0.0.1:"+tl.GetServerPort()+"/user/ping",
		"root",
		"toor1234",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(result, "User not found") {
		t.Fatalf("verify delete success - expected 'User not found' but got '%s'", result)
	}
}

func TestGetUsersWithAuth(t *testing.T) {
	authFilePath := createTempAuthFile(t)
	defer os.Remove(authFilePath)
	teardown, tl := simpleSetup(t, &Config{
		AuthFile: authFilePath,
	})
	defer teardown()

	// No auth info provided - Must fail
	result, err := get("http://127.0.0.1:" + tl.GetServerPort() + "/users")
	if err != nil {
		t.Fatal(err)
	}
	if result != "Unauthorized\n" {
		t.Fatalf("no auth info - expected 'Unauthorized' but got '%s'", result)
	}

	// Correct auth info provided - Must pass
	result, err = httpRequestNoBodyWithBasicAuth(
		http.MethodGet,
		"http://127.0.0.1:"+tl.GetServerPort()+"/users",
		"root",
		"toor1234",
	)
	if err != nil {
		t.Fatal(err)
	}
	users := []*settings.User{}
	err = json.Unmarshal([]byte(result), &users)
	if len(users) != 3 {
		t.Fatalf("list of all users - expected user info json for all users but got '%s'", result)
	}
}

func TestAddAuthfile(t *testing.T) {

	authFilePath := createTempAuthFile(t)
	defer os.Remove(authFilePath)
	teardown, tl := simpleSetup(t, &Config{
		AuthFile: authFilePath,
	})
	defer teardown()

	// Invalid auth file - admin flag for requesting user is being set to false
	usersJson := `[
				 {"username":"root","password":"toor1234","addresses":[".*"],"is_admin":false},
				 {"username":"foo","password":"bar12345","addresses":["^9001","^9002","^9003"],"is_admin":false},
				 {"username":"ping","password":"pong1234","addresses":["^80[0-9]{2}"],"is_admin":false}
				]`

	result, err := httpRequestWithBodyWithBasicAuth(
		http.MethodPost,
		"http://127.0.0.1:"+tl.GetServerPort()+"/authfile",
		usersJson,
		"root",
		"toor1234",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(result, "file must include the current requesting user with admin permission") {
		t.Fatalf("invalid file - expected to fail but got '%s'", result)
	}

	// Invalid auth file - empty file
	result, err = httpRequestWithBodyWithBasicAuth(
		http.MethodPost,
		"http://127.0.0.1:"+tl.GetServerPort()+"/authfile",
		"[]",
		"root",
		"toor1234",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(result, "No users found in file") {
		t.Fatalf("invalid file - expected to fail but got '%s'", result)
	}

	// Invalid user settings in file - foo has no addresses assigned
	usersJson = `[
				 {"username":"root","password":"toor1234","addresses":[".*"],"is_admin":true},
				 {"username":"foo","password":"bar12345","addresses":[],"is_admin":false},
				 {"username":"ping","password":"pong1234","addresses":["^80[0-9]{2}"],"is_admin":false}
				]`

	result, err = httpRequestWithBodyWithBasicAuth(
		http.MethodPost,
		"http://127.0.0.1:"+tl.GetServerPort()+"/authfile",
		usersJson,
		"root",
		"toor1234",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(result, "invalid user setting for") {
		t.Fatalf("invalid file - expected to fail but got '%s'", result)
	}

	// Valid auth file with new user and modifications to existing user
	usersJson = `[
				 {"username":"root","password":"toor1234","addresses":[".*"],"is_admin":true},
				 {"username":"foo","password":"bar12345","addresses":["^9001"],"is_admin":false},
				 {"username":"newuser","password":"newuser1234","addresses":["^80[0-9]{2}"],"is_admin":false}
				]`

	result, err = httpRequestWithBodyWithBasicAuth(
		http.MethodPost,
		"http://127.0.0.1:"+tl.GetServerPort()+"/authfile",
		usersJson,
		"root",
		"toor1234",
	)
	if err != nil {
		t.Fatal(err)
	}

	if result != "" {
		t.Fatalf("valid file - expected to pass but got '%s'", result)
	}

	// Deleted user foo - Must fail
	result, err = httpRequestNoBodyWithBasicAuth(
		http.MethodGet,
		"http://127.0.0.1:"+tl.GetServerPort()+"/user/foor",
		"root",
		"toor1234",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(result, "User not found") {
		t.Fatalf("deleted user- expected user to fail but got '%s'", result)
	}

	// Newly created user - Must pass
	result, err = httpRequestNoBodyWithBasicAuth(
		http.MethodGet,
		"http://127.0.0.1:"+tl.GetServerPort()+"/user/newuser",
		"root",
		"toor1234",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "newuser1234") {
		t.Fatalf("valid user info - expected user info json but got '%s'", result)
	}

}
