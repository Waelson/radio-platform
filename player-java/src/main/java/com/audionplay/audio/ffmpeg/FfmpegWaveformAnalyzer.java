package com.audionplay.audio.ffmpeg;

import com.audionplay.audio.WaveformAnalyzer;
import javafx.application.Platform;

import java.io.InputStream;
import java.util.List;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.function.Consumer;

/**
 * Analisa waveform decodificando o arquivo via FFmpeg em baixa resolução.
 *
 * Usa 8000 Hz mono para análise — suficiente para forma de onda visual
 * e muito mais rápido do que 44100 Hz stereo.
 *
 * Para uma faixa de 6 minutos:
 *   8000 Hz × 2 bytes × 360s = ~5.5 MB — aceitável em memória.
 *
 * A análise roda numa thread de background; o resultado é entregue
 * via onReady na FX thread.
 */
public class FfmpegWaveformAnalyzer implements WaveformAnalyzer {

    private static final int ANALYSIS_RATE = 8000; // Hz — suficiente para visual

    private final ExecutorService executor =
        Executors.newSingleThreadExecutor(r -> {
            Thread t = new Thread(r, "waveform-analyzer");
            t.setDaemon(true);
            return t;
        });

    @Override
    public void analyze(String filePath, int numBars,
                        Consumer<double[]> onReady, Consumer<String> onError) {
        executor.submit(() -> {
            try {
                double[] peaks = doAnalyze(filePath, numBars);
                Platform.runLater(() -> onReady.accept(peaks));
            } catch (Exception e) {
                String msg = "Falha na análise de waveform: " + e.getMessage();
                Platform.runLater(() -> onError.accept(msg));
            }
        });
    }

    private double[] doAnalyze(String filePath, int numBars) throws Exception {
        // stderr descartado via redirect — evita deadlock de pipe
        Process ffmpeg = new ProcessBuilder(buildCommand(filePath))
            .redirectError(ProcessBuilder.Redirect.DISCARD)
            .start();

        byte[] allBytes;
        try (InputStream pcm = ffmpeg.getInputStream()) {
            allBytes = pcm.readAllBytes();
        }
        ffmpeg.waitFor();

        if (allBytes.length == 0) {
            throw new IllegalStateException("FFmpeg não produziu dados de áudio.");
        }

        return extractPeaks(allBytes, numBars);
    }

    /**
     * Divide os samples em janelas iguais e calcula o valor RMS de cada janela.
     * Retorna array normalizado [0.0, 1.0].
     */
    private double[] extractPeaks(byte[] pcm, int numBars) {
        // Cada sample = 2 bytes (s16le mono)
        int totalSamples  = pcm.length / 2;
        int samplesPerBar = Math.max(1, totalSamples / numBars);
        double[] peaks    = new double[numBars];
        double maxPeak    = 0.0;

        for (int bar = 0; bar < numBars; bar++) {
            int start = bar * samplesPerBar;
            int end   = Math.min(start + samplesPerBar, totalSamples);
            double sumSq = 0;

            for (int i = start; i < end; i++) {
                int byteIdx = i * 2;
                if (byteIdx + 1 >= pcm.length) break;
                short s = (short) ((pcm[byteIdx + 1] << 8) | (pcm[byteIdx] & 0xFF));
                sumSq += (double) s * s;
            }

            double rms = Math.sqrt(sumSq / (end - start));
            peaks[bar] = rms;
            if (rms > maxPeak) maxPeak = rms;
        }

        // Normaliza para [0.0, 1.0]
        if (maxPeak > 0) {
            for (int i = 0; i < peaks.length; i++) {
                peaks[i] = peaks[i] / maxPeak;
            }
        }

        return peaks;
    }

    private static List<String> buildCommand(String filePath) {
        java.util.List<String> cmd = new java.util.ArrayList<>();
        cmd.add(FfmpegLocator.ffmpeg());

        String[] parts = filePath.split("\\|");
        if (parts.length > 1) {
            // Multi-arquivo (ex.: hora-certa): usa filtro concat para encadear
            for (String part : parts) { cmd.add("-i"); cmd.add(part); }
            StringBuilder filter = new StringBuilder();
            for (int i = 0; i < parts.length; i++) filter.append("[").append(i).append(":a]");
            filter.append("concat=n=").append(parts.length).append(":v=0:a=1[outa]");
            cmd.add("-filter_complex"); cmd.add(filter.toString());
            cmd.add("-map"); cmd.add("[outa]");
        } else {
            cmd.add("-i"); cmd.add(filePath);
        }

        cmd.add("-f");        cmd.add("s16le");
        cmd.add("-ar");       cmd.add(String.valueOf(ANALYSIS_RATE));
        cmd.add("-ac");       cmd.add("1");
        cmd.add("-loglevel"); cmd.add("quiet");
        cmd.add("pipe:1");
        return cmd;
    }
}
