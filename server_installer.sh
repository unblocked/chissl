#!/usr/bin/env bash
#
# This script downloads and installs the chiSSL server binary for your platform from GitHub Releases.
# It creates a systemd service and starts the server with --auth and --dashboard enabled.
# NOTE: The script will prompt for an admin password (or auto-generate one) and pass it via --auth.
#
#
# Usage:
#   ./script_name.sh <domain_name> [port]
#
# Arguments:
#   domain_name: A fully qualified domain name (required)
#   port: An optional port number (default is 443 if not provided)
#
# Example:
#   ./script_name.sh subdomain.example.com
#  or
#   ./script_name.sh subdomain.example.com 8443
#
# To download and execute this script from a GitHub public repository in a single line:
#   bash <(curl -fsSL https://raw.githubusercontent.com/unblocked/chissl/v2.0/server_installer.sh) <domain_name> [port] [admin_password]

# Target chiSSL version tag
VERSION_TAG="v2.0"

# Function to display usage
usage() {
    echo "Usage: $0 <domain_name> [port]"
    exit 1
}

# Function to validate FQDN
is_valid_fqdn() {
    local fqdn=$1
    if [[ $fqdn =~ ^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$ && ${#fqdn} -le 253 ]]; then
        return 0
    else
        return 1
    fi
}

# Function to validate port number
is_valid_port() {
    local port=$1
    if [[ $port -ge 1 && $port -le 65535 ]]; then
        return 0
    else
        return 1
    fi
}

# Check if at least one argument is provided
if [ "$#" -lt 1 ]; then
    usage
fi

FQDN=$1
PORT=${2:-"443"}

# Admin user/password handling
ADMIN_USER=${ADMIN_USER:-admin}
ADMIN_PASS="${3-}"
if [ -z "$ADMIN_PASS" ]; then
    if [ -t 0 ]; then
        # Prompt if interactive
        read -s -p "Enter admin password (leave empty to auto-generate): " ADMIN_PASS_INPUT || true
        echo
        if [ -n "$ADMIN_PASS_INPUT" ]; then
            ADMIN_PASS="$ADMIN_PASS_INPUT"
        fi
    fi
fi
# Auto-generate if still empty
if [ -z "$ADMIN_PASS" ]; then
    ADMIN_PASS=$(tr -dc A-Za-z0-9 </dev/urandom | head -c 16)
fi

# Validate the domain name
if ! is_valid_fqdn "$FQDN"; then
    echo "Error: $FQDN is not a valid fully qualified domain name (FQDN)."
    exit 1
fi

# Only Linux is supported for server installer
if [ "$OS" != "linux" ]; then
    echo "This installer currently supports Linux only."
    exit 1
fi

# Validate the port number
if ! is_valid_port "$PORT"; then
    echo "Error: $PORT is not a valid port number. It should be between 1 and 65535."
    exit 1
fi

# Define variables
REPO_OWNER="unblocked"
REPO_NAME="chissl"
BASE_URL="https://github.com/$REPO_OWNER/$REPO_NAME/releases/download/v$LATEST_VERSION"
INSTALL_PATH="/usr/local/bin/chissl"
SERVICE_NAME="chissl"
ADMIN_PASS=$(tr -dc A-Za-z0-9 </dev/urandom | head -c 13)

# Detect OS and architecture
OS=$(uname | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

# Map architecture to supported binaries
case $ARCH in
  x86_64)
    ARCH="amd64"
    ;;
  aarch64)
    ARCH="arm64"
    ;;
  armv5*)
    ARCH="armv5"
    ;;
  armv6*)
    ARCH="armv6"
    ;;
  armv7*)
    ARCH="armv7"
    ;;
  i386)
    ARCH="386"
    ;;
  *)
    echo "Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

# Resolve server asset URL from GitHub release (robust to naming)
API_URL="https://api.github.com/repos/$REPO_OWNER/$REPO_NAME/releases/tags/$VERSION_TAG"
AUTH_HEADER=()
if [ -n "${GITHUB_TOKEN-}" ]; then AUTH_HEADER=( -H "Authorization: Bearer $GITHUB_TOKEN" ); fi
URLS=$(curl -fsSL "${AUTH_HEADER[@]}" "$API_URL" | grep -oE '"browser_download_url": "[^"]+"' | cut -d '"' -f4)
if [ -z "$URLS" ]; then
  echo "Failed to query release assets for $VERSION_TAG from GitHub API"
  exit 1
fi
if [ "$ARCH" = "386" ]; then
  echo "Unsupported architecture for server: $ARCH"
  exit 1
fi
SERVER_URL=$(echo "$URLS" | grep -Ei 'chissl[-_]?server' | grep -Ei 'linux' | grep -Ei "$ARCH" | head -n1)
if [ -z "$SERVER_URL" ] && [ "$ARCH" = "armv7" ]; then
  SERVER_URL=$(echo "$URLS" | grep -Ei 'chissl[-_]?server' | grep -Ei 'linux' | grep -Ei 'arm_?7' | head -n1)
fi
if [ -z "$SERVER_URL" ]; then
  echo "Could not find a server binary asset for linux/$ARCH in release $VERSION_TAG"
  echo "$URLS" | sed 's/^/  /'
  exit 1
fi

TMP_BIN=$(mktemp)
echo "Downloading $SERVER_URL"
curl -fL "$SERVER_URL" -o "$TMP_BIN"
sudo install -m 0755 "$TMP_BIN" "$INSTALL_PATH"
rm -f "$TMP_BIN"


# Create a systemd service file
echo "Creating systemd service"
sudo tee /etc/systemd/system/$SERVICE_NAME.service > /dev/null <<EOL
[Unit]
Description=Chissl Service
After=network.target

[Service]
ExecStart=$INSTALL_PATH server -v --port $PORT --tls-domain $FQDN --auth "$ADMIN_USER:$ADMIN_PASS" --dashboard
Restart=always
User=root

[Install]
WantedBy=multi-user.target
EOL

if [ $? -ne 0 ]; then
    echo "Failed to create /etc/systemd/system/$SERVICE_NAME.service script"
    exit 1
fi


# Reload systemd, enable and start the service
echo "Reloading systemd and starting $SERVICE_NAME"
sudo systemctl daemon-reload
sudo systemctl enable $SERVICE_NAME
sudo systemctl stop $SERVICE_NAME || true
sudo systemctl start $SERVICE_NAME

if ! sudo systemctl status $SERVICE_NAME --no-pager ; then
     echo "$SERVICE_NAME startup failed. Check 'journalctl -xe' for more info "
     exit 1
fi

# Verify health endpoint - Checks health and SSL cert
echo
echo "Verifying service health endpoint at https://$FQDN:$PORT/health"
if ! curl https://$FQDN:$PORT/health -m 5 ; then
     echo "Failed to reach health endpoint via https://$FQDN:$PORT/health"
     exit 1
fi

echo
echo "============================================================================================="
echo "$SERVICE_NAME installation and setup complete."
echo "Admin user: $ADMIN_USER"
echo "Admin password: $ADMIN_PASS"
echo "Dashboard: https://$FQDN:$PORT/dashboard"
echo "Server started with: $INSTALL_PATH server -v --port $PORT --tls-domain $FQDN --auth '$ADMIN_USER:******' --dashboard"
echo "Manage users and settings via the dashboard or the REST API."
echo "============================================================================================="