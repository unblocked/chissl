# HTTPS Inspection & MITM

chiSSL can act as a TLS-terminating proxy to inspect and modify HTTPS requests/responses for debugging.

## Capabilities
- Terminate TLS at listeners (`use_tls`) with custom certs
- Forward to upstreams over HTTP/HTTPS with host/URL rewriting
- Capture and display headers and bodies (smart JSON formatting)
- Live event streaming and rotating logs per tunnel/listener

## Example listener
```json
{
  "name": "mitm-proxy",
  "port": 8443,
  "mode": "proxy",
  "target_url": "https://upstream.example.com",
  "use_tls": true
}
```

## Security
- Only test traffic you own or have permission to inspect
- Protect dashboard/API with auth (default)
- Rotate certs/tokens; monitor security events

