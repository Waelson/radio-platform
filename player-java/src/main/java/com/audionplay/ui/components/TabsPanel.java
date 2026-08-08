package com.audionplay.ui.components;

import com.audionplay.ui.Theme;
import javafx.geometry.Insets;
import javafx.geometry.Pos;
import javafx.scene.control.Label;
import javafx.scene.control.ScrollPane;
import javafx.scene.layout.*;

import static com.audionplay.ui.Theme.*;

/**
 * Painel com abas Hot Keys — visual fiel ao hkp-panel do player.html.
 */
public class TabsPanel extends VBox {

    public TabsPanel() {
        super(0);
        setStyle(
            "-fx-background-color:#0a1520;" +
            "-fx-border-color:#20384c;-fx-border-width:1;" +
            "-fx-border-radius:12;-fx-background-radius:12;"
        );
        VBox.setVgrow(this, Priority.ALWAYS);

        getChildren().addAll(
            buildTabs(),
            buildHeader(),
            buildGridWrap()
        );
    }

    // ── hkp-tabs ──────────────────────────────────────────────────────────────

    private HBox buildTabs() {
        HBox tabs = new HBox(5,
            tabBtn("Hot Keys",       true),
            tabBtn("Voice Track",    false),
            tabBtn("Routing rápido", false),
            tabBtn("Notas",          false)
        );
        tabs.setPadding(new Insets(7, 8, 0, 8));
        tabs.setStyle("-fx-background-color:rgba(5,14,22,0.4);");
        return tabs;
    }

    private Label tabBtn(String text, boolean active) {
        Label l = new Label(text);
        l.setPrefHeight(30);
        l.setPadding(new Insets(0, 10, 0, 10));
        l.setStyle("-fx-font-size:12px;-fx-font-weight:700;-fx-cursor:hand;" +
            (active
                ? "-fx-text-fill:#00d4ff;" +
                  "-fx-background-color:#0a1520;" +
                  "-fx-border-color:#20384c #20384c #0a1520 #20384c;-fx-border-width:1;" +
                  "-fx-background-radius:8 8 0 0;-fx-border-radius:8 8 0 0;"
                : "-fx-text-fill:#2a3f52;-fx-background-color:transparent;"));
        return l;
    }

    // ── hkp-header ────────────────────────────────────────────────────────────

    private HBox buildHeader() {
        // profile pill
        Label ppLabel = new Label("PERFIL");
        ppLabel.setStyle("-fx-font-size:9px;-fx-font-weight:700;-fx-text-fill:#4a6478;");

        Label ppName = new Label("—");
        ppName.setStyle("-fx-font-size:12px;-fx-font-weight:700;-fx-text-fill:#eef7ff;");

        VBox ppText = new VBox(0, ppLabel, ppName);

        Label ppArrow = new Label("▾");
        ppArrow.setStyle("-fx-font-size:9px;-fx-text-fill:#4a6478;");

        HBox pill = new HBox(8, ppText, ppArrow);
        pill.setAlignment(Pos.CENTER_LEFT);
        pill.setPadding(new Insets(4, 10, 4, 10));
        pill.setMinWidth(180);
        pill.setStyle(
            "-fx-background-color:rgba(255,255,255,0.04);" +
            "-fx-border-color:#20384c;-fx-border-width:1;" +
            "-fx-border-radius:7;-fx-background-radius:7;" +
            "-fx-cursor:hand;"
        );

        // reload button
        Label reloadBtn = new Label("↺");
        reloadBtn.setPrefWidth(40);
        reloadBtn.setMinHeight(32);
        reloadBtn.setAlignment(Pos.CENTER);
        reloadBtn.setStyle(
            "-fx-font-size:20px;-fx-text-fill:#9cb5ba;" +
            "-fx-background-color:transparent;-fx-cursor:hand;"
        );

        // open button
        Label openBtn = new Label("⧉");
        openBtn.setPrefWidth(40);
        openBtn.setMinHeight(32);
        openBtn.setAlignment(Pos.CENTER);
        openBtn.setStyle(
            "-fx-font-size:17px;-fx-text-fill:#9cb5ba;" +
            "-fx-background-color:transparent;-fx-cursor:hand;"
        );

        Region spacer = new Region();
        HBox.setHgrow(spacer, Priority.ALWAYS);

        HBox hdr = new HBox(8, pill, reloadBtn, spacer, openBtn);
        hdr.setAlignment(Pos.CENTER_LEFT);
        hdr.setPadding(new Insets(6, 10, 6, 10));
        hdr.setStyle("-fx-border-color:#20384c;-fx-border-width:0 0 1 0;");
        return hdr;
    }

    // ── hkp-grid-wrap ─────────────────────────────────────────────────────────

    private VBox buildGridWrap() {
        GridPane grid = new GridPane();
        grid.setHgap(5);
        grid.setVgap(5);

        ColumnConstraints cc = new ColumnConstraints();
        cc.setHgrow(Priority.ALWAYS);
        cc.setPercentWidth(25);
        grid.getColumnConstraints().addAll(cc, clone(cc), clone(cc), clone(cc));

        // Sample hot key cards
        String[][] cards = {
            {"1", "Aqui você ouve as melhores",    "VINHETA", "00:03"},
            {"2", "A rádio que faz a sua cabeça",  "VINHETA", "00:03"},
            {"3", "As melhores músicas estão aqui","VINHETA", "00:02"},
            {"4", "", "", ""},
            {"5", "", "", ""},
            {"6", "", "", ""},
            {"7", "", "", ""},
            {"8", "", "", ""},
        };

        for (int i = 0; i < cards.length; i++) {
            grid.add(hkpCard(cards[i][0], cards[i][1], cards[i][2], cards[i][3]),
                i % 4, i / 4);
        }

        ScrollPane scroll = new ScrollPane(grid);
        scroll.setFitToWidth(true);
        scroll.setFitToHeight(false);
        scroll.setHbarPolicy(ScrollPane.ScrollBarPolicy.NEVER);
        scroll.setStyle(
            "-fx-background-color:#0a1520;-fx-background:#0a1520;" +
            "-fx-border-color:transparent;"
        );
        VBox.setVgrow(scroll, Priority.ALWAYS);

        VBox wrap = new VBox(scroll);
        wrap.setPadding(new Insets(7));
        VBox.setVgrow(wrap, Priority.ALWAYS);
        return wrap;
    }

    // ── hkp-card ─────────────────────────────────────────────────────────────

    private VBox hkpCard(String slot, String name, String type, String time) {
        boolean empty = name == null || name.isEmpty();

        // card border color (vinheta = cyan default)
        String borderColor = "rgba(32,230,255,0.45)";
        String borderLeft  = "rgba(32,230,255,0.75)";

        VBox card = new VBox(0);
        card.setMinHeight(78);
        card.setStyle(
            "-fx-background-color:#07111a;" +
            "-fx-border-color:" + borderColor + ";-fx-border-width:1;" +
            "-fx-border-left-width:3;-fx-border-radius:10;-fx-background-radius:10;" +
            "-fx-cursor:hand;"
        );

        if (empty) {
            card.setStyle(
                "-fx-background-color:rgba(6,16,25,0.5);" +
                "-fx-border-color:rgba(32,56,76,0.4);-fx-border-width:1;" +
                "-fx-border-radius:10;-fx-background-radius:10;"
            );
            card.getChildren().add(new Label());
            return card;
        }

        // inner content
        VBox inner = new VBox(2);
        inner.setPadding(new Insets(5, 7, 4, 7));
        VBox.setVgrow(inner, Priority.ALWAYS);

        // top row: slot + cue
        Label slotLbl = new Label(slot);
        slotLbl.setStyle(
            "-fx-font-size:9px;-fx-font-weight:800;" +
            "-fx-text-fill:rgba(255,255,255,0.75);" +
            "-fx-background-color:rgba(0,0,0,0.30);" +
            "-fx-background-radius:4;-fx-padding:1 5 1 5;"
        );

        Label cueLbl = new Label("🎧");
        cueLbl.setStyle("-fx-font-size:14px;-fx-text-fill:rgba(255,255,255,0.5);-fx-cursor:hand;");
        cueLbl.setPrefWidth(20); cueLbl.setPrefHeight(20);
        cueLbl.setAlignment(Pos.CENTER);

        Region topSpacer = new Region();
        HBox.setHgrow(topSpacer, Priority.ALWAYS);

        HBox topRow = new HBox(4, slotLbl, topSpacer, cueLbl);
        topRow.setAlignment(Pos.TOP_LEFT);

        // name
        Label nameLbl = new Label(name);
        nameLbl.setStyle(
            "-fx-font-size:11px;-fx-font-weight:700;-fx-text-fill:#ffffff;" +
            "-fx-wrap-text:true;"
        );
        nameLbl.setWrapText(true);

        // sub (type)
        Label subLbl = new Label(type);
        subLbl.setStyle("-fx-font-size:10px;-fx-text-fill:rgba(255,255,255,0.55);");

        // time
        Label timeLbl = new Label(time);
        timeLbl.setStyle(
            "-fx-font-size:10px;-fx-font-weight:800;-fx-text-fill:rgba(255,255,255,0.8);" +
            "-fx-font-family:'Courier New';"
        );

        inner.getChildren().addAll(topRow, nameLbl, subLbl, timeLbl);

        // progress bar (3px)
        Region bar = new Region();
        bar.setPrefHeight(3);
        bar.setStyle("-fx-background-color:rgba(255,255,255,0.1);");

        card.getChildren().addAll(inner, bar);
        return card;
    }

    // ── Helpers ───────────────────────────────────────────────────────────────

    private static ColumnConstraints clone(ColumnConstraints src) {
        ColumnConstraints cc = new ColumnConstraints();
        cc.setHgrow(src.getHgrow());
        cc.setPercentWidth(src.getPercentWidth());
        return cc;
    }
}
