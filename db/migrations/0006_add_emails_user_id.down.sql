-- Rollback for 0006_add_emails_user_id.up.sql
-- Remove index and column added in the up migration
DROP INDEX IF EXISTS emails_user_id_idx;
ALTER TABLE emails DROP COLUMN IF EXISTS user_id;
