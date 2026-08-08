package com.audionplay.db.entity;

import java.time.LocalDateTime;

/**
 * Representa uma categoria (tabela {@code categories}).
 */
public record CategoryEntity(
    String        id,
    String        name,
    String        description,
    String        color,       // hex, ex: "#FF5733"
    String        nameKey,     // chave normalizada (slug)
    LocalDateTime createdAt
) {}
