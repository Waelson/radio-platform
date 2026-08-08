package com.audionplay.db.repository;

import com.audionplay.db.Database;
import com.audionplay.db.PooledConnection;
import com.audionplay.db.entity.HotkeyProfileEntity;
import com.audionplay.db.entity.HotkeyProfileEntity.HotkeyButton;

import java.sql.*;
import java.time.LocalDateTime;
import java.util.ArrayList;
import java.util.List;
import java.util.Optional;

/**
 * Acesso às tabelas {@code hotkey_profiles} e {@code hotkey_buttons} via JDBC puro.
 */
public class HotkeyRepository {

    /** Retorna todos os perfis sem carregar os botões. */
    public List<HotkeyProfileEntity> findAllProfiles() throws SQLException {
        String sql = "SELECT * FROM hotkey_profiles ORDER BY name COLLATE NOCASE";
        try (PooledConnection pc = Database.pool().borrow();
             PreparedStatement ps = pc.get().prepareStatement(sql);
             ResultSet rs = ps.executeQuery()) {
            List<HotkeyProfileEntity> list = new ArrayList<>();
            while (rs.next()) list.add(mapProfileWithoutButtons(rs));
            return list;
        }
    }

    /** Retorna um perfil com todos os seus botões ordenados por posição. */
    public Optional<HotkeyProfileEntity> findProfileById(String id) throws SQLException {
        String sqlP = "SELECT * FROM hotkey_profiles WHERE id = ?";
        String sqlB = "SELECT * FROM hotkey_buttons WHERE profile_id = ? ORDER BY position";

        try (PooledConnection pc = Database.pool().borrow()) {
            Connection conn = pc.get();

            HotkeyProfileEntity profile;
            try (PreparedStatement ps = conn.prepareStatement(sqlP)) {
                ps.setString(1, id);
                try (ResultSet rs = ps.executeQuery()) {
                    if (!rs.next()) return Optional.empty();
                    profile = mapProfileWithoutButtons(rs);
                }
            }

            List<HotkeyButton> buttons = new ArrayList<>();
            try (PreparedStatement ps = conn.prepareStatement(sqlB)) {
                ps.setString(1, id);
                try (ResultSet rs = ps.executeQuery()) {
                    while (rs.next()) buttons.add(mapButton(rs));
                }
            }

            return Optional.of(new HotkeyProfileEntity(
                profile.id(), profile.name(), profile.columns(),
                profile.createdAt(), profile.updatedAt(),
                buttons
            ));
        }
    }

    /** Retorna os botões de um perfil diretamente por profile_id. */
    public List<HotkeyButton> findButtonsByProfile(String profileId) throws SQLException {
        String sql = "SELECT * FROM hotkey_buttons WHERE profile_id = ? ORDER BY position";
        try (PooledConnection pc = Database.pool().borrow();
             PreparedStatement ps = pc.get().prepareStatement(sql)) {
            ps.setString(1, profileId);
            List<HotkeyButton> list = new ArrayList<>();
            try (ResultSet rs = ps.executeQuery()) {
                while (rs.next()) list.add(mapButton(rs));
            }
            return list;
        }
    }

    // ── Mapeamento ────────────────────────────────────────────────────────────

    private HotkeyProfileEntity mapProfileWithoutButtons(ResultSet rs) throws SQLException {
        return new HotkeyProfileEntity(
            rs.getString("id"),
            rs.getString("name"),
            rs.getInt("columns"),
            parseDateTime(rs.getString("created_at")),
            parseDateTime(rs.getString("updated_at")),
            List.of()
        );
    }

    private HotkeyButton mapButton(ResultSet rs) throws SQLException {
        return new HotkeyButton(
            rs.getString("id"),
            rs.getString("profile_id"),
            rs.getInt("position"),
            rs.getString("label"),
            rs.getString("sub_label"),
            rs.getString("icon"),
            rs.getInt("palette"),
            rs.getString("track_id"),
            rs.getString("track_path"),
            rs.getString("track_title"),
            rs.getString("track_artist"),
            rs.getString("track_type"),
            rs.getInt("duration_ms"),
            parseDateTime(rs.getString("created_at"))
        );
    }

    private static LocalDateTime parseDateTime(String value) {
        if (value == null || value.isBlank()) return null;
        try { return LocalDateTime.parse(value.replace(" ", "T")); }
        catch (Exception e) { return null; }
    }
}
