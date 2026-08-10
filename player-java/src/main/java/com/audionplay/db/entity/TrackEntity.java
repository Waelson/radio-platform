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
        MUSIC, VINHETA, JINGLE, SPOT, EFEITOS, HORA_CERTA;

        public static TrackType of(String v) {
            if (v == null) return MUSIC;
            return switch (v.trim().toUpperCase()) {
                case "HORA_CERTA" -> HORA_CERTA;
                case "VINHETA"    -> VINHETA;
                case "JINGLE"     -> JINGLE;
                case "SPOT"       -> SPOT;
                case "EFEITOS"    -> EFEITOS;
                default           -> MUSIC;
            };
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

    /**
     * Cria um item virtual de Hora Certa (não vem do banco de dados).
     *
     * @param concatPath  caminhos dos arquivos separados por "|"  (hora|minuto)
     * @param durationMs  duração total estimada em ms
     */
    public static TrackEntity horaCerta(String concatPath, int durationMs) {
        return new TrackEntity(
            null, concatPath, "Hora Certa", "", "",
            TrackType.HORA_CERTA, durationMs,
            null, null, null, null,
            null, null, LoudnessStatus.PENDING, null, null,
            null, null, null, null,
            java.time.LocalDateTime.now()
        );
    }
}
