# Authentication & RBAC

chiSSL supports Basic Auth, API tokens, and Auth0 SSO. Admin-only APIs are protected behind admin middleware; user APIs require user authentication. Unauthenticated requests return 401; authenticated but unauthorized return 403.

## Methods
- Basic: `Authorization: Basic ...`
- API Token: `Authorization: Bearer <token>`
- Auth0 SSO: browser login via configured provider

## Examples
```bash
# Admin (list users)
curl -u admin:adminpass https://server/api/users

# User (list listeners)
curl -H "Authorization: Bearer $TOKEN" https://server/api/listeners
```

See API reference for endpoints and schemas: [API](../api.md).

