package com.audionplay.db.entity;

import java.time.LocalDateTime;

/**
 * Representa uma faixa do catálogo (tabela {@code tracks}).
 *
 * Campos opcionais (nullable) são representados como tipos de referência:
 * {@code Integer}, {@code Double}, {@code String} (nullable).
 */
public record TrackEntity(
    String          id,
    String          path,
    String          title,
    String          artist,
    String          album,
    TrackType       type,
    int             durationMs,
    String          category,       // nullable
    String          isrc,
    String          composer,
    String          publisher,
    Double          loudnessLufs,   // nullable
    Double          truePeakDbtp,   // nullable
    LoudnessStatus  loudnessStatus,
    String          loudnessError,
    LocalDateTime   loudnessAnalyzedAt, // nullable
    Integer         cueInMs,        // nullable
    Integer         introMs,        // nullable
    Integer         outroMs,        // nullable
    Integer         cueOutMs,       // nullable
    LocalDateTime   indexedAt
) {

    // ── Enums mapeados ao CHECK do banco ──────────────────────────────────────

    public enum TrackType {
        MUSIC, VINHETA, JINGLE, SPOT, EFEITOS;

        public static TrackType of(String v) {
            return v == null ? MUSIC : valueOf(v.trim().toUpperCase());
        }
    }

    public enum LoudnessStatus {
        PENDING, ANALYZING, DONE, ERROR;

        public static LoudnessStatus of(String v) {
            if (v == null) return PENDING;
            return switch (v.trim().toLowerCase()) {
                case "done"      -> DONE;
                case "analyzing" -> ANALYZING;
                case "error"     -> ERROR;
                default          -> PENDING;
            };
        }
    }

    // ── Helpers de conveniência ───────────────────────────────────────────────

    /** Duração em segundos como double (usado pelo AudioPlayer). */
    public double durationSeconds() {
        return durationMs / 1000.0;
    }

    /** True se a faixa já passou pela análise de loudness. */
    public boolean isLoudnessAnalyzed() {
        return loudnessStatus == LoudnessStatus.DONE;
    }
}
