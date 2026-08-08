package com.audionplay.ui.components;

import com.audionplay.ui.Theme;
import javafx.geometry.Insets;
import javafx.geometry.Pos;
import javafx.scene.control.Button;
import javafx.scene.control.Label;
import javafx.scene.layout.*;
import javafx.scene.paint.Color;
import javafx.scene.shape.Circle;

import static com.audionplay.ui.Theme.*;

/**
 * Card de um item da fila de reprodução.
 *
 * Layout: [drag handle] [seção esquerda: ícone+hora+dur] [divisor] [seção direita: título+artista+badge]
 * Hover: exibe botões de ação (play, remover, cue) sobrepostos à direita.
 *
 * Use os factory methods para criar cada tipo:
 *   QueueCard.music(...)   — música
 *   QueueCard.vinheta(...) — vinheta / jingle
 *   QueueCard.horaCerta(...)
 */
public class QueueCard extends HBox {

    private Runnable onPlayAction;

    /** Wired ao botão ▶ no hover. Pode ser chamado após a construção. */
    public void setOnPlay(Runnable handler) { this.onPlayAction = handler; }

    // ── Factory methods ───────────────────────────────────────────────────────

    public static QueueCard music(String title, String artist, String time, String dur, boolean prBadge) {
        return new QueueCard("♪", title, artist, "Música", time, dur, GRAD_MUS, ACC_MUS, prBadge, false);
    }

    public static QueueCard vinheta(String title, String time, String dur) {
        return new QueueCard("🔊", title, "", "Vinheta", time, dur, GRAD_JIN, ACC_JIN, false, false);
    }

    public static QueueCard horaCerta(String time, String dur) {
        return new QueueCard("🕐", "HORA CERTA", null, null, time, dur, GRAD_HORA, ACC_HORA, false, true);
    }

    // ── Constructor ───────────────────────────────────────────────────────────

    private QueueCard(String iconText, String title, String artist,
                      String typeName, String time, String dur,
                      String bgGradient, String accentColor,
                      boolean prBadge, boolean horaCertaStyle) {
        super(0);
        setMinHeight(58);
        setMaxWidth(Double.MAX_VALUE);
        setStyle(
            "-fx-background-color:" + bgGradient + ";" +
            "-fx-background-radius:10;" +
            "-fx-border-color:#FFFFFF12;" +
            "-fx-border-radius:10;" +
            "-fx-border-width:1;"
        );

        StackPane rightStack = buildRightStack(title, artist, typeName, prBadge, horaCertaStyle);
        getChildren().addAll(
            buildHandle(),
            buildLeft(iconText, time, dur, accentColor),
            buildDivider(),
            rightStack
        );
    }

    // ── Seções internas ───────────────────────────────────────────────────────

    private VBox buildHandle() {
        GridPane dots = new GridPane();
        dots.setHgap(3); dots.setVgap(4);
        for (int r = 0; r < 3; r++)
            for (int c = 0; c < 2; c++)
                dots.add(new Circle(2, Color.web("#FFFFFF40")), c, r);
        VBox handle = new VBox(dots);
        handle.setAlignment(Pos.CENTER);
        handle.setPrefWidth(20);
        handle.setPadding(new Insets(0, 2, 0, 6));
        return handle;
    }

    private VBox buildLeft(String iconText, String time, String dur, String accentColor) {
        VBox left = new VBox(4);
        left.setAlignment(Pos.CENTER);
        left.setPrefWidth(95);
        left.setMinWidth(95);
        left.setPadding(new Insets(6, 8, 6, 4));
        left.setStyle("-fx-background-color:#00000018;-fx-background-radius:9 0 0 9;");
        left.getChildren().addAll(
            Theme.lbl(iconText, "-fx-font-size:22px;-fx-text-fill:" + accentColor + ";"),
            Theme.lbl(time,     "-fx-font-size:10px;-fx-text-fill:#CCCCCC;"),
            Theme.lbl(dur,      "-fx-font-size:11px;-fx-font-weight:bold;-fx-text-fill:white;")
        );
        return left;
    }

    private Pane buildDivider() {
        Pane d = new Pane();
        d.setPrefWidth(3);
        d.setStyle("-fx-background-color:#FFFFFF30;");
        return d;
    }

    private StackPane buildRightStack(String title, String artist, String typeName,
                                       boolean prBadge, boolean horaCertaStyle) {
        VBox right = buildRightContent(title, artist, typeName, prBadge, horaCertaStyle);
        HBox actions = buildActionsOverlay();

        StackPane stack = new StackPane(right, actions);
        StackPane.setAlignment(right,   Pos.CENTER_LEFT);
        StackPane.setAlignment(actions, Pos.CENTER_RIGHT);
        HBox.setHgrow(stack, Priority.ALWAYS);

        setOnMouseEntered(e -> actions.setVisible(true));
        setOnMouseExited(e  -> actions.setVisible(false));
        return stack;
    }

    private VBox buildRightContent(String title, String artist, String typeName,
                                    boolean prBadge, boolean horaCertaStyle) {
        VBox right = new VBox();
        right.setPadding(new Insets(8, 14, 8, 16));
        right.setAlignment(horaCertaStyle ? Pos.CENTER_LEFT : Pos.TOP_LEFT);
        right.setMaxWidth(Double.MAX_VALUE);

        if (horaCertaStyle) {
            VBox.setVgrow(right, Priority.ALWAYS);
            right.getChildren().add(Theme.lbl(title, "-fx-font-size:17px;-fx-font-weight:bold;-fx-text-fill:white;"));
        } else {
            right.setSpacing(5);
            HBox titleRow = new HBox(8);
            titleRow.setAlignment(Pos.CENTER_LEFT);
            titleRow.getChildren().add(Theme.lbl(title, "-fx-font-size:14px;-fx-font-weight:bold;-fx-text-fill:white;"));
            if (prBadge) {
                Label pr = new Label("PR");
                pr.setStyle("-fx-font-size:9px;-fx-font-weight:bold;-fx-text-fill:white;" +
                            "-fx-background-color:#2ECC71;-fx-background-radius:4;-fx-padding:2 5 2 5;");
                titleRow.getChildren().add(pr);
            }
            right.getChildren().add(titleRow);

            if (artist != null && !artist.isEmpty())
                right.getChildren().add(Theme.lbl(artist, "-fx-font-size:12px;-fx-text-fill:#CCCCCC;"));

            if (typeName != null && !typeName.isEmpty()) {
                Label badge = new Label(typeName);
                badge.setStyle(
                    "-fx-font-size:10px;-fx-text-fill:#AAAAAA;" +
                    "-fx-background-color:#00000030;-fx-background-radius:5;" +
                    "-fx-padding:2 10 2 10;-fx-border-color:#FFFFFF22;" +
                    "-fx-border-radius:5;-fx-border-width:1;"
                );
                right.getChildren().add(badge);
            }
        }
        return right;
    }

    private HBox buildActionsOverlay() {
        String base = "-fx-background-radius:8;-fx-padding:7 13 7 13;-fx-cursor:hand;-fx-font-size:13px;";
        Button play   = new Button("▶");
        play.setStyle(base + "-fx-background-color:#1E3A4A;-fx-text-fill:#4ECDC4;");
        play.setOnAction(e -> { if (onPlayAction != null) onPlayAction.run(); });
        Button remove = new Button("✕");
        remove.setStyle(base + "-fx-background-color:#2A2A3A;-fx-text-fill:#AAAAAA;");
        Button cue    = new Button("🎧");
        cue.setStyle(base + "-fx-background-color:#2A2A3A;-fx-text-fill:#AAAAAA;");

        HBox actions = new HBox(6, play, remove, cue);
        actions.setAlignment(Pos.CENTER_RIGHT);
        actions.setPadding(new Insets(0, 12, 0, 0));
        actions.setVisible(false);
        return actions;
    }
}
