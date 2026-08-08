package com.audionplay.db.repository;

import com.audionplay.db.Database;
import com.audionplay.db.PooledConnection;
import com.audionplay.db.entity.PlaylistEntity;
import com.audionplay.db.entity.PlaylistEntity.PlaylistItem;

import java.sql.*;
import java.time.LocalDateTime;
import java.util.ArrayList;
import java.util.List;
import java.util.Optional;

/**
 * Acesso às tabelas {@code playlists} e {@code playlist_items} via JDBC puro.
 */
public class PlaylistRepository {

    /** Retorna todas as playlists sem carregar os itens. */
    public List<PlaylistEntity> findAll() throws SQLException {
        String sql = "SELECT * FROM playlists ORDER BY name COLLATE NOCASE";
        try (PooledConnection pc = Database.pool().borrow();
             PreparedStatement ps = pc.get().prepareStatement(sql);
             ResultSet rs = ps.executeQuery()) {
            List<PlaylistEntity> list = new ArrayList<>();
            while (rs.next()) list.add(mapWithoutItems(rs));
            return list;
        }
    }

    /** Retorna uma playlist com todos os seus itens ordenados por posição. */
    public Optional<PlaylistEntity> findById(String id) throws SQLException {
        String sqlP = "SELECT * FROM playlists WHERE id = ?";
        String sqlI = "SELECT * FROM playlist_items WHERE playlist_id = ? ORDER BY position";

        try (PooledConnection pc = Database.pool().borrow()) {
            Connection conn = pc.get();

            PlaylistEntity playlist;
            try (PreparedStatement ps = conn.prepareStatement(sqlP)) {
                ps.setString(1, id);
                try (ResultSet rs = ps.executeQuery()) {
                    if (!rs.next()) return Optional.empty();
                    playlist = mapWithoutItems(rs);
                }
            }

            List<PlaylistItem> items = new ArrayList<>();
            try (PreparedStatement ps = conn.prepareStatement(sqlI)) {
                ps.setString(1, id);
                try (ResultSet rs = ps.executeQuery()) {
                    while (rs.next()) {
                        items.add(new PlaylistItem(
                            rs.getString("id"),
                            rs.getString("track_id"),
                            rs.getInt("position")
                        ));
                    }
                }
            }

            return Optional.of(new PlaylistEntity(
                playlist.id(), playlist.name(), playlist.category(),
                playlist.createdAt(), playlist.updatedAt(), items
            ));
        }
    }

    private PlaylistEntity mapWithoutItems(ResultSet rs) throws SQLException {
        return new PlaylistEntity(
            rs.getString("id"),
            rs.getString("name"),
            rs.getString("category"),
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
