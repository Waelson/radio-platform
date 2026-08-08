package com.audionplay.db.entity;

import java.time.LocalDateTime;
import java.util.List;

/**
 * Representa uma playlist com seus itens (tabelas {@code playlists} + {@code playlist_items}).
 *
 * {@code items} é preenchido apenas quando carregado via
 * {@code PlaylistRepository.findById()} — em listagens usa {@link #withoutItems()}.
 */
public record PlaylistEntity(
    String             id,
    String             name,
    String             category,   // nullable
    LocalDateTime      createdAt,
    LocalDateTime      updatedAt,
    List<PlaylistItem> items       // vazio quando não carregado
) {

    public record PlaylistItem(
        String id,
        String trackId,
        int    position
    ) {}

    /** Retorna instância sem items (para uso em listagens). */
    public PlaylistEntity withoutItems() {
        return new PlaylistEntity(id, name, category, createdAt, updatedAt, List.of());
    }
}
