package com.audionplay.ui.components;

import com.audionplay.ui.Theme;
import javafx.animation.KeyFrame;
import javafx.animation.Timeline;
import javafx.geometry.Insets;
import javafx.geometry.Pos;
import javafx.scene.control.Label;
import javafx.scene.image.Image;
import javafx.scene.image.ImageView;
import javafx.scene.layout.HBox;
import javafx.scene.layout.Priority;
import javafx.scene.layout.Region;
import javafx.scene.layout.StackPane;
import javafx.scene.layout.VBox;
import javafx.scene.paint.Color;
import javafx.scene.shape.Circle;
import javafx.scene.effect.DropShadow;
import javafx.stage.Popup;
import javafx.util.Duration;

import java.time.LocalDateTime;
import java.time.format.DateTimeFormatter;
import java.util.Locale;

import static com.audionplay.ui.Theme.*;

/**
 * Barra superior: logo, espaçador, card do usuário e relógio.
 * Estilo fiel ao design do player.html.
 */
public class TopBar extends HBox {

    private final Label clockLabel;
    private final Label dateLabel;
    private Popup userMenu;

    public TopBar() {
        super(0);
        setAlignment(Pos.CENTER_LEFT);
        setMinHeight(66);
        setStyle(
            "-fx-background-color:#000000;" +
            "-fx-border-color:#20384c;-fx-border-width:0 0 1 0;"
        );
        setPadding(new Insets(0, 20, 0, 20));

        clockLabel = new Label("--:--:--");
        clockLabel.setStyle(
            "-fx-font-family:'Courier New',monospace;" +
            "-fx-font-size:23px;" +
            "-fx-font-weight:bold;" +
            "-fx-text-fill:#eef7ff;" +
            "-fx-font-variant-numeric:tabular-nums;"
        );

        dateLabel = new Label("...");
        dateLabel.setStyle(
            "-fx-font-size:10px;" +
            "-fx-text-fill:#88a0b5;"
        );

        HBox.setHgrow(buildNavTabs(), Priority.ALWAYS);

        getChildren().addAll(
            buildLogo(),
            buildNavTabs(),
            buildSpacer(),
            buildUserCard(),
            buildClockBlock()
        );

        startClock();
    }

    // ── Logo ──────────────────────────────────────────────────────────────────

    private HBox buildLogo() {
        javafx.scene.Node icon;
        var resource = getClass().getResourceAsStream("/audion-logo.png");
        if (resource != null) {
            ImageView iv = new ImageView(new Image(resource));
            iv.setFitHeight(46);
            iv.setPreserveRatio(true);
            iv.setSmooth(true);
            icon = iv;
        } else {
            // fallback se o arquivo não for encontrado
            StackPane circle = new StackPane(
                new Circle(20, Color.web(BLUE)),
                Theme.lbl("A", "-fx-font-size:18px;-fx-font-weight:bold;-fx-text-fill:white;")
            );
            icon = circle;
        }

        VBox text = new VBox(1,
            Theme.lbl("Audion Play",     "-fx-font-size:16px;-fx-font-weight:bold;-fx-text-fill:#eef7ff;"),
            Theme.lbl("BROADCAST SUITE", "-fx-font-size:10px;-fx-text-fill:#88a0b5;")
        );
        text.setAlignment(Pos.CENTER_LEFT);

        HBox logo = new HBox(12, icon, text);
        logo.setAlignment(Pos.CENTER_LEFT);
        logo.setPadding(new Insets(0, 20, 0, 0));
        return logo;
    }

    // ── Nav tabs (placeholder) ────────────────────────────────────────────────

    private HBox buildNavTabs() {
        HBox nav = new HBox(4);
        nav.setAlignment(Pos.CENTER);
        HBox.setHgrow(nav, Priority.ALWAYS);
        return nav;
    }

    // ── Spacer ────────────────────────────────────────────────────────────────

    private javafx.scene.layout.Region buildSpacer() {
        javafx.scene.layout.Region sp = new javafx.scene.layout.Region();
        HBox.setHgrow(sp, Priority.ALWAYS);
        return sp;
    }

    // ── User card ─────────────────────────────────────────────────────────────

    private HBox buildUserCard() {
        // Avatar circular com iniciais
        StackPane avatar = new StackPane(
            new Circle(15, Color.web("#1a3050")),
            Theme.lbl("WN", "-fx-font-size:10px;-fx-font-weight:bold;-fx-text-fill:white;")
        );

        // Nome e cargo
        Label nameLbl = Theme.lbl("Waelson Nunes",
            "-fx-font-size:13px;-fx-font-weight:bold;-fx-text-fill:#eef7ff;");
        Label roleLbl = Theme.lbl("ADMIN",
            "-fx-font-size:10px;-fx-text-fill:#88a0b5;");
        VBox info = new VBox(1, nameLbl, roleLbl);
        info.setAlignment(Pos.CENTER_LEFT);

        // Chevron
        Label chevron = Theme.lbl("▾",
            "-fx-font-size:10px;-fx-text-fill:#88a0b5;");

        // Pill container
        HBox pill = new HBox(8, avatar, info, chevron);
        pill.setAlignment(Pos.CENTER_LEFT);
        pill.setPadding(new Insets(5, 12, 5, 8));
        pill.setMaxHeight(javafx.scene.layout.Region.USE_PREF_SIZE);
        pill.setStyle(
            "-fx-background-color:#000000;" +
            "-fx-border-color:#20384c;-fx-border-width:1;" +
            "-fx-border-radius:999;-fx-background-radius:999;" +
            "-fx-cursor:hand;"
        );

        userMenu = buildUserMenu();

        pill.setOnMouseClicked(e -> {
            if (userMenu.isShowing()) {
                userMenu.hide();
            } else {
                javafx.geometry.Bounds b = pill.localToScreen(pill.getBoundsInLocal());
                userMenu.show(pill, b.getMinX(), b.getMaxY() + 6);
            }
        });

        HBox wrap = new HBox(pill);
        wrap.setAlignment(Pos.CENTER);
        wrap.setMaxHeight(javafx.scene.layout.Region.USE_PREF_SIZE);
        wrap.setPadding(new Insets(0, 24, 0, 0));
        return wrap;
    }

    // ── User dropdown menu ────────────────────────────────────────────────────

    private Popup buildUserMenu() {
        VBox box = new VBox(0);
        box.setStyle(
            "-fx-background-color:#0d1a1f;" +
            "-fx-border-color:rgba(45,216,255,0.22);-fx-border-width:1;" +
            "-fx-border-radius:8;-fx-background-radius:8;" +
            "-fx-padding:4 0 4 0;"
        );
        box.setMinWidth(180);

        DropShadow shadow = new DropShadow();
        shadow.setColor(Color.color(0, 0, 0, 0.55));
        shadow.setRadius(28);
        shadow.setOffsetY(8);
        box.setEffect(shadow);

        box.getChildren().addAll(
            menuItem("🔑  Alterar senha",    false),
            menuSep(),
            menuItem("⇄  Trocar operador", true),
            menuItem("⏻  Sair",             true)
        );

        Popup popup = new Popup();
        popup.setAutoHide(true);
        popup.getContent().add(box);
        return popup;
    }

    private javafx.scene.Node menuItem(String text, boolean danger) {
        Label lbl = new Label(text);
        String normalColor = danger ? "#ff6b6b" : "#dceff2";
        String hoverBg     = danger ? "rgba(255,70,70,0.10)" : "rgba(45,216,255,0.10)";

        lbl.setStyle(
            "-fx-font-size:13px;-fx-font-weight:600;" +
            "-fx-text-fill:" + normalColor + ";" +
            "-fx-padding:9 16 9 16;" +
            "-fx-cursor:hand;"
        );
        lbl.setMaxWidth(Double.MAX_VALUE);

        lbl.setOnMouseEntered(e -> lbl.setStyle(
            "-fx-font-size:13px;-fx-font-weight:600;" +
            "-fx-text-fill:" + normalColor + ";" +
            "-fx-padding:9 16 9 16;" +
            "-fx-cursor:hand;" +
            "-fx-background-color:" + hoverBg + ";"
        ));
        lbl.setOnMouseExited(e -> lbl.setStyle(
            "-fx-font-size:13px;-fx-font-weight:600;" +
            "-fx-text-fill:" + normalColor + ";" +
            "-fx-padding:9 16 9 16;" +
            "-fx-cursor:hand;"
        ));
        return lbl;
    }

    private javafx.scene.layout.Region menuSep() {
        javafx.scene.layout.Region sep = new javafx.scene.layout.Region();
        sep.setPrefHeight(1);
        sep.setMaxHeight(1);
        sep.setStyle("-fx-background-color:rgba(45,216,255,0.10);-fx-padding:0;");
        VBox.setMargin(sep, new Insets(4, 0, 4, 0));
        return sep;
    }

    // ── Clock block ───────────────────────────────────────────────────────────

    private VBox buildClockBlock() {
        VBox block = new VBox(3, clockLabel, dateLabel);
        block.setAlignment(Pos.CENTER_RIGHT);
        block.setPadding(new Insets(0, 0, 0, 0));
        return block;
    }

    // ── Clock tick ────────────────────────────────────────────────────────────

    private void startClock() {
        Locale ptBR = new Locale("pt", "BR");
        DateTimeFormatter tf = DateTimeFormatter.ofPattern("HH:mm:ss");
        DateTimeFormatter df = DateTimeFormatter.ofPattern("EEEE, d 'De' MMMM 'De' yyyy", ptBR);

        Timeline tl = new Timeline(new KeyFrame(Duration.seconds(1), e -> {
            LocalDateTime now = LocalDateTime.now();
            clockLabel.setText(now.format(tf));
            dateLabel.setText(capitalizeWords(now.format(df)));
        }));
        tl.setCycleCount(Timeline.INDEFINITE);
        tl.play();
        // dispara imediatamente para não começar com "--:--:--"
        tl.getKeyFrames().get(0).getOnFinished().handle(null);
    }

    private static String capitalizeWords(String s) {
        String[] parts = s.split(" ");
        StringBuilder sb = new StringBuilder();
        for (String p : parts) {
            if (sb.length() > 0) sb.append(" ");
            if (!p.isEmpty()) sb.append(Character.toUpperCase(p.charAt(0))).append(p.substring(1));
        }
        return sb.toString();
    }
}
