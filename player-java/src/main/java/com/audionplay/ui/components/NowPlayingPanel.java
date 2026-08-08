package com.audionplay.ui.components;

import com.audionplay.domain.PlaybackState;
import com.audionplay.ui.Theme;
import com.audionplay.ui.WaveformView;
import javafx.geometry.Insets;
import javafx.geometry.Pos;
import javafx.scene.control.Button;
import javafx.scene.control.Label;
import javafx.scene.control.ProgressBar;
import javafx.scene.layout.*;

import java.util.function.Consumer;

import static com.audionplay.ui.Theme.*;

/**
 * Painel central "Tocando Agora".
 *
 * Expõe setters de estado (chamados pelos callbacks do PlayerController)
 * e setters de ação (chamados uma vez durante o setup pelo MainWindow).
 */
public class NowPlayingPanel extends VBox {

    private final Label trackTitleLabel;
    private final Label trackArtistLabel;
    private final Label currentTimeLabel;
    private final Label remainingTimeLabel;
    private final Label totalTimeLabel;
    private final Label statusLabel;
    private final ProgressBar progressBar;
    private final WaveformView waveformView;
    private final Button pauseBtn;
    private final Button stopBtn;
    private final Button nextBtn;

    private Runnable onPlayQueue;
    private Runnable onStopAction;
    private boolean  isPlaying = false;

    public NowPlayingPanel() {
        super(10);
        setStyle(
            "-fx-background-color:" + BG_PANEL + ";" +
            "-fx-background-radius:10;" +
            "-fx-border-color:" + BORDER + ";" +
            "-fx-border-radius:10;-fx-border-width:1;"
        );
        setPadding(new Insets(16));

        // ── Inicialização dos campos ──────────────────────────────────────────
        trackTitleLabel = Theme.lbl("Nenhuma faixa carregada",
            "-fx-font-size:22px;-fx-font-weight:bold;-fx-text-fill:white;");
        trackArtistLabel = Theme.lbl("—",
            "-fx-font-size:13px;-fx-text-fill:" + TEXT_SEC + ";");
        currentTimeLabel = Theme.lbl("00:00",
            "-fx-font-size:11px;-fx-text-fill:" + TEXT_SEC + ";");
        remainingTimeLabel = Theme.lbl("00:00",
            "-fx-font-size:28px;-fx-font-weight:bold;-fx-text-fill:white;-fx-font-family:'Courier New';");
        totalTimeLabel = Theme.lbl("00:00",
            "-fx-font-size:11px;-fx-text-fill:" + TEXT_SEC + ";");
        statusLabel = Theme.lbl("",
            "-fx-font-size:10px;-fx-text-fill:" + TEXT_SEC + ";");

        progressBar = new ProgressBar(0.0);
        progressBar.setPrefHeight(5);
        progressBar.setMaxWidth(Double.MAX_VALUE);
        progressBar.setStyle("-fx-accent:" + BLUE + ";-fx-control-inner-background:#2A2E42;");

        waveformView = new WaveformView();
        waveformView.setPrefHeight(64);
        waveformView.setMaxWidth(Double.MAX_VALUE);
        waveformView.setStyle("-fx-background-color:#0E1117;-fx-background-radius:6;");

        pauseBtn = primaryBtn("⏸  Pausar", "#252A40", true);
        stopBtn  = new Button("▶  Play");
        applyPlayStyle();
        stopBtn.setDisable(true);   // habilitado quando houver item na fila
        HBox.setHgrow(stopBtn, Priority.ALWAYS);
        stopBtn.setMaxWidth(Double.MAX_VALUE);
        stopBtn.setOnAction(e -> {
            if (isPlaying) { if (onStopAction  != null) onStopAction.run(); }
            else           { if (onPlayQueue   != null) onPlayQueue.run();  }
        });
        nextBtn = primaryBtn("Próximo  ⏭", "#252A40", true);
        nextBtn.setDisable(true);   // habilitado quando houver próximo item

        HBox controls = new HBox(10, pauseBtn, stopBtn, nextBtn);
        controls.setAlignment(Pos.CENTER);

        // ── Montagem do layout ────────────────────────────────────────────────
        getChildren().addAll(
            buildHeader(),
            trackTitleLabel,
            trackArtistLabel,
            statusLabel,
            buildMeta(),
            waveformView,
            progressBar,
            buildTimes(),
            controls
        );
    }

    // ── API de estado — chamada pelos callbacks do controller ─────────────────

    public void setTrack(String title, String artist, String totalTime) {
        trackTitleLabel.setText(title);
        trackArtistLabel.setText(artist);
        totalTimeLabel.setText(totalTime);
    }

    public void setProgress(double frac) {
        progressBar.setProgress(frac);
        waveformView.setProgress(frac);
    }

    public void setCurrentTime(String t)         { currentTimeLabel.setText(t); }
    public void setRemainingTime(String t)        { remainingTimeLabel.setText(t); }
    public void setWaveformPeaks(double[] peaks)  { waveformView.setPeaks(peaks); }
    public void clearWaveform()                   { waveformView.clear(); }
    public void setStatus(String msg)             { statusLabel.setText(msg); }

    public void resetProgress() {
        progressBar.setProgress(0);
        waveformView.setProgress(0);
    }

    public void applyState(PlaybackState state) {
        switch (state) {
            case PLAYING -> {
                pauseBtn.setText("⏸  Pausar");
                isPlaying = true;
                applyStopStyle();
            }
            case PAUSED -> pauseBtn.setText("▶  Retomar");
            case STOPPED -> {
                pauseBtn.setText("⏸  Pausar");
                isPlaying = false;
                applyPlayStyle();
                stopBtn.setDisable(false); // re-habilita (updateControls ajusta depois)
                resetProgress();
            }
        }
        statusLabel.setText("");
    }

    public void setCrossfadeInProgress(boolean inProgress) {
        nextBtn.setDisable(inProgress);
        nextBtn.setText(inProgress ? "⏭  Crossfade..." : "Próximo  ⏭");
    }

    // ── API de eventos — chamada uma vez no setup ─────────────────────────────

    public void setOnOpenFile(Runnable h)               { /* botão removido */ }
    public void setOnPause(Runnable h)                  { pauseBtn.setOnAction(e -> h.run()); }
    public void setOnStop(Runnable h)                   { this.onStopAction = h; }
    public void setOnPlayQueue(Runnable h)              { this.onPlayQueue = h; }
    public void setOnNext(Runnable h)                   { nextBtn.setOnAction(e -> h.run()); }
    public void setOnCrossfade(Consumer<Button> h)      { /* substituído por setOnNext */ }
    public void setOnSeek(Consumer<Double> h)           { waveformView.setOnSeek(h); }

    public void setPlayEnabled(boolean enabled)         { stopBtn.setDisable(isPlaying ? false : !enabled); }
    public void setNextEnabled(boolean enabled)         { nextBtn.setDisable(!enabled); }

    // ── Layout interno ────────────────────────────────────────────────────────

    private HBox buildHeader() {
        Label badge = Theme.lbl("● TOCANDO AGORA",
            "-fx-font-size:11px;-fx-font-weight:bold;-fx-text-fill:#FF4A4A;" +
            "-fx-background-color:#280A0A;-fx-background-radius:4;" +
            "-fx-border-color:#4A1010;-fx-border-radius:4;-fx-border-width:1;" +
            "-fx-padding:4 10 4 10;");
        Label auto = Theme.lbl("AUTOMÁTICO",
            "-fx-font-size:11px;-fx-font-weight:bold;-fx-text-fill:" + GREEN + ";" +
            "-fx-border-color:" + GREEN + ";-fx-border-radius:4;-fx-border-width:1;" +
            "-fx-padding:4 10 4 10;");
        HBox hdr = new HBox();
        hdr.setAlignment(Pos.CENTER_LEFT);
        hdr.getChildren().addAll(badge, Theme.hSpacer(), auto);
        return hdr;
    }

    private HBox buildMeta() {
        HBox row = new HBox(20);
        row.setPadding(new Insets(8, 12, 8, 12));
        row.setStyle("-fx-background-color:#14172280;-fx-background-radius:6;");
        String[] labels = {"INÍCIO", "CUE", "INTRO", "OUTRO", "CUE OUT", "GAIN", "ID"};
        String[] values = {"—", "—", "—", "—", "—", "—", "—"};
        for (int i = 0; i < labels.length; i++) {
            row.getChildren().add(new VBox(2,
                Theme.lbl(labels[i], "-fx-font-size:9px;-fx-text-fill:" + TEXT_MUT + ";"),
                Theme.lbl(values[i], "-fx-font-size:11px;-fx-text-fill:" + TEXT_PRI + ";")
            ));
        }
        return row;
    }

    private HBox buildTimes() {
        Region t1 = new Region(); HBox.setHgrow(t1, Priority.ALWAYS);
        Region t2 = new Region(); HBox.setHgrow(t2, Priority.ALWAYS);
        HBox times = new HBox();
        times.setAlignment(Pos.CENTER);
        times.getChildren().addAll(currentTimeLabel, t1, remainingTimeLabel, t2, totalTimeLabel);
        return times;
    }

    private void applyPlayStyle() {
        stopBtn.setText("▶  Play");
        stopBtn.setStyle(
            "-fx-background-color:" + GREEN + ";-fx-text-fill:white;-fx-font-size:13px;" +
            "-fx-font-weight:bold;-fx-padding:12 0 12 0;-fx-background-radius:8;-fx-cursor:hand;"
        );
    }

    private void applyStopStyle() {
        stopBtn.setText("■  Stop");
        stopBtn.setStyle(
            "-fx-background-color:#B22020;-fx-text-fill:white;-fx-font-size:13px;" +
            "-fx-font-weight:bold;-fx-padding:12 0 12 0;-fx-background-radius:8;-fx-cursor:hand;"
        );
    }

    private Button primaryBtn(String text, String bgColor, boolean grows) {
        Button b = new Button(text);
        b.setStyle(
            "-fx-background-color:" + bgColor + ";-fx-text-fill:white;" +
            "-fx-font-size:13px;-fx-font-weight:bold;" +
            "-fx-padding:12 0 12 0;-fx-background-radius:8;" +
            "-fx-border-color:" + BORDER + ";-fx-border-radius:8;-fx-border-width:1;" +
            "-fx-cursor:hand;"
        );
        if (grows) {
            HBox.setHgrow(b, Priority.ALWAYS);
            b.setMaxWidth(Double.MAX_VALUE);
        }
        return b;
    }
}
