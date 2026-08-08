package com.audionplay.ui.components;

import com.audionplay.ui.Theme;
import javafx.animation.KeyFrame;
import javafx.animation.Timeline;
import javafx.geometry.Insets;
import javafx.geometry.Pos;
import javafx.scene.control.Label;
import javafx.scene.layout.*;
import javafx.util.Duration;

import static com.audionplay.ui.Theme.*;

/**
 * Painel VU Meter — visual fiel ao or-card "Programa / PFL" do player.html.
 */
public class VuMeterPanel extends VBox {

    private static final String[] CH = {"L", "R", "P", "M"};

    private final Label   lufsBadge;
    private final Region[]    fills     = new Region[4];
    private final Label[]     valLabels = new Label[4];
    private final StackPane[] barWraps  = new StackPane[4];

    private volatile float levelL = 0f;
    private volatile float levelR = 0f;
    private Timeline decayTimeline;

    public VuMeterPanel() {
        super(0);
        setStyle(OR_CARD);

        // ── LUFS badge ────────────────────────────────────────────────────────
        lufsBadge = new Label("LUFS —");
        lufsBadge.setStyle(
            "-fx-pref-height:27;" +
            "-fx-background-color:#0e1b28;" +
            "-fx-border-color:#20384c;-fx-border-width:1;" +
            "-fx-border-radius:7;-fx-background-radius:7;" +
            "-fx-padding:0 8 0 8;" +
            "-fx-font-size:10px;-fx-font-weight:600;-fx-text-fill:#88a0b5;"
        );

        HBox hdr = Theme.orCardHeader("Programa / PFL", lufsBadge);

        // ── Meters grid (2 colunas) ───────────────────────────────────────────
        GridPane grid = new GridPane();
        grid.setPadding(new Insets(11));
        grid.setHgap(9);
        grid.setVgap(9);

        ColumnConstraints cc1 = new ColumnConstraints();
        cc1.setHgrow(Priority.ALWAYS);
        ColumnConstraints cc2 = new ColumnConstraints();
        cc2.setHgrow(Priority.ALWAYS);
        grid.getColumnConstraints().addAll(cc1, cc2);

        for (int i = 0; i < 4; i++) {
            // ch label
            Label ch = new Label(CH[i]);
            ch.setStyle("-fx-font-size:9px;-fx-font-weight:600;-fx-text-fill:#88a0b5;");
            ch.setMinWidth(15);

            // fill region
            fills[i] = new Region();
            fills[i].setPrefHeight(10);
            fills[i].setPrefWidth(0);
            fills[i].setStyle(
                "-fx-background-color:linear-gradient(to right," +
                    "#36d399 0%,#36d399 68%," +
                    "#ffbf4b 68%,#ffbf4b 88%," +
                    "#f87272 88%,#f87272 100%);" +
                "-fx-background-radius:99;"
            );

            // bar wrap
            barWraps[i] = new StackPane(fills[i]);
            barWraps[i].setPrefHeight(10);
            barWraps[i].setMaxHeight(10);
            barWraps[i].setAlignment(Pos.CENTER_LEFT);
            barWraps[i].setStyle(
                "-fx-background-color:#061018;" +
                "-fx-background-radius:99;" +
                "-fx-border-color:rgba(49,83,109,0.5);-fx-border-width:1;-fx-border-radius:99;"
            );
            HBox.setHgrow(barWraps[i], Priority.ALWAYS);

            // val label
            valLabels[i] = new Label("—");
            valLabels[i].setStyle("-fx-font-size:9px;-fx-text-fill:#88a0b5;");
            valLabels[i].setMinWidth(35);
            valLabels[i].setAlignment(Pos.CENTER_RIGHT);

            HBox row = new HBox(6, ch, barWraps[i], valLabels[i]);
            row.setAlignment(Pos.CENTER_LEFT);

            grid.add(row, i % 2, i / 2);
        }

        // ── Scale row (-60 -30 -18 -9 -3 0) ──────────────────────────────────
        String[] ticks = {"-60", "-30", "-18", "-9", "-3", "0"};
        HBox scale = new HBox();
        scale.setPadding(new Insets(0, 0, 0, 21));
        for (int i = 0; i < ticks.length; i++) {
            Label lbl = new Label(ticks[i]);
            lbl.setStyle("-fx-font-size:8px;-fx-text-fill:#4a6478;");
            scale.getChildren().add(lbl);
            if (i < ticks.length - 1) {
                Region sp = new Region();
                HBox.setHgrow(sp, Priority.ALWAYS);
                scale.getChildren().add(sp);
            }
        }
        grid.add(scale, 0, 2);
        GridPane.setColumnSpan(scale, 2);

        getChildren().addAll(hdr, grid);
    }

    // ── API pública ───────────────────────────────────────────────────────────

    public void applyLevels(float[] levels) {
        levelL = levels[0];
        levelR = levels[1];
        float p = (levelL + levelR) / 2f;
        float m = (float) Math.sqrt(Math.max(0f, levelL * levelR));
        float[] all = {levelL, levelR, p, m};
        for (int i = 0; i < 4; i++) setFill(i, all[i]);
    }

    public void startDecay() {
        if (decayTimeline != null) decayTimeline.stop();
        decayTimeline = new Timeline(new KeyFrame(Duration.millis(50), e -> {
            levelL = Math.max(0f, levelL * 0.80f);
            levelR = Math.max(0f, levelR * 0.80f);
            float p = (levelL + levelR) / 2f;
            float m = (float) Math.sqrt(Math.max(0f, levelL * levelR));
            float[] all = {levelL, levelR, p, m};
            for (int i = 0; i < 4; i++) setFill(i, all[i]);
            if (levelL < 0.001f && levelR < 0.001f) {
                decayTimeline.stop();
                for (Label l : valLabels) l.setText("—");
                for (Region f : fills) f.setPrefWidth(0);
            }
        }));
        decayTimeline.setCycleCount(Timeline.INDEFINITE);
        decayTimeline.play();
    }

    // ── Helpers ───────────────────────────────────────────────────────────────

    private void setFill(int idx, float level) {
        double w = barWraps[idx].getWidth();
        if (w > 0) fills[idx].setPrefWidth(level * w);
        valLabels[idx].setText(toDb(level));
    }

    private static String toDb(float rms) {
        if (rms < 0.0001f) return "—";
        double db = 20.0 * Math.log10(rms);
        return String.format(java.util.Locale.US, "%.1f", db);
    }
}
