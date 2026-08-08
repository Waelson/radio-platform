package com.audionplay.db.entity;

import java.time.LocalDateTime;
import java.util.List;

/**
 * Representa um bloco/break (tabelas {@code breaks} + {@code break_items}).
 *
 * {@code items} é preenchido apenas via {@code BreakRepository.findById()}.
 */
public record BreakEntity(
    String        id,
    String        name,
    String        openTrackId,   // nullable
    String        closeTrackId,  // nullable
    LocalDateTime createdAt,
    LocalDateTime updatedAt,
    List<BreakItem> items
) {

    public record BreakItem(
        String id,
        String trackId,
        int    position
    ) {}

    public BreakEntity withoutItems() {
        return new BreakEntity(id, name, openTrackId, closeTrackId, createdAt, updatedAt, List.of());
    }
}
