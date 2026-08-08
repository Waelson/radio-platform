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
 * Estados:
 *   playing=true → gradiente vermelho (is-current), ícone e textos rosas
 *   nextBadge=true → badge "PRÓXIMO" branco sobreposto ao título
 */
public class QueueCard extends HBox {

    private Runnable onPlayAction;
    private Runnable onRemoveAction;

    public void setOnPlay(Runnable handler)   { this.onPlayAction   = handler; }
    public void setOnRemove(Runnable handler) { this.onRemoveAction = handler; }

    // ── Factory methods ───────────────────────────────────────────────────────

    /** Item em reprodução: gradiente vermelho, ícone e textos #ff8fa1. */
    public static QueueCard playing(String title, String artist, String time, String dur) {
        return new QueueCard(
            "\u25AE\u25AE\u25AE",  // ▮▮▮ simula barras VU
            title, artist, null, time, dur,
            "linear-gradient(to bottom,#7a1520 0%,#6e111c 22%,#5e0e18 47%,#2e0408 49%,#1e0205 72%,#140103 100%)",
            "rgba(255,77,103,0.80)",
            "#d47a85",
            false, false, true
        );
    }

    /** Item de música normal; nextBadge=true exibe badge "PRÓXIMO". */
    public static QueueCard music(String title, String artist, String time, String dur, boolean nextBadge) {
        return new QueueCard("♪", title, artist, "Música", time, dur,
                GRAD_MUS, ACC_MUS, BDR_MUS, nextBadge, false, false);
    }

    /** Item de vinheta/jingle. */
    public static QueueCard vinheta(String title, String time, String dur) {
        return new QueueCard("♪", title, "", "Vinheta", time, dur,
                GRAD_JIN, ACC_JIN, BDR_JIN, false, false, false);
    }

    /** Hora certa. */
    public static QueueCard horaCerta(String time, String dur) {
        return new QueueCard("🕐", "HORA CERTA", null, null, time, dur,
                GRAD_HORA, ACC_HORA, BDR_HORA, false, true, false);
    }

    // ── Constructor ───────────────────────────────────────────────────────────

    private QueueCard(String iconText, String title, String artist,
                      String typeName, String time, String dur,
                      String bgGradient, String accentColor, String borderColor,
                      boolean nextBadge, boolean horaCertaStyle, boolean isPlaying) {
        super(0);
        setMinHeight(76);
        setMaxWidth(Double.MAX_VALUE);
        setStyle(
            "-fx-background-color:" + bgGradient + ";" +
            "-fx-background-radius:9;" +
            "-fx-border-color:" + borderColor + ";" +
            "-fx-border-radius:9;" +
            "-fx-border-width:1;" +
            "-fx-padding:6 8 6 " + (isPlaying ? "8" : "5") + ";"
        );

        StackPane rightStack = buildRightStack(title, artist, typeName, nextBadge, horaCertaStyle, isPlaying);
        getChildren().addAll(
            buildHandle(isPlaying),
            buildIndexCol(iconText, time, dur, isPlaying),
            buildTypeBar(accentColor),
            rightStack
        );
    }

    // ── Handle (⠿) ────────────────────────────────────────────────────────────

    private VBox buildHandle(boolean isPlaying) {
        Label dots = new Label("⠿");
        dots.setStyle("-fx-font-size:14px;-fx-text-fill:" +
            (isPlaying ? "rgba(255,77,103,0.30)" : "rgba(255,255,255,0.20)") + ";");
        VBox handle = new VBox(dots);
        handle.setAlignment(Pos.CENTER);
        handle.setPrefWidth(18);
        handle.setPadding(new Insets(0, 4, 0, 0));
        if (isPlaying) { handle.setVisible(false); handle.setManaged(false); }
        return handle;
    }

    // ── q-index: ícone + hora + duração ──────────────────────────────────────

    private VBox buildIndexCol(String iconText, String time, String dur, boolean isPlaying) {
        String tintColor  = isPlaying ? "#ff8fa1" : "rgba(255,255,255,0.70)";
        String timeColor  = isPlaying ? "#ff8fa1" : "#88a0b5";
        String durColor   = isPlaying ? "#ff8fa1" : "#ffffff";

        Label icon = new Label(iconText);
        icon.setStyle("-fx-font-size:" + (isPlaying ? "14" : "22") + "px;-fx-text-fill:" + tintColor + ";");

        Label timeLbl = new Label(time);
        timeLbl.setStyle(
            "-fx-font-size:10px;-fx-text-fill:" + timeColor + ";" +
            "-fx-font-family:'Courier New',monospace;"
        );

        Label durLbl = new Label(dur);
        durLbl.setStyle(
            "-fx-font-size:10px;-fx-font-weight:bold;-fx-text-fill:" + durColor + ";" +
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
                                      boolean nextBadge, boolean horaCertaStyle, boolean isPlaying) {
        VBox content = buildTrackContent(title, artist, typeName, nextBadge, horaCertaStyle, isPlaying);
        HBox actions = buildActionsOverlay(isPlaying);

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
                                    boolean nextBadge, boolean horaCertaStyle, boolean isPlaying) {
        VBox track = new VBox(3);
        track.setAlignment(Pos.CENTER_LEFT);
        track.setPadding(new Insets(0, 8, 0, 0));
        track.setMaxWidth(Double.MAX_VALUE);

        if (horaCertaStyle) {
            Label t = new Label("HORA CERTA");
            t.setStyle("-fx-font-size:13px;-fx-font-weight:900;-fx-text-fill:#ffffff;");
            track.getChildren().add(t);
        } else {
            // título + badge "PRÓXIMO"
            HBox titleRow = new HBox(6);
            titleRow.setAlignment(Pos.CENTER_LEFT);

            Label titleLbl = new Label(title);
            titleLbl.setStyle("-fx-font-size:13px;-fx-font-weight:bold;-fx-text-fill:#d7e3ec;");
            titleLbl.setMaxWidth(Double.MAX_VALUE);
            HBox.setHgrow(titleLbl, Priority.ALWAYS);
            titleRow.getChildren().add(titleLbl);

            if (nextBadge) {
                Label prox = new Label("PRÓXIMO");
                prox.setStyle(
                    "-fx-font-size:8px;-fx-font-weight:800;-fx-text-fill:#ffffff;" +
                    "-fx-background-color:rgba(57,120,255,0.20);" +
                    "-fx-border-color:#ffffff;-fx-border-width:1;" +
                    "-fx-background-radius:4;-fx-border-radius:4;" +
                    "-fx-padding:1 5 1 5;"
                );
                titleRow.getChildren().add(prox);
            }
            track.getChildren().add(titleRow);

            // artista
            if (artist != null && !artist.isEmpty()) {
                Label artistLbl = new Label(artist);
                artistLbl.setStyle("-fx-font-size:10px;-fx-text-fill:#ffffff;");
                track.getChildren().add(artistLbl);
            }

            // badge de tipo
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

    private HBox buildActionsOverlay(boolean isPlaying) {
        String normal = "-fx-background-radius:8;-fx-padding:6 12 6 12;-fx-cursor:hand;-fx-font-size:13px;" +
                        "-fx-background-color:rgba(0,0,0,0.75);" +
                        "-fx-border-color:rgba(255,255,255,0.30);-fx-border-width:1;-fx-border-radius:8;" +
                        "-fx-text-fill:#ffffff;";
        String hover  = "-fx-background-radius:8;-fx-padding:6 12 6 12;-fx-cursor:hand;-fx-font-size:13px;" +
                        "-fx-background-color:rgba(0,0,0,0.95);" +
                        "-fx-border-color:rgba(255,255,255,0.70);-fx-border-width:1;-fx-border-radius:8;" +
                        "-fx-text-fill:#ffffff;";

        Button cue = new Button("🎧");
        cue.setStyle(normal);
        cue.setOnMouseEntered(e -> cue.setStyle(hover));
        cue.setOnMouseExited(e  -> cue.setStyle(normal));

        HBox actions;
        if (isPlaying) {
            actions = new HBox(6, cue);
        } else {
            Button play = new Button("▶");
            play.setStyle(normal);
            play.setOnMouseEntered(e -> play.setStyle(hover));
            play.setOnMouseExited(e  -> play.setStyle(normal));
            play.setOnAction(e -> { if (onPlayAction != null) onPlayAction.run(); });

            Button remove = new Button("✕");
            remove.setStyle(normal);
            remove.setOnMouseEntered(e -> remove.setStyle(hover));
            remove.setOnMouseExited(e  -> remove.setStyle(normal));
            remove.setOnAction(e -> { if (onRemoveAction != null) onRemoveAction.run(); });

            actions = new HBox(6, play, remove, cue);
        }

        actions.setAlignment(Pos.CENTER_RIGHT);
        actions.setPadding(new Insets(0, 8, 0, 0));
        actions.setVisible(false);
        return actions;
    }
}
