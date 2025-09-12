# Web Dashboard

Manage tunnels, listeners, users, and view live traffic from a simple, responsive UI.

## Enable
```bash
# Recommended: automatic TLS with dashboard and admin auth
chissl server -v \
  --tls-domain your.domain.com \
  --auth admin:REPLACE_ME \
  --dashboard
```

## Access
- URL: https://your.domain.com/dashboard
- Custom path: `--dashboard-path /custom`

## Auth
- Basic auth by default (`--auth user:pass`)
- Optional Auth0 SSO (see Auth0 page)

## Sections
- Tunnels: list, details, delete; capture views for requests/responses
- Listeners: mock/proxy endpoints; create/update/delete
- Users (admin): list/create/update/delete
- Sessions: view active sessions
- System/Stats: server info and runtime metrics

## API (used by the dashboard)
- System/Stats
  - GET /api/system
  - GET /api/stats
- Users (admin)
  - GET/POST/PUT/DELETE /api/users
- Listeners
  - GET/POST /api/listeners
  - GET/PUT/DELETE /api/listener/{id}
- Tunnels
  - GET /api/tunnels
  - GET/DELETE /api/tunnels/{id}
- Sessions
  - GET /api/sessions

Notes:
- Endpoints require authentication (basic or JWT when SSO enabled)
- Logs are viewable in the dashboard; public logs API may be restricted

## Troubleshooting
- Ensure `--dashboard` is enabled and TLS configured
- Check server logs for errors
- Verify credentials or SSO config
- Confirm firewall/ports are open
