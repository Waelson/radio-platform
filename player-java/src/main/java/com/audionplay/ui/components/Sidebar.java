package com.audionplay.ui.components;

import javafx.geometry.Insets;
import javafx.geometry.Pos;
import javafx.scene.Cursor;
import javafx.scene.control.Label;
import javafx.scene.layout.Priority;
import javafx.scene.layout.Region;
import javafx.scene.layout.VBox;

import java.util.function.Consumer;

/**
 * Rail de navegação lateral — visual idêntico ao player.html.
 *
 * Largura: 74px
 * Cada botão: 58px × 54px, borda-radius 12, ícone 18px, label 9px uppercase.
 * Ativo: cor cyan (#20e6ff), borda left 3px cyan, gradient bg.
 * Hover: clareamento e fundo sutil.
 */
public class Sidebar extends VBox {

    private static final String CYAN      = "#20e6ff";
    private static final String COLOR_OFF = "#4a6478";
    private static final String BG_RAIL   = "rgba(7,16,25,0.97)";
    private static final String BORDER    = "#20384c";

    private VBox activeBtn;
    private final Consumer<String> onNavigate;

    public Sidebar(Consumer<String> onNavigate) {
        super(4);
        this.onNavigate = onNavigate;

        setPrefWidth(74); setMinWidth(74); setMaxWidth(74);
        setStyle(
            "-fx-background-color:" + BG_RAIL + ";" +
            "-fx-border-color:" + BORDER + ";-fx-border-width:0 1 0 0;"
        );
        setPadding(new Insets(10, 6, 10, 6));
        setAlignment(Pos.TOP_CENTER);

        VBox noAr    = railBtn("▶",    "NO AR",    "NO AR");
        VBox catalog = railBtn("⊞",    "CATÁLOGO", "CATÁLOGO");
        VBox rotacao = railBtn("↻",    "ROTAÇÃO",  "ROTAÇÃO");
        VBox config  = railBtn("⚙",    "CONFIG",   "CONFIG");

        activeBtn = noAr;
        applyActive(noAr);

        Region spacer = new Region();
        VBox.setVgrow(spacer, Priority.ALWAYS);

        getChildren().addAll(noAr, catalog, rotacao, spacer, config);
    }

    public Sidebar() { this(s -> {}); }

    // ── Fábrica de botão ──────────────────────────────────────────────────────

    private VBox railBtn(String icon, String label, String key) {
        Label icoLbl = new Label(icon);
        icoLbl.setStyle(
            "-fx-font-size:18px;-fx-text-fill:" + COLOR_OFF + ";-fx-line-height:1;"
        );

        Label lblLbl = new Label(label);
        lblLbl.setStyle(
            "-fx-font-size:9px;-fx-font-weight:bold;-fx-text-fill:" + COLOR_OFF + ";" +
            "-fx-letter-spacing:.3px;"
        );

        VBox btn = new VBox(3, icoLbl, lblLbl);
        btn.setAlignment(Pos.CENTER);
        btn.setPrefWidth(58);
        btn.setMinHeight(54);
        btn.setPadding(new Insets(6, 3, 6, 3));
        btn.setCursor(Cursor.HAND);
        applyInactive(btn);

        btn.setOnMouseEntered(e -> { if (btn != activeBtn) applyHover(btn); });
        btn.setOnMouseExited(e  -> { if (btn != activeBtn) applyInactive(btn); });
        btn.setOnMouseClicked(e -> {
            if (activeBtn != null) applyInactive(activeBtn);
            applyActive(btn);
            activeBtn = btn;
            onNavigate.accept(key);
        });

        return btn;
    }

    // ── Estilos de estado ─────────────────────────────────────────────────────

    private static void applyInactive(VBox btn) {
        btn.setStyle(
            "-fx-background-color:transparent;" +
            "-fx-border-color:transparent;-fx-border-radius:12;-fx-background-radius:12;"
        );
        setChildrenColor(btn, COLOR_OFF);
    }

    private static void applyHover(VBox btn) {
        btn.setStyle(
            "-fx-background-color:rgba(255,255,255,0.04);" +
            "-fx-border-color:transparent;-fx-border-radius:12;-fx-background-radius:12;"
        );
        setChildrenColor(btn, "#c8c8c8");
    }

    private static void applyActive(VBox btn) {
        btn.setStyle(
            "-fx-background-color:linear-gradient(from 0% 0% to 100% 100%, rgba(32,230,255,0.12), rgba(57,120,255,0.05));" +
            "-fx-border-color:" + CYAN + " transparent transparent transparent;" +
            "-fx-border-width:0 0 0 3;" +
            "-fx-border-radius:12;-fx-background-radius:12;"
        );
        setChildrenColor(btn, CYAN);
    }

    private static void setChildrenColor(VBox btn, String color) {
        btn.getChildren().forEach(c -> {
            if (c instanceof Label lbl) {
                String s = lbl.getStyle().replaceAll("-fx-text-fill:[^;]+;", "");
                lbl.setStyle(s + "-fx-text-fill:" + color + ";");
            }
        });
    }
}
