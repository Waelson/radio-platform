package com.audionplay.ui.components;

import com.audionplay.ui.Theme;
import javafx.geometry.Insets;
import javafx.geometry.Pos;
import javafx.scene.layout.*;
import javafx.scene.paint.Color;
import javafx.scene.shape.Rectangle;

import static com.audionplay.ui.Theme.*;

/**
 * Barra "Próximo no Ar" exibida abaixo do painel Now Playing.
 */
public class NextTrackBar extends HBox {

    private final javafx.scene.control.Label titleLbl;
    private final javafx.scene.control.Label metaLbl;
    private final javafx.scene.control.Label durLbl;

    public NextTrackBar() {
        super(14);
        setPadding(new Insets(14, 16, 14, 16));
        setAlignment(Pos.CENTER_LEFT);
        setStyle(
            "-fx-background-color:" + BG_PANEL + ";" +
            "-fx-background-radius:10;" +
            "-fx-border-color:" + BORDER + ";" +
            "-fx-border-radius:10;-fx-border-width:1;"
        );
        setMinHeight(76);

        StackPane iconWrap = new StackPane();
        Rectangle ib = new Rectangle(50, 50);
        ib.setFill(Color.web("#1A2540"));
        ib.setArcWidth(8); ib.setArcHeight(8);
        iconWrap.getChildren().addAll(ib, Theme.lbl("♪", "-fx-font-size:22px;-fx-text-fill:" + BLUE + ";"));

        titleLbl = Theme.lbl("—", "-fx-font-size:15px;-fx-font-weight:bold;-fx-text-fill:white;");
        metaLbl  = Theme.lbl("Nenhuma faixa na fila", "-fx-font-size:11px;-fx-text-fill:" + TEXT_SEC + ";");
        durLbl   = Theme.lbl("--:--", "-fx-font-size:22px;-fx-font-weight:bold;-fx-text-fill:" + BLUE + ";-fx-font-family:'Courier New';");

        VBox info = new VBox(4,
            Theme.lbl("PRÓXIMO NO AR", "-fx-font-size:10px;-fx-font-weight:bold;-fx-text-fill:" + BLUE + ";"),
            titleLbl,
            metaLbl
        );
        HBox.setHgrow(info, Priority.ALWAYS);

        VBox dur = new VBox(4, durLbl, Theme.lbl("DURAÇÃO", "-fx-font-size:9px;-fx-text-fill:" + TEXT_SEC + ";"));
        dur.setAlignment(Pos.CENTER_RIGHT);

        getChildren().addAll(iconWrap, info, dur);
    }

    public void update(String title, String artist, String duration) {
        titleLbl.setText(title);
        metaLbl.setText(artist != null && !artist.isBlank() ? artist : "—");
        durLbl.setText(duration);
    }

    public void clear() {
        titleLbl.setText("—");
        metaLbl.setText("Nenhuma faixa na fila");
        durLbl.setText("--:--");
    }
}
