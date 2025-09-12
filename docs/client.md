# Client

Create reverse tunnels from your machine to the chiSSL server. You can specify one or more remote mappings inline or via a YAML profile.

## Quick start
```bash
# Single mapping: expose local 80 via server port 8080
chissl client --auth user:pass https://tunnel.your.domain "8080->80"

# Multiple mappings
chissl client --auth user:pass https://tunnel.your.domain \
  "8080->80" "8443->443:myapp.local" "9000:0.0.0.0->9000"
```

## YAML profile
Place a file at $HOME/.chissl/profile.yaml, or pass --profile /path/to/profile.yaml.

```yaml
server: "https://tunnel.your.domain"
auth: "user:pass"
keepalive: 30s
verbose: true
remotes:
  - "8080->80"
  - "8443->443:myapp.local"
# Optional
proxy: "http://admin:password@proxy.local:8081"
headers:
  Foo: ["Bar"]
tls:
  tls-skip-verify: false
  tls-ca: "/path/to/ca"
  tls-cert: "/path/to/cert"
  tls-key: "/path/to/key"
  hostname: "tunnel.your.domain"
```

## Flags (common)
- --auth user:pass: client authentication
- --keepalive 25s: keep connection alive through proxies
- --proxy URL: use HTTP CONNECT or SOCKS5 to reach the server
- --hostname, --sni: override Host/SNI when needed
- --tls-*: trust roots and client certs for TLS transport

