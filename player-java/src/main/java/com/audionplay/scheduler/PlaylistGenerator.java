package com.audionplay.scheduler;

import com.audionplay.db.Database;
import com.audionplay.db.PooledConnection;
import com.audionplay.db.entity.TrackEntity;
import com.audionplay.db.entity.TrackEntity.LoudnessStatus;
import com.audionplay.db.entity.TrackEntity.TrackType;

import java.sql.*;
import java.text.Normalizer;
import java.time.*;
import java.util.*;

/**
 * Gerador de playlist baseado em clocks e regras de separação.
 *
 * Traduz a lógica do library/internal/scheduler/generator.go diretamente no player-java,
 * usando o mesmo library.db via SQLite. Não depende de nenhum serviço externo.
 *
 * Algoritmo em 3 fases por slot:
 *   1. Estrito  — aplica todas as regras de separação
 *   2. Relaxado — descarta a regra de menor janela e tenta novamente
 *   3. Fallback — usa a faixa tocada há mais tempo (ou aleatória)
 */
public class PlaylistGenerator {

    // ── Tipos de saída ────────────────────────────────────────────────────────

    public record GeneratedItem(
        int hour, int position, String slotType, String clockName, String categoryName,
        TrackEntity track
    ) {}

    public record GenerateResult(List<GeneratedItem> items, List<String> warnings) {}

    // ── Tipos internos ────────────────────────────────────────────────────────

    private record SeparationRule(String id, String field, int minSepMinutes) {}

    private record SlotData(
        String id, int position, String slotType,
        String categoryId, String categoryName,
        String fixedTrackId, long durationHintMs
    ) {}

    private record ClockData(String id, String name, List<SlotData> slots) {}

    private record ResolveResult(TrackEntity track, String warning) {}

    private record RelaxResult(List<SeparationRule> rules, String droppedField) {}

    // ── Ponto de entrada ──────────────────────────────────────────────────────

    /**
     * Gera uma playlist a partir de {@code from} cobrindo {@code hours} horas.
     *
     * @param from   data/hora inicial (sem fuso — tratada como horário local)
     * @param hours  número de horas a cobrir (1–24)
     */
    public GenerateResult generate(LocalDateTime from, int hours) throws SQLException {
        if (hours <= 0) hours = 1;
        if (hours > 24) hours = 24;

        List<SeparationRule> rules = loadSeparationRules();
        int maxLookbackMin = maxLookbackMinutes(rules);

        // cursor e sinceMs em milissegundos desde epoch (UTC)
        long fromMs   = from.toInstant(ZoneOffset.UTC).toEpochMilli();
        long sinceMs  = from.minusMinutes(maxLookbackMin).toInstant(ZoneOffset.UTC).toEpochMilli();

        // Histórico persistente do rotation_log
        Map<String, Long> persistentHistory = loadRecentTrackIds(sinceMs);   // trackId  → playedAt ms
        Map<String, Long> persistentArtist  = loadRecentArtists(sinceMs);    // artist↓  → playedAt ms

        // Histórico acumulado durante esta geração
        Map<String, Long> sessionHistory  = new HashMap<>();
        Map<String, Long> sessionArtist   = new HashMap<>();
        Map<String, Long> sessionCategory = new HashMap<>();

        List<GeneratedItem> items    = new ArrayList<>();
        List<String>        warnings = new ArrayList<>();

        long limitMs       = (long) hours * 3_600_000L;
        long accumulatedMs = 0L;
        boolean done = false;

        for (int h = 0; h < hours && !done; h++) {
            LocalDateTime t = from.plusHours(h);
            int weekday = t.getDayOfWeek().getValue() % 7; // 0=Dom … 6=Sab
            int hour    = t.getHour();

            ClockData clock = getClockForHour(weekday, hour);
            if (clock == null) {
                warnings.add(String.format("hora %d (%s): nenhum clock configurado na grade — hora ignorada",
                    hour, t.getDayOfWeek()));
                continue;
            }

            long cursor = fromMs + (long) h * 3_600_000L; // posição simulada em ms

            for (SlotData slot : clock.slots()) {
                if (accumulatedMs >= limitMs) { done = true; break; }

                ResolveResult res = resolveSlot(
                    slot, cursor, rules,
                    persistentHistory, persistentArtist,
                    sessionHistory, sessionArtist, sessionCategory
                );

                if (res.track() == null) {
                    String msg = res.warning() != null && !res.warning().isBlank()
                        ? res.warning()
                        : "nenhuma faixa disponível — slot ignorado";
                    warnings.add(String.format("hora %d, slot %d (%s, tipo %s): %s",
                        hour, slot.position(), clock.name(), slot.slotType(), msg));
                    continue;
                }

                if (res.warning() != null && !res.warning().isBlank()) {
                    warnings.add(String.format("hora %d, slot %d (%s): %s",
                        hour, slot.position(), clock.name(), res.warning()));
                }

                TrackEntity track = res.track();
                long dur = slot.durationHintMs() > 0 ? slot.durationHintMs() : track.durationMs();
                if (dur == 0) dur = 3 * 60_000L; // fallback 3 min

                // Lookahead cap
                if (!items.isEmpty() && accumulatedMs + dur > limitMs) { done = true; break; }

                items.add(new GeneratedItem(
                    hour, slot.position(), slot.slotType(),
                    clock.name(), slot.categoryName(), track
                ));
                accumulatedMs += dur;

                // Registra no histórico de sessão
                sessionHistory.put(track.id(), cursor);
                if (track.artist() != null && !track.artist().isBlank()) {
                    sessionArtist.put(normalizeField(track.artist()), cursor);
                }
                if (slot.categoryId() != null && !slot.categoryId().isBlank()) {
                    sessionCategory.put(slot.categoryId(), cursor);
                }
                cursor += dur;
            }
        }

        return new GenerateResult(items, warnings);
    }

    // ── Resolver de slot ──────────────────────────────────────────────────────

    private ResolveResult resolveSlot(
        SlotData slot, long cursor,
        List<SeparationRule> rules,
        Map<String, Long> persistentHistory,
        Map<String, Long> persistentArtist,
        Map<String, Long> sessionHistory,
        Map<String, Long> sessionArtist,
        Map<String, Long> sessionCategory
    ) throws SQLException {

        // FIXED — faixa pinada
        if ("FIXED".equals(slot.slotType())) {
            if (slot.fixedTrackId() == null || slot.fixedTrackId().isBlank()) {
                return new ResolveResult(null, "slot FIXED sem track_id configurado");
            }
            TrackEntity t = findTrackById(slot.fixedTrackId());
            return t != null
                ? new ResolveResult(t, "")
                : new ResolveResult(null, "track fixo não encontrado: " + slot.fixedTrackId());
        }

        // HORA_CERTA — sentinela (path resolvido em tempo de reprodução pelo player)
        if ("HORA_CERTA".equals(slot.slotType())) {
            TrackEntity sentinel = new TrackEntity(
                "hora_certa", "", "Hora Certa", "", "",
                TrackType.HORA_CERTA, 10_000,
                null, null, null, null, null, null,
                LoudnessStatus.PENDING, "", null,
                null, null, null, null, null
            );
            return new ResolveResult(sentinel, "");
        }

        // Carrega candidatos conforme tipo do slot
        List<TrackEntity> candidates;
        switch (slot.slotType()) {
            case "CATEGORY" -> {
                if (slot.categoryId() == null || slot.categoryId().isBlank()) {
                    return new ResolveResult(null, "slot CATEGORY sem category_id configurado");
                }
                candidates = tracksByCategory(slot.categoryId());
            }
            case "JINGLE", "SPOT", "VINHETA" -> candidates = tracksByType(slot.slotType());
            default -> { return new ResolveResult(null, "tipo de slot desconhecido: " + slot.slotType()); }
        }

        if (candidates.isEmpty()) return new ResolveResult(null, "");

        // Fase 1 — estrito
        List<TrackEntity> filtered = applyAllRules(
            candidates, rules, slot, cursor,
            persistentHistory, persistentArtist,
            sessionHistory, sessionArtist, sessionCategory
        );
        if (!filtered.isEmpty()) return new ResolveResult(pick(filtered), "");

        // Fase 2 — relaxado
        if (!rules.isEmpty()) {
            RelaxResult relaxed = relaxRules(rules);
            filtered = applyAllRules(
                candidates, relaxed.rules(), slot, cursor,
                persistentHistory, persistentArtist,
                sessionHistory, sessionArtist, sessionCategory
            );
            if (!filtered.isEmpty()) {
                return new ResolveResult(pick(filtered),
                    "separação relaxada (regra \"" + relaxed.droppedField() + "\" descartada)");
            }
        }

        // Fase 3 — fallback: mais antiga na categoria, ou aleatória
        TrackEntity oldest = findOldestInCategory(candidates, slot.categoryId());
        if (oldest != null) {
            return new ResolveResult(oldest, "separação ignorada (fallback — candidatos insuficientes)");
        }
        return new ResolveResult(pick(candidates), "separação ignorada (fallback — sem histórico)");
    }

    // ── Separação ─────────────────────────────────────────────────────────────

    private List<TrackEntity> applyAllRules(
        List<TrackEntity> candidates, List<SeparationRule> rules, SlotData slot, long cursor,
        Map<String, Long> persistentHistory, Map<String, Long> persistentArtist,
        Map<String, Long> sessionHistory, Map<String, Long> sessionArtist,
        Map<String, Long> sessionCategory
    ) {
        List<TrackEntity> out = new ArrayList<>();
        for (TrackEntity t : candidates) {
            if (!violatesRules(t, slot, rules, cursor, persistentHistory, persistentArtist,
                               sessionHistory, sessionArtist, sessionCategory)) {
                out.add(t);
            }
        }
        return out;
    }

    private boolean violatesRules(
        TrackEntity t, SlotData slot, List<SeparationRule> rules, long cursor,
        Map<String, Long> persistentHistory, Map<String, Long> persistentArtist,
        Map<String, Long> sessionHistory, Map<String, Long> sessionArtist,
        Map<String, Long> sessionCategory
    ) {
        for (SeparationRule r : rules) {
            long cutoff = cursor - (long) r.minSepMinutes() * 60_000L;
            switch (r.field()) {
                case "title" -> {
                    Long p = persistentHistory.get(t.id());
                    if (p != null && p > cutoff) return true;
                    Long s = sessionHistory.get(t.id());
                    if (s != null && s > cutoff) return true;
                }
                case "artist" -> {
                    if (t.artist() == null || t.artist().isBlank()) continue;
                    // persistentArtist carregado do rotation_log
                    Long p = persistentArtist.get(normalizeField(t.artist()));
                    if (p != null && p > cutoff) return true;
                    Long s = sessionArtist.get(normalizeField(t.artist()));
                    if (s != null && s > cutoff) return true;
                }
                case "category" -> {
                    if (slot.categoryId() == null || slot.categoryId().isBlank()) continue;
                    Long s = sessionCategory.get(slot.categoryId());
                    if (s != null && s > cutoff) return true;
                }
                // album — simplificação igual ao Go: verificação de sessão não implementada
            }
        }
        return false;
    }

    private RelaxResult relaxRules(List<SeparationRule> rules) {
        if (rules.isEmpty()) return new RelaxResult(rules, "");
        int minIdx = 0;
        for (int i = 1; i < rules.size(); i++) {
            if (rules.get(i).minSepMinutes() < rules.get(minIdx).minSepMinutes()) minIdx = i;
        }
        String dropped = rules.get(minIdx).field();
        List<SeparationRule> out = new ArrayList<>(rules);
        out.remove(minIdx);
        return new RelaxResult(out, dropped);
    }

    private static TrackEntity pick(List<TrackEntity> tracks) {
        return tracks.get(new Random().nextInt(tracks.size()));
    }

    private static int maxLookbackMinutes(List<SeparationRule> rules) {
        int max = 120;
        for (SeparationRule r : rules) {
            if (r.minSepMinutes() > max) max = r.minSepMinutes();
        }
        return max;
    }

    // ── Consultas SQL ─────────────────────────────────────────────────────────

    private List<SeparationRule> loadSeparationRules() throws SQLException {
        String sql = "SELECT id, field, min_sep_minutes FROM separation_rules";
        try (PooledConnection pc = Database.pool().borrow();
             Statement st = pc.get().createStatement();
             ResultSet rs = st.executeQuery(sql)) {
            List<SeparationRule> list = new ArrayList<>();
            while (rs.next()) {
                list.add(new SeparationRule(
                    rs.getString("id"), rs.getString("field"), rs.getInt("min_sep_minutes")
                ));
            }
            return list;
        }
    }

    /**
     * Retorna trackId → played_at (ms epoch) para faixas tocadas desde sinceMs.
     * Usa o maior played_at por track para o caso de múltiplas ocorrências.
     */
    private Map<String, Long> loadRecentTrackIds(long sinceMs) throws SQLException {
        String sql = """
            SELECT track_id, MAX(strftime('%s', played_at)) * 1000 AS played_ms
            FROM rotation_log
            WHERE played_at >= datetime(?, 'unixepoch')
            GROUP BY track_id
            """;
        Map<String, Long> map = new HashMap<>();
        try (PooledConnection pc = Database.pool().borrow();
             PreparedStatement ps = pc.get().prepareStatement(sql)) {
            ps.setLong(1, sinceMs / 1000L);
            try (ResultSet rs = ps.executeQuery()) {
                while (rs.next()) map.put(rs.getString(1), rs.getLong(2));
            }
        }
        return map;
    }

    /**
     * Retorna artist_normalizado → played_at (ms epoch) mais recente, desde sinceMs.
     */
    private Map<String, Long> loadRecentArtists(long sinceMs) throws SQLException {
        String sql = """
            SELECT artist, MAX(strftime('%s', played_at)) * 1000 AS played_ms
            FROM rotation_log
            WHERE played_at >= datetime(?, 'unixepoch') AND artist != ''
            GROUP BY artist
            """;
        Map<String, Long> map = new HashMap<>();
        try (PooledConnection pc = Database.pool().borrow();
             PreparedStatement ps = pc.get().prepareStatement(sql)) {
            ps.setLong(1, sinceMs / 1000L);
            try (ResultSet rs = ps.executeQuery()) {
                while (rs.next()) map.put(normalizeField(rs.getString(1)), rs.getLong(2));
            }
        }
        return map;
    }

    private ClockData getClockForHour(int weekday, int hour) throws SQLException {
        String sql = """
            SELECT c.id, c.name
            FROM clock_schedule cs JOIN clocks c ON c.id = cs.clock_id
            WHERE cs.weekday = ? AND cs.hour = ?
            """;
        String clockId = null, clockName = null;
        try (PooledConnection pc = Database.pool().borrow();
             PreparedStatement ps = pc.get().prepareStatement(sql)) {
            ps.setInt(1, weekday);
            ps.setInt(2, hour);
            try (ResultSet rs = ps.executeQuery()) {
                if (!rs.next()) return null;
                clockId   = rs.getString("id");
                clockName = rs.getString("name");
            }
        }
        List<SlotData> slots = loadSlots(clockId);
        return new ClockData(clockId, clockName, slots);
    }

    private List<SlotData> loadSlots(String clockId) throws SQLException {
        String sql = """
            SELECT s.id, s.position, s.slot_type, s.category_id,
                   cat.name AS category_name, s.fixed_track_id, s.duration_hint_ms
            FROM clock_slots s
            LEFT JOIN categories cat ON cat.id = s.category_id
            WHERE s.clock_id = ? ORDER BY s.position
            """;
        List<SlotData> list = new ArrayList<>();
        try (PooledConnection pc = Database.pool().borrow();
             PreparedStatement ps = pc.get().prepareStatement(sql)) {
            ps.setString(1, clockId);
            try (ResultSet rs = ps.executeQuery()) {
                while (rs.next()) {
                    list.add(new SlotData(
                        rs.getString("id"),
                        rs.getInt("position"),
                        rs.getString("slot_type"),
                        rs.getString("category_id"),
                        rs.getString("category_name"),
                        rs.getString("fixed_track_id"),
                        rs.getLong("duration_hint_ms")
                    ));
                }
            }
        }
        return list;
    }

    private List<TrackEntity> tracksByCategory(String categoryId) throws SQLException {
        String sql = """
            SELECT t.* FROM tracks t
            JOIN track_categories tc ON tc.track_id = t.id
            WHERE tc.category_id = ? AND t.duration_ms > 0
            ORDER BY t.title COLLATE NOCASE
            """;
        return queryTracks(sql, categoryId);
    }

    private List<TrackEntity> tracksByType(String type) throws SQLException {
        String sql = "SELECT * FROM tracks WHERE type = ? AND duration_ms > 0";
        return queryTracks(sql, type);
    }

    private TrackEntity findTrackById(String id) throws SQLException {
        String sql = "SELECT * FROM tracks WHERE id = ?";
        List<TrackEntity> list = queryTracks(sql, id);
        return list.isEmpty() ? null : list.get(0);
    }

    private List<TrackEntity> queryTracks(String sql, String param) throws SQLException {
        List<TrackEntity> list = new ArrayList<>();
        try (PooledConnection pc = Database.pool().borrow();
             PreparedStatement ps = pc.get().prepareStatement(sql)) {
            ps.setString(1, param);
            try (ResultSet rs = ps.executeQuery()) {
                while (rs.next()) list.add(mapTrack(rs));
            }
        }
        return list;
    }

    /** Retorna o track mais antigo do rotation_log que ainda consta em candidates. */
    private TrackEntity findOldestInCategory(List<TrackEntity> candidates, String categoryId) throws SQLException {
        if (categoryId == null || categoryId.isBlank() || candidates.isEmpty()) return null;
        String sql = """
            SELECT track_id FROM rotation_log
            WHERE category_id = ?
            ORDER BY played_at ASC LIMIT 1
            """;
        String oldestId = null;
        try (PooledConnection pc = Database.pool().borrow();
             PreparedStatement ps = pc.get().prepareStatement(sql)) {
            ps.setString(1, categoryId);
            try (ResultSet rs = ps.executeQuery()) {
                if (rs.next()) oldestId = rs.getString(1);
            }
        }
        if (oldestId == null) return null;
        for (TrackEntity t : candidates) {
            if (t.id().equals(oldestId)) return t;
        }
        return null;
    }

    // ── Mapeamento ResultSet → TrackEntity ────────────────────────────────────

    private TrackEntity mapTrack(ResultSet rs) throws SQLException {
        return new TrackEntity(
            rs.getString("id"),
            rs.getString("path"),
            coalesce(rs.getString("title"), ""),
            coalesce(rs.getString("artist"), ""),
            coalesce(rs.getString("album"), ""),
            TrackType.of(rs.getString("type")),
            rs.getInt("duration_ms"),
            rs.getString("category"),
            nullStr(rs, "isrc"),
            nullStr(rs, "composer"),
            nullStr(rs, "publisher"),
            nullDbl(rs, "loudness_lufs"),
            nullDbl(rs, "true_peak_dbtp"),
            LoudnessStatus.of(rs.getString("loudness_status")),
            coalesce(rs.getString("loudness_error"), ""),
            parseDateTime(rs.getString("loudness_analyzed_at")),
            nullInt(rs, "cue_in_ms"),
            nullInt(rs, "intro_ms"),
            nullInt(rs, "outro_ms"),
            nullInt(rs, "cue_out_ms"),
            parseDateTime(rs.getString("indexed_at"))
        );
    }

    private static String nullStr(ResultSet rs, String col) throws SQLException {
        String v = rs.getString(col); return rs.wasNull() ? null : v;
    }
    private static Double nullDbl(ResultSet rs, String col) throws SQLException {
        double v = rs.getDouble(col); return rs.wasNull() ? null : v;
    }
    private static Integer nullInt(ResultSet rs, String col) throws SQLException {
        int v = rs.getInt(col); return rs.wasNull() ? null : v;
    }
    private static String coalesce(String s, String def) {
        return s == null ? def : s;
    }
    private static LocalDateTime parseDateTime(String v) {
        if (v == null || v.isBlank()) return null;
        try { return LocalDateTime.parse(v.replace(" ", "T")); } catch (Exception e) { return null; }
    }

    // ── Normalização de campos ────────────────────────────────────────────────

    private static String normalizeField(String s) {
        if (s == null) return "";
        String nfd = Normalizer.normalize(s, Normalizer.Form.NFD);
        return nfd.replaceAll("\\p{InCombiningDiacriticalMarks}", "").toLowerCase().strip();
    }
}
