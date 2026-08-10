package com.audionplay.ui;

import com.audionplay.audio.MixerChannel;
import com.audionplay.audio.SoftwareMixer;
import com.audionplay.audio.ffmpeg.FfmpegAudioPlayer;
import com.audionplay.audio.ffmpeg.FfmpegLocator;
import com.audionplay.audio.ffmpeg.FfmpegWaveformAnalyzer;
import com.audionplay.audio.ffmpeg.LinearCrossfadeEngine;
import com.audionplay.db.entity.TrackEntity;
import com.audionplay.domain.PlaybackState;
import com.audionplay.domain.Track;
import com.audionplay.ui.components.*;
import javafx.application.Platform;
import javafx.geometry.Insets;
import javafx.scene.control.Button;
import javafx.scene.layout.*;

import com.audionplay.ui.components.Toast;
import javafx.stage.FileChooser;
import javafx.stage.Stage;

import java.io.File;

/**
 * Orquestrador da janela principal.
 *
 * Responsabilidades:
 *  - Instanciar e posicionar os componentes visuais
 *  - Criar o PlayerController e injetar as dependências de áudio
 *  - Conectar os callbacks do controller nos componentes
 *  - Implementar as ações do usuário (abrir arquivo, crossfade, pause)
 *  - Gerenciar a navegação entre telas (Playout ↔ Catálogo)
 *
 * Não contém nenhum código de layout ou estilo — tudo está nos componentes.
 */
public class MainWindow {

    private PlayerController controller;
    private Stage stage;

    // ── Componentes de tela ───────────────────────────────────────────────────
    private NowPlayingPanel nowPlaying;
    private RightPanel      rightPanel;
    private QueuePanel      queuePanel;
    private VBox            playoutCenter;
    private CatalogPanel    catalogPanel;
    private RotacaoPanel    rotacaoPanel;
    private NextTrackBar    nextTrackBar;
    private TabsPanel       tabsPanel;
    private StackPane       rootStack;

    // ── Mixer de software e cart player (Hot Keys) ────────────────────────
    private SoftwareMixer     mixer;
    private FfmpegAudioPlayer cartPlayer;

    // ── Ponto de entrada ──────────────────────────────────────────────────────

    public StackPane build(Stage stage) {
        this.stage = stage;

        nowPlaying   = new NowPlayingPanel();
        rightPanel   = new RightPanel();
        queuePanel   = new QueuePanel();
        catalogPanel = new CatalogPanel();
        rotacaoPanel = new RotacaoPanel();
        tabsPanel    = new TabsPanel();

        mixer = new SoftwareMixer(3); // ch0=program, ch1=secondary, ch2=cart
        mixer.start();
        buildController();
        wireActions();

        BorderPane root = new BorderPane();
        root.setStyle("-fx-background-color:" + Theme.BG_MAIN + ";");
        root.setTop(new TopBar());

        playoutCenter = buildPlayoutCenter();

        // Área central comutável: Playout, Catálogo ou Rotação
        StackPane contentArea = new StackPane(playoutCenter, catalogPanel, rotacaoPanel);
        HBox.setHgrow(contentArea, Priority.ALWAYS);
        catalogPanel.setVisible(false);
        catalogPanel.setManaged(false);
        rotacaoPanel.setVisible(false);
        rotacaoPanel.setManaged(false);

        Sidebar sidebar = new Sidebar(this::handleNavigation);

        HBox body = new HBox(0, sidebar, queuePanel, contentArea, rightPanel);
        body.setFillHeight(true);
        root.setCenter(body);

        rootStack = new StackPane(root);
        return rootStack;
    }

    // ── Navegação entre telas ─────────────────────────────────────────────────

    private void handleNavigation(String section) {
        boolean isPlayout  = section.equals("NO AR");
        boolean isCatalog  = section.equals("CATÁLOGO");
        boolean isRotacao  = section.equals("ROTAÇÃO");

        show(playoutCenter, isPlayout);
        show(queuePanel,    isPlayout);
        show(rightPanel,    isPlayout);
        show(catalogPanel,  isCatalog);
        show(rotacaoPanel,  isRotacao);

        if (isCatalog) catalogPanel.reload();
    }

    private static void show(Region node, boolean visible) {
        node.setVisible(visible);
        node.setManaged(visible);
    }

    // ── Montagem do painel Playout ────────────────────────────────────────────

    private VBox buildPlayoutCenter() {
        VBox.setVgrow(tabsPanel, Priority.ALWAYS);

        nextTrackBar = new NextTrackBar();

        VBox center = new VBox(10, nowPlaying, nextTrackBar, tabsPanel);
        center.setStyle("-fx-background-color:" + Theme.BG_MAIN + ";");
        center.setPadding(new Insets(14));
        VBox.setVgrow(center, Priority.ALWAYS);
        return center;
    }

    // ── Wiring do controller ──────────────────────────────────────────────────

    private void buildController() {
        // Cada player recebe um canal exclusivo no mixer
        cartPlayer = new FfmpegAudioPlayer(mixer.getChannel(2));
        controller  = new PlayerController(
            new FfmpegAudioPlayer(mixer.getChannel(0)),  // primary
            new FfmpegAudioPlayer(mixer.getChannel(1)),  // secondary (crossfade)
            new FfmpegWaveformAnalyzer(),
            new LinearCrossfadeEngine()
        );

        VuMeterPanel vuMeter = rightPanel.getVuMeter();

        controller.setOnProgressUpdate(nowPlaying::setProgress);
        controller.setOnTimeUpdate(nowPlaying::setCurrentTime);
        controller.setOnRemainingUpdate(nowPlaying::setRemainingTime);
        controller.setOnWaveformReady(nowPlaying::setWaveformPeaks);
        controller.setOnError(err -> nowPlaying.setStatus("Erro: " + err));
        controller.setOnStateChange(state -> {
            nowPlaying.applyState(state);
            if (state != PlaybackState.PLAYING) vuMeter.startDecay();
            if (state == PlaybackState.STOPPED) queuePanel.setIsPlaying(false);
            updateControls();
        });
        controller.setOnTrackEnd(() -> Platform.runLater(this::playNext));
        // VU meter alimentado pelo mixer — reflete o mix de todos os canais
        mixer.setOnLevelUpdate(vuMeter::applyLevels);
    }

    // ── Wiring das ações da UI ────────────────────────────────────────────────

    private void wireActions() {
        nowPlaying.setOnOpenFile(this::openFile);
        nowPlaying.setOnPause(this::togglePause);
        nowPlaying.setOnStop(controller::stop);
        nowPlaying.setOnPlayQueue(() -> {
            queuePanel.peekFirst().ifPresent(this::playTrack);
            updateControls();
        });
        nowPlaying.setOnNext(this::playNext);
        nowPlaying.setOnSeek(controller::seekFraction);

        rightPanel.getVolume().setOnProgramVolumeChange(controller::setVolume);
        rightPanel.getVolume().setOnBotoneiraVolumeChange(cartPlayer::setVolume);

        // Do catálogo: tocar faixa diretamente ou adicionar à fila
        catalogPanel.setOnPlay(this::playTrack);
        catalogPanel.setOnAddToQueue(entity -> {
            queuePanel.addTrack(entity);
            String title = entity.title().isBlank() ? "Faixa" : entity.title();
            Toast.show(rootStack, "\"" + title + "\" adicionado à fila");
        });

        // Da fila: tocar item ao clicar ▶ no hover do card
        queuePanel.setOnPlay(entity -> {
            queuePanel.jumpToItem(entity);
            playTrack(entity);
        });
        queuePanel.setOnQueueChanged(this::updateControls);

        // Rotação: enfileira faixas geradas e permite CUE preview
        rotacaoPanel.setOnEnqueue(tracks -> {
            tracks.forEach(entity -> {
                queuePanel.addTrack(entity);
            });
            int n = tracks.size();
            Toast.show(rootStack, n + " faixa" + (n != 1 ? "s" : "") + " adicionada" + (n != 1 ? "s" : "") + " à fila");
        });
        rotacaoPanel.setOnCue(entity -> {
            cartPlayer.stop();
            Track t = new Track(
                entity.path(),
                entity.title().isBlank() ? entity.path() : entity.title(),
                entity.artist(),
                entity.durationMs() / 1000.0
            );
            cartPlayer.load(t);
            cartPlayer.play();
        });

        // Hot Keys: toca via cart player dedicado
        tabsPanel.setOnCartPlay(btn -> {
            Track t = new Track(
                btn.trackPath(),
                btn.trackTitle() != null && !btn.trackTitle().isBlank() ? btn.trackTitle() : btn.label(),
                btn.trackArtist() != null ? btn.trackArtist() : "",
                btn.durationMs() / 1000.0
            );
            cartPlayer.stop();
            cartPlayer.load(t);
            cartPlayer.play();
        });
    }

    // ── Ações do usuário ──────────────────────────────────────────────────────

    private void openFile() {
        FileChooser chooser = new FileChooser();
        chooser.setTitle("Abrir arquivo de áudio");
        chooser.getExtensionFilters().addAll(
            new FileChooser.ExtensionFilter("Áudio", "*.mp3", "*.wav", "*.flac", "*.ogg", "*.aac", "*.m4a"),
            new FileChooser.ExtensionFilter("Todos", "*.*")
        );
        File file = chooser.showOpenDialog(stage);
        if (file == null) return;

        nowPlaying.setStatus("Analisando waveform...");
        nowPlaying.clearWaveform();
        nowPlaying.resetProgress();
        nowPlaying.setCurrentTime("00:00");
        nowPlaying.setRemainingTime("00:00");

        double duration = probeDuration(file.getAbsolutePath());
        Track track = new Track(file.getAbsolutePath(), file.getName(), "—", duration);
        nowPlaying.setTrack(track.title(), track.artist(), Theme.formatTime(duration));
        controller.loadAndPlay(track);
    }

    private void playTrack(TrackEntity entity) {
        handleNavigation("NO AR");

        nowPlaying.setStatus("Carregando...");
        nowPlaying.clearWaveform();
        nowPlaying.resetProgress();
        nowPlaying.setCurrentTime("00:00");
        nowPlaying.setRemainingTime("00:00");

        double duration = entity.durationMs() / 1000.0;
        Double cueIn  = entity.cueInMs()  != null ? entity.cueInMs()  / 1000.0 : null;
        Double cueOut = entity.cueOutMs() != null ? entity.cueOutMs() / 1000.0 : null;
        Track track = new Track(
            entity.path(),
            entity.title().isBlank() ? entity.path() : entity.title(),
            entity.artist(),
            duration,
            cueIn,
            cueOut
        );
        nowPlaying.setTrack(track.title(), track.artist(), Theme.formatTime(duration));
        nowPlaying.setTrackMeta(entity);
        controller.loadAndPlay(track);
        queuePanel.setIsPlaying(true);
        updateNextTrackBar();
    }

    /** Remove o item atual da fila e inicia o próximo. */
    private void playNext() {
        queuePanel.pollFirst().ifPresentOrElse(
            next -> { playTrack(next); updateControls(); },
            () -> { queuePanel.setIsPlaying(false); nowPlaying.clearTrack(); updateControls(); }
        );
    }

    private void updateControls() {
        int size = queuePanel.size();
        boolean playing = controller.getState() == PlaybackState.PLAYING;
        nowPlaying.setPlayEnabled(size > 0);
        nowPlaying.setNextEnabled(playing && size >= 2);
        updateNextTrackBar();
    }

    private void updateNextTrackBar() {
        if (nextTrackBar == null) return;
        queuePanel.peekSecond().ifPresentOrElse(
            e -> nextTrackBar.update(
                e.title().isBlank() ? e.path() : e.title(),
                e.artist(),
                Theme.formatTime(e.durationMs() / 1000.0)
            ),
            () -> nextTrackBar.clear()
        );
    }

    private void openCrossfade(Button nextBtn) {
        if (controller.getLoadedTrack() == null) {
            nowPlaying.setStatus("Carregue uma faixa primeiro.");
            return;
        }
        if (controller.getState() != PlaybackState.PLAYING) {
            nowPlaying.setStatus("Inicie a reprodução antes de fazer crossfade.");
            return;
        }

        FileChooser chooser = new FileChooser();
        chooser.setTitle("Selecionar próxima faixa (crossfade)");
        chooser.getExtensionFilters().add(
            new FileChooser.ExtensionFilter("Áudio", "*.mp3", "*.wav", "*.flac", "*.ogg", "*.aac", "*.m4a")
        );
        File file = chooser.showOpenDialog(stage);
        if (file == null) return;

        double duration = probeDuration(file.getAbsolutePath());
        Track nextTrack = new Track(file.getAbsolutePath(), file.getName(), "—", duration);

        nowPlaying.setCrossfadeInProgress(true);
        nowPlaying.setStatus("Crossfade em andamento (3.5s)...");

        controller.crossfadeTo(nextTrack, () -> {
            nowPlaying.setTrack(nextTrack.title(), nextTrack.artist(), Theme.formatTime(nextTrack.durationSeconds()));
            nowPlaying.resetProgress();
            nowPlaying.clearWaveform();
            nowPlaying.setStatus("Analisando waveform...");
            nowPlaying.setCrossfadeInProgress(false);

            new FfmpegWaveformAnalyzer().analyze(
                nextTrack.filePath(), 200,
                peaks -> { nowPlaying.setWaveformPeaks(peaks); nowPlaying.setStatus(""); },
                err   -> nowPlaying.setStatus("Erro waveform: " + err)
            );
        });
    }

    private void togglePause() {
        if (controller.getState() == PlaybackState.PLAYING) {
            controller.pause();
        } else {
            controller.play();
        }
    }

    // ── Utilitários ───────────────────────────────────────────────────────────

    private double probeDuration(String filePath) {
        try {
            Process p = new ProcessBuilder(
                FfmpegLocator.ffprobe(), "-v", "quiet",
                "-show_entries", "format=duration",
                "-of", "csv=p=0", filePath
            ).start();
            String out = new String(p.getInputStream().readAllBytes()).trim();
            p.waitFor();
            return Double.parseDouble(out.replace(',', '.'));
        } catch (Exception e) {
            return 0.0;
        }
    }
}
