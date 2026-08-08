package com.audionplay.db.repository;

import com.audionplay.db.Database;
import com.audionplay.db.PooledConnection;
import com.audionplay.db.entity.BreakEntity;
import com.audionplay.db.entity.BreakEntity.BreakItem;

import java.sql.*;
import java.time.LocalDateTime;
import java.util.ArrayList;
import java.util.List;
import java.util.Optional;

/**
 * Acesso às tabelas {@code breaks} e {@code break_items} via JDBC puro.
 */
public class BreakRepository {

    /** Retorna todos os breaks sem carregar os itens. */
    public List<BreakEntity> findAll() throws SQLException {
        String sql = "SELECT * FROM breaks ORDER BY name COLLATE NOCASE";
        try (PooledConnection pc = Database.pool().borrow();
             PreparedStatement ps = pc.get().prepareStatement(sql);
             ResultSet rs = ps.executeQuery()) {
            List<BreakEntity> list = new ArrayList<>();
            while (rs.next()) list.add(mapWithoutItems(rs));
            return list;
        }
    }

    /** Retorna um break com todos os seus itens ordenados por posição. */
    public Optional<BreakEntity> findById(String id) throws SQLException {
        String sqlB = "SELECT * FROM breaks WHERE id = ?";
        String sqlI = "SELECT * FROM break_items WHERE break_id = ? ORDER BY position";

        try (PooledConnection pc = Database.pool().borrow()) {
            Connection conn = pc.get();

            BreakEntity breakEntity;
            try (PreparedStatement ps = conn.prepareStatement(sqlB)) {
                ps.setString(1, id);
                try (ResultSet rs = ps.executeQuery()) {
                    if (!rs.next()) return Optional.empty();
                    breakEntity = mapWithoutItems(rs);
                }
            }

            List<BreakItem> items = new ArrayList<>();
            try (PreparedStatement ps = conn.prepareStatement(sqlI)) {
                ps.setString(1, id);
                try (ResultSet rs = ps.executeQuery()) {
                    while (rs.next()) {
                        items.add(new BreakItem(
                            rs.getString("id"),
                            rs.getString("track_id"),
                            rs.getInt("position")
                        ));
                    }
                }
            }

            return Optional.of(new BreakEntity(
                breakEntity.id(), breakEntity.name(),
                breakEntity.openTrackId(), breakEntity.closeTrackId(),
                breakEntity.createdAt(), breakEntity.updatedAt(),
                items
            ));
        }
    }

    private BreakEntity mapWithoutItems(ResultSet rs) throws SQLException {
        return new BreakEntity(
            rs.getString("id"),
            rs.getString("name"),
            rs.getString("open_track_id"),
            rs.getString("close_track_id"),
            parseDateTime(rs.getString("created_at")),
            parseDateTime(rs.getString("updated_at")),
            List.of()
        );
    }

    private static LocalDateTime parseDateTime(String value) {
        if (value == null || value.isBlank()) return null;
        try { return LocalDateTime.parse(value.replace(" ", "T")); }
        catch (Exception e) { return null; }
    }
}
