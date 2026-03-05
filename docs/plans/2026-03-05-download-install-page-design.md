# Download & Install Page Design

## Problem

Users need a way to download pre-built keyforge binaries for Android, macOS, and Debian-based Linux — both from the web UI and via `curl | sh` one-liner.

## Approach

**Static install script + GitHub Releases** (Approach A). The install script is embedded in the Go binary. Download links point to GitHub Releases. No proxying or server-side release tracking.

## Platform Matrix

| OS | Arch | Binary Name | Use Case |
|---|---|---|---|
| linux | amd64 | keyforge-linux-amd64 | Debian servers, VMs |
| linux | arm64 | keyforge-linux-arm64 | ARM servers, LXC on ARM |
| linux | arm | keyforge-linux-arm | Raspberry Pi (32-bit) |
| darwin | amd64 | keyforge-darwin-amd64 | Intel Macs |
| darwin | arm64 | keyforge-darwin-arm64 | Apple Silicon Macs |
| android | arm64 | keyforge-android-arm64 | Termux on tablets/phones |

## Install Script (`install.sh`)

Served at `GET /install.sh` (public, no auth, embedded like `enroll.sh`).

### Behavior

1. Detect OS (`uname -s`) and arch (`uname -m`), map to Go naming
2. Fetch latest release tag from GitHub API (`api.github.com/repos/driversti/keyforge/releases/latest`)
3. Download correct binary from GitHub Releases
4. Install to `~/.local/bin` by default (no root), `$PREFIX/bin` on Termux, `/usr/local/bin` only with `--global`
5. Add `~/.local/bin` to PATH in shell profile if not already present
6. Optionally run `keyforge enroll` if `--name` and `--token` flags are provided

### Usage

```bash
# Just install
curl -sSL http://keyforge:9315/install.sh | sh

# Install + enroll
curl -sSL http://keyforge:9315/install.sh | sh -s -- \
  --name "my-tablet" \
  --token abc123 \
  --server http://keyforge:9315 \
  --accept-ssh
```

### Termux Detection

Check for `$PREFIX` env var (always set by Termux). Use `$PREFIX/bin` as install dir.

## GitHub Actions Release Workflow

File: `.github/workflows/release.yml`

- Trigger: `push: tags: ['v*']`
- Build matrix: cross-compile all 6 targets
- Ldflags: `-X main.version=$TAG`
- Creates GitHub Release with all binaries + `install.sh` attached
- Binary naming: `keyforge-<os>-<arch>`

### Release flow

```bash
git tag v0.1.0
git push origin v0.1.0
# GitHub Actions builds all binaries, creates release
```

## Download Web Page

Public page at `GET /download` (no auth).

### Layout

- Hero section: one-liner install command + copy button
- Platform cards grid (6 cards): OS label, architecture, direct GitHub download link
- Usage examples section: install only, install + enroll

### Download links

Point to `https://github.com/driversti/keyforge/releases/latest/download/keyforge-<os>-<arch>` so they always resolve to the newest version.

## Navigation Changes

- Add "Download" link to nav bar, visible to all users (public)
- Existing nav items remain behind auth

## Version Info

Add to `cmd/keyforge/main.go`:

```go
var version = "dev"
```

Set via ldflags: `go build -ldflags "-X main.version=$TAG"`

Enables `keyforge --version` and future use in health endpoint, user-agent, etc.

## Files Changed

| Component | File | Action |
|---|---|---|
| Install script | `internal/server/scripts/install.sh` | Create |
| Serve install.sh | `internal/server/server.go` | Modify (add route) |
| Download page template | `internal/web/templates/download.html` | Create |
| Download handler | `internal/web/web.go` | Modify (add handler) |
| Nav bar | `internal/web/templates/layout.html` | Modify (add link) |
| Styles | `internal/web/static/style.css` | Modify (download page styles) |
| CI workflow | `.github/workflows/release.yml` | Create |
| Version flag | `cmd/keyforge/main.go` | Modify (add version var + flag) |
| Makefile | `Makefile` | Modify (add cross-compile targets) |
