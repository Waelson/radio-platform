package com.audionplay.ui.components;

import com.audionplay.ui.Theme;
import javafx.geometry.Pos;
import javafx.scene.control.Label;
import javafx.scene.layout.*;

import static com.audionplay.ui.Theme.*;

/**
 * Painel de seleção do modo de operação (Live Assist / Automático).
 */
public class OpModePanel extends VBox {

    public OpModePanel() {
        super(10);

        HBox hdr = new HBox();
        hdr.setAlignment(Pos.CENTER_LEFT);
        hdr.getChildren().addAll(
            Theme.lbl("MODO DE OPERAÇÃO",
                "-fx-font-size:10px;-fx-font-weight:bold;-fx-text-fill:" + TEXT_SEC + ";"),
            Theme.hSpacer(),
            Theme.lbl("ESTUDIO-PRINCIPAL",
                "-fx-font-size:9px;-fx-text-fill:" + TEXT_SEC + ";" +
                "-fx-background-color:#14172280;-fx-background-radius:4;" +
                "-fx-padding:3 8 3 8;-fx-border-color:" + BORDER + ";" +
                "-fx-border-radius:4;-fx-border-width:1;")
        );

        HBox btns = new HBox(0);
        btns.setStyle(
            "-fx-background-color:#14172280;" +
            "-fx-background-radius:8;" +
            "-fx-border-color:" + BORDER + ";" +
            "-fx-border-radius:8;-fx-border-width:1;"
        );

        Label liveAssist = Theme.lbl("Live Assist",
            "-fx-font-size:13px;-fx-text-fill:" + TEXT_SEC + ";-fx-padding:10 0 10 0;");
        liveAssist.setMaxWidth(Double.MAX_VALUE);
        liveAssist.setAlignment(Pos.CENTER);
        HBox.setHgrow(liveAssist, Priority.ALWAYS);

        Label automatico = Theme.lbl("Automático",
            "-fx-font-size:13px;-fx-font-weight:bold;-fx-text-fill:#142A14;" +
            "-fx-background-color:" + GREEN + ";-fx-background-radius:6;-fx-padding:10 0 10 0;");
        automatico.setMaxWidth(Double.MAX_VALUE);
        automatico.setAlignment(Pos.CENTER);
        HBox.setHgrow(automatico, Priority.ALWAYS);

        btns.getChildren().addAll(liveAssist, automatico);
        getChildren().addAll(hdr, btns);
    }
}
