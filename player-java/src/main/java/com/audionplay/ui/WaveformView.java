package com.audionplay.ui;

import javafx.scene.canvas.Canvas;
import javafx.scene.canvas.GraphicsContext;
import javafx.scene.layout.Pane;
import javafx.scene.paint.Color;
import javafx.scene.text.Font;
import javafx.scene.text.FontWeight;

import java.util.ArrayList;
import java.util.Comparator;
import java.util.List;
import java.util.function.Consumer;

/**
 * Componente JavaFX que exibe a waveform de uma faixa.
 *
 * Responsabilidades:
 *  - Desenhar os picos de amplitude como barras verticais
 *  - Colorir a porção já reproduzida de forma diferente
 *  - Exibir o playhead (cursor de posição atual)
 *  - Exibir marcadores de CUE IN, INTRO, OUTRO e CUE OUT
 *  - Notificar cliques para seeking via onSeek callback
 */
public class WaveformView extends Pane {

    private static final Color COLOR_PLAYED   = Color.web("#4A6CF7");
    private static final Color COLOR_UNPLAYED = Color.web("#2E3450");
    private static final Color COLOR_PLAYHEAD = Color.WHITE;
    private static final Color COLOR_BG       = Color.web("#0E1117");

    // Cores dos marcadores — iguais ao player.html
    private static final Color COL_CUE_IN  = Color.web("#00d4ff", 0.85);
    private static final Color COL_INTRO   = Color.web("#36d399", 0.85);
    private static final Color COL_OUTRO   = Color.web("#ffbf4b", 0.85);
    private static final Color COL_CUE_OUT = Color.web("#f87272", 0.85);

    private static final double BADGE_H   = 13;
    private static final double BADGE_PAD = 3;
    private static final double BADGE_GAP = 2;  // espaço vertical entre rows

    private final Canvas canvas = new Canvas();

    private double[] peaks    = new double[0];
    private double   progress = 0.0; // 0.0 – 1.0

    // Marcadores — frações [0,1] do arquivo completo; null = não definido
    private Double cueInFrac  = null;
    private Double introFrac  = null;
    private Double outroFrac  = null;
    private Double cueOutFrac = null;

    private Consumer<Double> onSeek;

    public WaveformView() {
        canvas.widthProperty().bind(widthProperty());
        canvas.heightProperty().bind(heightProperty());
        getChildren().add(canvas);

        widthProperty().addListener((o, ov, nv) -> redraw());
        heightProperty().addListener((o, ov, nv) -> redraw());

        canvas.setOnMouseClicked(e -> {
            if (onSeek != null && canvas.getWidth() > 0)
                onSeek.accept(e.getX() / canvas.getWidth());
        });
    }

    // ── API pública ───────────────────────────────────────────────────────────

    public void setPeaks(double[] peaks) {
        this.peaks = peaks != null ? peaks : new double[0];
        redraw();
    }

    public void setProgress(double progress) {
        this.progress = Math.max(0.0, Math.min(1.0, progress));
        redraw();
    }

    /**
     * Define os marcadores de CUE points para exibição no waveform.
     * Qualquer parâmetro null é ignorado (marcador não desenhado).
     *
     * @param cueInMs   CUE IN em ms  (ou null)
     * @param introMs   INTRO em ms   (ou null)
     * @param outroMs   OUTRO em ms   (ou null)
     * @param cueOutMs  CUE OUT em ms (ou null)
     * @param durationMs duração total do arquivo em ms
     */
    public void setCueMarkers(Integer cueInMs, Integer introMs,
                              Integer outroMs, Integer cueOutMs,
                              int durationMs) {
        if (durationMs <= 0) { clearMarkers(); return; }
        cueInFrac  = cueInMs  != null ? (double) cueInMs  / durationMs : null;
        introFrac  = introMs  != null ? (double) introMs  / durationMs : null;
        outroFrac  = outroMs  != null ? (double) outroMs  / durationMs : null;
        cueOutFrac = cueOutMs != null ? (double) cueOutMs / durationMs : null;
        redraw();
    }

    public void clearMarkers() {
        cueInFrac = introFrac = outroFrac = cueOutFrac = null;
        redraw();
    }

    public void setOnSeek(Consumer<Double> onSeek) { this.onSeek = onSeek; }

    public void clear() {
        peaks    = new double[0];
        progress = 0.0;
        clearMarkers();
    }

    // ── Desenho ──────────────────────────────────────────────────────────────

    private void redraw() {
        double w = canvas.getWidth();
        double h = canvas.getHeight();
        if (w <= 0 || h <= 0) return;

        GraphicsContext gc = canvas.getGraphicsContext2D();

        gc.setFill(COLOR_BG);
        gc.fillRect(0, 0, w, h);

        if (peaks.length == 0) {
            // Flat line quando sem waveform
            gc.setStroke(Color.web("#ffffff26"));
            gc.setLineWidth(1);
            gc.strokeLine(0, h / 2, w, h / 2);
        } else {
            double cy    = h / 2.0;
            double barW  = Math.max(1.0, (w / peaks.length) - 1.0);
            double playX = w * progress;

            for (int i = 0; i < peaks.length; i++) {
                double x  = i * (w / peaks.length);
                double bh = Math.max(2.0, peaks[i] * h * 0.60);
                gc.setFill(x < playX ? COLOR_PLAYED : COLOR_UNPLAYED);
                gc.fillRect(x, cy - bh / 2.0, barW, bh);
            }

            // Playhead
            gc.setFill(COLOR_PLAYHEAD);
            gc.fillRect(playX - 1, 0, 2, h);
        }

        // Marcadores CUE
        drawCueMarkers(gc, w, h);
    }

    private void drawCueMarkers(GraphicsContext gc, double w, double h) {
        // Coleta marcadores definidos, ordenados por posição
        record Marker(double frac, Color color, String label) {}
        List<Marker> markers = new ArrayList<>();
        if (cueInFrac  != null) markers.add(new Marker(cueInFrac,  COL_CUE_IN,  "CUE-IN"));
        if (introFrac  != null) markers.add(new Marker(introFrac,  COL_INTRO,   "INTRO"));
        if (outroFrac  != null) markers.add(new Marker(outroFrac,  COL_OUTRO,   "OUTRO"));
        if (cueOutFrac != null) markers.add(new Marker(cueOutFrac, COL_CUE_OUT, "CUE-OUT"));
        if (markers.isEmpty()) return;

        markers.sort(Comparator.comparingDouble(Marker::frac));

        gc.setFont(Font.font("SansSerif", FontWeight.BOLD, 9));

        // Atribui rows para evitar sobreposição de labels (greedy, como no player.html)
        double[] rowEnds = {0, 0, 0};
        int[]    rows    = new int[markers.size()];
        for (int i = 0; i < markers.size(); i++) {
            double x  = markers.get(i).frac() * w;
            double tw = estimateTextWidth(markers.get(i).label(), 9);
            double bw = tw + BADGE_PAD * 2;
            double bx = Math.max(0, Math.min(x - bw / 2.0, w - bw));
            int row = -1;
            for (int r = 0; r < rowEnds.length; r++) {
                if (bx >= rowEnds[r]) { row = r; break; }
            }
            if (row == -1) row = 0;
            rowEnds[row] = bx + bw + BADGE_GAP;
            rows[i] = row;
        }

        // Desenha cada marcador
        for (int i = 0; i < markers.size(); i++) {
            Marker m  = markers.get(i);
            double x  = m.frac() * w;
            int    row = rows[i];

            // Linha vertical
            gc.setStroke(m.color());
            gc.setLineWidth(1.5);
            gc.setLineDashes();
            gc.strokeLine(x, 0, x, h);

            // Badge label
            double tw = estimateTextWidth(m.label(), 9);
            double bw = tw + BADGE_PAD * 2;
            double bx = Math.max(0, Math.min(x - bw / 2.0, w - bw));
            double by = 1 + row * (BADGE_H + BADGE_GAP);

            gc.setFill(m.color());
            gc.fillRoundRect(bx, by, bw, BADGE_H, 3, 3);
            gc.setFill(Color.BLACK);
            gc.fillText(m.label(), bx + BADGE_PAD, by + BADGE_H - 3.5);
        }
    }

    /** Estimativa de largura de texto (evita chamar measureText no canvas). */
    private static double estimateTextWidth(String text, double fontSize) {
        return text.length() * fontSize * 0.6;
    }
}
