package com.audionplay.ui.components;

import com.audionplay.ui.Theme;
import javafx.animation.KeyFrame;
import javafx.animation.Timeline;
import javafx.geometry.Insets;
import javafx.geometry.Orientation;
import javafx.geometry.Pos;
import javafx.scene.control.Label;
import javafx.scene.control.Separator;
import javafx.scene.layout.HBox;
import javafx.scene.layout.Priority;
import javafx.scene.layout.StackPane;
import javafx.scene.layout.VBox;
import javafx.scene.paint.Color;
import javafx.scene.shape.Circle;
import javafx.util.Duration;

import java.time.LocalDateTime;
import java.time.format.DateTimeFormatter;
import java.util.Locale;

import static com.audionplay.ui.Theme.*;

/**
 * Barra superior: logo, abas de navegação, relógio e dados do usuário.
 */
public class TopBar extends HBox {

    private final Label clockLabel;
    private final Label dateLabel;

    public TopBar() {
        super(0);
        setAlignment(Pos.CENTER_LEFT);
        setStyle("-fx-background-color:#1C2030;-fx-border-color:" + BORDER + ";-fx-border-width:0 0 1 0;");
        setPadding(new Insets(10, 20, 10, 20));

        clockLabel = new Label("--:--:--");
        clockLabel.setStyle("-fx-font-size:22px;-fx-font-weight:bold;-fx-text-fill:" + TEXT_PRI + ";-fx-font-family:'Courier New';");
        dateLabel = new Label("...");
        dateLabel.setStyle("-fx-font-size:10px;-fx-text-fill:" + TEXT_SEC + ";");

        HBox nav = buildNavTabs();
        HBox.setHgrow(nav, Priority.ALWAYS);
        nav.setAlignment(Pos.CENTER);

        getChildren().addAll(buildLogo(), nav, buildTopRight());
        startClock();
    }

    private HBox buildLogo() {
        StackPane circle = new StackPane(
            new Circle(20, Color.web(BLUE)),
            Theme.lbl("A", "-fx-font-size:18px;-fx-font-weight:bold;-fx-text-fill:white;")
        );
        VBox text = new VBox(1,
            Theme.lbl("Audion Play",    "-fx-font-size:14px;-fx-font-weight:bold;-fx-text-fill:" + TEXT_PRI + ";"),
            Theme.lbl("BROADCAST SUITE","-fx-font-size:8px;-fx-text-fill:" + TEXT_SEC + ";")
        );
        HBox logo = new HBox(12, circle, text);
        logo.setAlignment(Pos.CENTER_LEFT);
        logo.setPadding(new Insets(0, 50, 0, 0));
        return logo;
    }

    private HBox buildNavTabs() {
        HBox nav = new HBox(4);
        nav.setAlignment(Pos.CENTER);
        nav.getChildren().addAll(
            navTab("Playout",   BLUE,     true),
            navTab("Library",   GREEN,    false),
            navTab("STREAMING", TEXT_SEC, false)
        );
        return nav;
    }

    private HBox navTab(String text, String color, boolean active) {
        HBox tab = new HBox(8,
            new Circle(4, Color.web(color)),
            Theme.lbl(text, "-fx-font-size:13px;-fx-font-weight:" + (active ? "bold" : "normal") +
                            ";-fx-text-fill:" + (active ? TEXT_PRI : TEXT_SEC) + ";")
        );
        tab.setAlignment(Pos.CENTER);
        tab.setPadding(new Insets(7, 18, 7, 18));
        if (active) tab.setStyle("-fx-background-color:#252A40;-fx-background-radius:7;");
        return tab;
    }

    private HBox buildTopRight() {
        StackPane avatar = new StackPane(
            new Circle(18, Color.web("#243060")),
            Theme.lbl("WN", "-fx-font-size:11px;-fx-font-weight:bold;-fx-text-fill:white;")
        );
        VBox user = new VBox(2,
            Theme.lbl("Waelson Nunes", "-fx-font-size:12px;-fx-font-weight:bold;-fx-text-fill:" + TEXT_PRI + ";"),
            Theme.lbl("ADMIN",         "-fx-font-size:9px;-fx-text-fill:" + TEXT_SEC + ";")
        );
        VBox clk = new VBox(2, clockLabel, dateLabel);
        clk.setAlignment(Pos.CENTER_RIGHT);

        HBox right = new HBox(16, avatar, user, new Separator(Orientation.VERTICAL), clk);
        right.setAlignment(Pos.CENTER_RIGHT);
        return right;
    }

    private void startClock() {
        Locale ptBR = new Locale("pt", "BR");
        DateTimeFormatter tf = DateTimeFormatter.ofPattern("HH:mm:ss");
        DateTimeFormatter df = DateTimeFormatter.ofPattern("EEEE, d 'de' MMMM 'de' yyyy", ptBR);
        Timeline tl = new Timeline(new KeyFrame(Duration.seconds(1), e -> {
            LocalDateTime now = LocalDateTime.now();
            clockLabel.setText(now.format(tf));
            String d = now.format(df);
            dateLabel.setText(Character.toUpperCase(d.charAt(0)) + d.substring(1));
        }));
        tl.setCycleCount(Timeline.INDEFINITE);
        tl.play();
    }
}
