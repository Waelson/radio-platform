package com.audionplay.ui.components;

import com.audionplay.ui.Theme;
import javafx.animation.KeyFrame;
import javafx.animation.Timeline;
import javafx.geometry.Pos;
import javafx.scene.control.Label;
import javafx.scene.control.ProgressBar;
import javafx.scene.layout.*;
import javafx.util.Duration;

import static com.audionplay.ui.Theme.*;

/**
 * Painel VU Meter com canais L, R, P, M e decaimento suave.
 */
public class VuMeterPanel extends VBox {

    private final ProgressBar vuL, vuR, vuP, vuM;
    private final Label vuLVal, vuRVal, vuPVal, vuMVal;

    private volatile float levelL = 0f;
    private volatile float levelR = 0f;
    private Timeline decayTimeline;

    public VuMeterPanel() {
        super(10);
        setStyle(
            "-fx-background-color:#14172280;" +
            "-fx-background-radius:8;-fx-padding:12;" +
            "-fx-border-color:" + BORDER + ";" +
            "-fx-border-radius:8;-fx-border-width:1;"
        );

        HBox hdr = new HBox();
        hdr.setAlignment(Pos.CENTER_LEFT);
        hdr.getChildren().addAll(
            Theme.lbl("PROGRAMA / PFL", "-fx-font-size:10px;-fx-font-weight:bold;-fx-text-fill:" + TEXT_SEC + ";"),
            Theme.hSpacer(),
            Theme.lbl("LUFS -29.4",     "-fx-font-size:10px;-fx-text-fill:" + TEXT_SEC + ";")
        );

        vuL = vuBar(); vuR = vuBar(); vuP = vuBar(); vuM = vuBar();
        vuLVal = vuVal("---"); vuRVal = vuVal("---");
        vuPVal = vuVal("---"); vuMVal = vuVal("---");

        GridPane grid = new GridPane();
        grid.setHgap(6); grid.setVgap(8);
        grid.addRow(0, vuLbl("L"), vuL, vuLVal, vuLbl("R"), vuR, vuRVal);
        grid.addRow(1, vuLbl("P"), vuP, vuPVal, vuLbl("M"), vuM, vuMVal);

        ColumnConstraints c0 = new ColumnConstraints(14);
        ColumnConstraints c1 = new ColumnConstraints(); c1.setHgrow(Priority.ALWAYS);
        ColumnConstraints c2 = new ColumnConstraints(38);
        grid.getColumnConstraints().addAll(c0, c1, c2, c0, c1, c2);

        getChildren().addAll(hdr, grid);
    }

    // ── API pública ───────────────────────────────────────────────────────────

    /** Atualiza os medidores com os níveis RMS recebidos do engine. */
    public void applyLevels(float[] levels) {
        levelL = levels[0];
        levelR = levels[1];
        float p = (levelL + levelR) / 2f;
        float m = (float) Math.sqrt(levelL * levelR + 0.0001f);

        vuL.setProgress(levelL); vuR.setProgress(levelR);
        vuP.setProgress(p);      vuM.setProgress(m);
        vuLVal.setText(toDb(levelL)); vuRVal.setText(toDb(levelR));
        vuPVal.setText(toDb(p));      vuMVal.setText(toDb(m));
    }

    /** Inicia o decaimento suave dos VUs até zero (chamado ao pausar/parar). */
    public void startDecay() {
        if (decayTimeline != null) decayTimeline.stop();
        decayTimeline = new Timeline(new KeyFrame(Duration.millis(50), e -> {
            levelL = Math.max(0f, levelL * 0.80f);
            levelR = Math.max(0f, levelR * 0.80f);
            float p = (levelL + levelR) / 2f;
            float m = (float) Math.sqrt(levelL * levelR + 0.0001f);

            vuL.setProgress(levelL); vuR.setProgress(levelR);
            vuP.setProgress(p);      vuM.setProgress(m);

            if (levelL < 0.001f && levelR < 0.001f) {
                decayTimeline.stop();
                vuLVal.setText("---"); vuRVal.setText("---");
                vuPVal.setText("---"); vuMVal.setText("---");
            } else {
                vuLVal.setText(toDb(levelL)); vuRVal.setText(toDb(levelR));
                vuPVal.setText(toDb(p));      vuMVal.setText(toDb(m));
            }
        }));
        decayTimeline.setCycleCount(Timeline.INDEFINITE);
        decayTimeline.play();
    }

    // ── Helpers internos ──────────────────────────────────────────────────────

    private ProgressBar vuBar() {
        ProgressBar b = new ProgressBar(0);
        b.setPrefHeight(15);
        b.setMaxWidth(Double.MAX_VALUE);
        b.setStyle("-fx-accent:" + GREEN + ";-fx-control-inner-background:#2A2E42;");
        return b;
    }

    private Label vuLbl(String t) {
        return Theme.lbl(t, "-fx-font-size:10px;-fx-font-weight:bold;-fx-text-fill:" + TEXT_SEC + ";");
    }

    private Label vuVal(String t) {
        return Theme.lbl(t, "-fx-font-size:10px;-fx-text-fill:" + TEXT_PRI + ";");
    }

    private static String toDb(float rms) {
        if (rms < 0.0001f) return "---";
        double db = 20.0 * Math.log10(rms);
        return String.format(java.util.Locale.US, "%.1f", db);
    }
}
