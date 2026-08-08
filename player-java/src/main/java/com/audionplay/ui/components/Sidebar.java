package com.audionplay.ui.components;

import com.audionplay.ui.Theme;
import javafx.geometry.Insets;
import javafx.geometry.Pos;
import javafx.scene.Cursor;
import javafx.scene.layout.Priority;
import javafx.scene.layout.Region;
import javafx.scene.layout.VBox;

import java.util.ArrayList;
import java.util.List;
import java.util.function.Consumer;

import static com.audionplay.ui.Theme.*;

/**
 * Barra lateral de navegação com ícones.
 *
 * Aceita um callback {@code onNavigate} chamado com a chave da seção
 * quando o usuário clica em um item ("NO AR", "CATÁLOGO", etc.).
 */
public class Sidebar extends VBox {

    private final List<VBox> navItems = new ArrayList<>();
    private VBox activeItem;

    public Sidebar(Consumer<String> onNavigate) {
        super(4);
        setPrefWidth(64); setMinWidth(64); setMaxWidth(64);
        setStyle("-fx-background-color:" + BG_DEEP + ";-fx-border-color:" + BORDER + ";-fx-border-width:0 1 0 0;");
        setPadding(new Insets(12, 0, 12, 0));
        setAlignment(Pos.TOP_CENTER);

        VBox noAr    = sideItem("▶", "NO AR",    true,  onNavigate);
        VBox catalog = sideItem("⊞", "CATÁLOGO", false, onNavigate);
        VBox rotacao = sideItem("↻", "ROTAÇÃO",  false, onNavigate);

        activeItem = noAr;
        navItems.addAll(List.of(noAr, catalog, rotacao));

        Region spacer = new Region();
        VBox.setVgrow(spacer, Priority.ALWAYS);

        getChildren().addAll(noAr, catalog, rotacao, spacer,
            sideItem("⚙", "CONFIG", false, onNavigate));
    }

    public Sidebar() {
        this(s -> {});
    }

    private VBox sideItem(String icon, String label, boolean active, Consumer<String> onNavigate) {
        String color = active ? BLUE : TEXT_SEC;
        VBox v = new VBox(4,
            Theme.lbl(icon,  "-fx-font-size:17px;-fx-text-fill:" + color + ";"),
            Theme.lbl(label, "-fx-font-size:7px;-fx-text-fill:" + color + ";-fx-text-alignment:center;")
        );
        v.setAlignment(Pos.CENTER);
        v.setPadding(new Insets(10, 4, 10, 4));
        v.setPrefWidth(64);
        v.setCursor(Cursor.HAND);

        if (active) applyActiveStyle(v);

        v.setOnMouseClicked(e -> {
            if (activeItem != null) applyInactiveStyle(activeItem);
            applyActiveStyle(v);
            activeItem = v;
            onNavigate.accept(label);
        });

        return v;
    }

    private static void applyActiveStyle(VBox v) {
        v.setStyle("-fx-background-color:#1A2240;-fx-border-color:" + BLUE + ";-fx-border-width:0 0 0 3;");
        v.getChildren().forEach(c -> c.setStyle(c.getStyle()
            .replaceAll("-fx-text-fill:[^;]+;", "-fx-text-fill:" + BLUE + ";")));
    }

    private static void applyInactiveStyle(VBox v) {
        v.setStyle("");
        v.getChildren().forEach(c -> c.setStyle(c.getStyle()
            .replaceAll("-fx-text-fill:[^;]+;", "-fx-text-fill:" + TEXT_SEC + ";")));
    }
}
