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
import com.audionplay.horacerta.HoraCertaConfig;
import com.audionplay.horacerta.HoraCertaResolver;
import com.audionplay.horacerta.HoraCertaScheduler;
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
    private GridPane        contentGrid;

    // ── Mixer de software e cart player (Hot Keys) ────────────────────────
    private SoftwareMixer     mixer;
    private FfmpegAudioPlayer cartPlayer;

    // ── Hora Certa ────────────────────────────────────────────────────────
    private HoraCertaScheduler horaCertaScheduler;

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
        initHoraCerta();

        BorderPane root = new BorderPane();
        root.setStyle("-fx-background-color:" + Theme.BG_MAIN + ";");
        root.setTop(new TopBar());

        playoutCenter = buildPlayoutCenter();

        // Coluna central da tela "No Ar"
        StackPane contentArea = new StackPane(playoutCenter);

        Sidebar sidebar = new Sidebar(this::handleNavigation);

        // GridPane para "No Ar": 3 colunas exatamente iguais via percentWidth
        contentGrid = new GridPane();
        ColumnConstraints pc1 = new ColumnConstraints(); pc1.setPercentWidth(33.333); pc1.setFillWidth(true);
        ColumnConstraints pc2 = new ColumnConstraints(); pc2.setPercentWidth(33.333); pc2.setFillWidth(true);
        ColumnConstraints pc3 = new ColumnConstraints(); pc3.setPercentWidth(33.334); pc3.setFillWidth(true);
        contentGrid.getColumnConstraints().addAll(pc1, pc2, pc3);
        RowConstraints row = new RowConstraints();
        row.setVgrow(Priority.ALWAYS);
        row.setFillHeight(true);
        contentGrid.getRowConstraints().add(row);
        contentGrid.add(queuePanel,  0, 0);
        contentGrid.add(contentArea, 1, 0);
        contentGrid.add(rightPanel,  2, 0);
        GridPane.setFillWidth(queuePanel,  true);
        GridPane.setFillWidth(contentArea, true);
        GridPane.setFillWidth(rightPanel,  true);
        GridPane.setFillHeight(queuePanel,  true);
        GridPane.setFillHeight(contentArea, true);
        GridPane.setFillHeight(rightPanel,  true);

        // Área pós-sidebar: alterna entre grid "No Ar" (3 colunas) e telas full-width
        catalogPanel.setVisible(false);
        catalogPanel.setManaged(false);
        rotacaoPanel.setVisible(false);
        rotacaoPanel.setManaged(false);
        StackPane mainContent = new StackPane(contentGrid, catalogPanel, rotacaoPanel);
        HBox.setHgrow(mainContent, Priority.ALWAYS);

        HBox body = new HBox(0, sidebar, mainContent);
        body.setFillHeight(true);
        root.setCenter(body);

        rootStack = new StackPane(root);
        return rootStack;
    }

    // ── Navegação entre telas ─────────────────────────────────────────────────

    private void handleNavigation(String section) {
        boolean isPlayout = section.equals("NO AR");
        boolean isCatalog = section.equals("CATÁLOGO");
        boolean isRotacao = section.equals("ROTAÇÃO");

        show(contentGrid,  isPlayout);
        show(catalogPanel, isCatalog);
        show(rotacaoPanel, isRotacao);

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
        controller.setOnIntroCountdown(nowPlaying::setIntroCountdown);
        controller.setOnError(err -> nowPlaying.setStatus("Erro: " + err));
        controller.setOnStateChange(state -> {
            nowPlaying.applyState(state);
            if (state != PlaybackState.PLAYING) vuMeter.startDecay();
            if (state == PlaybackState.STOPPED) {
                queuePanel.setIsPlaying(false);
                nowPlaying.setIntroCountdown(0);
            }
            updateControls();
        });
        controller.setOnTrackEnd(() -> Platform.runLater(this::playNext));
        controller.setOnAutoCrossfade(() -> {
            // A fila usa Option A: index 0 = faixa tocando, index 1 = próxima
            queuePanel.peekSecond().ifPresent(nextEntity -> {
                double dur    = nextEntity.durationMs() / 1000.0;
                Double ci     = nextEntity.cueInMs()  != null ? nextEntity.cueInMs()  / 1000.0 : null;
                Double co     = nextEntity.cueOutMs() != null ? nextEntity.cueOutMs() / 1000.0 : null;
                Double ot     = nextEntity.outroMs()  != null ? nextEntity.outroMs()  / 1000.0 : null;
                Double it     = nextEntity.introMs()  != null ? nextEntity.introMs()  / 1000.0 : null;
                Track nextTrack = new Track(
                    nextEntity.path(),
                    nextEntity.title().isBlank() ? nextEntity.path() : nextEntity.title(),
                    nextEntity.artist(), dur, ci, co, ot, it
                );
                nowPlaying.setCrossfadeInProgress(true);
                nowPlaying.setStatus("Crossfade automático...");
                controller.crossfadeTo(nextTrack, () -> {
                    queuePanel.pollFirst(); // remove nextEntity da fila (agora está tocando)
                    nowPlaying.setTrack(nextTrack.title(), nextTrack.artist(), Theme.formatTime(nextTrack.effectiveDuration()));
                    nowPlaying.setTrackMeta(nextEntity);
                    nowPlaying.resetProgress();
                    nowPlaying.clearWaveform();
                    nowPlaying.setCrossfadeInProgress(false);
                    nowPlaying.setStatus("Analisando waveform...");
                    new com.audionplay.audio.ffmpeg.FfmpegWaveformAnalyzer().analyze(
                        nextTrack.filePath(), 200,
                        peaks -> { nowPlaying.setWaveformPeaks(peaks); nowPlaying.setStatus(""); },
                        err   -> nowPlaying.setStatus("Erro waveform: " + err)
                    );
                    updateControls();
                });
            });
        });
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

        nowPlaying.clearWaveform();
        nowPlaying.resetProgress();
        nowPlaying.setCurrentTime("00:00");
        nowPlaying.setRemainingTime("00:00");

        boolean isHoraCerta = entity.type() == TrackEntity.TrackType.HORA_CERTA;

        // Hora-certa com path vazio (sentinela da rotação) → resolve os arquivos agora
        if (isHoraCerta && entity.path().isBlank()) {
            try {
                HoraCertaConfig cfg = HoraCertaConfig.load();
                HoraCertaResolver resolver = new HoraCertaResolver(cfg);
                java.time.LocalTime now = java.time.LocalTime.now();
                java.util.List<String> paths = resolver.resolve(now);
                int durationMs = resolver.probeTotalDurationMs(paths);
                if (durationMs <= 0) durationMs = 10_000;
                String concatPath = String.join("|", paths);
                entity = TrackEntity.horaCerta(concatPath, durationMs);
            } catch (Exception ex) {
                nowPlaying.setStatus("Hora Certa: " + ex.getMessage());
                System.err.println("[HoraCerta] Erro ao resolver para playback: " + ex.getMessage());
                // Pula para o próximo item
                Platform.runLater(this::playNext);
                return;
            }
        }

        double duration = entity.durationMs() / 1000.0;
        Double cueIn  = entity.cueInMs()  != null ? entity.cueInMs()  / 1000.0 : null;
        Double cueOut = entity.cueOutMs() != null ? entity.cueOutMs() / 1000.0 : null;
        Double outro  = entity.outroMs()  != null ? entity.outroMs()  / 1000.0 : null;
        Double intro  = entity.introMs()  != null ? entity.introMs()  / 1000.0 : null;

        String title = isHoraCerta ? "Hora Certa"
            : (entity.title().isBlank() ? entity.path() : entity.title());

        Track track = new Track(entity.path(), title, entity.artist(),
            duration, cueIn, cueOut, outro, intro);

        nowPlaying.setTrack(track.title(), track.artist(), Theme.formatTime(duration));
        nowPlaying.setTrackMeta(entity);
        nowPlaying.setStatus(isHoraCerta ? "Hora Certa..." : "Carregando...");

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

    // ── Hora Certa ────────────────────────────────────────────────────────────

    private void initHoraCerta() {
        horaCertaScheduler = new HoraCertaScheduler();

        horaCertaScheduler.setOnFire((paths, time) -> {
            // Resolve duração total dos arquivos
            HoraCertaConfig cfg      = HoraCertaConfig.load();
            HoraCertaResolver resolver = new HoraCertaResolver(cfg);
            int durationMs = resolver.probeTotalDurationMs(paths);
            if (durationMs <= 0) durationMs = 10_000; // fallback 10s

            // Caminho concat: "hora.mp3|minuto.mp3"
            String concatPath = String.join("|", paths);
            TrackEntity hc    = TrackEntity.horaCerta(concatPath, durationMs);

            // Insere após o item atual (AFTER_CURRENT, igual ao Go)
            queuePanel.insertAfterCurrent(hc);

            String label = String.format("Hora Certa %02d:%02d enfileirada", time.getHour(), time.getMinute());
            Toast.show(rootStack, label);

            // Se a fila estava vazia e o engine está parado, inicia imediatamente
            if (controller.getState() == PlaybackState.STOPPED) {
                queuePanel.peekFirst().ifPresent(this::playTrack);
            }
            updateControls();
        });

        horaCertaScheduler.setOnError(err ->
            Toast.show(rootStack, "Hora Certa: " + err));

        // Na primeira execução (hoursDir vazio), aplica e salva os defaults
        HoraCertaConfig cfg = HoraCertaConfig.load();
        if (cfg.hoursDir().isBlank()) {
            cfg = HoraCertaConfig.defaults();
            cfg.save();
        }
        horaCertaScheduler.applyConfig(cfg);

        // Botão ⏰ na info-bar da fila abre o dialog de configuração
        queuePanel.setOnHoraCertaBtn(() -> {
            HoraCertaConfigDialog dialog = new HoraCertaConfigDialog(stage.getScene().getWindow());
            dialog.setOnSaved(() -> {
                // Reaplica configuração ao salvar
                horaCertaScheduler.applyConfig(HoraCertaConfig.load());
                Toast.show(rootStack, "Configuração de Hora Certa salva");
            });
            dialog.show();
        });
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
