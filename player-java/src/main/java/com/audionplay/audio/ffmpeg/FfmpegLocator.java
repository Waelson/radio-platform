package com.audionplay.audio.ffmpeg;

import java.io.File;
import java.util.List;

/**
 * Detecta o caminho absoluto do ffmpeg/ffprobe no sistema.
 *
 * Procura nas localizações mais comuns em macOS, Linux e Windows.
 * Se não encontrar, retorna o nome simples e torce para que esteja no PATH.
 */
public class FfmpegLocator {

    private static final List<String> SEARCH_PATHS = List.of(
        "/opt/homebrew/bin",   // macOS Apple Silicon (Homebrew)
        "/usr/local/bin",      // macOS Intel (Homebrew)
        "/usr/bin",            // Linux
        "/usr/local/sbin",     // Linux alternativo
        "C:\\ffmpeg\\bin"      // Windows
    );

    private static String ffmpegPath;
    private static String ffprobePath;

    public static String ffmpeg() {
        if (ffmpegPath == null) ffmpegPath = find("ffmpeg");
        return ffmpegPath;
    }

    public static String ffprobe() {
        if (ffprobePath == null) ffprobePath = find("ffprobe");
        return ffprobePath;
    }

    private static String find(String binary) {
        // Tenta o nome simples primeiro (caso já esteja no PATH do processo)
        for (String dir : SEARCH_PATHS) {
            File f = new File(dir, binary);
            if (f.exists() && f.canExecute()) {
                System.out.println("[FfmpegLocator] Encontrado: " + f.getAbsolutePath());
                return f.getAbsolutePath();
            }
        }
        System.err.println("[FfmpegLocator] '" + binary + "' não encontrado. Usando nome simples.");
        return binary;
    }
}
