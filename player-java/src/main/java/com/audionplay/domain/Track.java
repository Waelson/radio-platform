package com.audionplay.domain;

/**
 * Representa uma faixa de áudio carregada no player.
 * Imutável — todas as propriedades são definidas na criação.
 */
public record Track(
    String filePath,
    String title,
    String artist,
    double durationSeconds
) {
    public static Track unknown(String filePath) {
        String name = filePath.substring(filePath.lastIndexOf('/') + 1);
        return new Track(filePath, name, "Desconhecido", 0.0);
    }
}
