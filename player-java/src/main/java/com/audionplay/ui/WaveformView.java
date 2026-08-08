package com.audionplay.ui;

import javafx.scene.canvas.Canvas;
import javafx.scene.canvas.GraphicsContext;
import javafx.scene.layout.Pane;
import javafx.scene.paint.Color;

import java.util.function.Consumer;

/**
 * Componente JavaFX que exibe a waveform de uma faixa.
 *
 * Responsabilidades:
 *  - Desenhar os picos de amplitude como barras verticais
 *  - Colorir a porção já reproduzida de forma diferente
 *  - Exibir o playhead (cursor de posição atual)
 *  - Notificar cliques para seeking via onSeek callback
 *
 * Não conhece nada sobre áudio — só recebe peaks[] e progress.
 */
public class WaveformView extends Pane {

    private static final Color COLOR_PLAYED   = Color.web("#4A6CF7");
    private static final Color COLOR_UNPLAYED = Color.web("#2E3450");
    private static final Color COLOR_PLAYHEAD = Color.WHITE;
    private static final Color COLOR_CUE      = Color.web("#F0C04066");
    private static final Color COLOR_BG       = Color.web("#0E1117");

    private final Canvas canvas = new Canvas();

    private double[] peaks    = new double[0];
    private double   progress = 0.0; // 0.0 – 1.0

    private Consumer<Double> onSeek; // recebe fração 0.0–1.0

    public WaveformView() {
        canvas.widthProperty().bind(widthProperty());
        canvas.heightProperty().bind(heightProperty());
        getChildren().add(canvas);

        widthProperty().addListener((o, ov, nv) -> redraw());
        heightProperty().addListener((o, ov, nv) -> redraw());

        // Clique para seek
        canvas.setOnMouseClicked(e -> {
            if (onSeek != null && canvas.getWidth() > 0) {
                onSeek.accept(e.getX() / canvas.getWidth());
            }
        });
    }

    /** Define os picos analisados e redesenha. */
    public void setPeaks(double[] peaks) {
        this.peaks = peaks != null ? peaks : new double[0];
        redraw();
    }

    /** Atualiza o progresso (0.0–1.0) e redesenha o playhead. */
    public void setProgress(double progress) {
        this.progress = Math.max(0.0, Math.min(1.0, progress));
        redraw();
    }

    public void setOnSeek(Consumer<Double> onSeek) {
        this.onSeek = onSeek;
    }

    /** Limpa tudo (ex: quando nenhuma faixa está carregada). */
    public void clear() {
        this.peaks    = new double[0];
        this.progress = 0.0;
        redraw();
    }

    // ── Desenho ──────────────────────────────────────────────────────────────

    private void redraw() {
        double w = canvas.getWidth();
        double h = canvas.getHeight();
        if (w <= 0 || h <= 0) return;

        GraphicsContext gc = canvas.getGraphicsContext2D();

        // Fundo
        gc.setFill(COLOR_BG);
        gc.fillRect(0, 0, w, h);

        if (peaks.length == 0) return;

        double cy    = h / 2.0;
        double barW  = Math.max(1.0, (w / peaks.length) - 1.0);
        double playX = w * progress;

        for (int i = 0; i < peaks.length; i++) {
            double x  = i * (w / peaks.length);
            double bh = Math.max(2.0, peaks[i] * h * 0.88);

            gc.setFill(x < playX ? COLOR_PLAYED : COLOR_UNPLAYED);
            gc.fillRect(x, cy - bh / 2.0, barW, bh);
        }

        // Área CUE (primeiros 2% após o playhead — ilustrativo)
        double cueEnd = Math.min(w, playX + w * 0.02);
        gc.setFill(COLOR_CUE);
        gc.fillRect(playX, 0, cueEnd - playX, h);

        // Playhead
        gc.setFill(COLOR_PLAYHEAD);
        gc.fillRect(playX - 1, 0, 2, h);
    }
}
