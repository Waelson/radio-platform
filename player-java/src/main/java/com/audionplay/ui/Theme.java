package com.audionplay.ui;

import javafx.geometry.Pos;
import javafx.scene.control.Button;
import javafx.scene.control.Label;
import javafx.scene.layout.HBox;
import javafx.scene.layout.Priority;
import javafx.scene.layout.Region;

/**
 * Paleta de cores e utilitários visuais compartilhados por todos os componentes.
 */
public final class Theme {

    // ── Cores de fundo ────────────────────────────────────────────────────────
    public static final String BG_DEEP  = "#0E1017";
    public static final String BG_MAIN  = "#13161E";
    public static final String BG_PANEL = "#1A1D27";
    public static final String BG_CARD  = "#1E2130";

    // ── Cores de borda e texto ────────────────────────────────────────────────
    public static final String BORDER   = "#2A2E42";
    public static final String TEXT_PRI = "#E8EAF0";
    public static final String TEXT_SEC = "#8B8FA8";
    public static final String TEXT_MUT = "#4A4E6A";

    // ── Cores de destaque ─────────────────────────────────────────────────────
    public static final String BLUE  = "#4A6CF7";
    public static final String GREEN = "#2ECC71";
    public static final String RED   = "#E74C3C";

    // ── Gradientes e acentos dos cards da fila (fiéis ao player.html) ─────────
    public static final String GRAD_MUS  =
        "linear-gradient(to bottom,#1a6875 0%,#155e69 22%,#0d4f5a 47%,#022830 49%,#011e24 72%,#011318 100%)";
    public static final String GRAD_JIN  =
        "linear-gradient(to bottom,#6a2080 0%,#5e1c72 22%,#521863 47%,#220830 49%,#170522 72%,#0f0316 100%)";
    public static final String GRAD_HORA =
        "linear-gradient(to bottom,#5752aa 0%,#4a469d 22%,#403c94 47%,#171473 49%,#0e0c68 72%,#07065a 100%)";
    public static final String ACC_MUS   = "#00d4ff";   // barra lateral música (ciano)
    public static final String ACC_JIN   = "#d36aff";   // barra lateral vinheta/jingle (roxo)
    public static final String ACC_HORA  = "#ffffff";   // barra lateral hora certa (branco)
    public static final String BDR_MUS   = "#7ad4e0";   // borda música
    public static final String BDR_JIN   = "#b87ad4";   // borda vinheta/jingle
    public static final String BDR_HORA  = "#9d9ab8";   // borda hora certa

    private Theme() {}

    // ── Utilitários visuais ───────────────────────────────────────────────────

    /** Cria um Label com estilo inline. */
    public static Label lbl(String text, String style) {
        Label l = new Label(text);
        l.setStyle(style);
        return l;
    }

    /** Spacer horizontal que expande dentro de um HBox. */
    public static Region hSpacer() {
        Region r = new Region();
        HBox.setHgrow(r, Priority.ALWAYS);
        return r;
    }

    /** Botão com borda colorida e fundo transparente. */
    public static Button outlineBtn(String text, String color) {
        Button b = new Button(text);
        b.setStyle(
            "-fx-background-color:transparent;" +
            "-fx-text-fill:" + color + ";" +
            "-fx-border-color:" + color + "55;" +
            "-fx-border-radius:6;-fx-border-width:1;" +
            "-fx-padding:5 10 5 10;-fx-cursor:hand;"
        );
        return b;
    }

    /** Formata segundos como MM:SS. */
    public static String formatTime(double sec) {
        if (sec < 0) sec = 0;
        return String.format("%02d:%02d", (int)(sec / 60), (int)(sec % 60));
    }
}
