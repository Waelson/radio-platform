-- Migration 007: settings (key → value)
--
-- Tabela genérica de configurações operacionais mantidas no banco de dados.
-- Permite alterar configurações em tempo de execução sem reiniciar o serviço.
-- Extensível: novas configurações são adicionadas com INSERT OR IGNORE sem nova migration.

CREATE TABLE IF NOT EXISTS settings (
    key        TEXT     PRIMARY KEY,
    value      TEXT     NOT NULL DEFAULT '',
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- Valores padrão das configurações do log de transmissão
INSERT OR IGNORE INTO settings (key, value) VALUES
    ('transmission_log.dir',                '/var/radioflow/transmission-logs'),
    ('transmission_log.file_name_template', 'transmission_{date}_{hour}.jsonl'),
    ('transmission_log.poll_interval',      '5m'),
    ('transmission_log.grace_period',       '15m'),
    ('transmission_log.retention_days',     '30');

-- Dados da emissora (usados no cabeçalho da declaração ECAD)
INSERT OR IGNORE INTO settings (key, value) VALUES
    ('station.name',      ''),
    ('station.cnpj',      ''),
    ('station.frequency', ''),
    ('station.type',      'FM'),
    ('station.city',      ''),
    ('station.state',     '');
