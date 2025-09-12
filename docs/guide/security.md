# Security Controls

## Auth enforcement
- All /api/* endpoints are protected; public endpoints are listed in the API reference
- Admin-only routes use admin middleware; user routes require user auth

## Login backoff
- Exponential backoff and temporary lockouts after repeated failures
- Optional per-IP buckets

## IP rate limiting
- Max requests per minute per IP with temporary ban on abuse

## Security events & webhooks
- In-memory event ring with optional DB persistence
- Dispatch to JSON or Slack-formatted webhooks

