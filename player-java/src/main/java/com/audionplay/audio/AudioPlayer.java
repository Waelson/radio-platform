package com.audionplay.audio;

import com.audionplay.domain.PlaybackState;
import com.audionplay.domain.Track;
import java.util.function.Consumer;

/**
 * Contrato de um player de áudio.
 *
 * Implementações são responsáveis por:
 *  - Decodificar o arquivo (via FFmpeg, MediaPlayer, etc.)
 *  - Controlar play/pause/stop/seek
 *  - Aplicar volume em tempo real
 *  - Notificar a UI via callbacks (sempre na FX thread)
 */
public interface AudioPlayer {

    /** Carrega uma faixa. Não inicia reprodução. */
    void load(Track track);

    /** Inicia ou retoma a reprodução. */
    void play();

    /** Pausa a reprodução sem perder a posição. */
    void pause();

    /** Para completamente e libera recursos. */
    void stop();

    /**
     * Vai para a posição indicada em segundos.
     * Pode ser chamado durante reprodução, pausa ou parado.
     */
    void seek(double seconds);

    /** Volume de 0.0 (mudo) a 1.0 (máximo). Aplicado em tempo real. */
    void setVolume(float volume);

    float getVolume();

    /** Posição atual em segundos. */
    double getCurrentTime();

    /** Duração total em segundos. 0 se não carregado. */
    double getDuration();

    PlaybackState getState();

    /** Chamado na FX thread quando a faixa termina naturalmente. */
    void setOnEndOfTrack(Runnable callback);

    /** Chamado na FX thread a cada atualização de posição (~100ms). */
    void setOnTimeUpdate(Consumer<Double> callback);

    /**
     * Chamado na FX thread a cada buffer de áudio processado.
     * float[0] = RMS do canal esquerdo  (0.0 – 1.0)
     * float[1] = RMS do canal direito   (0.0 – 1.0)
     */
    void setOnLevelUpdate(Consumer<float[]> callback);

    /** Libera todos os recursos (processos, threads, audio lines). */
    void dispose();
}
