# Listeners (Mock & Proxy)

Mock listeners simulate endpoints; proxy listeners forward to upstreams with optional rewrites. Both support TLS termination and capture.

## Mock listener
```json
{
  "name": "mock-user",
  "mode": "mock",
  "port": 8081
}
```

## Proxy listener
```json
{
  "name": "proxy-upstream",
  "mode": "proxy",
  "port": 8080,
  "target_url": "https://api.example.com"
}
```

