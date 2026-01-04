# Task: Implement getOrCreate for API Keys in Device Auth Flow

## Context

When users authenticate via the device code flow, the CLI sends a `key_name` parameter (format: `<hostname> (Confab CLI)`). Currently, the backend always creates a new API key. This leads to duplicate keys when users re-authenticate from the same machine.

## Requirements

1. Modify the device token endpoint to reuse an existing API key if one with the same name already exists for the authenticated user.
2. Enforce uniqueness on API key names per user (add database constraint).

## Current Flow

1. `POST /auth/device/code` — client sends `{"key_name": "..."}`
2. User authorizes in browser
3. `POST /auth/device/token` — client polls with `device_code`
4. Backend creates new API key, returns `{"access_token": "...", "token_type": "bearer"}`

## New Behavior

In step 4, before creating a new API key:

1. Look up existing API keys for the authenticated user
2. If a key with matching `key_name` exists, return that key's token
3. Otherwise, create a new key as before

## Database Changes

- Add unique constraint on `(user_id, key_name)` for API keys table
- Migration should handle any existing duplicates first (keep most recent, delete others)

## API Contract

- **No changes to request/response schema**
- **No changes to HTTP status codes**
- Matching is exact string match on `key_name`
- Return the existing key's token value (not a new token for the same key)

## Edge Cases

- Key exists but is revoked/disabled: treat as non-existent, create new key

## Testing

- Auth from new machine → creates key
- Auth again from same machine (same hostname) → returns same key, no new key created
- Auth from different machine → creates separate key
- Verify unique constraint prevents duplicate names via direct DB insert
