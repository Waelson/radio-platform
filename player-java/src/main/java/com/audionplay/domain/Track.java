package com.audionplay.domain;

/**
 * Representa uma faixa de áudio carregada no player.
 * Imutável — todas as propriedades são definidas na criação.
 */
public record Track(
    String filePath,
    String title,
    String artist,
    double durationSeconds,
    Double cueInSeconds,    // null = início do arquivo
    Double cueOutSeconds,   // null = fim do arquivo
    Double outroSeconds     // null = sem crossfade automático
) {
    /** Construtor sem CUE/outro points. */
    public Track(String filePath, String title, String artist, double durationSeconds) {
        this(filePath, title, artist, durationSeconds, null, null, null);
    }

    /** Construtor com CUE IN/OUT mas sem outro. */
    public Track(String filePath, String title, String artist, double durationSeconds,
                 Double cueInSeconds, Double cueOutSeconds) {
        this(filePath, title, artist, durationSeconds, cueInSeconds, cueOutSeconds, null);
    }

    public static Track unknown(String filePath) {
        String name = filePath.substring(filePath.lastIndexOf('/') + 1);
        return new Track(filePath, name, "Desconhecido", 0.0);
    }

    /** Ponto de início efetivo em segundos. */
    public double effectiveStart() {
        return cueInSeconds != null ? cueInSeconds : 0.0;
    }

    /** Duração efetiva da janela CUE IN→CUE OUT para barra de progresso. */
    public double effectiveDuration() {
        double end = cueOutSeconds != null ? cueOutSeconds : durationSeconds;
        return Math.max(0, end - effectiveStart());
    }
}
