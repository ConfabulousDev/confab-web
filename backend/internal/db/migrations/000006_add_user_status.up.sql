-- Add status column to users table for soft-delete functionality
-- Step 1: Add nullable column
ALTER TABLE users ADD COLUMN status VARCHAR(20);

-- Step 2: Set all existing users to 'active'
UPDATE users SET status = 'active';

-- Step 3: Make column NOT NULL (no default - all queries must provide status)
ALTER TABLE users ALTER COLUMN status SET NOT NULL;

-- Add index for filtering by status
CREATE INDEX idx_users_status ON users(status);
