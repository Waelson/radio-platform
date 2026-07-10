CREATE TABLE IF NOT EXISTS hotkey_profiles (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    columns    INTEGER NOT NULL DEFAULT 4,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS hotkey_buttons (
    id           TEXT PRIMARY KEY,
    profile_id   TEXT NOT NULL REFERENCES hotkey_profiles(id) ON DELETE CASCADE,
    position     INTEGER NOT NULL DEFAULT 0,
    label        TEXT NOT NULL DEFAULT '',
    sub_label    TEXT NOT NULL DEFAULT '',
    icon         TEXT NOT NULL DEFAULT '',
    palette      INTEGER NOT NULL DEFAULT 0,
    track_id     TEXT REFERENCES tracks(id) ON DELETE SET NULL,
    track_path   TEXT NOT NULL DEFAULT '',
    track_title  TEXT NOT NULL DEFAULT '',
    track_artist TEXT NOT NULL DEFAULT '',
    track_type   TEXT NOT NULL DEFAULT '',
    duration_ms  INTEGER NOT NULL DEFAULT 0,
    created_at   DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_hotkey_buttons_profile
    ON hotkey_buttons(profile_id, position);
