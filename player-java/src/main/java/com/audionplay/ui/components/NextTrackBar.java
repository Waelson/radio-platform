package com.audionplay.ui.components;

import com.audionplay.ui.Theme;
import javafx.geometry.Insets;
import javafx.geometry.Pos;
import javafx.scene.control.Label;
import javafx.scene.layout.*;

/**
 * Card "Próximo no Ar" — visual fiel ao .next-deck do player.html.
 *
 * Layout: [next-cover 74px] [info: label+título+artista] [next-count: dur+DURAÇÃO]
 */
public class NextTrackBar extends HBox {

    private final Label titleLbl;
    private final Label artistLbl;
    private final Label durLbl;

    public NextTrackBar() {
        super(13);
        setAlignment(Pos.CENTER_LEFT);
        setPadding(new Insets(12));
        setMinHeight(118);
        setStyle(
            "-fx-background-color:#0e1b28;" +
            "-fx-background-radius:14;" +
            "-fx-border-color:#20384c;" +
            "-fx-border-radius:14;-fx-border-width:1;"
        );

        // ── Cover (♫ com gradiente) ───────────────────────────────────────────
        Label coverIcon = new Label("♫");
        coverIcon.setStyle("-fx-font-size:26px;-fx-text-fill:#3978ff;");

        StackPane cover = new StackPane(coverIcon);
        cover.setAlignment(Pos.CENTER);
        cover.setPrefWidth(92);
        cover.setPrefHeight(74);
        cover.setMinWidth(92);
        cover.setMaxWidth(92);
        cover.setStyle(
            "-fx-background-color:linear-gradient(from 100% 0% to 0% 100%," +
                "rgba(57,120,255,0.20),rgba(211,106,255,0.08))," +
                "#0b1723;" +
            "-fx-background-radius:11;" +
            "-fx-border-color:#20384c;-fx-border-width:1;" +
            "-fx-border-radius:11;"
        );

        // ── Info (label + título + artista) ───────────────────────────────────
        Label nextLabel = new Label("Próximo no ar");
        nextLabel.setStyle(
            "-fx-font-size:10px;-fx-font-weight:bold;-fx-text-fill:#3978ff;"
        );

        titleLbl = new Label("—");
        titleLbl.setStyle(
            "-fx-font-size:18px;-fx-font-weight:bold;-fx-text-fill:#eef7ff;"
        );
        titleLbl.setMaxWidth(Double.MAX_VALUE);

        artistLbl = new Label("Nenhuma faixa na fila");
        artistLbl.setStyle("-fx-font-size:12px;-fx-text-fill:#88a0b5;");
        artistLbl.setMaxWidth(Double.MAX_VALUE);

        VBox info = new VBox(0, nextLabel, titleLbl, artistLbl);
        VBox.setMargin(titleLbl,  new Insets(5, 0, 0, 0));
        VBox.setMargin(artistLbl, new Insets(4, 0, 0, 0));
        info.setAlignment(Pos.CENTER_LEFT);
        HBox.setHgrow(info, Priority.ALWAYS);

        // ── Duration (valor + "DURAÇÃO") ──────────────────────────────────────
        durLbl = new Label("—");
        durLbl.setStyle(
            "-fx-font-size:23px;-fx-font-weight:800;-fx-text-fill:#3978ff;" +
            "-fx-font-family:'Courier New',monospace;"
        );

        Label durLabel = new Label("DURAÇÃO");
        durLabel.setStyle("-fx-font-size:9px;-fx-text-fill:#88a0b5;");

        VBox durBox = new VBox(2, durLbl, durLabel);
        durBox.setAlignment(Pos.CENTER_RIGHT);
        durBox.setMinWidth(76);

        getChildren().addAll(cover, info, durBox);
    }

    public void update(String title, String artist, String duration) {
        titleLbl.setText(title != null && !title.isBlank() ? title : "—");
        artistLbl.setText(artist != null && !artist.isBlank() ? artist : "—");
        durLbl.setText(duration != null && !duration.isBlank() ? duration : "—");
    }

    public void clear() {
        titleLbl.setText("—");
        artistLbl.setText("Nenhuma faixa na fila");
        durLbl.setText("—");
    }
}
