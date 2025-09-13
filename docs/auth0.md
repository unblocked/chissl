# Auth0 SSO

Optional SSO via Auth0. When enabled, requests use JWT Bearer tokens; basic auth remains available for compatibility.

## Configure
```bash
chissl server \
  --auth0-enabled \
  --auth0-domain your-tenant.auth0.com \
  --auth0-client-id your_client_id \
  --auth0-client-secret your_client_secret \
  --auth0-audience https://your-api-identifier
```

Environment variables:
```bash
export CHISSL_AUTH0_ENABLED=true
export CHISSL_AUTH0_DOMAIN=your-tenant.auth0.com
export CHISSL_AUTH0_CLIENT_ID=your_client_id
export CHISSL_AUTH0_CLIENT_SECRET=your_client_secret
export CHISSL_AUTH0_AUDIENCE=https://your-api-identifier
```

## Use
- Dashboard handles redirects and token storage automatically
- Direct API calls:
```bash
# Obtain token (client credentials)
TOKEN=$(curl -s -X POST https://your-tenant.auth0.com/oauth/token \
  -H 'Content-Type: application/json' \
  -d '{
    "client_id": "your_client_id",
    "client_secret": "your_client_secret",
    "audience": "https://your-api-identifier",
    "grant_type": "client_credentials"
  }' | jq -r '.access_token')

# Call API with JWT
curl -H "Authorization: Bearer $TOKEN" https://your.domain.com/api/stats
```

Notes:
- Users are created on first successful Auth0 login (non-admin by default)
- Admins can still manage users via /api/users
- Always deploy behind HTTPS in production
