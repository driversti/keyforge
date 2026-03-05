# KeyForge

A centralized SSH public key registry. One binary, one command to enroll devices, full-mesh SSH connectivity.

## The Problem

You have servers, LXC containers, phones, laptops. Each needs SSH keys to talk to the others. You're either:
- Copying the same private key everywhere (insecure)
- Running `ssh-copy-id` to every server for every device (tedious)
- Manually managing `authorized_keys` across an ever-changing fleet (painful)

## The Solution

KeyForge is a single binary that runs as a server and a CLI client. Every device registers its public key once. Every server fetches all keys automatically.

```
                    ┌──────────────────┐
                    │  KeyForge Server │
                    │   (REST API +    │
                    │    Web UI +      │
                    │    SQLite)       │
                    └────────┬─────────┘
                             │
           ┌─────────────────┼─────────────────┐
           │                 │                 │
     ┌─────▼─────┐     ┌─────▼─────┐     ┌─────▼─────┐
     │  Laptop   │     │  Server   │     │   Phone   │
     │ registers │     │ registers │     │ registers │
     │  its key  │     │ + fetches │     │  its key  │
     │           │     │ all keys  │     │           │
     └───────────┘     └───────────┘     └───────────┘
```

Result: any enrolled device can SSH into any enrolled server. Servers can SSH to each other. No manual key distribution.

## Quick Start

### 1. Start the server

```bash
# Build
go build -o keyforge ./cmd/keyforge

# Start (generates an API key on first run — save it!)
./keyforge serve --port 9315 --data ./keyforge-data
```

Output on first run:
```
=== Generated API Key (save this!) ===
a1b2c3d4e5f6...
=======================================
KeyForge server listening on :9315
```

Open `http://localhost:9315` to access the Web UI (login with the API key).

### 2. Enroll your first device

On the machine you want to enroll:

```bash
# First, create an enrollment token (on any machine with the API key)
./keyforge token create --label "for-my-laptop" --expires 1h \
  --server http://keyforge:9315 --api-key YOUR_API_KEY

# Output: Token: xyz789...

# Then, on the device to enroll:
./keyforge enroll \
  --name "my-laptop" \
  --server http://keyforge:9315 \
  --token xyz789
```

This generates an SSH key (if needed) and registers the public key with KeyForge.

### 3. Enroll a server

Servers use the same flow but add `--accept-ssh` so they receive all keys:

```bash
./keyforge enroll \
  --name "web-server" \
  --server http://keyforge:9315 \
  --token abc123 \
  --accept-ssh
```

### 4. Install keys on the server

```bash
# One-time install
./keyforge keys --install --server http://keyforge:9315

# Set up periodic sync (every 15 minutes)
./keyforge keys --install --cron 15m --server http://keyforge:9315
```

That's it. Your laptop can now SSH into the web server, and any future enrolled device will be able to as well.

## Enrolling Without the Binary

Don't have the `keyforge` binary on the new device? Use the built-in enrollment script:

```bash
curl -sSL http://keyforge:9315/enroll.sh | sh -s -- \
  --name "lxc-nginx" \
  --token abc123 \
  --server http://keyforge:9315 \
  --accept-ssh \
  --sync-interval 15m
```

This downloads a shell script that generates keys, registers with the server, installs authorized_keys, and sets up cron — all in one command.

## Proxmox LXC Workflow

When creating a new LXC in Proxmox:

1. Open the KeyForge Web UI at `/authorized-keys`
2. Click **Copy All**
3. Paste into the Proxmox SSH key field during LXC creation
4. After the LXC boots, enroll it:
   ```bash
   curl -sSL http://keyforge:9315/enroll.sh | sh -s -- \
     --name "lxc-$(hostname)" \
     --token TOKEN \
     --server http://keyforge:9315 \
     --accept-ssh \
     --sync-interval 15m
   ```

## CLI Reference

### Server

```bash
keyforge serve [--port 9315] [--data ./keyforge-data]
```

### Enrollment Tokens

```bash
# Create a token (single-use, time-limited)
keyforge token create --label "for-pixel-8" --expires 1h \
  --server URL --api-key KEY

# List all tokens
keyforge token list --server URL --api-key KEY

# Delete a token
keyforge token delete TOKEN_ID --server URL --api-key KEY
```

### Device Enrollment

```bash
# Enroll this device (generates key if needed)
keyforge enroll --name "device-name" --token TOKEN \
  --server URL [--accept-ssh] [--key ~/.ssh/id_ed25519]
```

### Device Management

```bash
# List devices
keyforge device list --server URL --api-key KEY

# Manually add a device (paste an existing public key)
keyforge device add --name "old-laptop" --key "ssh-ed25519 AAAA..." \
  --server URL --api-key KEY [--accept-ssh] [--tags "linux,home"]

# Revoke a device (excluded from future syncs)
keyforge device revoke --name "lost-phone" --server URL --api-key KEY

# Reactivate a revoked device
keyforge device reactivate --name "found-phone" --server URL --api-key KEY

# Permanently delete
keyforge device delete --name "old-device" --server URL --api-key KEY
```

### Key Operations

```bash
# Print all active public keys to stdout
keyforge keys --server URL

# Install to ~/.ssh/authorized_keys (managed section only)
keyforge keys --install --server URL

# Install + set up cron sync
keyforge keys --install --cron 15m --server URL
```

### Push Keys to a Server

```bash
# Push keys to a specific server via SSH
keyforge push --target root@192.168.1.50 --server URL
```

## API

### Public Endpoints (no auth)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/authorized_keys` | All active public keys as plain text |
| `GET` | `/api/v1/health` | Health check |
| `GET` | `/enroll.sh` | Enrollment shell script |

### Protected Endpoints (API key: `Authorization: Bearer <key>`)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/devices` | List all devices |
| `GET` | `/api/v1/devices/:id` | Get a device |
| `POST` | `/api/v1/devices` | Register device (API key or enrollment token) |
| `PATCH` | `/api/v1/devices/:id` | Update device |
| `DELETE` | `/api/v1/devices/:id` | Delete device |
| `POST` | `/api/v1/devices/:id/revoke` | Revoke device |
| `POST` | `/api/v1/devices/:id/reactivate` | Reactivate device |
| `POST` | `/api/v1/tokens` | Create enrollment token |
| `GET` | `/api/v1/tokens` | List tokens |
| `DELETE` | `/api/v1/tokens/:id` | Delete token |

The `/api/v1/authorized_keys` endpoint returns plain text — pipe it directly into `authorized_keys`:

```bash
curl -s http://keyforge:9315/api/v1/authorized_keys >> ~/.ssh/authorized_keys
```

## How Keys Are Managed

KeyForge uses **managed section markers** in `authorized_keys`:

```
# manually added key stays untouched
ssh-rsa AAAA... admin@jumpbox

# --- KeyForge Managed Keys (DO NOT EDIT) ---
ssh-ed25519 AAAA... my-laptop
ssh-ed25519 AAAA... web-server
ssh-ed25519 AAAA... pixel-8
# --- End KeyForge Managed Keys ---
```

Only the section between the markers is updated during sync. Your manually-added keys are never touched.

## AuthorizedKeysCommand (Real-Time Key Lookup)

Instead of syncing keys via cron, you can configure sshd to query KeyForge on every login attempt:

### Setup

1. Copy the `keyforge` binary to a system-wide location:
   ```bash
   sudo cp keyforge /usr/local/bin/keyforge
   ```

2. Edit `/etc/ssh/sshd_config`:
   ```
   AuthorizedKeysCommand /usr/local/bin/keyforge keys --server http://keyforge:9315
   AuthorizedKeysCommandUser nobody
   ```

3. Restart sshd:
   ```bash
   sudo systemctl restart sshd
   ```

### How It Works

On every SSH login attempt, sshd runs the `keyforge keys` command. It fetches all active public keys from the KeyForge server and returns them to sshd for authentication.

### Cache Fallback

If the KeyForge server is unreachable, the command automatically returns the last successfully fetched keys from a local cache file (`~/.cache/keyforge/authorized_keys.cache` or `/var/cache/keyforge/authorized_keys.cache` for root). This prevents SSH lockout during server outages.

To disable caching: `keyforge keys --server URL --no-cache`

### Comparison: Cron vs AuthorizedKeysCommand

| | Cron Sync | AuthorizedKeysCommand |
|---|---|---|
| Key freshness | Delay (cron interval) | Real-time |
| Revocation speed | Minutes | Instant |
| Server dependency | Only during sync | Every SSH login |
| Offline behavior | Stale file works | Cache fallback |
| Setup complexity | Simple | Requires sshd_config |

## Security

- **Private keys never leave devices.** KeyForge only stores and distributes public keys.
- **Enrollment tokens** are single-use and time-limited (default: 1 hour). They prevent unauthorized device registration.
- **API key** is auto-generated on first run and stored in the SQLite database. Used for management operations.
- **Web UI** requires login with the API key (session cookie, 24h expiry).
- **The `/authorized_keys` endpoint is unauthenticated by design** — public keys are inherently public. They reveal identity but grant no access.
- For production: put KeyForge behind a reverse proxy with TLS, or access it over Tailscale.

## Building

```bash
# Build for current platform
make build

# Cross-compile (example)
GOOS=linux GOARCH=amd64 go build -o keyforge-linux-amd64 ./cmd/keyforge
GOOS=linux GOARCH=arm64 go build -o keyforge-linux-arm64 ./cmd/keyforge

# Run tests
make test
```

## Tech Stack

- **Go** — single static binary, zero runtime dependencies
- **SQLite** (modernc.org/sqlite) — embedded, pure Go, no CGo
- **Cobra** — CLI framework
- **htmx** — dynamic Web UI without JavaScript build pipelines
- **golang.org/x/crypto/ssh** — key generation and fingerprinting
