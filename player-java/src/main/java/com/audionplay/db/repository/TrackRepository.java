package com.audionplay.db.repository;

import com.audionplay.db.Database;
import com.audionplay.db.PooledConnection;
import com.audionplay.db.entity.TrackEntity;
import com.audionplay.db.entity.TrackEntity.LoudnessStatus;
import com.audionplay.db.entity.TrackEntity.TrackType;

import java.sql.*;
import java.time.LocalDateTime;
import java.util.ArrayList;
import java.util.List;
import java.util.Optional;

/**
 * Acesso à tabela {@code tracks} via JDBC puro.
 *
 * Todos os métodos adquirem e liberam conexão via try-with-resources
 * usando {@link com.audionplay.db.PooledConnection}.
 */
public class TrackRepository {

    // ── Consultas ─────────────────────────────────────────────────────────────

    /** Retorna todas as faixas ordenadas por título. */
    public List<TrackEntity> findAll() throws SQLException {
        String sql = "SELECT * FROM tracks ORDER BY title COLLATE NOCASE";
        try (PooledConnection pc = Database.pool().borrow();
             PreparedStatement ps = pc.get().prepareStatement(sql);
             ResultSet rs = ps.executeQuery()) {
            return collectTracks(rs);
        }
    }

    /** Retorna faixas de um tipo específico, ordenadas por título. */
    public List<TrackEntity> findByType(TrackType type) throws SQLException {
        String sql = "SELECT * FROM tracks WHERE type = ? ORDER BY title COLLATE NOCASE";
        try (PooledConnection pc = Database.pool().borrow();
             PreparedStatement ps = pc.get().prepareStatement(sql)) {
            ps.setString(1, type.name());
            try (ResultSet rs = ps.executeQuery()) {
                return collectTracks(rs);
            }
        }
    }

    /** Retorna faixas de uma categoria específica. */
    public List<TrackEntity> findByCategory(String category) throws SQLException {
        String sql = "SELECT * FROM tracks WHERE category = ? ORDER BY title COLLATE NOCASE";
        try (PooledConnection pc = Database.pool().borrow();
             PreparedStatement ps = pc.get().prepareStatement(sql)) {
            ps.setString(1, category);
            try (ResultSet rs = ps.executeQuery()) {
                return collectTracks(rs);
            }
        }
    }

    /**
     * Busca textual em título, artista e álbum.
     * Aceita substring sem precisar de '%' no parâmetro.
     */
    public List<TrackEntity> search(String term) throws SQLException {
        String like = "%" + term.toLowerCase() + "%";
        String sql  = """
            SELECT * FROM tracks
            WHERE lower(title) LIKE ? OR lower(artist) LIKE ? OR lower(album) LIKE ?
            ORDER BY title COLLATE NOCASE
            """;
        try (PooledConnection pc = Database.pool().borrow();
             PreparedStatement ps = pc.get().prepareStatement(sql)) {
            ps.setString(1, like);
            ps.setString(2, like);
            ps.setString(3, like);
            try (ResultSet rs = ps.executeQuery()) {
                return collectTracks(rs);
            }
        }
    }

    /**
     * Busca com filtro opcional de tipo + texto.
     * Qualquer parâmetro nulo é ignorado.
     */
    public List<TrackEntity> search(String term, TrackType type) throws SQLException {
        if (type == null) return search(term);

        String like = "%" + term.toLowerCase() + "%";
        String sql  = """
            SELECT * FROM tracks
            WHERE type = ?
              AND (lower(title) LIKE ? OR lower(artist) LIKE ? OR lower(album) LIKE ?)
            ORDER BY title COLLATE NOCASE
            """;
        try (PooledConnection pc = Database.pool().borrow();
             PreparedStatement ps = pc.get().prepareStatement(sql)) {
            ps.setString(1, type.name());
            ps.setString(2, like);
            ps.setString(3, like);
            ps.setString(4, like);
            try (ResultSet rs = ps.executeQuery()) {
                return collectTracks(rs);
            }
        }
    }

    /** Retorna uma faixa pelo id. */
    public Optional<TrackEntity> findById(String id) throws SQLException {
        String sql = "SELECT * FROM tracks WHERE id = ?";
        try (PooledConnection pc = Database.pool().borrow();
             PreparedStatement ps = pc.get().prepareStatement(sql)) {
            ps.setString(1, id);
            try (ResultSet rs = ps.executeQuery()) {
                if (rs.next()) return Optional.of(map(rs));
            }
        }
        return Optional.empty();
    }

    /** Retorna uma faixa pelo path absoluto do arquivo. */
    public Optional<TrackEntity> findByPath(String path) throws SQLException {
        String sql = "SELECT * FROM tracks WHERE path = ?";
        try (PooledConnection pc = Database.pool().borrow();
             PreparedStatement ps = pc.get().prepareStatement(sql)) {
            ps.setString(1, path);
            try (ResultSet rs = ps.executeQuery()) {
                if (rs.next()) return Optional.of(map(rs));
            }
        }
        return Optional.empty();
    }

    /** Retorna faixas associadas a uma categoria (via track_categories). */
    public List<TrackEntity> findByCategoryId(String categoryId) throws SQLException {
        String sql = """
            SELECT t.* FROM tracks t
            JOIN track_categories tc ON tc.track_id = t.id
            WHERE tc.category_id = ?
            ORDER BY t.title COLLATE NOCASE
            """;
        try (PooledConnection pc = Database.pool().borrow();
             PreparedStatement ps = pc.get().prepareStatement(sql)) {
            ps.setString(1, categoryId);
            try (ResultSet rs = ps.executeQuery()) {
                return collectTracks(rs);
            }
        }
    }

    /** Conta o total de faixas no catálogo. */
    public int count() throws SQLException {
        try (PooledConnection pc = Database.pool().borrow();
             PreparedStatement ps = pc.get().prepareStatement("SELECT COUNT(*) FROM tracks");
             ResultSet rs = ps.executeQuery()) {
            return rs.next() ? rs.getInt(1) : 0;
        }
    }

    /** Conta faixas por tipo. */
    public int countByType(TrackType type) throws SQLException {
        try (PooledConnection pc = Database.pool().borrow();
             PreparedStatement ps = pc.get().prepareStatement("SELECT COUNT(*) FROM tracks WHERE type = ?")) {
            ps.setString(1, type.name());
            try (ResultSet rs = ps.executeQuery()) {
                return rs.next() ? rs.getInt(1) : 0;
            }
        }
    }

    /** Retorna página de faixas (LIMIT / OFFSET). */
    public List<TrackEntity> findPage(int limit, int offset) throws SQLException {
        String sql = "SELECT * FROM tracks ORDER BY title COLLATE NOCASE LIMIT ? OFFSET ?";
        try (PooledConnection pc = Database.pool().borrow();
             PreparedStatement ps = pc.get().prepareStatement(sql)) {
            ps.setInt(1, limit);
            ps.setInt(2, offset);
            try (ResultSet rs = ps.executeQuery()) {
                return collectTracks(rs);
            }
        }
    }

    // ── Mapeamento ResultSet → TrackEntity ────────────────────────────────────

    private List<TrackEntity> collectTracks(ResultSet rs) throws SQLException {
        List<TrackEntity> list = new ArrayList<>();
        while (rs.next()) list.add(map(rs));
        return list;
    }

    private TrackEntity map(ResultSet rs) throws SQLException {
        return new TrackEntity(
            rs.getString("id"),
            rs.getString("path"),
            rs.getString("title"),
            rs.getString("artist"),
            rs.getString("album"),
            TrackType.of(rs.getString("type")),
            rs.getInt("duration_ms"),
            rs.getString("category"),
            nullableString(rs, "isrc"),
            nullableString(rs, "composer"),
            nullableString(rs, "publisher"),
            nullableDouble(rs, "loudness_lufs"),
            nullableDouble(rs, "true_peak_dbtp"),
            LoudnessStatus.of(rs.getString("loudness_status")),
            rs.getString("loudness_error"),
            parseDateTime(rs.getString("loudness_analyzed_at")),
            nullableInt(rs, "cue_in_ms"),
            nullableInt(rs, "intro_ms"),
            nullableInt(rs, "outro_ms"),
            nullableInt(rs, "cue_out_ms"),
            parseDateTime(rs.getString("indexed_at"))
        );
    }

    // ── Helpers de tipo nullable ──────────────────────────────────────────────

    private static String nullableString(ResultSet rs, String col) throws SQLException {
        String v = rs.getString(col);
        return rs.wasNull() ? null : v;
    }

    private static Double nullableDouble(ResultSet rs, String col) throws SQLException {
        double v = rs.getDouble(col);
        return rs.wasNull() ? null : v;
    }

    private static Integer nullableInt(ResultSet rs, String col) throws SQLException {
        int v = rs.getInt(col);
        return rs.wasNull() ? null : v;
    }

    private static LocalDateTime parseDateTime(String value) {
        if (value == null || value.isBlank()) return null;
        try { return LocalDateTime.parse(value.replace(" ", "T")); }
        catch (Exception e) { return null; }
    }
}
