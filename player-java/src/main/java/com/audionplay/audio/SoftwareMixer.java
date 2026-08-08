package com.audionplay.audio;

import javafx.application.Platform;

import javax.sound.sampled.*;
import java.util.Arrays;
import java.util.function.Consumer;

/**
 * Mixer de software: combina N canais PCM s16le em um único SourceDataLine.
 *
 * Arquitetura:
 *   FfmpegAudioPlayer (program)   → MixerChannel[0] \
 *   FfmpegAudioPlayer (secondary) → MixerChannel[1]  → mixLoop → SourceDataLine → HW
 *   FfmpegAudioPlayer (cart)      → MixerChannel[2] /
 *
 * Thread model:
 *   - Cada player escreve no seu canal via offer() na própria PCM thread.
 *   - Uma única mixThread lê de todos os canais ativos, soma (int para headroom),
 *     limita a [-32768, 32767] e escreve no SourceDataLine.
 *   - O VU meter recebe o RMS do sinal já misturado (~100ms).
 *   - dataLine.write() bloqueia naturalmente, ditando o ritmo de 44100 frames/s.
 */
public class SoftwareMixer {

    public static final int SAMPLE_RATE      = 44100;
    public static final int CHANNELS         = 2;
    public static final int SAMPLE_BITS      = 16;
    public static final int BYTES_PER_FRAME  = CHANNELS * (SAMPLE_BITS / 8); // 4
    public static final int FRAMES_PER_CHUNK = 1024;                          // ~23ms
    public static final int BYTES_PER_CHUNK  = FRAMES_PER_CHUNK * BYTES_PER_FRAME; // 4096

    private final MixerChannel[] channels;
    private SourceDataLine       dataLine;
    private Thread               mixThread;
    private volatile boolean     running = false;

    private Consumer<float[]> onLevelUpdate;
    private long lastLevelMs = 0;

    public SoftwareMixer(int numChannels) {
        channels = new MixerChannel[numChannels];
        for (int i = 0; i < numChannels; i++) channels[i] = new MixerChannel();
    }

    // ── API pública ───────────────────────────────────────────────────────────

    public MixerChannel getChannel(int idx) { return channels[idx]; }

    public void setOnLevelUpdate(Consumer<float[]> cb) { onLevelUpdate = cb; }

    public void start() {
        if (running) return;
        try {
            AudioFormat fmt = new AudioFormat(SAMPLE_RATE, SAMPLE_BITS, CHANNELS, true, false);
            DataLine.Info info = new DataLine.Info(SourceDataLine.class, fmt);
            dataLine = (SourceDataLine) AudioSystem.getLine(info);
            dataLine.open(fmt, BYTES_PER_CHUNK * 8);
            dataLine.start();
        } catch (LineUnavailableException e) {
            System.err.println("[Mixer] Erro ao abrir SourceDataLine: " + e.getMessage());
            return;
        }
        running   = true;
        mixThread = new Thread(this::mixLoop, "software-mixer");
        mixThread.setDaemon(true);
        mixThread.start();
        System.out.println("[Mixer] Iniciado (" + channels.length + " canais, "
            + FRAMES_PER_CHUNK + " frames/chunk)");
    }

    public void stop() {
        running = false;
        if (mixThread != null) { mixThread.interrupt(); mixThread = null; }
        if (dataLine != null && dataLine.isOpen()) {
            dataLine.drain();
            dataLine.stop();
            dataLine.close();
            dataLine = null;
        }
    }

    // ── Mix loop ─────────────────────────────────────────────────────────────

    private void mixLoop() {
        // Buffers reutilizados a cada chunk para evitar GC
        byte[] chBuf  = new byte[BYTES_PER_CHUNK];
        int[]  mix    = new int[FRAMES_PER_CHUNK * CHANNELS]; // int = headroom para soma
        byte[] outBuf = new byte[BYTES_PER_CHUNK];

        while (running) {
            try {
                Arrays.fill(mix, 0);

                // Soma todos os canais ativos
                for (MixerChannel ch : channels) {
                    if (!ch.isActive()) continue;
                    Arrays.fill(chBuf, (byte) 0);
                    ch.drain(chBuf, 0, BYTES_PER_CHUNK);
                    float vol = ch.getVolume();
                    int samples = FRAMES_PER_CHUNK * CHANNELS;
                    for (int i = 0; i < samples; i++) {
                        int b = i * 2;
                        short s = (short) ((chBuf[b + 1] << 8) | (chBuf[b] & 0xFF));
                        mix[i] += (int) (s * vol);
                    }
                }

                // Limita, converte para bytes e computa RMS do mix
                double sumL = 0, sumR = 0;
                int samples = FRAMES_PER_CHUNK * CHANNELS;
                for (int i = 0; i < samples; i++) {
                    short s = (short) Math.max(-32768, Math.min(32767, mix[i]));
                    int b = i * 2;
                    outBuf[b]     = (byte)  (s & 0xFF);
                    outBuf[b + 1] = (byte) ((s >> 8) & 0xFF);
                    if (i % 2 == 0) sumL += (double) s * s;
                    else             sumR += (double) s * s;
                }

                dataLine.write(outBuf, 0, BYTES_PER_CHUNK);

                // Notifica VU meter (~100ms)
                long now = System.currentTimeMillis();
                if (onLevelUpdate != null && now - lastLevelMs >= 100) {
                    lastLevelMs = now;
                    float rmsL = (float) (Math.sqrt(sumL / FRAMES_PER_CHUNK) / 32768.0);
                    float rmsR = (float) (Math.sqrt(sumR / FRAMES_PER_CHUNK) / 32768.0);
                    float[] levels = {rmsL, rmsR};
                    Platform.runLater(() -> onLevelUpdate.accept(levels));
                }

            } catch (Exception e) {
                System.err.println("[Mixer] Erro no mix loop (recuperando): " + e.getMessage());
                // Limpa todos os canais para evitar dados corrompidos
                for (MixerChannel ch : channels) ch.clear();
            }
        }
    }
}
