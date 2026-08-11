package com.audionplay.ui.components;

import com.audionplay.ui.Theme;
import javafx.geometry.Insets;
import javafx.geometry.Pos;
import javafx.scene.control.Label;
import javafx.scene.layout.*;

import static com.audionplay.ui.Theme.*;

/**
 * Painel de agendamentos — visual fiel ao or-card "Agendamentos" do player.html.
 */
public class SchedulingPanel extends VBox {

    public SchedulingPanel() {
        super(0);
        setStyle(OR_CARD);
        VBox.setVgrow(this, Priority.ALWAYS);

        // ── Header com botão Atualizar ─────────────────────────────────────────
        Label refreshBtn = new Label("Atualizar");
        refreshBtn.setStyle(
            "-fx-pref-height:27;" +
            "-fx-background-color:#000000;" +
            "-fx-border-color:#20384c;-fx-border-width:1;" +
            "-fx-border-radius:7;-fx-background-radius:7;" +
            "-fx-padding:0 8 0 8;" +
            "-fx-font-size:10px;-fx-text-fill:#88a0b5;-fx-cursor:hand;"
        );

        HBox hdr = Theme.orCardHeader("Agendamentos", refreshBtn);

        // ── Lista (or-ctx-list) ────────────────────────────────────────────────
        VBox list = new VBox();
        list.setPadding(new Insets(7));
        VBox.setVgrow(list, Priority.ALWAYS);

        Label empty = new Label("Nenhum evento agendado para hoje");
        empty.setStyle("-fx-font-size:11px;-fx-text-fill:#4a6478;");
        empty.setPadding(new Insets(16, 0, 16, 0));
        empty.setMaxWidth(Double.MAX_VALUE);
        empty.setAlignment(Pos.CENTER);
        list.getChildren().add(empty);

        getChildren().addAll(hdr, list);
    }
}
