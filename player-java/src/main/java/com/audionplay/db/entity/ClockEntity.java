package com.audionplay.db.entity;

/** Representa um clock de rotação (tabela {@code clocks}). */
public record ClockEntity(
    String id,
    String name,
    int    slotCount,
    long   totalHintMs
) {}
