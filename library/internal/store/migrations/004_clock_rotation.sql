-- Categorias de rotação musical
CREATE TABLE IF NOT EXISTS categories (
    id          TEXT    PRIMARY KEY,
    name        TEXT    NOT NULL UNIQUE,
    description TEXT    NOT NULL DEFAULT '',
    color       TEXT    NOT NULL DEFAULT '#888888',
    created_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- Associação M:N faixa <-> categoria
CREATE TABLE IF NOT EXISTS track_categories (
    track_id    TEXT NOT NULL REFERENCES tracks(id)     ON DELETE CASCADE,
    category_id TEXT NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    PRIMARY KEY (track_id, category_id)
);
CREATE INDEX IF NOT EXISTS idx_track_categories_category ON track_categories(category_id);
CREATE INDEX IF NOT EXISTS idx_track_categories_track    ON track_categories(track_id);

-- Clocks: templates de 60 minutos
CREATE TABLE IF NOT EXISTS clocks (
    id         TEXT    PRIMARY KEY,
    name       TEXT    NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- Slots ordenados dentro de um clock
-- slot_type: CATEGORY | JINGLE | SPOT | VINHETA | HORA_CERTA | FIXED
CREATE TABLE IF NOT EXISTS clock_slots (
    id               TEXT    PRIMARY KEY,
    clock_id         TEXT    NOT NULL REFERENCES clocks(id) ON DELETE CASCADE,
    position         INTEGER NOT NULL,
    slot_type        TEXT    NOT NULL CHECK(slot_type IN ('CATEGORY','JINGLE','SPOT','VINHETA','HORA_CERTA','FIXED')),
    category_id      TEXT    REFERENCES categories(id) ON DELETE SET NULL,
    fixed_track_id   TEXT    REFERENCES tracks(id)     ON DELETE SET NULL,
    duration_hint_ms INTEGER NOT NULL DEFAULT 0,
    UNIQUE(clock_id, position)
);
CREATE INDEX IF NOT EXISTS idx_clock_slots_clock ON clock_slots(clock_id);

-- Grade 24x7: qual clock toca em cada hora de cada dia da semana
-- weekday: 0=domingo, 1=segunda, ..., 6=sabado
CREATE TABLE IF NOT EXISTS clock_schedule (
    weekday  INTEGER NOT NULL CHECK(weekday BETWEEN 0 AND 6),
    hour     INTEGER NOT NULL CHECK(hour    BETWEEN 0 AND 23),
    clock_id TEXT    REFERENCES clocks(id) ON DELETE SET NULL,
    PRIMARY KEY (weekday, hour)
);

-- Regras de separação mínima entre faixas
-- field: artist | title | category | album
CREATE TABLE IF NOT EXISTS separation_rules (
    id              TEXT    PRIMARY KEY,
    field           TEXT    NOT NULL CHECK(field IN ('artist','title','category','album')),
    min_sep_minutes INTEGER NOT NULL DEFAULT 60
);

-- Log append-only do que foi programado/tocado (base para regras de separação entre sessões)
CREATE TABLE IF NOT EXISTS rotation_log (
    id          TEXT    PRIMARY KEY,
    track_id    TEXT    NOT NULL,
    played_at   DATETIME NOT NULL,
    clock_id    TEXT    NOT NULL DEFAULT '',
    slot_type   TEXT    NOT NULL DEFAULT '',
    category_id TEXT    NOT NULL DEFAULT '',
    artist      TEXT    NOT NULL DEFAULT '',
    title       TEXT    NOT NULL DEFAULT '',
    album       TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_rotation_log_played_at ON rotation_log(played_at);
CREATE INDEX IF NOT EXISTS idx_rotation_log_track_id  ON rotation_log(track_id);
CREATE INDEX IF NOT EXISTS idx_rotation_log_artist    ON rotation_log(artist);
