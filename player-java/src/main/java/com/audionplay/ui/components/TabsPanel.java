package com.audionplay.ui.components;

import com.audionplay.ui.Theme;
import javafx.geometry.Insets;
import javafx.geometry.Pos;
import javafx.scene.control.Label;
import javafx.scene.layout.*;

import static com.audionplay.ui.Theme.*;

/**
 * Painel com abas (Hot Keys, Voice Track, Routing, Notas).
 * Exibe o conteúdo da aba Hot Keys por padrão.
 */
public class TabsPanel extends VBox {

    public TabsPanel() {
        super(0);
        setStyle(
            "-fx-background-color:" + BG_PANEL + ";" +
            "-fx-background-radius:10;" +
            "-fx-border-color:" + BORDER + ";" +
            "-fx-border-radius:10;-fx-border-width:1;"
        );
        VBox.setVgrow(this, Priority.ALWAYS);

        HBox tabBar = new HBox(0,
            tabLbl("Hot Keys",      true),
            tabLbl("Voice Track",   false),
            tabLbl("Routing rápido",false),
            tabLbl("Notas",         false)
        );
        tabBar.setStyle("-fx-border-color:" + BORDER + ";-fx-border-width:0 0 1 0;");

        VBox content = buildHotKeys();
        VBox.setVgrow(content, Priority.ALWAYS);

        getChildren().addAll(tabBar, content);
    }

    private Label tabLbl(String text, boolean active) {
        Label l = new Label(text);
        l.setPadding(new Insets(11, 20, 11, 20));
        l.setStyle("-fx-font-size:12px;" + (active
            ? "-fx-font-weight:bold;-fx-text-fill:white;-fx-border-color:transparent transparent " + BLUE + " transparent;-fx-border-width:0 0 2 0;"
            : "-fx-text-fill:" + TEXT_SEC + ";"));
        return l;
    }

    private VBox buildHotKeys() {
        VBox v = new VBox(14);
        v.setPadding(new Insets(16));

        Label name = Theme.lbl("Vinhetas Curtas",
            "-fx-font-size:13px;-fx-font-weight:bold;-fx-text-fill:" + TEXT_PRI + ";" +
            "-fx-background-color:#14172280;-fx-background-radius:6;-fx-padding:6 12 6 12;" +
            "-fx-border-color:" + BORDER + ";-fx-border-radius:6;-fx-border-width:1;");

        HBox perfil = new HBox(10,
            new VBox(3,
                Theme.lbl("PERFIL", "-fx-font-size:9px;-fx-text-fill:" + TEXT_MUT + ";"),
                name
            ),
            Theme.lbl("↻", "-fx-font-size:15px;-fx-text-fill:" + TEXT_SEC + ";"),
            Theme.hSpacer(),
            Theme.lbl("⧉", "-fx-font-size:15px;-fx-text-fill:" + TEXT_SEC + ";")
        );
        perfil.setAlignment(Pos.CENTER_LEFT);

        HBox cards = new HBox(10,
            hotKeyCard("1", "Aqui voce ouve as melhores",     "00:03"),
            hotKeyCard("2", "A radio que faz a sua cabeca",    "00:03"),
            hotKeyCard("3", "As melhores musicas estao aqui",  "00:02")
        );

        v.getChildren().addAll(perfil, cards);
        return v;
    }

    private VBox hotKeyCard(String num, String title, String dur) {
        VBox card = new VBox(8);
        card.setPadding(new Insets(12));
        card.setStyle(
            "-fx-background-color:" + BG_CARD + ";" +
            "-fx-background-radius:8;-fx-border-color:" + BORDER + ";" +
            "-fx-border-radius:8;-fx-border-width:1;"
        );
        HBox.setHgrow(card, Priority.ALWAYS);

        HBox hdr = new HBox(
            Theme.lbl(num, "-fx-font-size:10px;-fx-text-fill:" + TEXT_MUT + ";")
        );
        hdr.getChildren().addAll(Theme.hSpacer(), Theme.lbl("🎧", "-fx-font-size:12px;"));

        Label tl = Theme.lbl(title, "-fx-font-size:12px;-fx-font-weight:bold;-fx-text-fill:" + TEXT_PRI + ";");
        tl.setWrapText(true);

        HBox footer = new HBox(8,
            Theme.lbl("VINHETA",
                "-fx-font-size:9px;-fx-text-fill:#B08FE8;" +
                "-fx-background-color:#241440;-fx-background-radius:3;-fx-padding:1 4 1 4;")
        );
        footer.getChildren().addAll(Theme.hSpacer(),
            Theme.lbl(dur, "-fx-font-size:11px;-fx-text-fill:" + TEXT_SEC + ";"));

        card.getChildren().addAll(hdr, tl, footer);
        return card;
    }
}
