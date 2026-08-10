package com.audionplay.audio.ffmpeg;

import com.audionplay.audio.AudioPlayer;
import com.audionplay.audio.MixerChannel;
import com.audionplay.audio.SoftwareMixer;
import com.audionplay.domain.PlaybackState;
import com.audionplay.domain.Track;
import javafx.application.Platform;

import javax.sound.sampled.*;
import java.io.InputStream;
import java.util.List;
import java.util.function.Consumer;

/**
 * Implementação de AudioPlayer usando FFmpeg como decoder.
 *
 * Dois modos de operação:
 *   - Standalone (channel=null): abre seu próprio SourceDataLine e envia PCM direto ao HW.
 *   - Mixer (channel != null): escreve PCM em um MixerChannel; o SoftwareMixer consolida
 *     todos os canais num único SourceDataLine, permitindo mix, controle de volume por canal
 *     e leitura unificada de VU meter.
 *
 * Thread model:
 *   - play() inicia uma thread daemon de leitura PCM.
 *   - Pause/resume controlados por wait/notify no pauseLock.
 *   - Callbacks de UI sempre despachados via Platform.runLater.
 */
public class FfmpegAudioPlayer implements AudioPlayer {

    private static final int SAMPLE_RATE  = 44100;
    private static final int CHANNELS     = 2;
    private static final int SAMPLE_BITS  = 16;
    private static final int BUFFER_BYTES = 8192;

    private final AudioFormat audioFormat =
        new AudioFormat(SAMPLE_RATE, SAMPLE_BITS, CHANNELS, true, false);

    // Modo mixer: canal no SoftwareMixer (null = modo standalone)
    private final MixerChannel channel;

    // Estado mutável
    private volatile PlaybackState state = PlaybackState.STOPPED;
    private volatile float         volume = 1.0f;
    private volatile double        seekOffset = 0.0;

    private Track          currentTrack;
    private volatile double cueOutSeconds = -1; // -1 = sem limite
    private Process        ffmpegProcess;
    private SourceDataLine dataLine;      // apenas no modo standalone
    private Thread         playbackThread;

    private final Object pauseLock = new Object();

    private Runnable          onEndOfTrack;
    private Consumer<Double>  onTimeUpdate;
    private Consumer<float[]> onLevelUpdate; // usado apenas no modo standalone

    private long lastLevelCallMs = 0;

    // ── Construtores ──────────────────────────────────────────────────────────

    /** Modo standalone: cada instância abre seu próprio SourceDataLine. */
    public FfmpegAudioPlayer() {
        this.channel = null;
    }

    /** Modo mixer: PCM é gravado no canal informado; o SoftwareMixer faz a saída. */
    public FfmpegAudioPlayer(MixerChannel channel) {
        this.channel = channel;
    }

    // ── Interface AudioPlayer ─────────────────────────────────────────────────

    @Override
    public synchronized void load(Track track) {
        stop();
        this.currentTrack  = track;
        this.seekOffset    = track.effectiveStart();
        this.cueOutSeconds = track.cueOutSeconds() != null ? track.cueOutSeconds() : -1;
    }

    @Override
    public synchronized void play() {
        if (currentTrack == null) return;

        if (state == PlaybackState.PAUSED) {
            state = PlaybackState.PLAYING;
            if (channel != null) channel.setPaused(false);
            else if (dataLine != null) dataLine.start();
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
        if (channel != null) channel.setPaused(true);
        else if (dataLine != null) dataLine.stop();
    }

    @Override
    public synchronized void stop() {
        if (state == PlaybackState.STOPPED) return;
        state = PlaybackState.STOPPED;
        synchronized (pauseLock) { pauseLock.notifyAll(); }
        if (channel != null) channel.clear(); // active=false libera offer() bloqueado
        killFfmpeg();
        // Interrompe a PCM thread caso ainda esteja bloqueada no offer()
        if (playbackThread != null) playbackThread.interrupt();
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
        if (channel != null) channel.setVolume(this.volume);
    }

    @Override public float         getVolume() { return volume; }
    @Override public PlaybackState getState()  { return state;  }

    @Override
    public double getCurrentTime() {
        if (channel != null)   return seekOffset + (double) channel.getFramesConsumed() / SAMPLE_RATE;
        if (dataLine == null)  return seekOffset;
        return seekOffset + (double) dataLine.getLongFramePosition() / SAMPLE_RATE;
    }

    @Override
    public double getDuration() {
        return currentTrack != null ? currentTrack.durationSeconds() : 0.0;
    }

    @Override public void setOnEndOfTrack(Runnable r)           { this.onEndOfTrack  = r; }
    @Override public void setOnTimeUpdate(Consumer<Double> c)   { this.onTimeUpdate  = c; }
    @Override public void setOnLevelUpdate(Consumer<float[]> c) { this.onLevelUpdate = c; }

    @Override
    public void dispose() { stop(); }

    // ── Implementação interna ─────────────────────────────────────────────────

    private void startPlayback(double fromSeconds) {
        // Aguarda thread anterior terminar (evita race condition de escritas sobrepostas)
        if (playbackThread != null && playbackThread.isAlive()) {
            try { playbackThread.join(300); } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            }
        }

        try {
            if (channel != null) {
                channel.clear();
                channel.setActive(true);
            } else {
                dataLine = AudioSystem.getSourceDataLine(audioFormat);
                dataLine.open(audioFormat, BUFFER_BYTES * 4);
                dataLine.start();
                System.out.println("[Player] SourceDataLine aberta: " + dataLine.getBufferSize() + " bytes");
            }

            double limitSec = (cueOutSeconds > 0) ? Math.max(0, cueOutSeconds - fromSeconds) : 0;
            List<String> cmd = buildCommand(currentTrack.filePath(), fromSeconds, limitSec);
            ffmpegProcess = new ProcessBuilder(cmd)
                .redirectError(ProcessBuilder.Redirect.DISCARD)
                .start();
            System.out.println("[Player] FFmpeg PID: " + ffmpegProcess.pid()
                + (channel != null ? " [mixer ch]" : " [standalone]"));

            state = PlaybackState.PLAYING;

            playbackThread = new Thread(() -> readPcmLoop(fromSeconds), "ffmpeg-pcm");
            playbackThread.setDaemon(true);
            playbackThread.start();

        } catch (Exception e) {
            state = PlaybackState.STOPPED;
            System.err.println("[Player] Erro ao iniciar: " + e.getMessage());
        }
    }

    private void readPcmLoop(double startSeconds) {
        seekOffset = startSeconds;
        try (InputStream pcm = ffmpegProcess.getInputStream()) {
            byte[] buf = new byte[BUFFER_BYTES];
            int read;
            while ((read = pcm.read(buf)) != -1) {
                // Aguarda se pausado
                synchronized (pauseLock) {
                    while (state == PlaybackState.PAUSED) pauseLock.wait();
                }
                if (state == PlaybackState.STOPPED) break;

                if (channel != null) {
                    // Modo mixer: envia raw ao canal; o mixer aplica volume e mistura
                    channel.offer(buf, read);
                } else {
                    // Modo standalone: aplica volume e envia direto ao hardware
                    byte[] out = applyVolume(buf, read, volume);
                    dataLine.write(out, 0, read);
                    // VU meter (~100ms)
                    long now = System.currentTimeMillis();
                    Consumer<float[]> luCb = onLevelUpdate;
                    if (luCb != null && now - lastLevelCallMs >= 100) {
                        lastLevelCallMs = now;
                        float[] levels = computeRms(out, read);
                        Platform.runLater(() -> luCb.accept(levels));
                    }
                }

                // Posição — captura o callback para evitar race condition com o crossfade
                double t = getCurrentTime();
                Consumer<Double> tuCb = onTimeUpdate;
                if (tuCb != null) Platform.runLater(() -> tuCb.accept(t));
            }

            // Fim natural do stream
            if (state != PlaybackState.STOPPED) {
                if (channel == null && dataLine != null) {
                    dataLine.drain();
                } else if (channel != null) {
                    // Aguarda o mixer consumir todo o buffer antes de sinalizar fim.
                    // Continua disparando onTimeUpdate para a UI refletir o progresso real.
                    while (!channel.isEmpty() && state != PlaybackState.STOPPED) {
                        double t = getCurrentTime();
                        if (onTimeUpdate != null) Platform.runLater(() -> onTimeUpdate.accept(t));
                        try { Thread.sleep(20); } catch (InterruptedException e) {
                            Thread.currentThread().interrupt();
                            break;
                        }
                    }
                    channel.setActive(false);
                }
                state = PlaybackState.STOPPED;
                // Garante que o progresso chegue a 100% antes de sinalizar o fim
                if (onTimeUpdate != null) {
                    double dur = getDuration();
                    Platform.runLater(() -> onTimeUpdate.accept(dur));
                }
                if (onEndOfTrack != null) Platform.runLater(onEndOfTrack);
            }

        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
        } catch (Exception e) {
            // Silencia erros esperados de stream fechado ao parar o player
            if (state != PlaybackState.STOPPED) {
                System.err.println("[Player] Erro PCM: " + e.getMessage());
            }
        } finally {
            if (channel == null) cleanupLine();
        }
    }

    private byte[] applyVolume(byte[] buf, int len, float vol) {
        if (vol == 1.0f) return buf;
        byte[] out = new byte[len];
        for (int i = 0; i < len - 1; i += 2) {
            short s = (short) ((buf[i + 1] << 8) | (buf[i] & 0xFF));
            s = (short) (s * vol);
            out[i]     = (byte)  (s & 0xFF);
            out[i + 1] = (byte) ((s >> 8) & 0xFF);
        }
        return out;
    }

    private void killFfmpeg() {
        if (ffmpegProcess != null) {
            ffmpegProcess.destroyForcibly();
            ffmpegProcess = null;
        }
        if (channel != null) {
            channel.setActive(false);
        } else {
            cleanupLine();
        }
    }

    private void cleanupLine() {
        if (dataLine != null && dataLine.isOpen()) {
            dataLine.stop();
            dataLine.close();
        }
        dataLine = null;
    }

    private static float[] computeRms(byte[] buf, int len) {
        double sumL = 0, sumR = 0;
        int frames = len / 4;
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

    private static List<String> buildCommand(String filePath, double fromSeconds, double limitSeconds) {
        java.util.List<String> cmd = new java.util.ArrayList<>();
        cmd.add(FfmpegLocator.ffmpeg());
        cmd.add("-ss"); cmd.add(String.format(java.util.Locale.US, "%.3f", fromSeconds));
        cmd.add("-i");  cmd.add(filePath);
        if (limitSeconds > 0) {
            cmd.add("-t"); cmd.add(String.format(java.util.Locale.US, "%.3f", limitSeconds));
        }
        cmd.add("-f");       cmd.add("s16le");
        cmd.add("-ar");      cmd.add(String.valueOf(SAMPLE_RATE));
        cmd.add("-ac");      cmd.add(String.valueOf(CHANNELS));
        cmd.add("-loglevel"); cmd.add("quiet");
        cmd.add("pipe:1");
        return cmd;
    }
}
