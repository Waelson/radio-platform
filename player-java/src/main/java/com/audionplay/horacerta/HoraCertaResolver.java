package com.audionplay.horacerta;

import com.audionplay.audio.ffmpeg.FfmpegLocator;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.LocalTime;
import java.util.ArrayList;
import java.util.List;

/**
 * Resolve a hora atual em uma sequência de caminhos de arquivos de áudio,
 * compatível com o horacerta.Resolver do playout Go.
 *
 * Convenção de nomes (configurável):
 *   Hora:    HRS{HH}.mp3  (ex.: HRS14.mp3)
 *   Minuto:  MIN{MM}.mp3  (ex.: MIN35.mp3)
 *
 * Às XX:00, se MIN00 não existir, toca apenas o arquivo de hora (sem erro),
 * exatamente como no playout Go.
 */
public class HoraCertaResolver {

    private final HoraCertaConfig cfg;

    public HoraCertaResolver(HoraCertaConfig cfg) {
        this.cfg = cfg;
    }

    /**
     * Retorna os caminhos absolutos dos arquivos a tocar para o tempo {@code t}.
     *
     * @throws IOException se um arquivo obrigatório não for encontrado
     */
    public List<String> resolve(LocalTime t) throws IOException {
        if (cfg.hoursDir() == null || cfg.hoursDir().isBlank()) {
            throw new IOException("Hora certa: pasta de horas não configurada");
        }

        String hh = String.format("%02d", t.getHour());
        String mm = String.format("%02d", t.getMinute());

        String hourPath = buildPath(cfg.hoursDir(), cfg.hourPattern(), "{HH}", hh);
        if (!Files.exists(Path.of(hourPath))) {
            throw new IOException("Hora certa: arquivo de hora não encontrado: " + hourPath);
        }

        List<String> paths = new ArrayList<>();
        paths.add(hourPath);

        String minDir = cfg.minutesDir() != null && !cfg.minutesDir().isBlank()
            ? cfg.minutesDir() : cfg.hoursDir();
        String minPath = buildPath(minDir, cfg.minutePattern(), "{MM}", mm);

        if (Files.exists(Path.of(minPath))) {
            paths.add(minPath);
        } else if (t.getMinute() != 0) {
            // Minuto != 0 e arquivo ausente → erro (mesmo comportamento do Go)
            throw new IOException("Hora certa: arquivo de minuto não encontrado: " + minPath);
        }
        // XX:00 sem MIN00 → toca apenas hora (sem erro), igual ao Go

        return paths;
    }

    /**
     * Retorna a duração total aproximada (ms) de uma lista de arquivos usando ffprobe.
     * Retorna 0 em caso de erro.
     */
    public int probeTotalDurationMs(List<String> paths) {
        long totalMs = 0;
        for (String path : paths) {
            try {
                Process p = new ProcessBuilder(
                    FfmpegLocator.ffprobe(), "-v", "quiet",
                    "-show_entries", "format=duration",
                    "-of", "csv=p=0", path
                ).start();
                String out = new String(p.getInputStream().readAllBytes()).trim();
                p.waitFor();
                if (!out.isEmpty()) {
                    totalMs += (long) (Double.parseDouble(out.replace(',', '.')) * 1000);
                }
            } catch (Exception ignored) {}
        }
        return (int) Math.min(totalMs, Integer.MAX_VALUE);
    }

    private static String buildPath(String dir, String pattern, String key, String value) {
        return Path.of(dir, pattern.replace(key, value)).toString();
    }
}
