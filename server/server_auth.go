package chserver

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/NextChapterSoftware/chissl/share/auth"
	"github.com/NextChapterSoftware/chissl/share/ccrypto"
	"github.com/NextChapterSoftware/chissl/share/database"
	"github.com/NextChapterSoftware/chissl/share/session"
	"github.com/NextChapterSoftware/chissl/share/settings"
)

// ctxKey is the private type for request-context values set by the auth middleware.
type ctxKey int

const (
	ctxKeyUsername ctxKey = iota
	ctxKeyAuthMethod
	ctxKeyUser     // *settings.User (authfile / --auth users)
	ctxKeyUserInfo // *auth.UserInfo (Auth0)
	ctxKeyDBUser   // *database.User (Auth0 auto-provisioned)
)

// Authentication methods recorded in the request context.
const (
	authMethodSession  = "session"
	authMethodAPIToken = "api_token"
	authMethodAuth0    = "auth0"
	authMethodBasic    = "basic"
)

// identity is the result of authenticating a request.
type identity struct {
	Username string
	Method   string
	User     *settings.User
	UserInfo *auth.UserInfo
	DBUser   *database.User
}

// context returns ctx annotated with this identity.
func (id *identity) context(ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, ctxKeyUsername, id.Username)
	ctx = context.WithValue(ctx, ctxKeyAuthMethod, id.Method)
	if id.User != nil {
		ctx = context.WithValue(ctx, ctxKeyUser, id.User)
	}
	if id.UserInfo != nil {
		ctx = context.WithValue(ctx, ctxKeyUserInfo, id.UserInfo)
	}
	if id.DBUser != nil {
		ctx = context.WithValue(ctx, ctxKeyDBUser, id.DBUser)
	}
	return ctx
}

func usernameFromContext(ctx context.Context) string {
	if u, ok := ctx.Value(ctxKeyUsername).(string); ok {
		return u
	}
	if info, ok := ctx.Value(ctxKeyUserInfo).(*auth.UserInfo); ok && info != nil {
		return info.Username
	}
	return ""
}

func authMethodFromContext(ctx context.Context) string {
	m, _ := ctx.Value(ctxKeyAuthMethod).(string)
	return m
}

// authError describes why a request could not be authenticated.
type authError struct {
	status     int
	message    string
	retryAfter time.Duration
}

func (e *authError) write(w http.ResponseWriter) {
	if e.retryAfter > 0 {
		w.Header().Set("Retry-After", fmt.Sprintf("%d", int(e.retryAfter.Seconds())))
	}
	http.Error(w, e.message, e.status)
}

var errUnauthorized = &authError{status: http.StatusUnauthorized, message: "Unauthorized"}

// adminUser returns the --auth admin user, if configured.
func (s *Server) adminUser() (name, pass string, ok bool) {
	if s.config == nil || s.config.Auth == "" {
		return "", "", false
	}
	name, pass = settings.ParseAuth(s.config.Auth)
	return name, pass, name != ""
}

// userExists reports whether username is a known account in any identity source.
func (s *Server) userExists(username string) bool {
	if username == "" {
		return false
	}
	if au, _, ok := s.adminUser(); ok && au == username {
		return true
	}
	if s.db != nil {
		if u, err := s.db.GetUser(username); err == nil && u != nil {
			return true
		}
	}
	_, found := s.users.Get(username)
	return found
}

// verifyPassword checks a username/password pair against, in order, the --auth
// admin, database users and authfile users. It returns the resolved user on
// success. Legacy plaintext database passwords are upgraded to hashes on a
// successful login.
func (s *Server) verifyPassword(username, password string) (*settings.User, bool) {
	if username == "" || password == "" {
		return nil, false
	}
	if au, ap, ok := s.adminUser(); ok && ccrypto.ConstantTimeEquals(username, au) {
		if ccrypto.ConstantTimeEquals(password, ap) {
			return &settings.User{Name: au, Addrs: []*regexp.Regexp{settings.UserAllowAll}, IsAdmin: true}, true
		}
		return nil, false
	}
	if s.db != nil {
		if dbUser, err := s.db.GetUser(username); err == nil && dbUser != nil {
			ok, legacy := ccrypto.CheckPassword(dbUser.Password, password)
			if !ok {
				return nil, false
			}
			if legacy {
				if hash, err := ccrypto.HashPassword(password); err == nil {
					dbUser.Password = hash
					if err := s.db.UpdateUser(dbUser); err != nil {
						s.Debugf("Failed to upgrade password hash for %s: %v", username, err)
					}
				}
			}
			return &settings.User{Name: dbUser.Username, IsAdmin: dbUser.IsAdmin, Addrs: []*regexp.Regexp{settings.UserAllowAll}}, true
		}
	}
	if user, found := s.users.Get(username); found && user.Pass != "" && ccrypto.ConstantTimeEquals(user.Pass, password) {
		return user, true
	}
	return nil, false
}

// isUserAdmin reports whether the authenticated user in ctx has admin rights.
func (s *Server) isUserAdmin(ctx context.Context) bool {
	username := usernameFromContext(ctx)
	if username == "" {
		return false
	}
	if au, _, ok := s.adminUser(); ok && username == au {
		return true
	}
	if u, ok := ctx.Value(ctxKeyUser).(*settings.User); ok && u != nil {
		return u.IsAdmin
	}
	if u, ok := ctx.Value(ctxKeyDBUser).(*database.User); ok && u != nil {
		return u.IsAdmin
	}
	if s.db != nil {
		if dbUser, err := s.db.GetUser(username); err == nil && dbUser != nil {
			return dbUser.IsAdmin
		}
	}
	if u, found := s.users.Get(username); found {
		return u.IsAdmin
	}
	return false
}

// isCrossSiteRequest reports whether a state-changing request appears to
// originate from another site. It is applied to cookie-authenticated requests
// only, as a CSRF defence; API clients use Authorization headers and are not
// affected.
func isCrossSiteRequest(r *http.Request) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
		return true
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil {
			return true
		}
		host := r.Host
		if fh := r.Header.Get("X-Forwarded-Host"); fh != "" {
			host = fh
		}
		return !strings.EqualFold(u.Host, host)
	}
	return false
}

// sessionFromRequest resolves the session cookie, if any, to a username.
func (s *Server) sessionFromRequest(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(session.CookieName)
	if err != nil || cookie.Value == "" || s.sessionStore == nil {
		return "", false
	}
	username, err := s.sessionStore.Lookup(cookie.Value)
	if err != nil || !s.userExists(username) {
		return "", false
	}
	return username, true
}

// authenticateRequest resolves the caller's identity from, in order, the
// session cookie, a Bearer API token or Auth0 JWT, and HTTP basic auth.
func (s *Server) authenticateRequest(r *http.Request) (*identity, *authError) {
	if username, ok := s.sessionFromRequest(r); ok {
		if isCrossSiteRequest(r) {
			return nil, &authError{status: http.StatusForbidden, message: "Forbidden: cross-site request"}
		}
		return &identity{Username: username, Method: authMethodSession}, nil
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, errUnauthorized
	}

	if strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if s.db != nil {
			if userToken, err := s.db.ValidateUserToken(token); err == nil && userToken != nil {
				return &identity{Username: userToken.Username, Method: authMethodAPIToken}, nil
			}
		}
		if s.auth0 != nil {
			if userInfo, err := s.auth0.ValidateToken(token); err == nil && userInfo != nil && userInfo.Username != "" {
				id := &identity{Username: userInfo.Username, Method: authMethodAuth0, UserInfo: userInfo}
				if s.db != nil {
					dbUser, err := s.db.GetUser(userInfo.Username)
					if err != nil {
						// Auto-provision SSO users without a usable local password.
						dbUser = &database.User{Username: userInfo.Username, Email: userInfo.Email}
						if err := s.db.CreateUser(dbUser); err != nil {
							return nil, &authError{status: http.StatusInternalServerError, message: "Failed to create user"}
						}
					}
					id.DBUser = dbUser
				}
				return id, nil
			}
		}
		return nil, errUnauthorized
	}

	if strings.HasPrefix(authHeader, "Basic ") {
		username, password, ok := s.decodeBasicAuthHeader(r.Header)
		if !ok {
			return nil, &authError{status: http.StatusUnauthorized, message: "Invalid basic auth format"}
		}
		ip := s.clientIP(r)
		if locked, retry := s.ipRateCheck(ip); locked {
			return nil, &authError{status: http.StatusTooManyRequests, message: "Too many requests. Please try again later.", retryAfter: retry}
		}
		if username != "" {
			locked, retryAfter, delay := s.nextLoginDelayFor(username, ip)
			if locked {
				return nil, &authError{status: http.StatusTooManyRequests, message: "Too many login attempts. Please try again later.", retryAfter: retryAfter}
			}
			if delay > 0 {
				time.Sleep(delay)
			}
		}
		user, ok := s.verifyPassword(username, password)
		s.ipRateRecord(ip)
		if !ok {
			if username != "" {
				s.recordLoginFailureFor(username, ip)
			}
			return nil, errUnauthorized
		}
		s.resetLoginBackoffFor(username, ip)
		return &identity{Username: username, Method: authMethodBasic, User: user}, nil
	}

	return nil, &authError{status: http.StatusUnauthorized, message: "Unsupported authentication method"}
}

// userAuthMiddleware admits any authenticated user.
func (s *Server) userAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, aerr := s.authenticateRequest(r)
		if aerr != nil {
			aerr.write(w)
			return
		}
		next.ServeHTTP(w, r.WithContext(id.context(r.Context())))
	}
}

// combinedAuthMiddleware admits authenticated users with admin rights.
func (s *Server) combinedAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, aerr := s.authenticateRequest(r)
		if aerr != nil {
			aerr.write(w)
			return
		}
		ctx := id.context(r.Context())
		if !s.isUserAdmin(ctx) {
			http.Error(w, "Admin privileges required", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// basicAuthMiddleware admits admins authenticating with HTTP basic auth only.
func (s *Server) basicAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := s.decodeBasicAuthHeader(r.Header)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		user, ok := s.verifyPassword(username, password)
		if !ok || !user.IsAdmin {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		id := &identity{Username: username, Method: authMethodBasic, User: user}
		next.ServeHTTP(w, r.WithContext(id.context(r.Context())))
	}
}

// getCurrentUsername returns the authenticated username for a request.
func (s *Server) getCurrentUsername(r *http.Request) string {
	if username := usernameFromContext(r.Context()); username != "" {
		return username
	}
	if username, ok := s.sessionFromRequest(r); ok {
		return username
	}
	if username, password, ok := s.decodeBasicAuthHeader(r.Header); ok {
		if _, valid := s.verifyPassword(username, password); valid {
			return username
		}
	}
	return ""
}

// getUsernameFromContext is retained for handlers that only have a context.
func (s *Server) getUsernameFromContext(ctx context.Context) string {
	return usernameFromContext(ctx)
}

// sessionTTL returns the configured session lifetime.
func (s *Server) sessionTTL() time.Duration {
	minutes := 24 * 60
	if s.config != nil && s.config.Security.SessionTTLMinutes > 0 {
		minutes = s.config.Security.SessionTTLMinutes
	}
	return time.Duration(minutes) * time.Minute
}

// requestIsTLS reports whether the request arrived over HTTPS, directly or via a proxy.
func requestIsTLS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// loginSession issues a session for username and sets the session cookie.
func (s *Server) loginSession(w http.ResponseWriter, r *http.Request, username string) error {
	ttl := s.sessionTTL()
	token, err := s.sessionStore.Issue(username, s.clientIP(r), ttl)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     session.CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   requestIsTLS(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(ttl.Seconds()),
	})
	return nil
}

// logoutSession revokes the request's session and clears the cookie.
func (s *Server) logoutSession(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(session.CookieName); err == nil && cookie.Value != "" && s.sessionStore != nil {
		_ = s.sessionStore.Revoke(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     session.CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   requestIsTLS(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// UsernameFromContext returns the authenticated username stored by the auth
// middleware, or "" if the request was not authenticated.
func UsernameFromContext(ctx context.Context) string {
	return usernameFromContext(ctx)
}
