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
 * Painel de streaming — visual fiel ao or-card "Streaming" do player.html.
 */
public class StreamingPanel extends VBox {

    public StreamingPanel() {
        super(0);
        setStyle(OR_CARD);
        VBox.setVgrow(this, Priority.ALWAYS);

        // ── Header com botão + Servidor ────────────────────────────────────────
        Label addBtn = new Label("+ Servidor");
        addBtn.setStyle(
            "-fx-pref-height:27;" +
            "-fx-background-color:#000000;" +
            "-fx-border-color:#20384c;-fx-border-width:1;" +
            "-fx-border-radius:7;-fx-background-radius:7;" +
            "-fx-padding:0 8 0 8;" +
            "-fx-font-size:10px;-fx-text-fill:#88a0b5;-fx-cursor:hand;"
        );

        HBox hdr = Theme.orCardHeader("Streaming", addBtn);

        // ── Card list ──────────────────────────────────────────────────────────
        VBox cardList = new VBox(7);
        cardList.setPadding(new Insets(7));
        VBox.setVgrow(cardList, Priority.ALWAYS);

        cardList.getChildren().add(buildStreamingCard(
            "CasterFM",
            "sapirncast.fm:19656 · MP3 · 96kbps",
            false
        ));

        getChildren().addAll(hdr, cardList);
    }

    // ── Builder de streaming-card ─────────────────────────────────────────────

    private VBox buildStreamingCard(String name, String info, boolean connected) {
        VBox card = new VBox(6);
        card.setPadding(new Insets(10, 11, 46, 11));
        card.setStyle(
            "-fx-background-color:#0a1520;" +
            "-fx-border-color:#20384c;-fx-border-width:1;" +
            "-fx-border-radius:10;-fx-background-radius:10;"
        );

        // streaming-card-header
        Circle dot = new Circle(3.5, Color.web("#4a6478"));

        Label nameLbl = new Label(name);
        nameLbl.setStyle(
            "-fx-font-size:12px;-fx-font-weight:700;-fx-text-fill:#d7e3ec;"
        );
        HBox.setHgrow(nameLbl, Priority.ALWAYS);

        Label badge = new Label("OFF");
        badge.setStyle(
            "-fx-font-size:9px;-fx-font-weight:800;" +
            "-fx-padding:2 7 2 7;-fx-background-radius:4;-fx-border-radius:4;" +
            "-fx-background-color:rgba(74,100,120,0.25);" +
            "-fx-border-color:#20384c;-fx-border-width:1;" +
            "-fx-text-fill:#4a6478;"
        );

        HBox cardHdr = new HBox(6, dot, nameLbl, badge);
        cardHdr.setAlignment(Pos.CENTER_LEFT);

        // info line
        Label infoLbl = new Label(info);
        infoLbl.setStyle("-fx-font-size:10px;-fx-text-fill:#4a6478;");

        card.getChildren().addAll(cardHdr, infoLbl);

        // ── Actions (simulam position: absolute bottom) ────────────────────────
        HBox actions = new HBox(5,
            streamBtn("Conectar", true,  false),
            streamBtn("Editar",   false, false),
            streamBtn("Remover",  false, false)
        );
        actions.setPadding(new Insets(8, 0, 0, 0));
        card.getChildren().add(actions);

        return card;
    }

    private Button streamBtn(String text, boolean primary, boolean danger) {
        Button b = new Button(text);
        String borderColor, textColor;
        if (primary) {
            borderColor = "rgba(32,230,255,0.35)";
            textColor   = "#00d4ff";
        } else if (danger) {
            borderColor = "rgba(248,114,114,0.35)";
            textColor   = "#f87272";
        } else {
            borderColor = "#20384c";
            textColor   = "#88a0b5";
        }
        b.setStyle(
            "-fx-font-size:10px;-fx-font-weight:700;" +
            "-fx-padding:3 10 3 10;-fx-background-radius:6;-fx-border-radius:6;" +
            "-fx-border-color:" + borderColor + ";-fx-border-width:1;" +
            "-fx-background-color:#000000;" +
            "-fx-text-fill:" + textColor + ";" +
            "-fx-cursor:hand;"
        );
        return b;
    }
}
