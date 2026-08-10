package com.audionplay.ui;

import com.audionplay.audio.AudioPlayer;
import com.audionplay.audio.CrossfadeEngine;
import com.audionplay.audio.WaveformAnalyzer;
import com.audionplay.domain.Track;
import com.audionplay.domain.PlaybackState;

import java.util.function.BiConsumer;
import java.util.function.Consumer;

/**
 * Cola a UI com o engine de áudio.
 *
 * Regras:
 *  - Não sabe nada sobre JavaFX layouts (não importa nenhum componente concreto)
 *  - Não sabe nada sobre FFmpeg
 *  - Recebe callbacks da UI (botões) e chama o engine
 *  - Recebe callbacks do engine e notifica a UI
 *
 * A MainWindow injeta as dependências e registra os callbacks.
 */
public class PlayerController {

    private static final double CROSSFADE_DURATION = 3.5; // segundos

    private AudioPlayer primaryPlayer;
    private AudioPlayer secondaryPlayer; // usado no crossfade
    private final WaveformAnalyzer waveformAnalyzer;
    private final CrossfadeEngine  crossfadeEngine;

    private Track   loadedTrack;
    private boolean crossfadeTriggered = false; // evita disparo duplo por outro_ms

    // Callbacks para a UI atualizar
    private Consumer<Double>     onProgressUpdate; // fração 0.0–1.0
    private Consumer<String>     onTimeUpdate;     // "MM:SS"
    private Consumer<String>     onRemainingUpdate;// "MM:SS"
    private Consumer<double[]>   onWaveformReady;
    private Consumer<String>     onError;
    private Consumer<PlaybackState> onStateChange;
    private Runnable             onTrackEnd;         // chamado ao fim natural da faixa
    private Runnable             onAutoCrossfade;    // chamado quando outro_ms é atingido
    private Consumer<Double>     onIntroCountdown;   // segundos restantes até intro (0 = limpar)

    public PlayerController(AudioPlayer primary,
                            AudioPlayer secondary,
                            WaveformAnalyzer analyzer,
                            CrossfadeEngine crossfade) {
        this.primaryPlayer   = primary;
        this.secondaryPlayer = secondary;
        this.waveformAnalyzer = analyzer;
        this.crossfadeEngine  = crossfade;

        // Conecta callbacks do engine de áudio para a UI
        primaryPlayer.setOnTimeUpdate(this::handleTimeUpdate);
        primaryPlayer.setOnEndOfTrack(this::handleEndOfTrack);
    }

    // ── Ações disparadas pela UI ──────────────────────────────────────────────

    public void loadAndPlay(Track track) {
        this.loadedTrack        = track;
        this.crossfadeTriggered = false;
        primaryPlayer.load(track);
        primaryPlayer.play();
        notifyState(PlaybackState.PLAYING);

        // Análise de waveform em background (suporta path multi-arquivo via concat filter)
        waveformAnalyzer.analyze(
            track.filePath(),
            200,
            peaks -> { if (onWaveformReady != null) onWaveformReady.accept(peaks); },
            err   -> { if (onError != null) onError.accept(err); }
        );
    }

    public void play() {
        primaryPlayer.play();
        notifyState(primaryPlayer.getState());
    }

    public void pause() {
        primaryPlayer.pause();
        notifyState(PlaybackState.PAUSED);
    }

    public void stop() {
        crossfadeEngine.cancel();
        primaryPlayer.stop();
        notifyState(PlaybackState.STOPPED);
    }

    /** Seek por fração (0.0–1.0) — recebida do clique na WaveformView. */
    public void seekFraction(double fraction) {
        if (loadedTrack == null) return;
        double target = fraction * loadedTrack.durationSeconds();
        primaryPlayer.seek(target);
    }

    /** Inicia crossfade para a próxima faixa e troca os players ao terminar. */
    public void crossfadeTo(Track nextTrack, Runnable onComplete) {
        if (crossfadeEngine.isRunning()) return;

        secondaryPlayer.load(nextTrack);

        crossfadeEngine.start(
            primaryPlayer,
            secondaryPlayer,
            CROSSFADE_DURATION,
            1.0f,
            () -> {
                // Reconecta callbacks no player que agora é o ativo
                secondaryPlayer.setOnTimeUpdate(this::handleTimeUpdate);
                secondaryPlayer.setOnEndOfTrack(this::handleEndOfTrack);
                secondaryPlayer.setOnLevelUpdate(onLevelUpdateCallback);

                // Desconecta o antigo primary
                primaryPlayer.setOnTimeUpdate(null);
                primaryPlayer.setOnEndOfTrack(null);
                primaryPlayer.setOnLevelUpdate(null);

                // Troca as referências
                AudioPlayer tmp = primaryPlayer;
                primaryPlayer   = secondaryPlayer;
                secondaryPlayer = tmp;

                loadedTrack        = nextTrack;
                crossfadeTriggered = false; // permite que o outro da próxima faixa dispare
                notifyState(PlaybackState.PLAYING);
                if (onComplete != null) onComplete.run();
            }
        );
    }

    // Guardamos a referência do callback de nível para reconectar após swap
    private Consumer<float[]> onLevelUpdateCallback;

    public void setVolume(float volume) {
        primaryPlayer.setVolume(volume);
        secondaryPlayer.setVolume(volume);
    }

    public void dispose() {
        primaryPlayer.dispose();
        secondaryPlayer.dispose();
    }

    // ── Registro de callbacks da UI ───────────────────────────────────────────

    public void setOnProgressUpdate(Consumer<Double> cb)      { this.onProgressUpdate  = cb; }
    public void setOnTimeUpdate(Consumer<String> cb)          { this.onTimeUpdate      = cb; }
    public void setOnRemainingUpdate(Consumer<String> cb)     { this.onRemainingUpdate = cb; }
    public void setOnWaveformReady(Consumer<double[]> cb)     { this.onWaveformReady   = cb; }
    public void setOnError(Consumer<String> cb)               { this.onError           = cb; }
    public void setOnStateChange(Consumer<PlaybackState> cb)  { this.onStateChange     = cb; }
    public void setOnTrackEnd(Runnable cb)                    { this.onTrackEnd        = cb; }
    public void setOnAutoCrossfade(Runnable cb)               { this.onAutoCrossfade   = cb; }
    public void setOnIntroCountdown(Consumer<Double> cb)      { this.onIntroCountdown  = cb; }

    /** float[0]=rmsL, float[1]=rmsR, normalizados 0.0–1.0 */
    public void setOnLevelUpdate(Consumer<float[]> cb) {
        this.onLevelUpdateCallback = cb;
        primaryPlayer.setOnLevelUpdate(cb);
    }

    // ── Handlers internos ─────────────────────────────────────────────────────

    private void handleTimeUpdate(double currentSeconds) {
        if (loadedTrack == null) return;
        double start    = loadedTrack.effectiveStart();
        double duration = loadedTrack.effectiveDuration();
        double elapsed  = Math.max(0, currentSeconds - start);
        double frac     = duration > 0 ? Math.min(1.0, elapsed / duration) : 0.0;

        if (onProgressUpdate  != null) onProgressUpdate.accept(frac);
        if (onTimeUpdate      != null) onTimeUpdate.accept(formatTime(elapsed));
        if (onRemainingUpdate != null) onRemainingUpdate.accept(formatTime(Math.max(0, duration - elapsed)));

        // Countdown de intro para o locutor
        Double intro = loadedTrack.introSeconds();
        Consumer<Double> icCb = onIntroCountdown;
        if (icCb != null) {
            double remaining = intro != null ? Math.max(0, intro - currentSeconds) : 0;
            javafx.application.Platform.runLater(() -> icCb.accept(remaining));
        }

        // Disparo automático de crossfade ao atingir outro_ms
        Double outro = loadedTrack.outroSeconds();
        if (!crossfadeTriggered && outro != null && currentSeconds >= outro) {
            crossfadeTriggered = true;
            if (onAutoCrossfade != null) javafx.application.Platform.runLater(onAutoCrossfade);
        }
    }

    private void handleEndOfTrack() {
        // Durante crossfade, o fim natural da música A é esperado e não deve acionar playNext
        if (crossfadeEngine.isRunning()) return;
        notifyState(PlaybackState.STOPPED);
        if (onTrackEnd != null) onTrackEnd.run();
    }

    private void notifyState(PlaybackState state) {
        if (onStateChange != null) onStateChange.accept(state);
    }

    private static String formatTime(double seconds) {
        if (seconds < 0) seconds = 0;
        int m = (int) (seconds / 60);
        int s = (int) (seconds % 60);
        return String.format("%02d:%02d", m, s);
    }

    // ── Getters de estado ─────────────────────────────────────────────────────

    public PlaybackState getState()   { return primaryPlayer.getState(); }
    public Track getLoadedTrack()     { return loadedTrack; }
}
