-- Migration 008: adiciona engine_id à tabela transmission_log
-- Permite identificar qual instância do Playout Engine gerou cada registro.
ALTER TABLE transmission_log ADD COLUMN engine_id TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_transmission_log_engine_id ON transmission_log(engine_id);
