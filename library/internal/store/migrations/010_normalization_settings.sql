-- Migration 010: normalization settings
--
-- Insere os valores padrão dos parâmetros de normalização automática de volume
-- na tabela settings (criada em 007). INSERT OR IGNORE garante idempotência.
--
-- normalization.enabled            : ativa/desativa a normalização globalmente
-- normalization.target_lufs        : target global de loudness (LUFS)
-- normalization.ceiling_dbtp       : teto de true peak (dBTP) anti-clipping
-- normalization.max_gain_db        : ganho máximo permitido (proteção contra ruído)
-- normalization.per_type_enabled   : ativa targets específicos por tipo de áudio
-- normalization.target_lufs_music  : target para MUSIC
-- normalization.target_lufs_jingle : target para JINGLE
-- normalization.target_lufs_vinheta: target para VINHETA
-- normalization.target_lufs_spot   : target para SPOT (spots comerciais costumam ser masterizados mais altos)
-- normalization.worker_concurrency : número de workers paralelos de análise

INSERT OR IGNORE INTO settings (key, value) VALUES
    ('normalization.enabled',               'true'),
    ('normalization.target_lufs',           '-16.0'),
    ('normalization.ceiling_dbtp',          '-1.0'),
    ('normalization.max_gain_db',           '12.0'),
    ('normalization.per_type_enabled',      'false'),
    ('normalization.target_lufs_music',     '-16.0'),
    ('normalization.target_lufs_jingle',    '-16.0'),
    ('normalization.target_lufs_vinheta',   '-18.0'),
    ('normalization.target_lufs_spot',      '-14.0'),
    ('normalization.worker_concurrency',    '2');
