-- Add user_id to emails and index for efficient per-user queries
ALTER TABLE emails
    ADD COLUMN IF NOT EXISTS user_id bigint;

-- Create index to speed up queries filtering by user_id
CREATE INDEX IF NOT EXISTS emails_user_id_idx ON emails (user_id);

-- Optional: depending on use-case, consider adding a foreign key to users table
-- ALTER TABLE emails
--     ADD CONSTRAINT fk_emails_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL;
