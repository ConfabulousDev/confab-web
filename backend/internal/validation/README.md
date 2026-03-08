# validation

Input validation and sanitization utilities for field length limits, email format checks, and domain restrictions.

## Files

| File | Role |
|------|------|
| `input.go` | Field length constants (matching DB constraints), validation functions, and UTF-8-safe truncation helpers |
| `input_test.go` | Tests for `ValidateExternalID` and truncation helpers with multi-byte UTF-8 strings |
| `email.go` | Email format validation, domain allowlist checking, email normalization, and domain list validation |
| `email_test.go` | Tests for email format validation, domain allowlist logic, `NormalizeEmail`, and domain list validation |

## Key API

### Email validation (`email.go`)

- **`IsValidEmail(email string) bool`** -- Validates email format using a regex that requires a TLD. Also rejects consecutive dots in the local part and enforces a 254-character maximum.
- **`NormalizeEmail(email string) string`** -- Lowercases and trims whitespace.
- **`IsAllowedEmailDomain(email string, allowedDomains []string) bool`** -- Checks if the email's domain is in the allowed list. Returns `true` if the list is empty (no restriction). Performs exact, case-insensitive domain match (no subdomain matching).
- **`ValidateDomainList(domains []string) error`** -- Validates that each domain entry has correct format with a TLD. Returns an error describing the first invalid entry.

### Field validation (`input.go`)

Each function returns `nil` if valid, or an error describing the violation:

- **`ValidateExternalID(externalID string) error`** -- Checks non-empty, length between 1-512, and valid UTF-8.
- **`ValidateCWD(cwd string) error`** -- Max 8192 characters.
- **`ValidateTranscriptPath(path string) error`** -- Max 8192 characters.
- **`ValidateSyncFileName(fileName string) error`** -- Max 512 characters.
- **`ValidateSummary(summary string) error`** -- Max 2048 characters.
- **`ValidateFirstUserMessage(msg string) error`** -- Max 8192 characters.
- **`ValidateAPIKeyName(name string) error`** -- Max 255 characters.
- **`ValidateHostname(hostname string) error`** -- Max 255 characters.
- **`ValidateUsername(username string) error`** -- Max 255 characters.

### Truncation helpers (`input.go`)

Truncation functions that safely cut strings at byte boundaries without splitting UTF-8 characters. Intended as a temporary grace period for older CLI clients that may send oversized values:

- **`TruncateSyncFileName(s string) string`**
- **`TruncateSummary(s string) string`**
- **`TruncateFirstUserMessage(s string) string`**

### Field size constants (`input.go`)

All `Max*Length` constants match the `VARCHAR` constraints in database migrations (000010, 000011). These are the source of truth for field size limits.

## How to Extend

### Adding a new field validation

1. Add a `Max*Length` constant in `input.go` matching the DB column constraint.
2. Write a `Validate*` function following the existing pattern (check `len(s) > Max*Length`, return descriptive error).
3. Optionally add a `Truncate*` helper if backward compatibility requires graceful degradation.
4. Call the validation function from the relevant API handler or sync endpoint.

## Invariants

- **Constants match database constraints.** The `Max*Length` constants must stay in sync with the `VARCHAR` sizes in the database migrations. The comment in `input.go` references migrations 000010 and 000011.
- **UTF-8 safety on truncation.** `truncateString` trims trailing bytes until the result is valid UTF-8, preventing half-character corruption at the boundary.
- **Email normalization is consistent.** `NormalizeEmail` is used by both `validation` and `admin` packages to ensure case-insensitive email comparison everywhere.
- **Domain matching is exact.** `IsAllowedEmailDomain` does not match subdomains. `example.com` does not match `sub.example.com`.

## Design Decisions

**Validation returns errors, not booleans.** Validation functions return descriptive `error` values so callers can pass them directly to HTTP error responses without constructing their own messages. The exception is `IsValidEmail` and `IsAllowedEmailDomain` which return booleans because the caller typically constructs a custom error message.

**Truncation helpers are temporary.** The `Truncate*` functions exist for a grace period while older CLI versions may send oversized fields. They are marked with a `TODO(2026-Q2)` for removal.

**Byte-length validation, not rune-length.** Length checks use `len(s)` (byte count) because PostgreSQL `VARCHAR(n)` counts characters but the Go-side check is conservative -- byte count is always >= character count for UTF-8 strings.

## Testing

```bash
go test ./internal/validation/...
```

Tests cover email format validation (valid/invalid addresses, edge cases), domain allowlist logic, `NormalizeEmail`, domain list validation, `ValidateExternalID`, and the truncation helpers with multi-byte UTF-8 strings.

## Dependencies

**Uses:** (standard library only: `fmt`, `regexp`, `strings`, `unicode/utf8`)

**Used by:** `internal/admin`, `internal/api`, `internal/auth`, `cmd/server/main.go`
