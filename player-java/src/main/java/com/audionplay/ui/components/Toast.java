package com.audionplay.ui.components;

import javafx.animation.FadeTransition;
import javafx.animation.PauseTransition;
import javafx.animation.SequentialTransition;
import javafx.geometry.Insets;
import javafx.geometry.Pos;
import javafx.scene.layout.HBox;
import javafx.scene.layout.StackPane;
import javafx.util.Duration;

import static com.audionplay.ui.Theme.*;

/**
 * Notificação temporária exibida sobre a UI.
 *
 * Uso: {@code Toast.show(rootStackPane, "Mensagem aqui")}
 */
public final class Toast {

    private Toast() {}

    /**
     * Exibe uma mensagem flutuante no canto inferior central do {@code root}.
     * Desaparece automaticamente após ~2.5s com fade-out.
     *
     * @param root    StackPane raiz da janela
     * @param message texto a exibir
     */
    public static void show(StackPane root, String message) {
        HBox toast = new HBox(
            com.audionplay.ui.Theme.lbl(
                "✓  " + message,
                "-fx-font-size:13px;-fx-text-fill:white;-fx-font-weight:bold;"
            )
        );
        toast.setStyle(
            "-fx-background-color:#1E3A2F;" +
            "-fx-border-color:#2ECC71;-fx-border-width:1;-fx-border-radius:8;" +
            "-fx-background-radius:8;-fx-padding:10 20 10 20;"
        );
        toast.setAlignment(Pos.CENTER_LEFT);
        toast.setMouseTransparent(true);
        toast.setOpacity(0);
        toast.setMaxWidth(javafx.scene.layout.Region.USE_PREF_SIZE);
        toast.setMaxHeight(javafx.scene.layout.Region.USE_PREF_SIZE);

        StackPane.setAlignment(toast, Pos.TOP_RIGHT);
        StackPane.setMargin(toast, new Insets(56, 20, 0, 0));

        root.getChildren().add(toast);

        FadeTransition fadeIn  = new FadeTransition(Duration.millis(180), toast);
        fadeIn.setFromValue(0); fadeIn.setToValue(1);

        PauseTransition hold   = new PauseTransition(Duration.millis(2000));

        FadeTransition fadeOut = new FadeTransition(Duration.millis(400), toast);
        fadeOut.setFromValue(1); fadeOut.setToValue(0);
        fadeOut.setOnFinished(e -> root.getChildren().remove(toast));

        new SequentialTransition(fadeIn, hold, fadeOut).play();
    }
}
