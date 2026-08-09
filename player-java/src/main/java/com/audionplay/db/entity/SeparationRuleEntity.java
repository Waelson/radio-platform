package com.audionplay.db.entity;

/** Representa uma regra de separação (tabela {@code separation_rules}). */
public record SeparationRuleEntity(
    String id,
    String field,         // artist | title | album | category
    int    minSepMinutes
) {}
