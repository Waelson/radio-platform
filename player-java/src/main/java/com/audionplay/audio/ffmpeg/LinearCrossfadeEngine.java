package com.audionplay.audio.ffmpeg;

import com.audionplay.audio.AudioPlayer;
import com.audionplay.audio.CrossfadeEngine;
import com.audionplay.domain.PlaybackState;
import javafx.animation.KeyFrame;
import javafx.animation.Timeline;
import javafx.util.Duration;

/**
 * Crossfade linear entre dois AudioPlayers.
 *
 * Estratégia:
 *  - Um Timeline JavaFX tick a cada 50ms atualiza os volumes de ambos os players.
 *  - Player 'from': volume de 1.0 → 0.0
 *  - Player 'to':   volume de 0.0 → targetVolume
 *  - Ao término, para o player 'from' e chama onComplete.
 *
 * Não há mistura manual de PCM — os dois SourceDataLines tocam simultaneamente
 * e o mixer de sistema operacional combina o áudio (suficiente para um protótipo).
 */
public class LinearCrossfadeEngine implements CrossfadeEngine {

    private static final int TICK_MS = 50;

    private Timeline timeline;
    private volatile boolean running = false;

    @Override
    public void start(AudioPlayer from, AudioPlayer to,
                      double durationSeconds, float targetVolume,
                      Runnable onComplete) {
        cancel();

        float initialFromVol = from.getVolume();
        to.setVolume(0f);

        // Garante que o player 'to' está tocando
        if (to.getState() != PlaybackState.PLAYING) to.play();

        int totalTicks = (int) (durationSeconds * 1000 / TICK_MS);
        int[] tick = {0};
        running = true;

        timeline = new Timeline(new KeyFrame(Duration.millis(TICK_MS), e -> {
            tick[0]++;
            float progress = Math.min(1.0f, (float) tick[0] / totalTicks);

            from.setVolume(initialFromVol * (1.0f - progress));
            to.setVolume(targetVolume * progress);

            if (progress >= 1.0f) {
                running = false;
                from.stop();
                timeline.stop();
                if (onComplete != null) onComplete.run();
            }
        }));

        timeline.setCycleCount(Timeline.INDEFINITE);
        timeline.play();
    }

    @Override
    public void cancel() {
        if (timeline != null) {
            timeline.stop();
            timeline = null;
        }
        running = false;
    }

    @Override
    public boolean isRunning() {
        return running;
    }
}
