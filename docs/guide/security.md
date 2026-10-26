# Security Controls

## Auth enforcement
- All /api/* endpoints are protected; public endpoints are listed in the API reference
- Admin-only routes use admin middleware; user routes require user auth
- Supported credentials: dashboard session cookie, `Authorization: Bearer <api token>`,
  Auth0 JWT (when enabled), and HTTP basic auth

## Sessions
- Dashboard and SSO logins issue an opaque, random session token. Only a SHA-256 of the
  token is stored (in the `sessions` table, or in memory when no database is configured)
- The cookie is `HttpOnly`, `SameSite=Lax`, and `Secure` on HTTPS; it expires after
  `session_ttl_minutes` and is revoked on logout
- State-changing requests authenticated by cookie are rejected when the `Origin` or
  `Sec-Fetch-Site` headers indicate a cross-site request

## Password storage
- Database passwords are stored as bcrypt hashes. Rows created before hashing are
  upgraded transparently on the user's next successful login
- Accounts provisioned through SSO have no local password and cannot log in with one
- Passwords are never returned by any API response

## Outbound requests
- Proxy listener targets and redirects are restricted to public addresses; loopback,
  private, link-local (including cloud metadata) and other non-routable ranges are refused
- Tunnel clients cannot ask the server to dial arbitrary hosts; all remotes are reverse

## Secrets at rest
- AI provider API keys are encrypted with AES-256-GCM. Set `CHISSL_ENCRYPTION_KEY`
  (32 bytes, or any longer passphrase) in the server environment; without it a built-in
  default key is used and a warning is logged at startup
- SSO client secrets are write-only through the API (`has_client_secret` reports presence)

## Login backoff
- Exponential backoff and temporary lockouts after repeated failures
- Optional per-IP buckets

## IP rate limiting
- Max requests per minute per IP with temporary ban on abuse

## Security events & webhooks
- In-memory event ring with optional DB persistence
- Dispatch to JSON or Slack-formatted webhooks
