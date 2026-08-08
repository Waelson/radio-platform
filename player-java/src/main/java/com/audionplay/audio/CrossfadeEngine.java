package com.audionplay.audio;

/**
 * Contrato para transição suave entre dois players.
 *
 * O crossfade faz fade-out no player atual e fade-in no próximo
 * simultaneamente, durante um período configurável.
 */
public interface CrossfadeEngine {

    /**
     * Inicia o crossfade.
     *
     * @param from             player que está tocando agora (será fadado para 0)
     * @param to               player que deve começar (será fadado para volume alvo)
     * @param durationSeconds  duração total do crossfade
     * @param targetVolume     volume final do player 'to' (normalmente 1.0)
     * @param onComplete       chamado quando o crossfade termina
     */
    void start(AudioPlayer from, AudioPlayer to,
               double durationSeconds, float targetVolume,
               Runnable onComplete);

    /** Cancela o crossfade em andamento imediatamente. */
    void cancel();

    boolean isRunning();
}
