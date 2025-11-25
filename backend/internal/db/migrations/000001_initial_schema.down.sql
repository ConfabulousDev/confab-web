-- Rollback initial schema
-- Drop tables in reverse dependency order

DROP TABLE IF EXISTS device_codes;
DROP TABLE IF EXISTS session_share_accesses;
DROP TABLE IF EXISTS session_share_invites;
DROP TABLE IF EXISTS session_shares;
DROP TABLE IF EXISTS files;
DROP TABLE IF EXISTS runs;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS web_sessions;
DROP TABLE IF EXISTS user_identities;
DROP TABLE IF EXISTS users;
