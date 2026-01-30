-- Remove password authentication support

DROP TABLE IF EXISTS identity_passwords;

ALTER TABLE users DROP COLUMN IF EXISTS is_admin;
