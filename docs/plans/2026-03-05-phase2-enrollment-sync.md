# KeyForge Phase 2 — Enrollment & Sync Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add enrollment token system, `enroll` CLI command (keygen + register), `keys --install` with managed section markers, periodic sync via cron/systemd timer, and `push` command for on-demand server updates.

**Architecture:** Enrollment tokens are short-lived, single-use secrets stored in the existing `enrollment_tokens` table. The `enroll` command generates an SSH key pair, registers the public key via the API using a token, and optionally sets up authorized_keys sync. The `push` command SSHes into target servers to update their authorized_keys.

**Tech Stack:** Go, cobra CLI, golang.org/x/crypto/ssh for key generation

**Spec:** `/Users/driversti/Projects/SPEC.md`

---

### Task 1: Enrollment Token Repository

**Files:**
- Create: `internal/db/token_repo.go`
- Create: `internal/db/token_repo_test.go`

Implement CRUD for enrollment tokens in the DB layer.

**Methods on *DB:**
- `CreateToken(label string, expiresAt time.Time) (*models.EnrollmentToken, error)` — generate UUID + 32-byte random token (base64url), INSERT
- `GetToken(id string) (*models.EnrollmentToken, error)` — SELECT by id
- `ValidateAndBurnToken(tokenValue string) (*models.EnrollmentToken, error)` — SELECT by token value, verify not used and not expired, UPDATE used=true, return token. This is the critical enrollment path.
- `ListTokens() ([]models.EnrollmentToken, error)` — SELECT all, ORDER BY created_at DESC
- `DeleteToken(id string) error` — DELETE by id

**Tests:** TestCreateToken, TestValidateAndBurnToken (valid), TestValidateAndBurnToken_Expired, TestValidateAndBurnToken_AlreadyUsed, TestListTokens, TestDeleteToken

---

### Task 2: Token API Endpoints + Web UI

**Files:**
- Modify: `internal/api/handlers.go` — add token handlers
- Modify: `internal/server/server.go` — add token routes
- Modify: `internal/api/handlers.go` — update CreateDevice to support enrollment token auth
- Create: `internal/web/templates/tokens.html` — token management page
- Modify: `internal/web/web.go` — add token page handlers
- Modify: `internal/web/templates/layout.html` — add Tokens nav link

**API endpoints:**
- `POST /api/v1/tokens` (API key auth) — create token, body: `{"label":"for-pixel-8","expires_in":"1h"}`
- `GET /api/v1/tokens` (API key auth) — list all tokens
- `DELETE /api/v1/tokens/{id}` (API key auth) — delete token

**Update CreateDevice endpoint:**
- If request body contains `enrollment_token`, validate it via `ValidateAndBurnToken`
- If valid, proceed with device creation (no API key required)
- If invalid/expired/used, return 401
- If neither enrollment_token nor API key provided, return 401
- Update the auth check: the route should accept EITHER API key OR enrollment token

**Web UI:**
- Tokens page: table of tokens (Label, Token value (masked), Expires, Used, Created), Create form, Delete button
- Add "Tokens" to nav in layout.html

**Token CLI commands:**
- `keyforge token create --label "for-pixel-8" --expires 1h`
- `keyforge token list`
- `keyforge token delete <id>`

Create `cmd/keyforge/token.go` and register in main.go.

---

### Task 3: SSH Key Generation

**Files:**
- Create: `internal/keys/keygen.go`
- Create: `internal/keys/keygen_test.go`

**Functions:**
- `GenerateED25519Key(path string) error` — generate ed25519 key pair, write private key to `path` and public key to `path.pub`. Set permissions 0600 on private key. If files already exist, return without error.
- `ReadPublicKey(path string) (string, error)` — read and return the public key string from a file
- `DefaultKeyPath() string` — returns `~/.ssh/id_ed25519`

**Tests:** TestGenerateED25519Key (creates files, correct format), TestGenerateED25519Key_ExistingKeySkipped, TestReadPublicKey

---

### Task 4: Enroll CLI Command

**Files:**
- Create: `cmd/keyforge/enroll.go`
- Modify: `cmd/keyforge/main.go` — register enroll command

**Command:** `keyforge enroll --name "pixel-8" --server https://keyforge.example.com --token "abc123" [--accept-ssh] [--key ~/.ssh/id_ed25519.pub]`

**What it does:**
1. If `--key` not specified, use default path (`~/.ssh/id_ed25519`)
2. If key doesn't exist at path, generate one via `keys.GenerateED25519Key`
3. Read the public key
4. POST to `/api/v1/devices` with `{name, public_key, accepts_ssh, enrollment_token}`
5. Print success message with device name and fingerprint
6. If `--accept-ssh` was set, also run `keys --install` to fetch and install authorized_keys

---

### Task 5: Keys Install with Managed Section

**Files:**
- Create: `internal/keys/install.go`
- Create: `internal/keys/install_test.go`
- Modify: `cmd/keyforge/keys.go` — add `--install` and `--cron` flags

**`internal/keys/install.go`:**
- `InstallKeys(keysContent string, authorizedKeysPath string) error` — write keys to authorized_keys using managed section markers:
  ```
  # --- KeyForge Managed Keys (DO NOT EDIT) ---
  ssh-ed25519 AAAA... pixel-8
  # --- End KeyForge Managed Keys ---
  ```
- If the file exists and has a managed section, replace only that section
- If the file exists without a managed section, append the section
- If the file doesn't exist, create it with the managed section only
- Preserve all content outside the managed section markers

**Tests:** TestInstallKeys_NewFile, TestInstallKeys_ExistingWithoutSection, TestInstallKeys_ExistingWithSection (replace), TestInstallKeys_PreservesManualKeys

**`cmd/keyforge/keys.go` updates:**
- `--install` flag: fetch keys from server, call InstallKeys with `~/.ssh/authorized_keys`
- `--cron <interval>` flag: set up a cron job or systemd timer for periodic sync
  - For simplicity in Phase 2, just create a cron entry: `*/15 * * * * keyforge keys --install --server <url>`
  - Print the cron entry and instructions, or install it directly via `crontab`

---

### Task 6: Push Command

**Files:**
- Create: `cmd/keyforge/push.go`
- Modify: `cmd/keyforge/main.go` — register push command

**Command:** `keyforge push [--name "pattern"]`

**What it does:**
1. Fetch device list from API (filtered to accepts_ssh=true)
2. For each matching device, the push would need the device's IP/hostname — but we don't store that yet
3. **Simplified for Phase 2:** Push is triggered via API endpoint `POST /api/v1/push` which returns a message saying "Push functionality requires device addresses (coming in Phase 3)". For now, just wire up the CLI command and API endpoint as stubs that explain the limitation.
4. Alternative: the push command simply prints a helper script that the user can run manually:
   ```
   # Run on each server to sync keys:
   curl -s https://keyforge.example.com/api/v1/authorized_keys | keyforge keys --install
   ```

Actually, let's make push useful NOW without storing IPs. The push command will:
1. Fetch authorized_keys content from the API
2. Print instructions for manual push, OR
3. Accept `--target user@host` flag to SSH into a specific server and install keys there

**With --target:**
```bash
keyforge push --target root@192.168.1.50
```
This SSHes into the target and runs a one-liner to install keys.

---

### Task 7: Integration Tests for Enrollment Flow

**Files:**
- Modify: `test/integration_test.go` — add enrollment flow tests

**Test: TestIntegration_EnrollmentFlow:**
1. Start server
2. Create enrollment token via API
3. Register device using enrollment token (no API key)
4. Verify device appears in list
5. Verify token is marked as used
6. Try using the same token again — should fail
7. Try using expired token — should fail

---

### Task 8: Enroll Shell Script

**Files:**
- Create: `scripts/enroll.sh`
- Modify: `internal/server/server.go` — serve enroll.sh at GET /enroll.sh

**`scripts/enroll.sh`:**
A curl-pipeable script that:
1. Parses --name, --token, --accept-ssh args
2. Checks if ssh-keygen is available
3. Generates ed25519 key if none exists
4. Registers with the KeyForge server using curl
5. Optionally installs authorized_keys and sets up cron

The script should determine the server URL from the URL it was downloaded from.

Serve it at `GET /enroll.sh` (no auth, embedded in binary).

---

## Summary

After completing all 8 tasks:
- Enrollment tokens: create, list, delete, validate+burn
- Token API + Web UI management page
- SSH key generation (ed25519)
- `keyforge enroll` CLI command (one-command device enrollment)
- `keyforge keys --install` with managed section markers
- `keyforge keys --install --cron 15m` for periodic sync
- `keyforge push --target user@host` for on-demand updates
- `keyforge token create/list/delete` CLI commands
- `enroll.sh` curl-pipeable enrollment script
- Integration tests for the enrollment flow
