-- Create token blacklist table for invalidated/logged out tokens
CREATE TABLE IF NOT EXISTS token_blacklist (
    id BIGSERIAL PRIMARY KEY,
    token_jti VARCHAR(255) NOT NULL UNIQUE, -- JWT ID (jti claim)
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    blacklisted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL, -- When token would naturally expire
    reason VARCHAR(50) DEFAULT 'logout' -- logout, password_changed, etc.
);

-- Index for fast lookup by JTI
CREATE INDEX idx_token_blacklist_jti ON token_blacklist(token_jti);

-- Index for cleanup of expired tokens
CREATE INDEX idx_token_blacklist_expires ON token_blacklist(expires_at);

-- Index by user for viewing user's invalidated tokens
CREATE INDEX idx_token_blacklist_user ON token_blacklist(user_id);

COMMENT ON TABLE token_blacklist IS 'Stores invalidated JWT tokens (logout, password change, etc.)';
COMMENT ON COLUMN token_blacklist.token_jti IS 'JWT ID from token claims - used to identify specific tokens';
COMMENT ON COLUMN token_blacklist.expires_at IS 'Original token expiration - can cleanup after this time';
