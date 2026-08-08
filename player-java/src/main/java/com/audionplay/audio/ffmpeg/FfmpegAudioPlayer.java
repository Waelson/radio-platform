package com.audionplay.audio.ffmpeg;

import com.audionplay.audio.AudioPlayer;
import com.audionplay.domain.PlaybackState;
import com.audionplay.domain.Track;
import javafx.application.Platform;

import javax.sound.sampled.*;
import java.io.InputStream;
import java.util.List;
import java.util.concurrent.atomic.AtomicReference;
import java.util.function.Consumer;

/**
 * Implementação de AudioPlayer usando FFmpeg como decoder.
 *
 * Fluxo:
 *   arquivo → FFmpeg → PCM s16le 44100 Hz stereo → applyVolume → SourceDataLine → placa de som
 *
 * Thread model:
 *   - play() inicia uma thread de leitura (daemon)
 *   - pause/resume controlados por wait/notify
 *   - todos os callbacks de UI são despachados via Platform.runLater
 */
public class FfmpegAudioPlayer implements AudioPlayer {

    private static final int SAMPLE_RATE  = 44100;
    private static final int CHANNELS     = 2;
    private static final int SAMPLE_BITS  = 16;
    private static final int BUFFER_BYTES = 8192;

    private final AudioFormat audioFormat =
        new AudioFormat(SAMPLE_RATE, SAMPLE_BITS, CHANNELS, true, false);

    // Estado mutável — acesso somente via métodos sincronizados ou volatile
    private volatile PlaybackState state = PlaybackState.STOPPED;
    private volatile float volume        = 1.0f;
    private volatile double seekOffset   = 0.0; // segundos passados ao FFmpeg via -ss

    private Track currentTrack;
    private Process ffmpegProcess;
    private SourceDataLine dataLine;
    private Thread playbackThread;

    // Pause: a thread de leitura espera neste monitor
    private final Object pauseLock = new Object();

    private Runnable onEndOfTrack;
    private Consumer<Double> onTimeUpdate;
    private Consumer<float[]> onLevelUpdate;

    // Controla frequência dos callbacks de nível (~100ms)
    private long lastLevelCallMs = 0;

    // ── Interface pública ─────────────────────────────────────────────────────

    @Override
    public synchronized void load(Track track) {
        stop();
        this.currentTrack = track;
        this.seekOffset   = 0.0;
    }

    @Override
    public synchronized void play() {
        if (currentTrack == null) return;

        if (state == PlaybackState.PAUSED) {
            // Retoma sem reiniciar FFmpeg
            state = PlaybackState.PLAYING;
            dataLine.start();
            synchronized (pauseLock) { pauseLock.notifyAll(); }
            return;
        }

        if (state == PlaybackState.PLAYING) return;

        startPlayback(seekOffset);
    }

    @Override
    public synchronized void pause() {
        if (state != PlaybackState.PLAYING) return;
        state = PlaybackState.PAUSED;
        dataLine.stop(); // para o hardware mas mantém o buffer
    }

    @Override
    public synchronized void stop() {
        if (state == PlaybackState.STOPPED) return;
        state = PlaybackState.STOPPED;
        synchronized (pauseLock) { pauseLock.notifyAll(); } // desbloqueia thread pausada
        killFfmpeg();
    }

    @Override
    public synchronized void seek(double seconds) {
        seekOffset = Math.max(0, seconds);
        boolean wasPlaying = state == PlaybackState.PLAYING;
        stop();
        if (wasPlaying) startPlayback(seekOffset);
    }

    @Override
    public void setVolume(float v) {
        this.volume = Math.max(0f, Math.min(1f, v));
    }

    @Override public float getVolume()        { return volume; }
    @Override public PlaybackState getState() { return state; }

    @Override
    public double getCurrentTime() {
        if (dataLine == null) return seekOffset;
        return seekOffset + (double) dataLine.getLongFramePosition() / SAMPLE_RATE;
    }

    @Override
    public double getDuration() {
        return currentTrack != null ? currentTrack.durationSeconds() : 0.0;
    }

    @Override public void setOnEndOfTrack(Runnable r)          { this.onEndOfTrack   = r; }
    @Override public void setOnTimeUpdate(Consumer<Double> c)  { this.onTimeUpdate   = c; }
    @Override public void setOnLevelUpdate(Consumer<float[]> c){ this.onLevelUpdate  = c; }

    @Override
    public void dispose() {
        stop();
    }

    // ── Implementação interna ─────────────────────────────────────────────────

    private void startPlayback(double fromSeconds) {
        try {
            System.out.println("[Player] Abrindo SourceDataLine...");
            dataLine = AudioSystem.getSourceDataLine(audioFormat);
            dataLine.open(audioFormat, BUFFER_BYTES * 4);
            dataLine.start();
            System.out.println("[Player] SourceDataLine aberta: " + dataLine.getBufferSize() + " bytes");

            List<String> cmd = buildCommand(currentTrack.filePath(), fromSeconds);
            System.out.println("[Player] Iniciando FFmpeg: " + String.join(" ", cmd));
            ffmpegProcess = new ProcessBuilder(cmd)
                .redirectError(ProcessBuilder.Redirect.DISCARD)
                .start();
            System.out.println("[Player] FFmpeg PID: " + ffmpegProcess.pid());

            state = PlaybackState.PLAYING;

            playbackThread = new Thread(() -> readPcmLoop(fromSeconds), "ffmpeg-pcm-reader");
            playbackThread.setDaemon(true);
            playbackThread.start();

        } catch (Exception e) {
            state = PlaybackState.STOPPED;
            System.err.println("[Player] Erro ao iniciar: " + e.getMessage());
            e.printStackTrace();
        }
    }

    private void readPcmLoop(double startSeconds) {
        seekOffset = startSeconds;
        System.out.println("[Player] Thread PCM iniciada.");

        try (InputStream pcm = ffmpegProcess.getInputStream()) {
            byte[] buf = new byte[BUFFER_BYTES];
            int read;
            long totalBytes = 0;

            while ((read = pcm.read(buf)) != -1) {
                if (totalBytes == 0) System.out.println("[Player] Primeiro chunk PCM: " + read + " bytes");
                totalBytes += read;
                // Aguarda se pausado
                synchronized (pauseLock) {
                    while (state == PlaybackState.PAUSED) pauseLock.wait();
                }
                if (state == PlaybackState.STOPPED) break;

                // Aplica volume e envia para a placa de som
                byte[] out = applyVolume(buf, read, volume);
                dataLine.write(out, 0, read);

                // Notifica nível de áudio (~100ms de intervalo)
                long now = System.currentTimeMillis();
                if (onLevelUpdate != null && now - lastLevelCallMs >= 100) {
                    lastLevelCallMs = now;
                    float[] levels = computeRms(out, read);
                    Platform.runLater(() -> onLevelUpdate.accept(levels));
                }

                // Notifica posição
                double t = getCurrentTime();
                if (onTimeUpdate != null) Platform.runLater(() -> onTimeUpdate.accept(t));
            }

            System.out.println("[Player] Fim do stream PCM. Total: " + totalBytes + " bytes");
            if (state != PlaybackState.STOPPED) {
                dataLine.drain();
                state = PlaybackState.STOPPED;
                if (onEndOfTrack != null) Platform.runLater(onEndOfTrack);
            }

        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
        } catch (Exception e) {
            System.err.println("[Player] Erro na thread PCM: " + e.getMessage());
            e.printStackTrace();
        } finally {
            cleanupLine();
        }
    }

    /**
     * Escala os samples PCM 16-bit pelo volume informado.
     * Formato: s16le (little-endian, signed 16-bit).
     */
    private byte[] applyVolume(byte[] buf, int len, float vol) {
        if (vol == 1.0f) return buf; // sem cópia desnecessária
        byte[] out = new byte[len];
        for (int i = 0; i < len - 1; i += 2) {
            short sample = (short) ((buf[i + 1] << 8) | (buf[i] & 0xFF));
            sample = (short) (sample * vol);
            out[i]     = (byte)  (sample & 0xFF);
            out[i + 1] = (byte) ((sample >> 8) & 0xFF);
        }
        return out;
    }

    private void killFfmpeg() {
        if (ffmpegProcess != null) {
            ffmpegProcess.destroyForcibly();
            ffmpegProcess = null;
        }
        cleanupLine();
    }

    private void cleanupLine() {
        if (dataLine != null && dataLine.isOpen()) {
            dataLine.stop();
            dataLine.close();
        }
    }

    /**
     * Calcula RMS separado por canal (L e R) para um buffer PCM s16le stereo.
     * Retorna float[]{rmsL, rmsR} normalizados em 0.0–1.0.
     */
    private static float[] computeRms(byte[] buf, int len) {
        double sumL = 0, sumR = 0;
        int frames = len / 4; // 4 bytes por frame (2 bytes * 2 canais)
        for (int i = 0; i < frames * 4; i += 4) {
            short l = (short) ((buf[i + 1] << 8) | (buf[i]     & 0xFF));
            short r = (short) ((buf[i + 3] << 8) | (buf[i + 2] & 0xFF));
            sumL += (double) l * l;
            sumR += (double) r * r;
        }
        float rmsL = frames > 0 ? (float) (Math.sqrt(sumL / frames) / 32768.0) : 0f;
        float rmsR = frames > 0 ? (float) (Math.sqrt(sumR / frames) / 32768.0) : 0f;
        return new float[]{rmsL, rmsR};
    }

    private static List<String> buildCommand(String filePath, double fromSeconds) {
        return List.of(
            FfmpegLocator.ffmpeg(),
            "-ss",    String.format(java.util.Locale.US, "%.3f", fromSeconds),
            "-i",     filePath,
            "-f",     "s16le",
            "-ar",    String.valueOf(SAMPLE_RATE),
            "-ac",    String.valueOf(CHANNELS),
            "-loglevel", "quiet",
            "pipe:1"
        );
    }
}
