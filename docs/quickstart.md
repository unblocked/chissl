# Quick Start

Get up and running with chiSSL in minutes. Install the client (for creating tunnels) and the server (with dashboard) using the steps below.

---

## Client install (macOS/Linux via Homebrew)

```bash
# Optional: remove an older Homebrew install first
brew uninstall chissl

# Add the tap (this uses the main repo as a tap)
brew tap unblocked/chissl https://github.com/unblocked/chissl

# Install chiSSL client
brew install unblocked/chissl/chissl
```

Alternative: download binaries directly from Releases:
- https://github.com/unblocked/chissl/releases

---

## Server install (Linux)

One‑liner installer script (v2.0):

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/unblocked/chissl/v2.0/server_installer.sh) FQDN_HERE [port] [admin_password]
```

This will:
- Download the correct Linux server binary for your CPU (amd64, arm64, or armv7)
- Install it to /usr/local/bin/chissl
- Create and start a systemd service using the recommended command:

```bash
/chissl server -v \
  --tls-domain FQDN_HERE \
  --auth ADMIN_USER:PASSWORD_SUPPLIED_BY_USER \
  --dashboard
```

Notes:
- If you omit the password argument, the installer will prompt for it.
- If you need to reconfigure, edit the systemd service and restart it (sudo systemctl daemon-reload && sudo systemctl restart chissl).

Manual download option (no installer):
- Download the Linux server binary that matches your architecture from Releases:
  - https://github.com/unblocked/chissl/releases
- Place it at /usr/local/bin/chissl and make it executable (chmod +x /usr/local/bin/chissl)
- Start the server with the command shown above

---

## Next steps
- Open the Dashboard at: https://FQDN_HERE/dashboard (use the admin credentials you set)
- Create a user and then connect a client:

```bash
chissl client --auth user:pass https://FQDN_HERE "8080->80"
```

