package com.audionplay.db.repository;

import com.audionplay.db.Database;
import com.audionplay.db.PooledConnection;
import com.audionplay.db.entity.CategoryEntity;

import java.sql.*;
import java.time.LocalDateTime;
import java.util.ArrayList;
import java.util.List;
import java.util.Optional;

/**
 * Acesso à tabela {@code categories} via JDBC puro.
 */
public class CategoryRepository {

    public List<CategoryEntity> findAll() throws SQLException {
        String sql = "SELECT * FROM categories ORDER BY name COLLATE NOCASE";
        try (PooledConnection pc = Database.pool().borrow();
             PreparedStatement ps = pc.get().prepareStatement(sql);
             ResultSet rs = ps.executeQuery()) {
            List<CategoryEntity> list = new ArrayList<>();
            while (rs.next()) list.add(map(rs));
            return list;
        }
    }

    public Optional<CategoryEntity> findById(String id) throws SQLException {
        String sql = "SELECT * FROM categories WHERE id = ?";
        try (PooledConnection pc = Database.pool().borrow();
             PreparedStatement ps = pc.get().prepareStatement(sql)) {
            ps.setString(1, id);
            try (ResultSet rs = ps.executeQuery()) {
                if (rs.next()) return Optional.of(map(rs));
            }
        }
        return Optional.empty();
    }

    public Optional<CategoryEntity> findByNameKey(String nameKey) throws SQLException {
        String sql = "SELECT * FROM categories WHERE name_key = ?";
        try (PooledConnection pc = Database.pool().borrow();
             PreparedStatement ps = pc.get().prepareStatement(sql)) {
            ps.setString(1, nameKey);
            try (ResultSet rs = ps.executeQuery()) {
                if (rs.next()) return Optional.of(map(rs));
            }
        }
        return Optional.empty();
    }

    private CategoryEntity map(ResultSet rs) throws SQLException {
        return new CategoryEntity(
            rs.getString("id"),
            rs.getString("name"),
            rs.getString("description"),
            rs.getString("color"),
            rs.getString("name_key"),
            parseDateTime(rs.getString("created_at"))
        );
    }

    private static LocalDateTime parseDateTime(String value) {
        if (value == null || value.isBlank()) return null;
        try { return LocalDateTime.parse(value.replace(" ", "T")); }
        catch (Exception e) { return null; }
    }
}
