# dbauth

Authentication and authorization database operations: OAuth identity management, password authentication with lockout, web session management, API key CRUD, and device code flow for CLI login.

## Files

| File | Role |
|------|------|
| `store.go` | `Store` struct definition and OpenTelemetry tracer |
| `oauth.go` | `FindOrCreateUserByOAuth` -- finds user by provider identity, links new identities to existing accounts by email match, or creates new users. Resolves pending share recipients on user creation. |
| `password.go` | `AuthenticatePassword`, `CreatePasswordUser`, `UpdateUserPassword`, `GetUserByEmail`, `IsUserAdmin`. Includes bcrypt verification, account lockout after failed attempts, and timing-attack mitigation. |
| `web_sessions.go` | `CreateWebSession`, `GetWebSession`, `DeleteWebSession` -- browser session management with expiration |
| `api_keys.go` | `ValidateAPIKey`, `CreateAPIKeyWithReturn`, `ReplaceAPIKey`, `ListAPIKeys`, `DeleteAPIKey`, `CountAPIKeys`, `UpdateAPIKeyLastUsed` -- API key lifecycle with per-user limits |
| `device_codes.go` | `CreateDeviceCode`, `GetDeviceCodeByUserCode`, `GetDeviceCodeByDeviceCode`, `AuthorizeDeviceCode`, `DeleteDeviceCode` -- OAuth device code flow for CLI authentication |

## Key API

- **`FindOrCreateUserByOAuth(ctx, info)`** -- Three-step flow: (1) find by provider+provider_id, (2) find by email and link identity, (3) create new user+identity. Resolves pending share recipients on new user creation (step 3).
- **`AuthenticatePassword(ctx, email, password)`** -- Verifies credentials with bcrypt. Tracks failed attempts and locks accounts after 5 failures for 15 minutes. Uses constant-time comparison even for nonexistent users.
- **`CreatePasswordUser(ctx, email, hash, isAdmin)`** -- Creates user, password identity, and credentials in one transaction. Derives display name from email prefix. Resolves pending share recipients.
- **`ReplaceAPIKey(ctx, userID, keyHash, name)`** -- Atomically replaces an existing key with the same name or creates a new one (subject to `MaxAPIKeysPerUser`). Used by CLI device code flow.
- **`ValidateAPIKey(ctx, keyHash)`** -- Returns user info for a key hash. Used by the auth middleware on every API request.
- **`AuthorizeDeviceCode(ctx, userCode, userID)`** -- Links a device code to a user. Only succeeds if the code is unexpired and unauthorized.

## How to Extend

1. **New OAuth provider**: No code changes needed -- `FindOrCreateUserByOAuth` is provider-agnostic. Just populate `OAuthUserInfo` with the new provider name.
2. **New auth method**: Add a new file (e.g., `saml.go`), create identity rows with a new `provider` value, and wire into the API layer.
3. **Adjusting lockout policy**: Change `MaxFailedAttempts` and `LockoutDuration` constants in `password.go`.

## Invariants

- API key limit is enforced at `MaxAPIKeysPerUser` (500) before insertion, not via database constraints.
- API key names are unique per user (enforced by database unique constraint; `ErrAPIKeyNameExists` on violation).
- Password lockout is tracked per-identity in `identity_passwords.failed_attempts`/`locked_until`. A successful login resets both.
- Both `FindOrCreateUserByOAuth` and `CreatePasswordUser` resolve pending share recipients (`session_share_recipients` rows with `user_id IS NULL`) for newly created users.
- Web sessions are filtered by `expires_at > NOW()` in the SELECT query, not by application-level expiry checks.
- Device codes must be both unexpired and unauthorized (`authorized_at IS NULL`) to be authorized.

## Design Decisions

- **Identity table pattern**: Users can have multiple identities (OAuth providers, password). The `user_identities` table links providers to users, with `identity_passwords` as a child table for password-specific data.
- **Account linking by email**: When a new OAuth identity's email matches an existing user, the identity is automatically linked rather than creating a duplicate account.
- **Timing-attack mitigation**: `AuthenticatePassword` performs a dummy bcrypt comparison for nonexistent users to prevent email enumeration via response timing.
- **Transactional user creation**: User + identity + credentials are created in a single transaction to prevent orphaned rows on partial failure.
- **`ReplaceAPIKey` atomicity**: Delete-then-insert is wrapped in a transaction so the old key is only removed if the new one is successfully created.

## Testing

- Integration tests per domain: `oauth_test.go`, `password_test.go`, `web_sessions_test.go`, `api_keys_test.go`, `device_codes_test.go`
- Tests cover: account linking, lockout progression and reset, key limit enforcement, device code lifecycle, and web session expiry.

## Dependencies

- `github.com/ConfabulousDev/confab-web/internal/db` -- Root DB package for types, errors, helpers
- `github.com/ConfabulousDev/confab-web/internal/models` -- `User`, `WebSession`, `APIKey`, `OAuthUserInfo`, `UserStatus` types
- `golang.org/x/crypto/bcrypt` -- Password hashing and verification
- `go.opentelemetry.io/otel` -- Distributed tracing
