CREATE TABLE IF NOT EXISTS tracks (
    id          TEXT PRIMARY KEY,
    path        TEXT NOT NULL UNIQUE,
    title       TEXT NOT NULL DEFAULT '',
    artist      TEXT NOT NULL DEFAULT '',
    type        TEXT NOT NULL CHECK(type IN ('MUSIC','VINHETA','JINGLE','SPOT')),
    duration_ms INTEGER NOT NULL DEFAULT 0,
    category    TEXT,
    indexed_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS playlists (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    category   TEXT,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS playlist_items (
    id          TEXT PRIMARY KEY,
    playlist_id TEXT NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
    track_id    TEXT NOT NULL REFERENCES tracks(id)    ON DELETE CASCADE,
    position    INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS breaks (
    id             TEXT PRIMARY KEY,
    name           TEXT NOT NULL,
    open_track_id  TEXT REFERENCES tracks(id) ON DELETE SET NULL,
    close_track_id TEXT REFERENCES tracks(id) ON DELETE SET NULL,
    created_at     DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at     DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS break_items (
    id       TEXT PRIMARY KEY,
    break_id TEXT NOT NULL REFERENCES breaks(id)  ON DELETE CASCADE,
    track_id TEXT NOT NULL REFERENCES tracks(id)  ON DELETE CASCADE,
    position INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_tracks_type     ON tracks(type);
CREATE INDEX IF NOT EXISTS idx_tracks_artist   ON tracks(artist);
CREATE INDEX IF NOT EXISTS idx_tracks_category ON tracks(category);

CREATE INDEX IF NOT EXISTS idx_playlist_items_playlist
    ON playlist_items(playlist_id, position);

CREATE INDEX IF NOT EXISTS idx_break_items_break
    ON break_items(break_id, position);
