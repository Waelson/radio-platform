package com.audionplay.ui.components;

import com.audionplay.ui.Theme;
import javafx.geometry.Insets;
import javafx.geometry.Pos;
import javafx.scene.control.Button;
import javafx.scene.control.Label;
import javafx.scene.layout.*;

import static com.audionplay.ui.Theme.*;

/**
 * Card de um item da fila de reprodução.
 *
 * Layout idêntico ao player.html:
 *   [drag ⠿] [q-index: ícone+hora+dur 48px] [q-type-bar 4px] [q-track: título+artista+badge]
 *
 * Hover: exibe botões ▶ e ✕ sobrepostos à direita.
 */
public class QueueCard extends HBox {

    private Runnable onPlayAction;

    /** Wired ao botão ▶ no hover. */
    public void setOnPlay(Runnable handler) { this.onPlayAction = handler; }

    // ── Factory methods ───────────────────────────────────────────────────────

    public static QueueCard music(String title, String artist, String time, String dur, boolean prBadge) {
        return new QueueCard("♪", title, artist, "Música", time, dur,
                GRAD_MUS, ACC_MUS, BDR_MUS, prBadge, false);
    }

    public static QueueCard vinheta(String title, String time, String dur) {
        return new QueueCard("♪", title, "", "Vinheta", time, dur,
                GRAD_JIN, ACC_JIN, BDR_JIN, false, false);
    }

    public static QueueCard horaCerta(String time, String dur) {
        return new QueueCard("🕐", "HORA CERTA", null, null, time, dur,
                GRAD_HORA, ACC_HORA, BDR_HORA, false, true);
    }

    // ── Constructor ───────────────────────────────────────────────────────────

    private QueueCard(String iconText, String title, String artist,
                      String typeName, String time, String dur,
                      String bgGradient, String accentColor, String borderColor,
                      boolean prBadge, boolean horaCertaStyle) {
        super(0);
        setMinHeight(76);
        setMaxWidth(Double.MAX_VALUE);
        setStyle(
            "-fx-background-color:" + bgGradient + ";" +
            "-fx-background-radius:9;" +
            "-fx-border-color:" + borderColor + ";" +
            "-fx-border-radius:9;" +
            "-fx-border-width:1;" +
            "-fx-padding:6 8 6 5;"
        );

        // Hover — clareamento fiel ao .queue-row:hover { filter: brightness(1.15) }
        setOnMouseEntered(e -> setStyle(getStyle() + "-fx-opacity:0.88;"));
        setOnMouseExited(e  -> setStyle(getStyle().replace("-fx-opacity:0.88;", "")));

        StackPane rightStack = buildRightStack(title, artist, typeName, prBadge, horaCertaStyle);
        getChildren().addAll(
            buildHandle(),
            buildIndexCol(iconText, time, dur),
            buildTypeBar(accentColor),
            rightStack
        );
    }

    // ── Handle (⠿) ────────────────────────────────────────────────────────────

    private VBox buildHandle() {
        Label dots = new Label("⠿");
        dots.setStyle("-fx-font-size:14px;-fx-text-fill:rgba(255,255,255,0.20);");
        VBox handle = new VBox(dots);
        handle.setAlignment(Pos.CENTER);
        handle.setPrefWidth(18);
        handle.setPadding(new Insets(0, 4, 0, 0));
        return handle;
    }

    // ── q-index: ícone + hora + duração ──────────────────────────────────────

    private VBox buildIndexCol(String iconText, String time, String dur) {
        Label icon = new Label(iconText);
        icon.setStyle("-fx-font-size:22px;-fx-text-fill:rgba(255,255,255,0.70);");

        Label timeLbl = new Label(time);
        timeLbl.setStyle(
            "-fx-font-size:10px;-fx-text-fill:#88a0b5;" +
            "-fx-font-family:'Courier New',monospace;"
        );

        Label durLbl = new Label(dur);
        durLbl.setStyle(
            "-fx-font-size:10px;-fx-font-weight:bold;-fx-text-fill:#ffffff;" +
            "-fx-font-family:'Courier New',monospace;"
        );

        VBox col = new VBox(2, icon, timeLbl, durLbl);
        col.setAlignment(Pos.CENTER);
        col.setPrefWidth(48);
        col.setMinWidth(48);
        col.setMaxWidth(48);
        return col;
    }

    // ── q-type-bar: barra colorida vertical ──────────────────────────────────

    private Region buildTypeBar(String accentColor) {
        Region bar = new Region();
        bar.setPrefWidth(4);
        bar.setMinWidth(4);
        bar.setMaxWidth(4);
        bar.setStyle(
            "-fx-background-color:" + accentColor + ";" +
            "-fx-background-radius:4;"
        );
        HBox.setMargin(bar, new Insets(0, 8, 0, 8));
        return bar;
    }

    // ── q-track: título + artista + badge + overlay de hover ─────────────────

    private StackPane buildRightStack(String title, String artist, String typeName,
                                       boolean prBadge, boolean horaCertaStyle) {
        VBox content = buildTrackContent(title, artist, typeName, prBadge, horaCertaStyle);
        HBox actions = buildActionsOverlay();

        StackPane stack = new StackPane(content, actions);
        StackPane.setAlignment(content, Pos.CENTER_LEFT);
        StackPane.setAlignment(actions, Pos.CENTER_RIGHT);
        HBox.setHgrow(stack, Priority.ALWAYS);

        setOnMouseEntered(e -> { actions.setVisible(true);  applyHover(true);  });
        setOnMouseExited(e  -> { actions.setVisible(false); applyHover(false); });
        return stack;
    }

    private void applyHover(boolean on) {
        String base = getStyle()
            .replace("-fx-effect:dropshadow(gaussian,rgba(255,255,255,0.06),4,0,0,0);", "");
        if (on) base += "-fx-effect:dropshadow(gaussian,rgba(255,255,255,0.06),4,0,0,0);";
        setStyle(base);
    }

    private VBox buildTrackContent(String title, String artist, String typeName,
                                    boolean prBadge, boolean horaCertaStyle) {
        VBox track = new VBox(3);
        track.setAlignment(Pos.CENTER_LEFT);
        track.setPadding(new Insets(0, 8, 0, 0));
        track.setMaxWidth(Double.MAX_VALUE);

        if (horaCertaStyle) {
            Label t = new Label("HORA CERTA");
            t.setStyle("-fx-font-size:13px;-fx-font-weight:900;-fx-text-fill:#ffffff;" +
                       "-fx-letter-spacing:0.12em;");
            track.getChildren().add(t);
        } else {
            // Linha do título
            HBox titleRow = new HBox(6);
            titleRow.setAlignment(Pos.CENTER_LEFT);

            Label titleLbl = new Label(title);
            titleLbl.setStyle(
                "-fx-font-size:13px;-fx-font-weight:bold;-fx-text-fill:#d7e3ec;" +
                "-fx-wrap-text:false;"
            );
            titleLbl.setMaxWidth(Double.MAX_VALUE);
            HBox.setHgrow(titleLbl, Priority.ALWAYS);
            titleRow.getChildren().add(titleLbl);

            if (prBadge) {
                Label pr = new Label("PR");
                pr.setStyle(
                    "-fx-font-size:9px;-fx-font-weight:800;-fx-text-fill:#00d4ff;" +
                    "-fx-background-color:rgba(0,212,255,0.12);" +
                    "-fx-border-color:rgba(0,212,255,0.30);-fx-border-width:1;" +
                    "-fx-background-radius:5;-fx-border-radius:5;" +
                    "-fx-padding:2 6 2 6;"
                );
                titleRow.getChildren().add(pr);
            }
            track.getChildren().add(titleRow);

            // Artista
            if (artist != null && !artist.isEmpty()) {
                Label artistLbl = new Label(artist);
                artistLbl.setStyle("-fx-font-size:10px;-fx-text-fill:#ffffff;");
                track.getChildren().add(artistLbl);
            }

            // Badge de tipo
            if (typeName != null && !typeName.isEmpty()) {
                Label badge = new Label(typeName);
                badge.setStyle(
                    "-fx-font-size:9px;-fx-font-weight:700;-fx-text-fill:#88a0b5;" +
                    "-fx-background-color:rgba(255,255,255,0.06);" +
                    "-fx-background-radius:5;-fx-padding:2 5 2 5;"
                );
                track.getChildren().add(badge);
            }
        }
        return track;
    }

    // ── Overlay de ações no hover ─────────────────────────────────────────────

    private HBox buildActionsOverlay() {
        String base = "-fx-background-radius:6;-fx-padding:6 12 6 12;-fx-cursor:hand;-fx-font-size:13px;";

        Button play = new Button("▶");
        play.setStyle(base + "-fx-background-color:rgba(45,216,255,0.15);-fx-text-fill:#20e6ff;");
        play.setOnMouseEntered(e -> play.setStyle(base + "-fx-background-color:rgba(45,216,255,0.25);-fx-text-fill:#20e6ff;"));
        play.setOnMouseExited(e  -> play.setStyle(base + "-fx-background-color:rgba(45,216,255,0.15);-fx-text-fill:#20e6ff;"));
        play.setOnAction(e -> { if (onPlayAction != null) onPlayAction.run(); });

        Button remove = new Button("✕");
        remove.setStyle(base + "-fx-background-color:rgba(255,77,103,0.15);-fx-text-fill:#ff4d67;");
        remove.setOnMouseEntered(e -> remove.setStyle(base + "-fx-background-color:rgba(255,77,103,0.30);-fx-text-fill:#ff4d67;"));
        remove.setOnMouseExited(e  -> remove.setStyle(base + "-fx-background-color:rgba(255,77,103,0.15);-fx-text-fill:#ff4d67;"));

        Button cue = new Button("🎧");
        cue.setStyle(base + "-fx-background-color:rgba(255,255,255,0.06);-fx-text-fill:#88a0b5;");
        cue.setOnMouseEntered(e -> cue.setStyle(base + "-fx-background-color:rgba(45,216,255,0.10);-fx-text-fill:#20e6ff;"));
        cue.setOnMouseExited(e  -> cue.setStyle(base + "-fx-background-color:rgba(255,255,255,0.06);-fx-text-fill:#88a0b5;"));

        HBox actions = new HBox(6, play, remove, cue);
        actions.setAlignment(Pos.CENTER_RIGHT);
        actions.setPadding(new Insets(0, 8, 0, 0));
        actions.setVisible(false);
        return actions;
    }
}
