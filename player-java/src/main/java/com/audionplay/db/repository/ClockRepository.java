package com.audionplay.db.repository;

import com.audionplay.db.Database;
import com.audionplay.db.PooledConnection;
import com.audionplay.db.entity.ClockEntity;
import com.audionplay.db.entity.ClockScheduleEntry;
import com.audionplay.db.entity.ClockSlotEntity;

import java.sql.*;
import java.util.ArrayList;
import java.util.List;
import java.util.UUID;

public class ClockRepository {

    // ── Clocks ────────────────────────────────────────────────────────────────

    public List<ClockEntity> findAll() throws SQLException {
        String sql = """
                SELECT c.id, c.name,
                       COUNT(s.id)                  AS slot_count,
                       COALESCE(SUM(s.duration_hint_ms), 0) AS total_hint_ms
                FROM clocks c
                LEFT JOIN clock_slots s ON s.clock_id = c.id
                GROUP BY c.id, c.name
                ORDER BY c.name COLLATE NOCASE
                """;
        try (PooledConnection pc = Database.pool().borrow();
             Statement st = pc.get().createStatement();
             ResultSet rs = st.executeQuery(sql)) {
            List<ClockEntity> list = new ArrayList<>();
            while (rs.next()) {
                list.add(new ClockEntity(
                    rs.getString("id"),
                    rs.getString("name"),
                    rs.getInt("slot_count"),
                    rs.getLong("total_hint_ms")
                ));
            }
            return list;
        }
    }

    public void create(String name) throws SQLException {
        try (PooledConnection pc = Database.pool().borrow();
             PreparedStatement ps = pc.get().prepareStatement(
                "INSERT INTO clocks (id, name) VALUES (?, ?)")) {
            ps.setString(1, UUID.randomUUID().toString());
            ps.setString(2, name);
            ps.executeUpdate();
        }
    }

    public void delete(String id) throws SQLException {
        try (PooledConnection pc = Database.pool().borrow();
             PreparedStatement ps = pc.get().prepareStatement(
                "DELETE FROM clocks WHERE id = ?")) {
            ps.setString(1, id);
            ps.executeUpdate();
        }
    }

    // ── Slots ─────────────────────────────────────────────────────────────────

    public List<ClockSlotEntity> findSlots(String clockId) throws SQLException {
        String sql = """
                SELECT s.id, s.clock_id, s.position, s.slot_type,
                       s.category_id, cat.name AS category_name, s.duration_hint_ms
                FROM clock_slots s
                LEFT JOIN categories cat ON cat.id = s.category_id
                WHERE s.clock_id = ?
                ORDER BY s.position
                """;
        try (PooledConnection pc = Database.pool().borrow();
             PreparedStatement ps = pc.get().prepareStatement(sql)) {
            ps.setString(1, clockId);
            try (ResultSet rs = ps.executeQuery()) {
                List<ClockSlotEntity> list = new ArrayList<>();
                while (rs.next()) {
                    list.add(new ClockSlotEntity(
                        rs.getString("id"),
                        rs.getString("clock_id"),
                        rs.getInt("position"),
                        rs.getString("slot_type"),
                        rs.getString("category_id"),
                        rs.getString("category_name"),
                        rs.getLong("duration_hint_ms")
                    ));
                }
                return list;
            }
        }
    }

    public void addSlot(String clockId, String slotType, String categoryId, long hintMs)
            throws SQLException {
        int nextPos;
        try (PooledConnection pc = Database.pool().borrow();
             PreparedStatement ps = pc.get().prepareStatement(
                "SELECT COALESCE(MAX(position), 0) + 1 FROM clock_slots WHERE clock_id = ?")) {
            ps.setString(1, clockId);
            try (ResultSet rs = ps.executeQuery()) {
                nextPos = rs.next() ? rs.getInt(1) : 1;
            }
        }
        try (PooledConnection pc = Database.pool().borrow();
             PreparedStatement ps = pc.get().prepareStatement(
                "INSERT INTO clock_slots (id, clock_id, position, slot_type, category_id, duration_hint_ms) "
              + "VALUES (?, ?, ?, ?, ?, ?)")) {
            ps.setString(1, UUID.randomUUID().toString());
            ps.setString(2, clockId);
            ps.setInt(3, nextPos);
            ps.setString(4, slotType);
            ps.setString(5, (categoryId == null || categoryId.isBlank()) ? null : categoryId);
            ps.setLong(6, hintMs);
            ps.executeUpdate();
        }
    }

    public void deleteSlot(String slotId) throws SQLException {
        try (PooledConnection pc = Database.pool().borrow();
             PreparedStatement ps = pc.get().prepareStatement(
                "DELETE FROM clock_slots WHERE id = ?")) {
            ps.setString(1, slotId);
            ps.executeUpdate();
        }
    }

    public void updateSlotHint(String slotId, long hintMs) throws SQLException {
        try (PooledConnection pc = Database.pool().borrow();
             PreparedStatement ps = pc.get().prepareStatement(
                "UPDATE clock_slots SET duration_hint_ms = ? WHERE id = ?")) {
            ps.setLong(1, hintMs);
            ps.setString(2, slotId);
            ps.executeUpdate();
        }
    }

    /**
     * Reordena os slots de um clock conforme a lista de IDs fornecida.
     * Usa posições negativas temporárias para evitar conflito com o UNIQUE(clock_id, position).
     */
    public void reorderSlots(String clockId, List<String> orderedIds) throws SQLException {
        try (PooledConnection pc = Database.pool().borrow()) {
            Connection conn = pc.get();
            conn.setAutoCommit(false);
            try {
                // Passo 1: posições negativas temporárias
                try (PreparedStatement ps = conn.prepareStatement(
                    "UPDATE clock_slots SET position = ? WHERE id = ? AND clock_id = ?")) {
                    for (int i = 0; i < orderedIds.size(); i++) {
                        ps.setInt(1, -(i + 1));
                        ps.setString(2, orderedIds.get(i));
                        ps.setString(3, clockId);
                        ps.addBatch();
                    }
                    ps.executeBatch();
                }
                // Passo 2: posições finais
                try (PreparedStatement ps = conn.prepareStatement(
                    "UPDATE clock_slots SET position = ? WHERE id = ? AND clock_id = ?")) {
                    for (int i = 0; i < orderedIds.size(); i++) {
                        ps.setInt(1, i + 1);
                        ps.setString(2, orderedIds.get(i));
                        ps.setString(3, clockId);
                        ps.addBatch();
                    }
                    ps.executeBatch();
                }
                conn.commit();
            } catch (SQLException e) {
                conn.rollback();
                throw e;
            } finally {
                conn.setAutoCommit(true);
            }
        }
    }

    // ── Grade 24×7 ────────────────────────────────────────────────────────────

    public List<ClockScheduleEntry> getGrid() throws SQLException {
        String sql = """
                SELECT cg.weekday, cg.hour, cg.clock_id, c.name AS clock_name
                FROM clock_schedule cg
                LEFT JOIN clocks c ON c.id = cg.clock_id
                WHERE cg.clock_id IS NOT NULL AND cg.clock_id != ''
                """;
        try (PooledConnection pc = Database.pool().borrow();
             Statement st = pc.get().createStatement();
             ResultSet rs = st.executeQuery(sql)) {
            List<ClockScheduleEntry> list = new ArrayList<>();
            while (rs.next()) {
                list.add(new ClockScheduleEntry(
                    rs.getInt("weekday"),
                    rs.getInt("hour"),
                    rs.getString("clock_id"),
                    rs.getString("clock_name")
                ));
            }
            return list;
        }
    }

    public void setGridCell(int weekday, int hour, String clockId) throws SQLException {
        if (clockId == null || clockId.isBlank()) {
            try (PooledConnection pc = Database.pool().borrow();
                 PreparedStatement ps = pc.get().prepareStatement(
                    "DELETE FROM clock_schedule WHERE weekday = ? AND hour = ?")) {
                ps.setInt(1, weekday);
                ps.setInt(2, hour);
                ps.executeUpdate();
            }
        } else {
            try (PooledConnection pc = Database.pool().borrow();
                 PreparedStatement ps = pc.get().prepareStatement(
                    "INSERT OR REPLACE INTO clock_schedule (weekday, hour, clock_id) VALUES (?, ?, ?)")) {
                ps.setInt(1, weekday);
                ps.setInt(2, hour);
                ps.setString(3, clockId);
                ps.executeUpdate();
            }
        }
    }

    public void clearDay(int weekday) throws SQLException {
        try (PooledConnection pc = Database.pool().borrow();
             PreparedStatement ps = pc.get().prepareStatement(
                "DELETE FROM clock_schedule WHERE weekday = ?")) {
            ps.setInt(1, weekday);
            ps.executeUpdate();
        }
    }
}
