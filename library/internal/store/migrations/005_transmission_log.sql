-- Migration 005: transmission_log + campos ECAD em tracks
--
-- Adiciona isrc, composer e publisher à tabela tracks para suporte ao ECAD.
-- Cria a tabela transmission_log para armazenar o histórico de reprodução
-- importado dos arquivos JSONL gerados pelo Playout Engine.

-- Campos ECAD na tabela tracks (ALTER TABLE é idempotente via migration tracking)
ALTER TABLE tracks ADD COLUMN isrc      TEXT NOT NULL DEFAULT '';
ALTER TABLE tracks ADD COLUMN composer  TEXT NOT NULL DEFAULT '';
ALTER TABLE tracks ADD COLUMN publisher TEXT NOT NULL DEFAULT '';

-- Log de transmissão
-- queue_item_id é UNIQUE: garante idempotência no BulkInsert (INSERT OR IGNORE)
CREATE TABLE IF NOT EXISTS transmission_log (
    id                 TEXT     PRIMARY KEY,
    queue_item_id      TEXT     NOT NULL DEFAULT '' UNIQUE,
    asset_id           TEXT     NOT NULL DEFAULT '',
    path               TEXT     NOT NULL DEFAULT '',
    title              TEXT     NOT NULL DEFAULT '',
    artist             TEXT     NOT NULL DEFAULT '',
    type               TEXT     NOT NULL DEFAULT '',   -- MUSIC|JINGLE|VINHETA|SPOT|CART
    isrc               TEXT     NOT NULL DEFAULT '',
    composer           TEXT     NOT NULL DEFAULT '',
    publisher          TEXT     NOT NULL DEFAULT '',
    duration_ms        INTEGER  NOT NULL DEFAULT 0,
    duration_played_ms INTEGER  NOT NULL DEFAULT 0,
    result             TEXT     NOT NULL DEFAULT '',   -- finished|skipped|failed
    status             TEXT     NOT NULL DEFAULT 'FINISHED',
    started_at         DATETIME NOT NULL,
    finished_at        DATETIME,
    break_id           TEXT     NOT NULL DEFAULT '',
    break_title        TEXT     NOT NULL DEFAULT '',
    break_role         TEXT     NOT NULL DEFAULT '',   -- open|spot|close
    break_position     INTEGER  NOT NULL DEFAULT 0,
    import_file_name   TEXT     NOT NULL DEFAULT ''    -- nome do arquivo JSONL de origem
);

CREATE INDEX IF NOT EXISTS idx_transmission_log_started_at ON transmission_log(started_at);
CREATE INDEX IF NOT EXISTS idx_transmission_log_type       ON transmission_log(type);
CREATE INDEX IF NOT EXISTS idx_transmission_log_status     ON transmission_log(status);
CREATE INDEX IF NOT EXISTS idx_transmission_log_asset_id   ON transmission_log(asset_id);
