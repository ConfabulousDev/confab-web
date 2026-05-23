-- CF-483: Demo mode per-user read-only flag.
--
-- Adds a single boolean column to `users`. When DEMO_IDENTITY_EMAIL is set,
-- the bootstrap path flips this column to TRUE for the designated demo
-- identity; the EnforceReadOnly HTTP middleware then rejects mutating
-- requests (POST/PUT/PATCH/DELETE) for any user whose row carries this
-- flag, returning the documented {"error":"read_only_user", ...} body.
--
-- Default FALSE means: when demo mode is unset (or for every non-demo
-- user), zero behavior change. Per CLAUDE.md the application owns
-- validation; we do not add a CHECK constraint here.

ALTER TABLE users ADD COLUMN read_only BOOLEAN NOT NULL DEFAULT FALSE;
