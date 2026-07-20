-- 013_users: tabela de usuários para autenticação do Player.
CREATE TABLE IF NOT EXISTS users (
    id               TEXT    PRIMARY KEY,
    email            TEXT    UNIQUE NOT NULL,
    name             TEXT    NOT NULL,
    password_hash    TEXT    NOT NULL,
    role             TEXT    NOT NULL DEFAULT 'operator'
                             CHECK (role IN ('admin', 'operator')),
    force_change_pwd INTEGER NOT NULL DEFAULT 0,
    created_at       TEXT    NOT NULL,
    updated_at       TEXT    NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_role  ON users(role);
