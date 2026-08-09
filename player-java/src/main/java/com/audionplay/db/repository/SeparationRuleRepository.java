package com.audionplay.db.repository;

import com.audionplay.db.Database;
import com.audionplay.db.PooledConnection;
import com.audionplay.db.entity.SeparationRuleEntity;

import java.sql.*;
import java.util.ArrayList;
import java.util.List;
import java.util.UUID;

public class SeparationRuleRepository {

    public List<SeparationRuleEntity> findAll() throws SQLException {
        String sql = "SELECT id, field, min_sep_minutes FROM separation_rules ORDER BY field";
        try (PooledConnection pc = Database.pool().borrow();
             Statement st = pc.get().createStatement();
             ResultSet rs = st.executeQuery(sql)) {
            List<SeparationRuleEntity> list = new ArrayList<>();
            while (rs.next()) {
                list.add(new SeparationRuleEntity(
                    rs.getString("id"),
                    rs.getString("field"),
                    rs.getInt("min_sep_minutes")
                ));
            }
            return list;
        }
    }

    public void create(String field, int minSepMinutes) throws SQLException {
        try (PooledConnection pc = Database.pool().borrow();
             PreparedStatement ps = pc.get().prepareStatement(
                "INSERT INTO separation_rules (id, field, min_sep_minutes) VALUES (?, ?, ?)")) {
            ps.setString(1, UUID.randomUUID().toString());
            ps.setString(2, field);
            ps.setInt(3, minSepMinutes);
            ps.executeUpdate();
        }
    }

    public void update(String id, int minSepMinutes) throws SQLException {
        try (PooledConnection pc = Database.pool().borrow();
             PreparedStatement ps = pc.get().prepareStatement(
                "UPDATE separation_rules SET min_sep_minutes = ? WHERE id = ?")) {
            ps.setInt(1, minSepMinutes);
            ps.setString(2, id);
            ps.executeUpdate();
        }
    }

    public void delete(String id) throws SQLException {
        try (PooledConnection pc = Database.pool().borrow();
             PreparedStatement ps = pc.get().prepareStatement(
                "DELETE FROM separation_rules WHERE id = ?")) {
            ps.setString(1, id);
            ps.executeUpdate();
        }
    }
}
