# HTTPS on-demand tunnels

Create secure tunnels on demand and capture traffic in real-time for troubleshooting and development.

## Create a proxy listener
```json
{
  "name": "https-proxy",
  "mode": "proxy",
  "port": 8443,
  "target_url": "https://upstream.example.com",
  "use_tls": true
}
```

## View live capture
Open the tunnel in the dashboard to see live requests and responses, including headers and formatted bodies. Use search and filters to locate specific exchanges.

## Tips
- Use custom certs or accept self-signed for local development
- Enable capture to record payloads and timing
- Host/SNI and path rewrites can be configured on proxy listeners

