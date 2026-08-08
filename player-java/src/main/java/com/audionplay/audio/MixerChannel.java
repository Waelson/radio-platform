package com.audionplay.audio;

import java.util.concurrent.LinkedBlockingQueue;
import java.util.concurrent.TimeUnit;

/**
 * Canal de entrada do SoftwareMixer com backpressure.
 *
 * Problema resolvido: sem backpressure, o FFmpeg despejava todos os dados de uma
 * música em milissegundos (muito mais rápido que o tempo real). A PCM thread
 * terminava, disparava onEndOfTrack prematuramente e o item sumia da fila.
 *
 * Solução: LinkedBlockingQueue com capacidade limitada. offer() bloqueia quando
 * a fila está cheia, forçando o FFmpeg a produzir no ritmo do mixer (~44100Hz).
 *
 * Thread model:
 *   - offer()  → chamado pela PCM thread do FfmpegAudioPlayer (bloqueia quando cheia)
 *   - drain()  → chamado pela mix thread do SoftwareMixer (nunca bloqueia)
 *   - clear()  → chamado pela thread do player ao stop(); libera offer() bloqueado
 */
public class MixerChannel {

    private static final int BYTES_PER_FRAME = 4;  // s16le stereo
    private static final int MAX_CHUNKS      = 64; // ~64 × 8192 bytes ≈ 3s de buffer

    private final LinkedBlockingQueue<byte[]> queue = new LinkedBlockingQueue<>(MAX_CHUNKS);

    // Chunk sendo lido parcialmente pelo mixer (acesso somente da mix thread)
    private volatile byte[] currentChunk = null;
    private int chunkOffset = 0;

    private long framesOffered   = 0;
    private long framesConsumed  = 0;
    private final Object statsLock = new Object();

    private volatile float   volume = 1.0f;
    private volatile boolean active = false;
    private volatile boolean paused = false;

    // ── Escrita com backpressure (PCM thread) ────────────────────────────────

    /**
     * Enfileira dados PCM. Bloqueia quando a fila está cheia (backpressure natural).
     * Sai sem bloquear se o canal for desativado via clear().
     */
    public void offer(byte[] data, int len) {
        if (!active) return;
        byte[] copy = new byte[len];
        System.arraycopy(data, 0, copy, 0, len);
        try {
            // Tenta enfileirar com timeout; re-verifica active a cada 50ms
            while (active && !Thread.currentThread().isInterrupted()) {
                if (queue.offer(copy, 50, TimeUnit.MILLISECONDS)) {
                    synchronized (statsLock) { framesOffered += len / BYTES_PER_FRAME; }
                    break;
                }
            }
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
        }
    }

    // ── Leitura sem bloqueio (mix thread) ────────────────────────────────────

    /**
     * Drena até {@code len} bytes para {@code dst[offset..]}. Preenche com zeros
     * se não houver dados. Nunca bloqueia.
     *
     * @return bytes efetivamente lidos (pode ser < len).
     */
    public int drain(byte[] dst, int offset, int len) {
        // Quando pausado: retorna 0 (silêncio) sem consumir o buffer.
        // Isso garante que o áudio para imediatamente e retoma do ponto certo.
        if (paused) return 0;
        int written = 0;
        while (written < len) {
            // Obtém o chunk em progresso (ou próximo da fila)
            byte[] chunk = currentChunk;
            if (chunk == null) {
                chunk = queue.poll(); // não bloqueia
                currentChunk = chunk;
                chunkOffset  = 0;
                if (chunk == null) break; // sem dados — resto já está em zeros
            }
            int available = chunk.length - chunkOffset;
            if (available <= 0) {
                currentChunk = null;
                continue;
            }
            int toCopy = Math.min(available, len - written);
            System.arraycopy(chunk, chunkOffset, dst, offset + written, toCopy);
            written     += toCopy;
            chunkOffset += toCopy;
            if (chunkOffset >= chunk.length) {
                currentChunk = null;
                chunkOffset  = 0;
            }
        }
        if (written > 0) {
            synchronized (statsLock) { framesConsumed += written / BYTES_PER_FRAME; }
        }
        return written;
    }

    // ── Controle ─────────────────────────────────────────────────────────────

    /**
     * Limpa o buffer e desativa o canal.
     * Libera qualquer PCM thread bloqueada no offer() via active=false.
     */
    public void clear() {
        active = false; // libera offer() bloqueado antes de limpar
        paused = false;
        queue.clear();
        currentChunk = null;
        chunkOffset  = 0;
        synchronized (statsLock) { framesOffered = 0; framesConsumed = 0; }
    }

    public long getFramesOffered()   { synchronized (statsLock) { return framesOffered;  } }
    public long getFramesConsumed()  { synchronized (statsLock) { return framesConsumed; } }

    /** True quando não há mais dados para o mixer consumir. */
    public boolean isEmpty() {
        return queue.isEmpty() && currentChunk == null;
    }

    public void    setActive(boolean a) { active = a; }
    public boolean isActive()           { return active; }
    public void    setPaused(boolean p) { paused = p; }
    public void    setVolume(float v)   { volume = Math.max(0f, Math.min(1f, v)); }
    public float   getVolume()          { return volume; }
}
