#!/bin/sh
# KeyForge install script
# Usage: curl -sSL https://keyforge.yurii.live/install.sh | sh
#
# Options:
#   --global            Install to /usr/local/bin (requires sudo)
#   --name NAME         Device name (triggers enrollment after install)
#   --token TOKEN       Enrollment token
#   --server URL        KeyForge server URL
#   --accept-ssh        Accept SSH connections (used with --name)
#   --key PATH          SSH key path (default: ~/.ssh/id_ed25519)
#   --sync-interval INT Sync interval for authorized_keys (e.g., 15m, 1h)

set -e

REPO="driversti/keyforge"
INSTALL_GLOBAL=""
ENROLL_NAME=""
ENROLL_TOKEN=""
ENROLL_SERVER=""
ENROLL_ACCEPT_SSH=""
ENROLL_KEY=""
ENROLL_SYNC_INTERVAL=""

while [ $# -gt 0 ]; do
    case "$1" in
        --global) INSTALL_GLOBAL="true"; shift;;
        --name) ENROLL_NAME="$2"; shift 2;;
        --token) ENROLL_TOKEN="$2"; shift 2;;
        --server) ENROLL_SERVER="$2"; shift 2;;
        --accept-ssh) ENROLL_ACCEPT_SSH="--accept-ssh"; shift;;
        --key) ENROLL_KEY="$2"; shift 2;;
        --sync-interval) ENROLL_SYNC_INTERVAL="$2"; shift 2;;
        *) echo "Unknown option: $1"; exit 1;;
    esac
done

# Detect OS
OS="$(uname -s)"
case "$OS" in
    Linux)
        if [ -n "$PREFIX" ] && echo "$PREFIX" | grep -q "com.termux"; then
            OS="android"
        else
            OS="linux"
        fi
        ;;
    Darwin) OS="darwin";;
    *) echo "Error: unsupported OS: $OS"; exit 1;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
    x86_64|amd64) ARCH="amd64";;
    aarch64|arm64) ARCH="arm64";;
    armv7l|armv6l) ARCH="arm";;
    *) echo "Error: unsupported architecture: $ARCH"; exit 1;;
esac

BINARY="keyforge-${OS}-${ARCH}"
echo "Detected platform: ${OS}/${ARCH}"

# Get latest release tag
echo "Fetching latest release..."
TAG=$(curl -sS "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')

if [ -z "$TAG" ]; then
    echo "Error: could not determine latest release. Check https://github.com/${REPO}/releases"
    exit 1
fi

echo "Latest version: $TAG"

# Determine install directory
if [ -n "$INSTALL_GLOBAL" ]; then
    INSTALL_DIR="/usr/local/bin"
    NEED_SUDO="true"
elif [ -n "$PREFIX" ] && echo "$PREFIX" | grep -q "com.termux"; then
    INSTALL_DIR="$PREFIX/bin"
    NEED_SUDO=""
else
    INSTALL_DIR="$HOME/.local/bin"
    NEED_SUDO=""
fi

# Download
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${TAG}/${BINARY}"
TMP_FILE="$(mktemp)"

echo "Downloading ${BINARY} (${TAG})..."
curl -sSL -o "$TMP_FILE" "$DOWNLOAD_URL"

if [ ! -s "$TMP_FILE" ]; then
    rm -f "$TMP_FILE"
    echo "Error: download failed. Check https://github.com/${REPO}/releases"
    exit 1
fi

# Install
mkdir -p "$INSTALL_DIR"
if [ "$NEED_SUDO" = "true" ]; then
    sudo mv "$TMP_FILE" "$INSTALL_DIR/keyforge"
    sudo chmod +x "$INSTALL_DIR/keyforge"
else
    mv "$TMP_FILE" "$INSTALL_DIR/keyforge"
    chmod +x "$INSTALL_DIR/keyforge"
fi

echo "Installed keyforge to $INSTALL_DIR/keyforge"

# Add to PATH if needed
if [ "$INSTALL_DIR" = "$HOME/.local/bin" ]; then
    case ":$PATH:" in
        *":$INSTALL_DIR:"*) ;;
        *)
            SHELL_NAME="$(basename "$SHELL")"
            case "$SHELL_NAME" in
                zsh)  PROFILE="$HOME/.zshrc";;
                bash) PROFILE="$HOME/.bashrc";;
                *)    PROFILE="$HOME/.profile";;
            esac
            echo "" >> "$PROFILE"
            echo "# Added by KeyForge installer" >> "$PROFILE"
            echo "export PATH=\"\$HOME/.local/bin:\$PATH\"" >> "$PROFILE"
            echo "Added $INSTALL_DIR to PATH in $PROFILE"
            echo "Run 'source $PROFILE' or open a new terminal to use keyforge."
            export PATH="$INSTALL_DIR:$PATH"
            ;;
    esac
fi

# Verify installation
if command -v keyforge >/dev/null 2>&1; then
    echo "Verified: $(keyforge --version)"
else
    echo "Installed successfully. Run: $INSTALL_DIR/keyforge --version"
fi

# Optional enrollment
if [ -n "$ENROLL_NAME" ] && [ -n "$ENROLL_TOKEN" ]; then
    echo ""
    echo "Starting enrollment..."
    ENROLL_CMD="keyforge enroll --name \"$ENROLL_NAME\" --token \"$ENROLL_TOKEN\""
    if [ -n "$ENROLL_SERVER" ]; then
        ENROLL_CMD="$ENROLL_CMD --server \"$ENROLL_SERVER\""
    fi
    if [ -n "$ENROLL_ACCEPT_SSH" ]; then
        ENROLL_CMD="$ENROLL_CMD --accept-ssh"
    fi
    if [ -n "$ENROLL_KEY" ]; then
        ENROLL_CMD="$ENROLL_CMD --key \"$ENROLL_KEY\""
    fi
    if [ -n "$ENROLL_SYNC_INTERVAL" ]; then
        ENROLL_CMD="$ENROLL_CMD --sync-interval \"$ENROLL_SYNC_INTERVAL\""
    fi
    eval "$ENROLL_CMD"
fi

echo ""
echo "Done! Run 'keyforge --help' to get started."
