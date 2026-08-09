package com.audionplay.db.entity;

/** Representa um slot dentro de um clock (tabela {@code clock_slots}). */
public record ClockSlotEntity(
    String id,
    String clockId,
    int    position,
    String slotType,      // CATEGORY | JINGLE | SPOT | VINHETA | HORA_CERTA | FIXED
    String categoryId,    // nullable
    String categoryName,  // nullable — resolvido via JOIN
    long   durationHintMs
) {}
