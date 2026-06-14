-- Irreversible by design: the deleted rows held raw session/device tokens that
-- were never recorded anywhere else, so there is nothing to restore. Rolling
-- back does not change the storage format (the application hashes tokens at the
-- store layer regardless), so this down migration is an intentional no-op.
SELECT 1;
