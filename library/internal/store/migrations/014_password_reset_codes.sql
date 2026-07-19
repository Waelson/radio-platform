-- 014_password_reset_codes: códigos de verificação para reset de senha via e-mail.
CREATE TABLE IF NOT EXISTS password_reset_codes (
    id         TEXT    PRIMARY KEY,
    user_id    TEXT    NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash  TEXT    NOT NULL,    -- bcrypt hash do código de 6 dígitos
    attempts   INTEGER NOT NULL DEFAULT 0,
    expires_at TEXT    NOT NULL,
    used       INTEGER NOT NULL DEFAULT 0,
    created_at TEXT    NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_prc_user_id    ON password_reset_codes(user_id);
CREATE INDEX IF NOT EXISTS idx_prc_expires_at ON password_reset_codes(expires_at);
