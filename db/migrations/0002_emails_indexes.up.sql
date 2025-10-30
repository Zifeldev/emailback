CREATE INDEX IF NOT EXISTS idx_emails_created_at ON emails (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_emails_language   ON emails (language);

CREATE UNIQUE INDEX IF NOT EXISTS uniq_emails_message_id_partial
ON emails (message_id)
WHERE message_id IS NOT NULL AND message_id <> '';