-- Migration 011: add cue point columns to tracks
-- All columns are nullable — NULL means "not set / use default behaviour".
ALTER TABLE tracks ADD COLUMN cue_in_ms  INTEGER DEFAULT NULL;
ALTER TABLE tracks ADD COLUMN intro_ms   INTEGER DEFAULT NULL;
ALTER TABLE tracks ADD COLUMN outro_ms   INTEGER DEFAULT NULL;
ALTER TABLE tracks ADD COLUMN cue_out_ms INTEGER DEFAULT NULL;
