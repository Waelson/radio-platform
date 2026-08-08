package com.audionplay.ui.components;

import com.audionplay.ui.Theme;
import javafx.geometry.Pos;
import javafx.scene.layout.*;
import javafx.scene.paint.Color;
import javafx.scene.shape.Circle;

import static com.audionplay.ui.Theme.*;

/**
 * Painel de configuração de servidores de streaming.
 */
public class StreamingPanel extends VBox {

    public StreamingPanel() {
        super(10);

        HBox hdr = new HBox();
        hdr.setAlignment(Pos.CENTER_LEFT);
        hdr.getChildren().addAll(
            Theme.lbl("STREAMING",
                "-fx-font-size:10px;-fx-font-weight:bold;-fx-text-fill:" + TEXT_SEC + ";"),
            Theme.hSpacer(),
            Theme.lbl("+ Servidor",
                "-fx-font-size:11px;-fx-text-fill:" + BLUE + ";-fx-cursor:hand;")
        );

        VBox card = new VBox(10);
        card.setStyle(
            "-fx-background-color:#14172280;" +
            "-fx-background-radius:8;-fx-padding:12;" +
            "-fx-border-color:" + BORDER + ";" +
            "-fx-border-radius:8;-fx-border-width:1;"
        );

        HBox sh = new HBox(8);
        sh.setAlignment(Pos.CENTER_LEFT);
        sh.getChildren().addAll(
            new Circle(5, Color.web("#404060")),
            Theme.lbl("CasterFM",
                "-fx-font-size:13px;-fx-font-weight:bold;-fx-text-fill:" + TEXT_PRI + ";"),
            Theme.hSpacer(),
            Theme.lbl("OFF",
                "-fx-font-size:10px;-fx-text-fill:" + TEXT_SEC + ";" +
                "-fx-background-color:#202440;-fx-background-radius:4;-fx-padding:2 8 2 8;")
        );

        HBox sbtn = new HBox(8,
            Theme.outlineBtn("Conectar", BLUE),
            Theme.outlineBtn("Editar",   TEXT_SEC),
            Theme.outlineBtn("Remover",  TEXT_SEC)
        );

        card.getChildren().addAll(
            sh,
            Theme.lbl("sapirncast.fm:19656 · MP3 · 96kbps",
                "-fx-font-size:10px;-fx-text-fill:" + TEXT_MUT + ";"),
            sbtn
        );

        getChildren().addAll(hdr, card);
    }
}
