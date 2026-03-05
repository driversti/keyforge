# Download & Install Page Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a public download page with pre-built binaries (via GitHub Releases) and a `curl | sh` install script that auto-detects platform, installs keyforge, and optionally enrolls the device.

**Architecture:** Embedded install.sh script served at `/install.sh` (like existing `/enroll.sh`). Download page at `/download` (public, no auth). GitHub Actions builds release binaries on version tags. Version info injected via ldflags.

**Tech Stack:** Go embed, shell script, GitHub Actions, HTML/CSS (existing dark theme)

---

### Task 1: Add version variable and --version flag

**Files:**
- Modify: `cmd/keyforge/main.go`

**Step 1: Add version variable and version command**

In `cmd/keyforge/main.go`, add a `version` variable and wire up Cobra's built-in version support:

```go
// Add after the existing var block (line 16-19):
var version = "dev"
```

Then set the version on `rootCmd` inside `init()`:

```go
func init() {
	rootCmd.Version = version
	// ... existing flags ...
}
```

Cobra automatically adds `--version` and `keyforge version` when `rootCmd.Version` is set.

**Step 2: Verify it works**

Run: `go build -o bin/keyforge ./cmd/keyforge && ./bin/keyforge --version`
Expected: `keyforge version dev`

Run: `go build -ldflags "-X main.version=v0.1.0" -o bin/keyforge ./cmd/keyforge && ./bin/keyforge --version`
Expected: `keyforge version v0.1.0`

**Step 3: Commit**

```bash
git add cmd/keyforge/main.go
git commit -m "feat: add --version flag with ldflags support"
```

---

### Task 2: Create the install script

**Files:**
- Create: `internal/server/scripts/install.sh`

**Step 1: Write the install script**

```sh
#!/bin/sh
# KeyForge install script
# Usage: curl -sSL https://your-keyforge-server/install.sh | sh
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
        # Check for Android/Termux
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

# Add to PATH if needed (skip for Termux and /usr/local/bin — already in PATH)
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
```

**Step 2: Verify the script is valid shell**

Run: `shellcheck internal/server/scripts/install.sh` (if available, otherwise `sh -n internal/server/scripts/install.sh`)

**Step 3: Commit**

```bash
git add internal/server/scripts/install.sh
git commit -m "feat: add install.sh script for curl-pipe installation"
```

---

### Task 3: Serve install.sh from the server

**Files:**
- Modify: `internal/server/server.go`

**Step 1: Add the embed directive and route**

In `server.go`, add a second embed directive after the existing one (line 15-16):

```go
//go:embed scripts/enroll.sh
var enrollScript []byte

//go:embed scripts/install.sh
var installScript []byte
```

Then add the route inside `routes()`, near the existing `enroll.sh` handler (after line 87):

```go
	// Serve the curl-pipeable install script.
	s.mux.HandleFunc("GET /install.sh", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write(installScript)
	})
```

**Step 2: Verify it compiles**

Run: `go build ./cmd/keyforge`

**Step 3: Commit**

```bash
git add internal/server/server.go
git commit -m "feat: serve install.sh at GET /install.sh"
```

---

### Task 4: Add download page handler

**Files:**
- Modify: `internal/web/web.go`

**Step 1: Add the DownloadPage handler**

Add this method to the `Handler` struct in `web.go` (after the `AuthorizedKeysPage` method, around line 311):

```go
// DownloadPage renders the public download/install page.
func (h *Handler) DownloadPage(w http.ResponseWriter, r *http.Request) {
	h.renderPage(w, "download.html", map[string]any{
		"ServerURL": h.serverURL,
	})
}
```

**Step 2: Verify it compiles**

Run: `go build ./cmd/keyforge`
Expected: Fails because `download.html` template doesn't exist yet. That's OK — we'll create it in the next task.

**Step 3: Commit**

```bash
git add internal/web/web.go
git commit -m "feat: add DownloadPage handler"
```

---

### Task 5: Create download page template

**Files:**
- Create: `internal/web/templates/download.html`

**Step 1: Create the template**

```html
{{define "title"}}Download{{end}}

{{define "content"}}
<div class="top-bar">
    <h1>Install KeyForge</h1>
</div>

<div class="card">
    <h2>Quick Install</h2>
    <p class="stat-label" style="margin-bottom:0.75rem;">Run this command to install keyforge on any supported platform:</p>
    <div class="install-cmd">
        <code id="install-cmd">curl -sSL {{.ServerURL}}/install.sh | sh</code>
        <button class="copy-btn" onclick="copyText('install-cmd')">Copy</button>
    </div>
</div>

<div class="card">
    <h2>Install + Enroll</h2>
    <p class="stat-label" style="margin-bottom:0.75rem;">Install the binary and enroll this device in one step:</p>
    <div class="install-cmd">
        <code id="enroll-cmd">curl -sSL {{.ServerURL}}/install.sh | sh -s -- \
  --name "my-device" \
  --token YOUR_TOKEN \
  --server {{.ServerURL}} \
  --accept-ssh</code>
        <button class="copy-btn" onclick="copyText('enroll-cmd')">Copy</button>
    </div>
</div>

<h2 style="margin-bottom:1rem;">Download Binaries</h2>
<p class="stat-label" style="margin-bottom:1rem;">Or download the binary directly from GitHub Releases:</p>

<div class="download-grid">
    <div class="download-card">
        <div class="download-os">Linux</div>
        <div class="download-arch">x86_64 (amd64)</div>
        <div class="download-desc">Debian, Ubuntu, servers, VMs</div>
        <a href="https://github.com/driversti/keyforge/releases/latest/download/keyforge-linux-amd64" class="btn btn-primary btn-sm">Download</a>
    </div>
    <div class="download-card">
        <div class="download-os">Linux</div>
        <div class="download-arch">ARM64 (aarch64)</div>
        <div class="download-desc">ARM servers, LXC on ARM</div>
        <a href="https://github.com/driversti/keyforge/releases/latest/download/keyforge-linux-arm64" class="btn btn-primary btn-sm">Download</a>
    </div>
    <div class="download-card">
        <div class="download-os">Linux</div>
        <div class="download-arch">ARM (v7, 32-bit)</div>
        <div class="download-desc">Raspberry Pi</div>
        <a href="https://github.com/driversti/keyforge/releases/latest/download/keyforge-linux-arm" class="btn btn-primary btn-sm">Download</a>
    </div>
    <div class="download-card">
        <div class="download-os">macOS</div>
        <div class="download-arch">Intel (amd64)</div>
        <div class="download-desc">Intel-based Macs</div>
        <a href="https://github.com/driversti/keyforge/releases/latest/download/keyforge-darwin-amd64" class="btn btn-primary btn-sm">Download</a>
    </div>
    <div class="download-card">
        <div class="download-os">macOS</div>
        <div class="download-arch">Apple Silicon (arm64)</div>
        <div class="download-desc">M1/M2/M3/M4 Macs</div>
        <a href="https://github.com/driversti/keyforge/releases/latest/download/keyforge-darwin-arm64" class="btn btn-primary btn-sm">Download</a>
    </div>
    <div class="download-card">
        <div class="download-os">Android</div>
        <div class="download-arch">ARM64</div>
        <div class="download-desc">Termux on tablets &amp; phones</div>
        <a href="https://github.com/driversti/keyforge/releases/latest/download/keyforge-android-arm64" class="btn btn-primary btn-sm">Download</a>
    </div>
</div>

<script>
function copyText(id) {
    var el = document.getElementById(id);
    navigator.clipboard.writeText(el.textContent.trim()).then(function() {
        var btn = el.parentElement.querySelector('.copy-btn');
        var original = btn.textContent;
        btn.textContent = 'Copied!';
        setTimeout(function() { btn.textContent = original; }, 2000);
    });
}
</script>
{{end}}
```

**Step 2: Verify it compiles**

Run: `go build ./cmd/keyforge`

**Step 3: Commit**

```bash
git add internal/web/templates/download.html
git commit -m "feat: add download page template"
```

---

### Task 6: Add download page styles

**Files:**
- Modify: `internal/web/static/style.css`

**Step 1: Add styles for download page components**

Append these styles at the end of `style.css` (before the `@media` queries at line 431):

```css
/* Install command */
.install-cmd {
    display: flex;
    align-items: flex-start;
    gap: 0.75rem;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 1rem;
}

.install-cmd code {
    flex: 1;
    font-family: monospace;
    font-size: 0.9rem;
    white-space: pre-wrap;
    word-break: break-all;
    line-height: 1.6;
    color: var(--success);
}

.install-cmd .copy-btn {
    flex-shrink: 0;
}

/* Download grid */
.download-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 1rem;
    margin-bottom: 2rem;
}

.download-card {
    background: var(--surface);
    border-radius: var(--radius);
    padding: 1.25rem;
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
}

.download-os {
    font-size: 1.1rem;
    font-weight: 600;
    color: var(--primary);
}

.download-arch {
    font-size: 0.9rem;
    color: var(--text);
}

.download-desc {
    font-size: 0.8rem;
    color: var(--text-muted);
    margin-bottom: 0.5rem;
}

.download-card .btn {
    align-self: flex-start;
}
```

Then add responsive rules inside the existing `@media (max-width: 768px)` block (around line 431):

```css
    .download-grid {
        grid-template-columns: repeat(2, 1fr);
    }

    .install-cmd {
        flex-direction: column;
    }
```

And inside the existing `@media (max-width: 480px)` block (around line 486):

```css
    .download-grid {
        grid-template-columns: 1fr;
    }
```

**Step 2: Verify it compiles**

Run: `go build ./cmd/keyforge`

**Step 3: Commit**

```bash
git add internal/web/static/style.css
git commit -m "feat: add download page styles"
```

---

### Task 7: Wire up route and nav link

**Files:**
- Modify: `internal/server/server.go`
- Modify: `internal/web/templates/layout.html`

**Step 1: Add the download route in server.go**

In `server.go` `routes()` method, add a public route near the other public routes (after the `install.sh` route):

```go
	// Public download page (no auth).
	s.mux.HandleFunc("GET /download", s.webHandler.DownloadPage)
```

**Step 2: Add Download link to nav bar**

In `layout.html`, add a "Download" link. Since it's a public page, add it before the "Logout" link but position it so it's visible to everyone. Update the nav to:

```html
    <nav>
        <div class="container">
            <a href="/" class="logo">KeyForge</a>
            <a href="/">Dashboard</a>
            <a href="/devices">Devices</a>
            <a href="/add">Add Device</a>
            <a href="/authorized-keys">Authorized Keys</a>
            <a href="/tokens">Tokens</a>
            <a href="/audit">Audit Log</a>
            <a href="/settings">Settings</a>
            <a href="/download">Download</a>
            <a href="/logout" style="margin-left:auto;">Logout</a>
        </div>
    </nav>
```

**Step 3: Build and manually test**

Run: `go build -o bin/keyforge ./cmd/keyforge && ./bin/keyforge serve --port 9315`
Then open `http://localhost:9315/download` — should render without login.

**Step 4: Commit**

```bash
git add internal/server/server.go internal/web/templates/layout.html
git commit -m "feat: wire up /download route and nav link"
```

---

### Task 8: Update Makefile with cross-compile targets

**Files:**
- Modify: `Makefile`

**Step 1: Add version and cross-compile targets**

Replace the Makefile content with:

```makefile
BINARY_NAME=keyforge
BUILD_DIR=bin
VERSION ?= dev
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

.PHONY: build run test clean release

build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/keyforge

run: build
	./$(BUILD_DIR)/$(BINARY_NAME) serve

test:
	go test ./...

clean:
	rm -rf $(BUILD_DIR)

release: clean
	GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64   ./cmd/keyforge
	GOOS=linux   GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64   ./cmd/keyforge
	GOOS=linux   GOARCH=arm   go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm     ./cmd/keyforge
	GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64  ./cmd/keyforge
	GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64  ./cmd/keyforge
	GOOS=android GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-android-arm64 ./cmd/keyforge
```

**Step 2: Test the build**

Run: `make build VERSION=v0.1.0 && ./bin/keyforge --version`
Expected: `keyforge version v0.1.0`

**Step 3: Commit**

```bash
git add Makefile
git commit -m "feat: add version ldflags and release cross-compile target to Makefile"
```

---

### Task 9: Create GitHub Actions release workflow

**Files:**
- Create: `.github/workflows/release.yml`

**Step 1: Create the workflow file**

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Run tests
        run: go test ./...

      - name: Build binaries
        run: |
          VERSION=${GITHUB_REF_NAME}
          LDFLAGS="-X main.version=${VERSION}"

          GOOS=linux   GOARCH=amd64 go build -ldflags "$LDFLAGS" -o bin/keyforge-linux-amd64   ./cmd/keyforge
          GOOS=linux   GOARCH=arm64 go build -ldflags "$LDFLAGS" -o bin/keyforge-linux-arm64   ./cmd/keyforge
          GOOS=linux   GOARCH=arm   go build -ldflags "$LDFLAGS" -o bin/keyforge-linux-arm     ./cmd/keyforge
          GOOS=darwin  GOARCH=amd64 go build -ldflags "$LDFLAGS" -o bin/keyforge-darwin-amd64  ./cmd/keyforge
          GOOS=darwin  GOARCH=arm64 go build -ldflags "$LDFLAGS" -o bin/keyforge-darwin-arm64  ./cmd/keyforge
          GOOS=android GOARCH=arm64 go build -ldflags "$LDFLAGS" -o bin/keyforge-android-arm64 ./cmd/keyforge

      - name: Create GitHub Release
        uses: softprops/action-gh-release@v2
        with:
          generate_release_notes: true
          files: |
            bin/keyforge-linux-amd64
            bin/keyforge-linux-arm64
            bin/keyforge-linux-arm
            bin/keyforge-darwin-amd64
            bin/keyforge-darwin-arm64
            bin/keyforge-android-arm64
            internal/server/scripts/install.sh
```

**Step 2: Validate YAML syntax**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))"` (if pyyaml available, otherwise just check indentation manually)

**Step 3: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "ci: add GitHub Actions release workflow for cross-platform builds"
```

---

### Task 10: End-to-end manual verification

**Step 1: Build and start the server**

Run: `make build && ./bin/keyforge serve --port 9315 --data ./test-data`

**Step 2: Verify /download page**

Open `http://localhost:9315/download` in browser (no login required).
Verify:
- One-liner install command is shown with correct server URL
- All 6 platform cards are displayed
- Download links point to `github.com/driversti/keyforge/releases/latest/download/...`
- Copy buttons work
- Install + enroll example is shown

**Step 3: Verify /install.sh**

Run: `curl -sS http://localhost:9315/install.sh | head -5`
Expected: First 5 lines of the install script.

**Step 4: Verify --version**

Run: `./bin/keyforge --version`
Expected: `keyforge version dev`

**Step 5: Clean up test data**

Run: `rm -rf ./test-data`

**Step 6: Final commit (if any fixups needed)**

If everything works, no commit needed. Otherwise fix and commit.
