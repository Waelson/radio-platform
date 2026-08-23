package com.audionplay.ui.components;

import com.audionplay.db.entity.TrackEntity;
import com.audionplay.domain.PlaybackState;
import com.audionplay.ui.Theme;
import com.audionplay.ui.WaveformView;
import javafx.geometry.Insets;
import javafx.geometry.Pos;
import javafx.scene.control.Button;
import javafx.scene.control.Label;
import javafx.scene.layout.*;
import javafx.scene.paint.Color;
import javafx.scene.shape.Circle;

import java.util.function.Consumer;

/**
 * Painel central "Tocando Agora" — visual fiel ao player.html.
 *
 * Seções (top→bottom):
 *   pn-topbar  : live-badge + spacer + mode-badge
 *   pn-content : categoria + título + artista + meta-row
 *   waveform   : WaveformView
 *   progress   : barra gradiente cyan→azul
 *   pn-times   : elapsed | countdown | total
 *   pn-controls: ⏸ Pausar | ▶ NO AR / ■ Stop | Próximo ⏭
 */
public class NowPlayingPanel extends VBox {

    // ── Campos de estado ──────────────────────────────────────────────────────
    private final Label   categoryLabel;          // tipo do áudio (MÚSICA, VINHETA…)
    private final Label  trackTitleLabel;
    private final Label  trackArtistLabel;
    private final Label  currentTimeLabel;
    private final Label  remainingTimeLabel;
    private final Label  totalTimeLabel;
    private final Label  statusLabel;          // mantido para compatibilidade
    private final Button pauseBtn;
    private final Button stopBtn;
    private final Button nextBtn;
    private final WaveformView waveformView;

    // ── Campos de progresso personalizados ────────────────────────────────────
    private final Region progressTrack;
    private final Region progressFill;
    private double currentFraction = 0.0;

    // ── Countdown de intro ────────────────────────────────────────────────────
    private final HBox  introCountdownRow;
    private final Label introCountdownLabel;

    // ── Campos do topbar ──────────────────────────────────────────────────────
    private final Circle liveDot;
    private final Label  liveBadgeLabel;
    private final HBox   liveBadge;

    // ── Callbacks ─────────────────────────────────────────────────────────────
    private Runnable onPlayQueue;
    private Runnable onStopAction;
    private boolean  isPlaying = false;

    // ── Estilos fixos dos botões ──────────────────────────────────────────────
    private static final String BTN_BASE =
        "-fx-min-height:46px;-fx-padding:0 14 0 14;" +
        "-fx-border-radius:10;-fx-background-radius:10;" +
        "-fx-font-size:15px;-fx-font-weight:bold;-fx-cursor:hand;";
    private static final String BTN_SECONDARY =
        BTN_BASE +
        "-fx-border-color:#20384c;-fx-border-width:1;" +
        "-fx-background-color:#000000;" +
        "-fx-text-fill:#d7e3ec;";
    private static final String BTN_PRIMARY_PLAY =
        BTN_BASE +
        "-fx-background-color:linear-gradient(to bottom,#00bfe9,#008aad);" +
        "-fx-border-color:#10cbed;-fx-border-width:1;" +
        "-fx-text-fill:#00141b;";
    private static final String BTN_PRIMARY_STOP =
        BTN_BASE +
        "-fx-background-color:linear-gradient(to bottom,#c0392b,#8e1a10);" +
        "-fx-border-color:rgba(255,77,103,0.70);-fx-border-width:1;" +
        "-fx-text-fill:#ffffff;";

    // ─────────────────────────────────────────────────────────────────────────

    public NowPlayingPanel() {
        super(0);
        setStyle(
            "-fx-background-color:#000000;" +
            "-fx-background-radius:12;" +
            "-fx-border-color:#20384c;" +
            "-fx-border-radius:12;-fx-border-width:1;"
        );

        // ── Inicializa campos ─────────────────────────────────────────────────
        categoryLabel = new Label("");
        categoryLabel.setStyle("-fx-font-size:10px;-fx-font-weight:bold;-fx-text-fill:#2dd8ff;");

        trackTitleLabel = new Label("—");
        trackTitleLabel.setStyle(
            "-fx-font-size:24px;-fx-font-weight:bold;-fx-text-fill:#eef7ff;"
        );
        trackTitleLabel.setMaxWidth(Double.MAX_VALUE);
        trackTitleLabel.setMinHeight(30);

        trackArtistLabel = new Label("");
        trackArtistLabel.setStyle("-fx-font-size:15px;-fx-text-fill:#b8cad8;");
        trackArtistLabel.setMinHeight(22);
        trackArtistLabel.setMaxWidth(Double.MAX_VALUE);

        currentTimeLabel = new Label("00:00");
        currentTimeLabel.setStyle(
            "-fx-font-size:13px;-fx-text-fill:#88a0b5;"
        );

        remainingTimeLabel = new Label("00:00");
        remainingTimeLabel.setStyle(
            "-fx-font-size:21px;-fx-font-weight:900;-fx-text-fill:#f4fbff;" +
            "-fx-font-family:'Courier New',monospace;"
        );

        totalTimeLabel = new Label("00:00");
        totalTimeLabel.setStyle(
            "-fx-font-size:13px;-fx-text-fill:#88a0b5;"
        );

        statusLabel = new Label("");
        statusLabel.setStyle("-fx-font-size:10px;-fx-text-fill:#88a0b5;");
        statusLabel.setVisible(false);
        statusLabel.setManaged(false);

        waveformView = new WaveformView();
        waveformView.setPrefHeight(67);
        waveformView.setMaxWidth(Double.MAX_VALUE);

        // Barra de progresso personalizada
        progressTrack = new Region();
        progressTrack.setPrefHeight(8);
        progressTrack.setMaxWidth(Double.MAX_VALUE);
        progressTrack.setStyle(
            "-fx-background-color:#07131d;" +
            "-fx-background-radius:99;" +
            "-fx-border-color:rgba(49,83,109,0.5);" +
            "-fx-border-radius:99;-fx-border-width:1;"
        );
        progressFill = new Region();
        progressFill.setPrefHeight(8);
        progressFill.setPrefWidth(0);
        progressFill.setMaxWidth(Region.USE_PREF_SIZE);  // impede o StackPane de esticá-lo
        progressFill.setStyle(
            "-fx-background-color:linear-gradient(to right,#20e6ff,#3978ff);" +
            "-fx-background-radius:99;" +
            "-fx-effect:dropshadow(gaussian,rgba(0,212,255,0.45),13,0,0,0);"
        );
        progressTrack.widthProperty().addListener((obs, o, n) ->
            progressFill.setPrefWidth(n.doubleValue() * currentFraction));

        // Countdown de intro
        introCountdownLabel = new Label("INTRO em 0.0s");
        introCountdownLabel.setStyle(
            "-fx-font-size:13px;-fx-font-weight:900;" +
            "-fx-text-fill:#36d399;" +
            "-fx-font-family:'Courier New',monospace;"
        );
        Label introIcon = new Label("▶");
        introIcon.setStyle("-fx-font-size:10px;-fx-text-fill:#36d399;");
        introCountdownRow = new HBox(6, introIcon, introCountdownLabel);
        introCountdownRow.setAlignment(Pos.CENTER);
        introCountdownRow.setPadding(new Insets(4, 16, 4, 16));
        introCountdownRow.setStyle(
            "-fx-background-color:rgba(54,211,153,0.09);" +
            "-fx-border-color:rgba(54,211,153,0.22);-fx-border-width:1 0 1 0;"
        );
        introCountdownRow.setVisible(false);
        introCountdownRow.setManaged(false);

        // Live badge
        liveDot = new Circle(3.5, Color.web("#4a6478"));
        liveBadgeLabel = new Label("NADA TOCANDO");
        liveBadgeLabel.setStyle(
            "-fx-font-size:10px;-fx-font-weight:800;-fx-text-fill:#88a0b5;"
        );
        liveBadge = new HBox(7);
        liveBadge.setAlignment(Pos.CENTER_LEFT);
        liveBadge.setMinHeight(26);
        liveBadge.setMaxHeight(26);
        liveBadge.getChildren().addAll(liveDot, liveBadgeLabel);
        applyBadgeIdle();

        // Botões
        pauseBtn = new Button("⏸  Pausar");
        pauseBtn.setStyle(BTN_SECONDARY);

        stopBtn = new Button("▶  NO AR");
        stopBtn.setStyle(BTN_PRIMARY_PLAY);
        stopBtn.setDisable(true);
        HBox.setHgrow(stopBtn, Priority.ALWAYS);
        stopBtn.setMaxWidth(Double.MAX_VALUE);
        stopBtn.setOnAction(e -> {
            if (isPlaying) { if (onStopAction != null) onStopAction.run(); }
            else           { if (onPlayQueue  != null) onPlayQueue.run();  }
        });

        nextBtn = new Button("Próximo  ⏭");
        nextBtn.setStyle(BTN_SECONDARY);
        nextBtn.setDisable(true);

        // ── Monta layout ──────────────────────────────────────────────────────
        VBox.setMargin(waveformView, new Insets(9, 16, 0, 16));

        getChildren().addAll(
            buildTopBar(),
            buildContent(),
            waveformView,
            buildProgressWrap(),
            introCountdownRow,
            buildTimesRow(),
            buildControls()
        );
    }

    // ── API de estado ─────────────────────────────────────────────────────────

    public void setTrack(String title, String artist, String totalTime) {
        trackTitleLabel.setText(title);
        trackArtistLabel.setText(artist);
        totalTimeLabel.setText(totalTime);
    }

    public void setProgress(double frac) {
        currentFraction = frac;
        double w = progressTrack.getWidth();
        if (w > 0) progressFill.setPrefWidth(w * frac);
        waveformView.setProgress(frac);
    }

    public void setCurrentTime(String t)        { currentTimeLabel.setText(t); }
    public void setRemainingTime(String t)      { remainingTimeLabel.setText(t); }

    /**
     * Atualiza o countdown de intro (tempo até o locutor parar de falar).
     * @param secondsRemaining segundos restantes até o intro; 0 ou negativo = ocultar.
     */
    public void setIntroCountdown(double secondsRemaining) {
        if (secondsRemaining > 0) {
            introCountdownLabel.setText(String.format("INTRO em %.1fs", secondsRemaining));
            introCountdownRow.setVisible(true);
            introCountdownRow.setManaged(true);
        } else {
            introCountdownRow.setVisible(false);
            introCountdownRow.setManaged(false);
        }
    }
    public void setWaveformPeaks(double[] peaks){ waveformView.setPeaks(peaks); }
    public void clearWaveform()                 { waveformView.clear(); }

    public void setStatus(String msg) {
        statusLabel.setText(msg);
        boolean show = msg != null && !msg.isEmpty();
        statusLabel.setVisible(show);
        statusLabel.setManaged(show);
    }

    /**
     * Preenche o tipo de áudio e os CUE points a partir de uma TrackEntity.
     * Chamado ao iniciar reprodução de um item da fila ou do catálogo.
     */
    public void setTrackMeta(TrackEntity entity) {
        // ── Tipo do áudio (badge de categoria) ────────────────────────────────
        String typeText;
        String typeColor;
        if (entity.type() != null) {
            typeText = switch (entity.type()) {
                case MUSIC      -> "MÚSICA";
                case VINHETA    -> "VINHETA";
                case JINGLE     -> "JINGLE";
                case SPOT       -> "SPOT";
                case EFEITOS    -> "EFEITOS";
                case HORA_CERTA -> "HORA CERTA";
            };
            typeColor = switch (entity.type()) {
                case MUSIC      -> "#2dd8ff";
                case VINHETA    -> "#ce93d8";
                case JINGLE     -> "#a5d6a7";
                case SPOT       -> "#ffcc80";
                case EFEITOS    -> "#ef9a9a";
                case HORA_CERTA -> "#9d9ab8";
            };
        } else {
            typeText  = "—";
            typeColor = "#88a0b5";
        }
        categoryLabel.setText(typeText);
        categoryLabel.setStyle("-fx-font-size:10px;-fx-font-weight:bold;-fx-text-fill:" + typeColor + ";");

        // Marcadores no waveform
        waveformView.setCueMarkers(entity.cueInMs(), entity.introMs(),
                                   entity.outroMs(), entity.cueOutMs(),
                                   entity.durationMs());
    }

    /** Limpa todos os dados exibidos após a fila esvaziar. */
    public void clearTrack() {
        trackTitleLabel.setText("—");
        trackArtistLabel.setText("");
        currentTimeLabel.setText("00:00");
        remainingTimeLabel.setText("00:00");
        totalTimeLabel.setText("00:00");
        clearWaveform();
        resetProgress();
        clearMeta();
        setIntroCountdown(0);
    }

    private void clearMeta() {
        categoryLabel.setText("");
    }

    public void resetProgress() {
        currentFraction = 0;
        progressFill.setPrefWidth(0);
        waveformView.setProgress(0);
    }

    public void applyState(PlaybackState state) {
        switch (state) {
            case PLAYING -> {
                isPlaying = true;
                applyStopStyle();
                applyBadgePlaying();
                pauseBtn.setText("⏸  Pausar");
            }
            case PAUSED -> {
                applyBadgePaused();
                pauseBtn.setText("▶  Retomar");
            }
            case STOPPED -> {
                isPlaying = false;
                applyPlayStyle();
                stopBtn.setDisable(false);
                applyBadgeIdle();
                pauseBtn.setText("⏸  Pausar");
                resetProgress();
            }
        }
        setStatus("");
    }

    public void setCrossfadeInProgress(boolean inProgress) {
        nextBtn.setDisable(inProgress);
        nextBtn.setText(inProgress ? "⏭  Crossfade..." : "Próximo  ⏭");
    }

    // ── API de eventos ────────────────────────────────────────────────────────

    public void setOnOpenFile(Runnable h)           { /* botão removido */ }
    public void setOnPause(Runnable h)              { pauseBtn.setOnAction(e -> h.run()); }
    public void setOnStop(Runnable h)               { this.onStopAction = h; }
    public void setOnPlayQueue(Runnable h)          { this.onPlayQueue = h; }
    public void setOnNext(Runnable h)               { nextBtn.setOnAction(e -> h.run()); }
    public void setOnCrossfade(Consumer<Button> h)  { /* substituído por setOnNext */ }
    public void setOnSeek(Consumer<Double> h)       { waveformView.setOnSeek(h); }

    public void setPlayEnabled(boolean enabled)     { stopBtn.setDisable(isPlaying ? false : !enabled); }
    public void setNextEnabled(boolean enabled)     { nextBtn.setDisable(!enabled); }

    // ── Seções de layout ──────────────────────────────────────────────────────

    /** pn-topbar: live-badge | spacer | mode-badge */
    private HBox buildTopBar() {

        Label modeBadge = new Label("AUTOMÁTICO");
        modeBadge.setStyle(
            "-fx-font-size:10px;-fx-font-weight:bold;-fx-text-fill:#36d399;" +
            "-fx-background-color:rgba(54,211,153,0.10);" +
            "-fx-border-color:rgba(54,211,153,0.24);-fx-border-width:1;" +
            "-fx-background-radius:7;-fx-border-radius:7;" +
            "-fx-padding:5 9 5 9;"
        );

        Region spacer = new Region();
        HBox.setHgrow(spacer, Priority.ALWAYS);

        HBox topbar = new HBox(9, liveBadge, spacer, modeBadge);
        topbar.setAlignment(Pos.CENTER_LEFT);
        topbar.setPadding(new Insets(0, 14, 0, 14));
        topbar.setMinHeight(48);
        topbar.setStyle(
            "-fx-background-color:linear-gradient(from 0% 0% to 0% 100%, #6b6b6b 0%, #555555 22%, #3a3a3a 47%, #080808 49%, #040404 72%, #000000 100%);" +
            "-fx-border-color:#333333;-fx-border-width:0 0 1 0;" +
            "-fx-background-radius:12 12 0 0;"
        );
        return topbar;
    }

    /** pn-content: categoria + título + artista + meta-row */
    private VBox buildContent() {

        VBox.setMargin(trackTitleLabel,  new Insets(6, 0, 0, 0));
        VBox.setMargin(trackArtistLabel, new Insets(6, 0, 0, 0));
        VBox.setMargin(statusLabel,      new Insets(4, 0, 0, 0));

        VBox content = new VBox(0,
            categoryLabel,
            trackTitleLabel,
            trackArtistLabel,
            statusLabel
        );
        content.setPadding(new Insets(16, 16, 2, 16));
        content.setMinHeight(116);
        return content;
    }

    /** pn-progress-wrap: track + fill sobrepostos */
    private VBox buildProgressWrap() {
        StackPane bar = new StackPane(progressTrack, progressFill);
        StackPane.setAlignment(progressFill, Pos.CENTER_LEFT);
        VBox wrap = new VBox(bar);
        wrap.setPadding(new Insets(11, 16, 0, 16));
        return wrap;
    }

    /** pn-times: elapsed | countdown | total */
    private HBox buildTimesRow() {
        Region l = new Region(); HBox.setHgrow(l, Priority.ALWAYS);
        Region r = new Region(); HBox.setHgrow(r, Priority.ALWAYS);
        HBox times = new HBox();
        times.setAlignment(Pos.BOTTOM_CENTER);
        times.getChildren().addAll(currentTimeLabel, l, remainingTimeLabel, r, totalTimeLabel);
        times.setPadding(new Insets(8, 16, 13, 16));
        return times;
    }

    /** pn-controls: botões centralizados, border-top */
    private HBox buildControls() {
        HBox center = new HBox(8, pauseBtn, stopBtn, nextBtn);
        center.setAlignment(Pos.CENTER);
        HBox.setHgrow(center, Priority.ALWAYS);

        HBox controls = new HBox(center);
        controls.setAlignment(Pos.CENTER);
        controls.setPadding(new Insets(11));
        controls.setStyle(
            "-fx-border-color:#20384c;-fx-border-width:1 0 0 0;" +
            "-fx-background-color:rgba(5,14,22,0.60);"
        );
        return controls;
    }

    // ── Estilos do botão principal ────────────────────────────────────────────

    // ── Helpers do live badge ─────────────────────────────────────────────────

    private static final String BADGE_STYLE_BASE =
        "-fx-background-radius:7;-fx-border-radius:7;-fx-border-width:1;-fx-padding:0 9 0 9;";

    private void applyBadgeIdle() {
        liveBadge.setStyle(BADGE_STYLE_BASE +
            "-fx-background-color:rgba(255,255,255,0.04);" +
            "-fx-border-color:#20384c;");
        liveDot.setFill(Color.web("#4a6478"));
        liveDot.setEffect(null);
        liveBadgeLabel.setText("NADA TOCANDO");
        liveBadgeLabel.setStyle("-fx-font-size:10px;-fx-font-weight:800;-fx-text-fill:#88a0b5;");
    }

    private void applyBadgePlaying() {
        liveBadge.setStyle(BADGE_STYLE_BASE +
            "-fx-background-color:rgba(255,77,103,0.13);" +
            "-fx-border-color:rgba(255,77,103,0.28);");
        liveDot.setFill(Color.web("#f87272"));
        liveDot.setEffect(new javafx.scene.effect.DropShadow(10, Color.web("#ff4d67")));
        liveBadgeLabel.setText("TOCANDO AGORA");
        liveBadgeLabel.setStyle("-fx-font-size:10px;-fx-font-weight:800;-fx-text-fill:#ff8fa1;");
    }

    private void applyBadgePaused() {
        liveBadge.setStyle(BADGE_STYLE_BASE +
            "-fx-background-color:rgba(251,191,36,0.08);" +
            "-fx-border-color:rgba(251,191,36,0.30);");
        liveDot.setFill(Color.web("#fbbf24"));
        liveDot.setEffect(new javafx.scene.effect.DropShadow(6, Color.web("#fbbf24")));
        liveBadgeLabel.setText("PAUSADO");
        liveBadgeLabel.setStyle("-fx-font-size:10px;-fx-font-weight:800;-fx-text-fill:#fbbf24;");
    }

    private void applyPlayStyle() {
        stopBtn.setText("▶  NO AR");
        stopBtn.setStyle(BTN_PRIMARY_PLAY);
    }

    private void applyStopStyle() {
        stopBtn.setText("■  Stop");
        stopBtn.setStyle(BTN_PRIMARY_STOP);
    }
}
