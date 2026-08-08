package com.audionplay.db.entity;

import java.time.LocalDateTime;
import java.util.List;

/**
 * Representa um perfil de hot keys (tabelas {@code hotkey_profiles} + {@code hotkey_buttons}).
 *
 * {@code buttons} é preenchido apenas via {@code HotkeyRepository.findProfileById()}.
 */
public record HotkeyProfileEntity(
    String             id,
    String             name,
    int                columns,
    LocalDateTime      createdAt,
    LocalDateTime      updatedAt,
    List<HotkeyButton> buttons
) {

    public record HotkeyButton(
        String id,
        String profileId,
        int    position,
        String label,
        String subLabel,
        String icon,
        int    palette,
        String trackId,      // nullable
        String trackPath,
        String trackTitle,
        String trackArtist,
        String trackType,
        int    durationMs,
        LocalDateTime createdAt
    ) {}

    public HotkeyProfileEntity withoutButtons() {
        return new HotkeyProfileEntity(id, name, columns, createdAt, updatedAt, List.of());
    }
}
