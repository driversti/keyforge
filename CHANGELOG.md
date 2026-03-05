# Changelog

## [v0.2.1] - 2026-03-05

### Changed
- Shortened quick enrollment codes from 8 digits to 4 for easier typing

## [v0.2.0] - 2026-03-05

### Added
- **Quick Enrollment URLs** — generate a one-liner (`curl -sSL .../e/12345678 | sh`) from the web UI to enroll devices with zero flags
- **Web password login** — set a short password in Settings instead of typing the full API key
- `KEYFORGE_PASSWORD` env var to seed initial web password on first startup
- `/e/{code}` endpoint with content negotiation (browsers get info page, curl gets script)
- Quick Enroll form on the Tokens page (device name, accept-ssh, sync interval, expiry)
- Integration tests for quick enrollment flow

## [v0.1.2] - 2026-03-05

### Fixed
- Tokens page: replaced hardcoded light-theme inline styles with dark theme CSS classes

## [v0.1.1] - 2026-03-05

### Improved
- Redesigned login page with centered layout, lock icon, and modern styling
- Fixed password input field missing dark theme styles
