package com.audionplay.ui.components;

import com.audionplay.ui.Theme;
import javafx.geometry.Insets;
import javafx.geometry.Pos;
import javafx.scene.layout.*;

import static com.audionplay.ui.Theme.*;

/**
 * Painel de agendamentos.
 */
public class SchedulingPanel extends VBox {

    public SchedulingPanel() {
        super(10);

        HBox hdr = new HBox();
        hdr.setAlignment(Pos.CENTER_LEFT);
        hdr.getChildren().addAll(
            Theme.lbl("AGENDAMENTOS",
                "-fx-font-size:10px;-fx-font-weight:bold;-fx-text-fill:" + TEXT_SEC + ";"),
            Theme.hSpacer(),
            Theme.lbl("Atualizar",
                "-fx-font-size:11px;-fx-text-fill:" + BLUE + ";-fx-cursor:hand;")
        );

        VBox empty = new VBox(
            Theme.lbl("Nenhum evento agendado para hoje",
                "-fx-font-size:11px;-fx-text-fill:" + TEXT_MUT + ";")
        );
        empty.setStyle(
            "-fx-background-color:#14172280;" +
            "-fx-background-radius:8;-fx-padding:20;" +
            "-fx-border-color:" + BORDER + ";" +
            "-fx-border-radius:8;-fx-border-width:1;"
        );
        empty.setAlignment(Pos.CENTER);

        getChildren().addAll(hdr, empty);
    }
}
