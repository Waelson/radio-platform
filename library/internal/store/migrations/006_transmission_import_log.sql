-- Migration 006: transmission_import_log
--
-- Registra cada tentativa de importação de um arquivo JSONL pelo importer.
-- Cada tentativa gera uma linha independentemente do resultado (success|failed).
-- Tentativas repetidas do mesmo arquivo (retry após falha) geram linhas distintas.

CREATE TABLE IF NOT EXISTS transmission_import_log (
    id               TEXT     PRIMARY KEY,
    file_name        TEXT     NOT NULL,
    started_at       DATETIME NOT NULL,
    finished_at      DATETIME,
    status           TEXT     NOT NULL DEFAULT 'running', -- running|success|failed
    records_total    INTEGER  NOT NULL DEFAULT 0,          -- linhas lidas no arquivo
    records_imported INTEGER  NOT NULL DEFAULT 0,          -- INSERTs efetivados (excluindo OR IGNORE)
    error_message    TEXT     NOT NULL DEFAULT ''          -- preenchido apenas em status=failed
);

CREATE INDEX IF NOT EXISTS idx_import_log_started_at ON transmission_import_log(started_at);
CREATE INDEX IF NOT EXISTS idx_import_log_status     ON transmission_import_log(status);
