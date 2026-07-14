-- Migration 009: loudness fields in tracks
--
-- Armazena o loudness integrado (LUFS) e o true peak (dBTP) medidos pelo ffmpeg
-- via filtro ebur128, além do status de análise por faixa.
--
-- loudness_lufs        : loudness integrado (EBU R128 / ITU-R BS.1770). NULL = não analisado.
-- true_peak_dbtp       : true peak em dBTP. NULL = não analisado.
-- loudness_status      : 'pending' | 'analyzing' | 'done' | 'error'
-- loudness_error       : mensagem de erro quando loudness_status = 'error'
-- loudness_analyzed_at : timestamp da última análise bem-sucedida

ALTER TABLE tracks ADD COLUMN loudness_lufs        REAL;
ALTER TABLE tracks ADD COLUMN true_peak_dbtp       REAL;
ALTER TABLE tracks ADD COLUMN loudness_status      TEXT NOT NULL DEFAULT 'pending';
ALTER TABLE tracks ADD COLUMN loudness_error       TEXT NOT NULL DEFAULT '';
ALTER TABLE tracks ADD COLUMN loudness_analyzed_at DATETIME;

CREATE INDEX IF NOT EXISTS idx_tracks_loudness_status ON tracks(loudness_status);
