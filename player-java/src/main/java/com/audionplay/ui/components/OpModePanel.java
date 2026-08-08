package com.audionplay.ui.components;

import com.audionplay.ui.Theme;
import javafx.geometry.Insets;
import javafx.geometry.Pos;
import javafx.scene.control.Label;
import javafx.scene.layout.*;

import static com.audionplay.ui.Theme.*;

/**
 * Painel Modo de Operação — visual fiel ao .or-card do player.html.
 */
public class OpModePanel extends VBox {

    public OpModePanel() {
        super(0);
        setStyle(OR_CARD);

        // ── Studio badge ──────────────────────────────────────────────────────
        Label studioBadge = new Label("Studio A");
        studioBadge.setStyle(
            "-fx-font-size:10px;-fx-font-weight:bold;-fx-text-fill:#88a0b5;" +
            "-fx-background-color:#0e1b28;" +
            "-fx-border-color:#20384c;-fx-border-width:1;" +
            "-fx-border-radius:99;-fx-background-radius:99;" +
            "-fx-padding:0 9 0 9;-fx-pref-height:24;"
        );

        // ── Header ────────────────────────────────────────────────────────────
        HBox hdr = Theme.orCardHeader("MODO DE OPERAÇÃO", studioBadge);

        // ── Botões de modo ────────────────────────────────────────────────────
        String inactiveStyle =
            "-fx-pref-height:50;-fx-max-width:Infinity;" +
            "-fx-background-color:rgba(255,255,255,0.025);" +
            "-fx-border-color:#20384c;-fx-border-width:1;" +
            "-fx-border-radius:10;-fx-background-radius:10;" +
            "-fx-text-fill:#88a0b5;-fx-font-size:10px;-fx-font-weight:bold;" +
            "-fx-cursor:hand;";
        String activeStyle =
            "-fx-pref-height:50;-fx-max-width:Infinity;" +
            "-fx-background-color:#36d399;" +
            "-fx-border-color:#36d399;-fx-border-width:1;" +
            "-fx-border-radius:10;-fx-background-radius:10;" +
            "-fx-text-fill:#031b12;-fx-font-size:10px;-fx-font-weight:bold;" +
            "-fx-cursor:hand;";

        Label liveAssist = new Label("Live Assist");
        liveAssist.setStyle(inactiveStyle);
        liveAssist.setMaxWidth(Double.MAX_VALUE);
        liveAssist.setAlignment(Pos.CENTER);
        HBox.setHgrow(liveAssist, Priority.ALWAYS);

        Label automatico = new Label("Automático");
        automatico.setStyle(activeStyle);
        automatico.setMaxWidth(Double.MAX_VALUE);
        automatico.setAlignment(Pos.CENTER);
        HBox.setHgrow(automatico, Priority.ALWAYS);

        HBox btns = new HBox(7, liveAssist, automatico);
        btns.setPadding(new Insets(9));

        getChildren().addAll(hdr, btns);
    }
}
