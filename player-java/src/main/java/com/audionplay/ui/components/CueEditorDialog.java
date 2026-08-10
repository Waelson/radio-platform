package com.audionplay.ui.components;

import com.audionplay.audio.ffmpeg.FfmpegAudioPlayer;
import com.audionplay.audio.ffmpeg.FfmpegWaveformAnalyzer;
import com.audionplay.db.entity.TrackEntity;
import com.audionplay.db.repository.TrackRepository;
import com.audionplay.domain.PlaybackState;
import com.audionplay.domain.Track;
import javafx.application.Platform;
import javafx.geometry.Insets;
import javafx.geometry.Pos;
import javafx.scene.Scene;
import javafx.scene.canvas.Canvas;
import javafx.scene.canvas.GraphicsContext;
import javafx.scene.control.Button;
import javafx.scene.control.Label;
import javafx.scene.input.KeyCode;
import javafx.scene.layout.*;
import javafx.scene.paint.Color;
import javafx.stage.Modality;
import javafx.stage.Stage;
import javafx.stage.StageStyle;
import javafx.stage.Window;

import java.sql.SQLException;

/**
 * Modal de edição de CUE Points — fiel ao cue-editor-overlay do player.html.
 *
 * Funcionalidades:
 *  - Waveform interativa: click = seek, duplo-click = definir pin (ponto de captura)
 *  - Zoom na waveform (botões − / + e scroll do mouse)
 *  - 4 marcadores: CUE IN (azul), INTRO (verde), OUTRO (laranja), CUE OUT (vermelho)
 *  - Capturar: usa posição do pin (se definido) ou posição atual de playback
 *  - Botões "TOCAR DO CUE IN / INTRO / OUTRO"
 *  - Salvar: persiste no SQLite via TrackRepository.updateCuePoints()
 */
public class CueEditorDialog extends Stage {

    // ── Paleta ────────────────────────────────────────────────────────────────
    private static final Color BG_OVERLAY  = Color.rgb(0, 0, 0, 0.72);
    private static final Color BG_MODAL    = Color.web("#091520");
    private static final Color BG_TIMELINE = Color.web("#061218");
    private static final Color BG_PANEL    = Color.web("#0d1e2c");
    private static final String BORDER_MODAL = "rgba(45,216,255,0.35)";

    private static final Color COL_CUE_IN  = Color.web("#4fc3f7");
    private static final Color COL_INTRO   = Color.web("#81c784");
    private static final Color COL_OUTRO   = Color.web("#ffb74d");
    private static final Color COL_CUE_OUT = Color.web("#ef5350");
    private static final Color COL_PLAYHEAD = Color.WHITE;
    private static final Color COL_PIN      = Color.web("#f5c542");
    private static final Color COL_WAVEFORM = Color.web("#4fc3f7", 0.80);
    private static final Color COL_WAVE_DIM = Color.web("#0e2a38");

    private static final double PAD_PX = 14; // padding horizontal no canvas

    // ── Estado ────────────────────────────────────────────────────────────────
    private final TrackEntity        track;
    private final Runnable           onSaved;
    private final FfmpegAudioPlayer  player   = new FfmpegAudioPlayer();
    private final TrackRepository    repo     = new TrackRepository();

    private double[] peaks      = new double[0];
    private double   durationMs = 0;
    private double   playPosMs  = 0;

    private Double cueInMs  = null;
    private Double introMs  = null;
    private Double outroMs  = null;
    private Double cueOutMs = null;
    private Double pinMs    = null;

    // ── Drag-to-zoom (rubber-band selection) ──────────────────────────────────
    private boolean selActive    = false;
    private double  selStartFrac = 0;
    private double  selEndFrac   = 0;
    private double  selStartPx   = 0;

    // ── Pan (Alt + drag) ──────────────────────────────────────────────────────
    private boolean panActive      = false;
    private double  panStartX      = 0;
    private double  panStartOffset = 0;

    private double zoom   = 1.0;
    private double offset = 0.0; // start of visible window as fraction [0, 1-1/zoom]

    // ── UI refs ───────────────────────────────────────────────────────────────
    private Canvas canvas;
    private Pane   canvasPane;
    private Label  lblPos, lblDur, lblZoom;
    private Label  lblCueIn, lblIntro, lblOutro, lblCueOut;
    private Label  lblError;

    // ── Construtor ────────────────────────────────────────────────────────────

    public CueEditorDialog(Window owner, TrackEntity track, Runnable onSaved) {
        this.track   = track;
        this.onSaved = onSaved;

        // Valores iniciais dos marcadores
        this.durationMs = track.durationMs();
        this.cueInMs  = track.cueInMs()  != null ? (double) track.cueInMs()  : null;
        this.introMs  = track.introMs()  != null ? (double) track.introMs()  : null;
        this.outroMs  = track.outroMs()  != null ? (double) track.outroMs()  : null;
        this.cueOutMs = track.cueOutMs() != null ? (double) track.cueOutMs() : null;

        initModality(Modality.APPLICATION_MODAL);
        initOwner(owner);
        initStyle(StageStyle.TRANSPARENT);

        StackPane root = new StackPane();
        root.setBackground(Background.fill(BG_OVERLAY));

        VBox modal = buildModal();
        StackPane.setAlignment(modal, Pos.CENTER);
        root.getChildren().add(modal);

        Scene scene = new Scene(root, owner.getWidth(), owner.getHeight());
        scene.setFill(Color.TRANSPARENT);
        scene.setOnKeyPressed(e -> { if (e.getCode() == KeyCode.ESCAPE) closeDialog(); });

        setScene(scene);
        setX(owner.getX());
        setY(owner.getY());

        // Callbacks do player
        player.setOnTimeUpdate(t -> {
            playPosMs = t * 1000.0;
            Platform.runLater(this::redraw);
        });
        player.setOnEndOfTrack(() -> Platform.runLater(this::redraw));

        setOnHidden(e -> player.stop());

        // Carrega waveform em background
        Platform.runLater(() ->
            new FfmpegWaveformAnalyzer().analyze(track.path(), 500,
                p -> { peaks = p; redraw(); },
                err -> { /* sem waveform */ }
            )
        );
    }

    // ── Construção da UI ──────────────────────────────────────────────────────

    private VBox buildModal() {
        VBox modal = new VBox(14);
        modal.setMaxWidth(700);
        modal.setMaxHeight(Region.USE_PREF_SIZE);
        modal.setPadding(new Insets(22, 24, 20, 24));
        modal.setStyle(
            "-fx-background-color:#091520;" +
            "-fx-border-color:" + BORDER_MODAL + ";" +
            "-fx-border-radius:14;" +
            "-fx-background-radius:14;"
        );
        modal.getChildren().addAll(
            buildHeader(),
            buildTimeline(),
            buildTimeBar(),
            buildMarkersGrid(),
            buildPlayFromRow(),
            buildFooter()
        );
        return modal;
    }

    private HBox buildHeader() {
        String title  = track.title()  != null && !track.title().isBlank()  ? track.title()  : "—";
        String artist = track.artist() != null && !track.artist().isBlank() ? track.artist() : "—";

        Label lTitle  = new Label(title);
        lTitle.setStyle("-fx-font-size:15px;-fx-font-weight:800;-fx-text-fill:#e8fbfb;");

        Label lArtist = new Label(artist);
        lArtist.setStyle("-fx-font-size:12px;-fx-text-fill:#77a4a3;-fx-font-weight:600;");

        VBox info = new VBox(2, lTitle, lArtist);
        HBox.setHgrow(info, Priority.ALWAYS);

        Button closeBtn = new Button("×");
        closeBtn.setStyle(
            "-fx-background-color:transparent;-fx-border-color:transparent;" +
            "-fx-text-fill:#88a0b5;-fx-font-size:18px;-fx-cursor:hand;-fx-padding:0 2 0 2;"
        );
        closeBtn.setOnMouseEntered(e -> closeBtn.setStyle(closeBtn.getStyle().replace("#88a0b5", "#ffffff")));
        closeBtn.setOnMouseExited(e  -> closeBtn.setStyle(closeBtn.getStyle().replace("#ffffff", "#88a0b5")));
        closeBtn.setOnAction(e -> closeDialog());

        HBox header = new HBox(info, closeBtn);
        header.setAlignment(Pos.TOP_RIGHT);
        return header;
    }

    private Pane buildTimeline() {
        canvas = new Canvas();
        canvasPane = new Pane(canvas);
        canvasPane.setPrefHeight(130);
        canvasPane.setMinHeight(130);
        canvasPane.setMaxHeight(130);
        canvas.widthProperty().bind(canvasPane.widthProperty());
        canvas.heightProperty().bind(canvasPane.heightProperty());
        canvasPane.setStyle(
            "-fx-background-color:#061218;" +
            "-fx-border-color:rgba(45,216,255,0.18);" +
            "-fx-border-radius:6;" +
            "-fx-background-radius:6;" +
            "-fx-cursor:crosshair;"
        );

        // Redesenha quando o canvas redimensiona
        canvas.widthProperty().addListener((o, ov, nv) -> redraw());
        canvas.heightProperty().addListener((o, ov, nv) -> redraw());

        // MousePressed: inicia seleção ou pan (Alt+drag)
        canvas.setOnMousePressed(e -> {
            if (durationMs <= 0) return;
            if (e.isAltDown() && zoom > 1) {
                panActive = true;
                panStartX = e.getX();
                panStartOffset = offset;
                canvas.setCursor(javafx.scene.Cursor.MOVE);
            } else {
                double frac = Math.max(0, Math.min(1, xToFrac(e.getX(), canvas.getWidth())));
                selActive = true;
                selStartFrac = frac;
                selEndFrac   = frac;
                selStartPx   = e.getX();
            }
        });

        // MouseDragged: atualiza seleção ou pan
        canvas.setOnMouseDragged(e -> {
            if (panActive) {
                double inner     = canvas.getWidth() - 2 * PAD_PX;
                double deltaPx   = e.getX() - panStartX;
                double deltaFrac = -(deltaPx / inner) / zoom;
                offset = panStartOffset + deltaFrac;
                clampOffset();
                redraw();
            } else if (selActive) {
                selEndFrac = Math.max(0, Math.min(1, xToFrac(e.getX(), canvas.getWidth())));
                redraw();
            }
        });

        // MouseReleased: finaliza pan, seek (click) ou zoom-to-region (drag)
        canvas.setOnMouseReleased(e -> {
            if (panActive) {
                panActive = false;
                canvas.setCursor(javafx.scene.Cursor.CROSSHAIR);
                return;
            }
            if (!selActive) return;
            selActive = false;
            double dragPx = Math.abs(e.getX() - selStartPx);
            if (dragPx < 5) {
                // Click simples: seek
                if (durationMs > 0) {
                    double frac = Math.max(0, Math.min(1, xToFrac(e.getX(), canvas.getWidth())));
                    pinMs = null;
                    seekTo(frac * durationMs);
                }
            } else {
                // Drag: zoom na região selecionada
                double selMin = Math.min(selStartFrac, selEndFrac);
                double selMax = Math.max(selStartFrac, selEndFrac);
                double span   = selMax - selMin;
                if (span > 0) {
                    offset = selMin;
                    zoom   = Math.min(16, 1.0 / span);
                    clampOffset();
                    updateZoomLabel();
                }
            }
            redraw();
        });

        // Duplo-click: define pin
        canvas.setOnMouseClicked(e -> {
            if (e.getClickCount() == 2 && durationMs > 0) {
                double frac = Math.max(0, Math.min(1, xToFrac(e.getX(), canvas.getWidth())));
                pinMs = frac * durationMs;
                redraw();
            }
        });

        // Scroll = zoom
        canvas.setOnScroll(e -> {
            if (e.getDeltaY() < 0) zoomOut();
            else                   zoomIn();
        });

        return canvasPane;
    }

    private HBox buildTimeBar() {
        lblPos  = monoLabel("0:00.000");
        lblDur  = monoLabel(msToCueTime(durationMs));
        lblZoom = new Label("1×");
        lblZoom.setStyle("-fx-font-size:11px;-fx-text-fill:#fff;-fx-min-width:24px;-fx-alignment:center;");

        Button btnMinus = zoomBtn("−");
        Button btnPlus  = zoomBtn("+");
        btnMinus.setOnAction(e -> zoomOut());
        btnPlus.setOnAction(e  -> zoomIn());

        HBox zoomBox = new HBox(5, btnMinus, lblZoom, btnPlus);
        zoomBox.setAlignment(Pos.CENTER);

        Region spacer = new Region();
        HBox.setHgrow(spacer, Priority.ALWAYS);
        Region spacer2 = new Region();
        HBox.setHgrow(spacer2, Priority.ALWAYS);

        HBox bar = new HBox(spacer, spacer2);
        bar.getChildren().setAll(lblPos, spacer, zoomBox, spacer2, lblDur);
        bar.setAlignment(Pos.CENTER_LEFT);
        return bar;
    }

    private GridPane buildMarkersGrid() {
        GridPane grid = new GridPane();
        grid.setHgap(10);
        for (int i = 0; i < 4; i++) {
            ColumnConstraints cc = new ColumnConstraints();
            cc.setPercentWidth(25);
            grid.getColumnConstraints().add(cc);
        }

        lblCueIn  = markerTimeLabel();
        lblIntro  = markerTimeLabel();
        lblOutro  = markerTimeLabel();
        lblCueOut = markerTimeLabel();

        grid.add(markerPanel("CUE IN",  "#4fc3f7", "rgba(79,195,247,0.35)",  lblCueIn,  "cueIn"),  0, 0);
        grid.add(markerPanel("INTRO",   "#81c784", "rgba(129,199,132,0.35)", lblIntro,  "intro"),  1, 0);
        grid.add(markerPanel("OUTRO",   "#ffb74d", "rgba(255,183,77,0.35)",  lblOutro,  "outro"),  2, 0);
        grid.add(markerPanel("CUE OUT", "#ef5350", "rgba(239,83,80,0.35)",   lblCueOut, "cueOut"), 3, 0);

        refreshMarkerLabels();
        return grid;
    }

    private VBox markerPanel(String name, String color, String border, Label timeLabel, String field) {
        Label lbl = new Label(name);
        lbl.setStyle("-fx-font-size:10px;-fx-font-weight:800;-fx-text-fill:" + color + ";");

        Button capBtn  = markerBtn("CAPTURAR", color, "rgba(" + hexToRgb(color) + ",0.28)", "rgba(" + hexToRgb(color) + ",0.12)");
        Button clrBtn  = markerBtn("LIMPAR",   "#ff6b6b", "rgba(255,107,107,0.22)", "rgba(255,107,107,0.10)");
        capBtn.setOnAction(e -> capture(field));
        clrBtn.setOnAction(e -> clear(field));

        HBox btns = new HBox(4, capBtn, clrBtn);

        VBox panel = new VBox(8, lbl, timeLabel, btns);
        panel.setPadding(new Insets(10, 12, 10, 12));
        panel.setStyle(
            "-fx-background-color:rgba(255,255,255,0.03);" +
            "-fx-border-color:" + border + ";" +
            "-fx-border-radius:8;" +
            "-fx-background-radius:8;"
        );
        return panel;
    }

    private HBox buildPlayFromRow() {
        Button btnCueIn = playFromBtn("▶ TOCAR DO CUE IN");
        Button btnIntro = playFromBtn("▶ TOCAR DO INTRO");
        Button btnOutro = playFromBtn("▶ TOCAR DO OUTRO");
        btnCueIn.setOnAction(e -> playFrom("cueIn"));
        btnIntro.setOnAction(e -> playFrom("intro"));
        btnOutro.setOnAction(e -> playFrom("outro"));
        HBox row = new HBox(8, btnCueIn, btnIntro, btnOutro);
        row.setAlignment(Pos.CENTER_LEFT);
        return row;
    }

    private VBox buildFooter() {
        lblError = new Label("");
        lblError.setStyle("-fx-font-size:11px;-fx-text-fill:#ff6b6b;");
        lblError.setVisible(false);

        Button cancelBtn = new Button("CANCELAR");
        cancelBtn.setStyle(
            "-fx-font-size:13px;-fx-font-weight:700;-fx-padding:8 22 8 22;-fx-border-radius:7;-fx-background-radius:7;" +
            "-fx-background-color:rgba(255,255,255,0.05);-fx-border-color:rgba(255,255,255,0.13);-fx-text-fill:#88a0b5;-fx-cursor:hand;"
        );
        cancelBtn.setOnAction(e -> closeDialog());

        Button saveBtn = new Button("SALVAR");
        saveBtn.setStyle(
            "-fx-font-size:13px;-fx-font-weight:700;-fx-padding:8 22 8 22;-fx-border-radius:7;-fx-background-radius:7;" +
            "-fx-background-color:#20e6ff;-fx-border-color:transparent;-fx-text-fill:#000;-fx-cursor:hand;"
        );
        saveBtn.setOnAction(e -> save());

        Region spacer = new Region();
        HBox.setHgrow(spacer, Priority.ALWAYS);
        HBox btnRow = new HBox(10, lblError, spacer, cancelBtn, saveBtn);
        btnRow.setAlignment(Pos.CENTER_LEFT);

        Separator sep = new Separator();
        sep.setStyle("-fx-background-color:rgba(255,255,255,0.07);");

        return new VBox(14, sep, btnRow);
    }

    // ── Waveform ──────────────────────────────────────────────────────────────

    private void redraw() {
        if (canvas == null) return;
        double w = canvas.getWidth();
        double h = canvas.getHeight();
        if (w <= 0 || h <= 0) return;

        GraphicsContext gc = canvas.getGraphicsContext2D();

        // Background
        gc.setFill(BG_TIMELINE);
        gc.fillRect(0, 0, w, h);

        // Waveform
        if (peaks.length > 0) {
            double viewSpan  = 1.0 / zoom;
            int    startPeak = (int)(offset * peaks.length);
            int    endPeak   = Math.min(peaks.length, (int)((offset + viewSpan) * peaks.length) + 1);
            startPeak = Math.max(0, startPeak);
            int visible = endPeak - startPeak;
            if (visible > 0) {
                double inner = w - 2 * PAD_PX;
                double barW  = inner / visible;
                double cy    = h / 2.0;
                for (int i = 0; i < visible; i++) {
                    int idx = startPeak + i;
                    if (idx >= peaks.length) break;
                    double x  = PAD_PX + i * barW;
                    double bh = Math.max(2, peaks[idx] * h * 0.88);
                    gc.setFill(COL_WAVEFORM);
                    gc.fillRect(x, cy - bh / 2.0, Math.max(1, barW - 0.5), bh);
                }
            }
        }

        // Região CUE IN → CUE OUT (overlay sutil)
        if (cueInMs != null && cueOutMs != null && durationMs > 0) {
            double x1 = fracToX(cueInMs / durationMs, w);
            double x2 = fracToX(cueOutMs / durationMs, w);
            gc.setFill(Color.web("#4fc3f7", 0.07));
            gc.fillRect(Math.max(PAD_PX, x1), 0, Math.max(0, x2 - x1), h);
        }

        // Região selecionada (rubber-band)
        if (selActive && Math.abs(selEndFrac - selStartFrac) > 0.001) {
            double x1 = Math.max(PAD_PX, fracToX(Math.min(selStartFrac, selEndFrac), w));
            double x2 = Math.min(w - PAD_PX, fracToX(Math.max(selStartFrac, selEndFrac), w));
            if (x2 > x1) {
                gc.setFill(Color.web("#20e6ff", 0.15));
                gc.fillRect(x1, 0, x2 - x1, h);
                gc.setStroke(Color.web("#20e6ff", 0.55));
                gc.setLineWidth(1);
                gc.setLineDashes();
                gc.strokeRect(x1, 0, x2 - x1, h);
            }
        }

        // Marcadores
        drawMarker(gc, cueInMs,  COL_CUE_IN,  w, h);
        drawMarker(gc, introMs,  COL_INTRO,   w, h);
        drawMarker(gc, outroMs,  COL_OUTRO,   w, h);
        drawMarker(gc, cueOutMs, COL_CUE_OUT, w, h);

        // Pin (duplo-clique)
        if (pinMs != null && durationMs > 0) {
            double x = fracToX(pinMs / durationMs, w);
            if (x >= PAD_PX && x <= w - PAD_PX) {
                gc.setStroke(COL_PIN);
                gc.setLineWidth(2);
                gc.setLineDashes(6, 4);
                gc.strokeLine(x, 0, x, h);
                gc.setLineDashes();
            }
        }

        // Playhead
        if (durationMs > 0) {
            double x = fracToX(playPosMs / durationMs, w);
            if (x >= PAD_PX && x <= w - PAD_PX) {
                gc.setFill(COL_PLAYHEAD);
                gc.fillRect(x - 1, 0, 2, h);
            }
        }

        // Atualiza labels
        lblPos.setText(msToCueTime(playPosMs));
        lblDur.setText(durationMs > 0 ? msToCueTime(durationMs) : "--:--.---");
        refreshMarkerLabels();
    }

    private void drawMarker(GraphicsContext gc, Double ms, Color color, double w, double h) {
        if (ms == null || durationMs <= 0) return;
        double frac = ms / durationMs;
        double x = fracToX(frac, w);
        if (x < 0 || x > w) return;
        // Glow
        gc.setFill(color.deriveColor(0, 1, 1, 0.25));
        gc.fillRect(x - 5, 0, 10, h);
        // Linha principal
        gc.setFill(color);
        gc.fillRect(x - 1, 0, 2, h);
    }

    private void refreshMarkerLabels() {
        if (lblCueIn  != null) setMarkerLabel(lblCueIn,  cueInMs);
        if (lblIntro  != null) setMarkerLabel(lblIntro,  introMs);
        if (lblOutro  != null) setMarkerLabel(lblOutro,  outroMs);
        if (lblCueOut != null) setMarkerLabel(lblCueOut, cueOutMs);
    }

    private void setMarkerLabel(Label lbl, Double ms) {
        if (ms != null) {
            lbl.setText(msToCueTime(ms));
            lbl.setStyle("-fx-font-family:'Courier New',monospace;-fx-font-size:14px;-fx-font-weight:700;-fx-text-fill:#e8fbfb;");
        } else {
            lbl.setText("—");
            lbl.setStyle("-fx-font-family:'Courier New',monospace;-fx-font-size:12px;-fx-text-fill:#88a0b5;");
        }
    }

    // ── Zoom ──────────────────────────────────────────────────────────────────

    private void zoomIn() {
        if (zoom >= 16) return;
        double center = offset + 0.5 / zoom;
        zoom = Math.min(16, zoom * 2);
        offset = center - 0.5 / zoom;
        clampOffset();
        updateZoomLabel();
        redraw();
    }

    private void zoomOut() {
        if (zoom <= 1) return;
        double center = offset + 0.5 / zoom;
        zoom = Math.max(1, zoom / 2);
        offset = center - 0.5 / zoom;
        clampOffset();
        updateZoomLabel();
        redraw();
    }

    private void clampOffset() {
        double span = 1.0 / zoom;
        offset = Math.max(0, Math.min(1 - span, offset));
    }

    private void updateZoomLabel() {
        if (lblZoom == null) return;
        String txt = (zoom == Math.floor(zoom))
            ? String.valueOf((int) zoom) + "×"
            : String.format("%.1f×", zoom);
        lblZoom.setText(txt);
    }

    // ── Marcadores ────────────────────────────────────────────────────────────

    private void capture(String field) {
        Double val = pinMs != null ? pinMs : playPosMs;
        pinMs = null;
        switch (field) {
            case "cueIn"  -> cueInMs  = val;
            case "intro"  -> introMs  = val;
            case "outro"  -> outroMs  = val;
            case "cueOut" -> cueOutMs = val;
        }
        redraw();
    }

    private void clear(String field) {
        switch (field) {
            case "cueIn"  -> cueInMs  = null;
            case "intro"  -> introMs  = null;
            case "outro"  -> outroMs  = null;
            case "cueOut" -> cueOutMs = null;
        }
        redraw();
    }

    // ── Playback ──────────────────────────────────────────────────────────────

    private void seekTo(double ms) {
        double sec = ms / 1000.0;
        playPosMs = ms;
        if (player.getState() == PlaybackState.STOPPED) {
            Track t = new Track(track.path(),
                track.title().isBlank() ? track.path() : track.title(),
                track.artist(), durationMs / 1000.0);
            player.load(t);
            player.seek(sec);
            player.play();
        } else {
            player.seek(sec);
        }
    }

    private void playFrom(String field) {
        Double ms = switch (field) {
            case "cueIn"  -> cueInMs  != null ? cueInMs  : 0.0;
            case "intro"  -> introMs  != null ? introMs  : 0.0;
            case "outro"  -> outroMs  != null ? outroMs  : 0.0;
            default       -> 0.0;
        };
        seekTo(ms);
    }

    // ── Salvar ────────────────────────────────────────────────────────────────

    private void save() {
        if (cueInMs != null && cueOutMs != null && cueInMs >= cueOutMs) {
            lblError.setText("Cue In deve ser menor que Cue Out.");
            lblError.setVisible(true);
            return;
        }
        lblError.setVisible(false);
        new Thread(() -> {
            try {
                Integer ci = cueInMs  != null ? cueInMs.intValue()  : null;
                Integer ii = introMs  != null ? introMs.intValue()  : null;
                Integer oi = outroMs  != null ? outroMs.intValue()  : null;
                Integer co = cueOutMs != null ? cueOutMs.intValue() : null;
                repo.updateCuePoints(track.id(), ci, ii, oi, co);
                Platform.runLater(() -> {
                    if (onSaved != null) onSaved.run();
                    closeDialog();
                });
            } catch (SQLException ex) {
                Platform.runLater(() -> {
                    lblError.setText("Erro ao salvar: " + ex.getMessage());
                    lblError.setVisible(true);
                });
            }
        }).start();
    }

    private void closeDialog() {
        player.stop();
        close();
    }

    // ── Utilidades de coordenadas ─────────────────────────────────────────────

    /** Fração total [0,1] → posição X no canvas. */
    private double fracToX(double frac, double w) {
        double inner   = w - 2 * PAD_PX;
        double span    = 1.0 / zoom;
        double relFrac = (frac - offset) / span;
        return PAD_PX + relFrac * inner;
    }

    /** Posição X no canvas → fração total [0,1]. */
    private double xToFrac(double x, double w) {
        double inner   = w - 2 * PAD_PX;
        double span    = 1.0 / zoom;
        double relFrac = (x - PAD_PX) / inner;
        return offset + relFrac * span;
    }

    // ── Utilitários de UI ─────────────────────────────────────────────────────

    private static Label monoLabel(String text) {
        Label l = new Label(text);
        l.setStyle("-fx-font-family:'Courier New',monospace;-fx-font-size:11px;-fx-text-fill:#fff;");
        return l;
    }

    private static Label markerTimeLabel() {
        Label l = new Label("—");
        l.setStyle("-fx-font-family:'Courier New',monospace;-fx-font-size:12px;-fx-text-fill:#88a0b5;");
        l.setMinHeight(20);
        return l;
    }

    private static Button zoomBtn(String text) {
        Button b = new Button(text);
        b.setStyle(
            "-fx-background-color:rgba(255,255,255,0.05);" +
            "-fx-border-color:rgba(255,255,255,0.14);-fx-border-radius:3;-fx-background-radius:3;" +
            "-fx-text-fill:#ccc;-fx-font-size:13px;-fx-font-weight:700;" +
            "-fx-min-width:22px;-fx-min-height:18px;-fx-max-width:22px;-fx-max-height:18px;" +
            "-fx-padding:0;-fx-cursor:hand;"
        );
        b.setOnMouseEntered(e -> b.setStyle(b.getStyle().replace("#ccc", "#fff").replace("rgba(255,255,255,0.05)", "rgba(255,255,255,0.14)")));
        b.setOnMouseExited(e  -> b.setStyle(b.getStyle().replace("#fff", "#ccc").replace("rgba(255,255,255,0.14)", "rgba(255,255,255,0.05)")));
        return b;
    }

    private static Button markerBtn(String text, String color, String border, String hoverBg) {
        Button b = new Button(text);
        String base = "-fx-flex:1;-fx-font-size:10px;-fx-font-weight:700;" +
            "-fx-padding:4 2 4 2;-fx-border-radius:4;-fx-background-radius:4;-fx-cursor:hand;" +
            "-fx-border-color:" + border + ";-fx-background-color:rgba(255,255,255,0.04);-fx-text-fill:" + color + ";";
        b.setStyle(base);
        b.setMaxWidth(Double.MAX_VALUE);
        HBox.setHgrow(b, Priority.ALWAYS);
        b.setOnMouseEntered(e -> b.setStyle(base.replace("rgba(255,255,255,0.04)", hoverBg)));
        b.setOnMouseExited(e  -> b.setStyle(base));
        return b;
    }

    private static Button playFromBtn(String text) {
        Button b = new Button(text);
        b.setStyle(
            "-fx-font-size:11px;-fx-font-weight:700;-fx-padding:6 12 6 12;-fx-border-radius:6;-fx-background-radius:6;" +
            "-fx-cursor:hand;-fx-border-color:rgba(45,216,255,0.22);-fx-background-color:rgba(45,216,255,0.06);-fx-text-fill:#20e6ff;"
        );
        b.setOnMouseEntered(e -> b.setStyle(b.getStyle().replace("rgba(45,216,255,0.06)", "rgba(45,216,255,0.18)")));
        b.setOnMouseExited(e  -> b.setStyle(b.getStyle().replace("rgba(45,216,255,0.18)", "rgba(45,216,255,0.06)")));
        return b;
    }

    // ── Helpers ───────────────────────────────────────────────────────────────

    /** Formata ms como "M:SS.mmm" (ex: "0:00.000", "3:01.442"). */
    private static String msToCueTime(double ms) {
        int total  = (int) Math.max(0, Math.round(ms));
        int msPart = total % 1000;
        int sec    = (total / 1000) % 60;
        int min    = total / 60000;
        return String.format("%d:%02d.%03d", min, sec, msPart);
    }

    /** Converte cor hex (#rrggbb) para string "r,g,b" para uso em rgba(). */
    private static String hexToRgb(String hex) {
        hex = hex.replace("#", "");
        int r = Integer.parseInt(hex.substring(0, 2), 16);
        int g = Integer.parseInt(hex.substring(2, 4), 16);
        int b = Integer.parseInt(hex.substring(4, 6), 16);
        return r + "," + g + "," + b;
    }

    private static class Separator extends Region {
        Separator() {
            setMinHeight(1); setPrefHeight(1); setMaxHeight(1);
            setStyle("-fx-background-color:rgba(255,255,255,0.07);");
            setMaxWidth(Double.MAX_VALUE);
        }
    }
}
