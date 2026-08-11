package com.audionplay.ui.components;

import com.audionplay.db.entity.HotkeyProfileEntity;
import com.audionplay.db.entity.HotkeyProfileEntity.HotkeyButton;
import com.audionplay.db.repository.HotkeyRepository;
import com.audionplay.ui.Theme;
import javafx.application.Platform;
import javafx.geometry.Insets;
import javafx.geometry.Pos;
import javafx.scene.control.ContextMenu;
import javafx.scene.control.Label;
import javafx.scene.control.MenuItem;
import javafx.scene.control.ScrollPane;
import javafx.scene.layout.*;

import java.sql.SQLException;
import java.util.ArrayList;
import java.util.List;
import java.util.Optional;
import java.util.function.Consumer;

import static com.audionplay.ui.Theme.*;

/**
 * Painel Hot Keys — visual fiel ao hkp-panel do player.html.
 *
 * Carrega perfis do banco via HotkeyRepository e renderiza os cards com
 * as paletas de cor correspondentes.
 */
public class TabsPanel extends VBox {

    // ── Paletas (espelham HKP_PALETTES do player.html) ────────────────────────
    // {grad, glow, border}
    private static final String[][] PALETTES = {
        {
            "linear-gradient(to bottom,#6a2080 0%,#5e1c72 22%,#521863 47%,#220830 49%,#170522 72%,#0f0316 100%)",
            "rgba(155,89,182,0.60)", "rgba(189,147,220,0.60)"
        },
        {
            "linear-gradient(to bottom,#1a6875 0%,#155e69 22%,#0d4f5a 47%,#022830 49%,#011e24 72%,#011318 100%)",
            "rgba(23,165,137,0.60)",  "rgba(118,215,196,0.60)"
        },
        {
            "linear-gradient(to bottom,#7a5c10 0%,#6e520d 22%,#5e4409 47%,#2e2003 49%,#1e1502 72%,#140e01 100%)",
            "rgba(212,172,13,0.60)",  "rgba(249,231,159,0.60)"
        },
        {
            "linear-gradient(to bottom,#7a1520 0%,#6e111c 22%,#5e0e18 47%,#2e0408 49%,#1e0205 72%,#140103 100%)",
            "rgba(231,76,60,0.60)",   "rgba(241,148,138,0.60)"
        },
        {
            "linear-gradient(to bottom,#1a6645 0%,#155a3c 22%,#0d4d33 47%,#022515 49%,#011b0f 72%,#01120a 100%)",
            "rgba(39,174,96,0.60)",   "rgba(130,224,170,0.60)"
        },
        {
            "linear-gradient(to bottom,#1a4070 0%,#153868 22%,#0d2e5a 47%,#021628 49%,#011020 72%,#010b18 100%)",
            "rgba(41,128,185,0.60)",  "rgba(133,193,233,0.60)"
        },
        {
            "linear-gradient(to bottom,#7a3d08 0%,#6e360a 22%,#5e2a07 47%,#2e1403 49%,#1e0e02 72%,#140901 100%)",
            "rgba(230,126,34,0.60)",  "rgba(240,178,122,0.60)"
        },
        {
            "linear-gradient(to bottom,#7a1020 0%,#6e0c1a 22%,#5e0a16 47%,#2e0408 49%,#1e0205 72%,#140103 100%)",
            "rgba(214,48,49,0.60)",   "rgba(248,165,194,0.60)"
        },
    };

    // ── Estado ────────────────────────────────────────────────────────────────
    private final HotkeyRepository             repo     = new HotkeyRepository();
    private final List<HotkeyProfileEntity>    profiles = new ArrayList<>();
    private String currentProfileId = null;

    // ── Componentes com estado ────────────────────────────────────────────────
    private Label     ppNameLabel;
    private GridPane  grid;
    private ContextMenu profileMenu;

    // ── Callbacks ─────────────────────────────────────────────────────────────
    private Consumer<HotkeyButton> onCartPlay;

    // ── Constructor ───────────────────────────────────────────────────────────

    public TabsPanel() {
        super(0);
        setStyle(
            "-fx-background-color:#000000;" +
            "-fx-border-color:#20384c;-fx-border-width:1;" +
            "-fx-border-radius:12;-fx-background-radius:12;"
        );
        VBox.setVgrow(this, Priority.ALWAYS);

        profileMenu = new ContextMenu();
        ppNameLabel = new Label("Carregando…");
        ppNameLabel.setStyle("-fx-font-size:12px;-fx-font-weight:700;-fx-text-fill:#eef7ff;");

        grid = new GridPane();
        grid.setHgap(5);
        grid.setVgap(5);
        for (int i = 0; i < 4; i++) {
            ColumnConstraints cc = new ColumnConstraints();
            cc.setPercentWidth(25);
            cc.setHgrow(Priority.ALWAYS);
            grid.getColumnConstraints().add(cc);
        }

        getChildren().addAll(
            buildTabs(),
            buildHeader(),
            buildGridWrap()
        );

        loadProfiles();
    }

    // ── API pública ───────────────────────────────────────────────────────────

    public void setOnCartPlay(Consumer<HotkeyButton> cb) {
        this.onCartPlay = cb;
    }

    // ── hkp-tabs ──────────────────────────────────────────────────────────────

    private HBox buildTabs() {
        HBox tabs = new HBox(5,
            tabLbl("Hot Keys",       true),
            tabLbl("Voice Track",    false),
            tabLbl("Routing rápido", false),
            tabLbl("Notas",          false)
        );
        tabs.setPadding(new Insets(7, 8, 0, 8));
        tabs.setStyle("-fx-background-color:#000000;");
        return tabs;
    }

    private Label tabLbl(String text, boolean active) {
        Label l = new Label(text);
        l.setPrefHeight(30);
        l.setPadding(new Insets(0, 10, 0, 10));
        if (active) {
            l.setStyle(
                "-fx-font-size:12px;-fx-font-weight:700;-fx-cursor:hand;-fx-text-fill:#00d4ff;" +
                "-fx-background-color:#000000;" +
                "-fx-border-color:#20384c #20384c #000000 #20384c;-fx-border-width:1;" +
                "-fx-background-radius:8 8 0 0;-fx-border-radius:8 8 0 0;"
            );
        } else {
            l.setStyle(
                "-fx-font-size:12px;-fx-font-weight:700;-fx-text-fill:#2a3f52;" +
                "-fx-background-color:transparent;"
            );
        }
        return l;
    }

    // ── hkp-header ────────────────────────────────────────────────────────────

    private HBox buildHeader() {
        // profile pill
        Label ppLabel = new Label("PERFIL");
        ppLabel.setStyle("-fx-font-size:9px;-fx-font-weight:700;-fx-text-fill:#4a6478;");

        VBox ppText = new VBox(0, ppLabel, ppNameLabel);

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
        pill.setOnMouseClicked(e -> {
            profileMenu.getItems().clear();
            for (HotkeyProfileEntity p : profiles) {
                MenuItem item = new MenuItem(p.name());
                boolean isCur = p.id().equals(currentProfileId);
                item.setStyle(isCur
                    ? "-fx-text-fill:#00d4ff;-fx-font-weight:700;"
                    : "-fx-text-fill:#88a0b5;");
                item.setOnAction(ae -> selectProfile(p.id()));
                profileMenu.getItems().add(item);
            }
            if (!profileMenu.getItems().isEmpty()) {
                profileMenu.show(pill, e.getScreenX(), e.getScreenY());
            }
        });

        // reload button
        Label reloadBtn = new Label("↺");
        reloadBtn.setPrefWidth(40);
        reloadBtn.setMinHeight(32);
        reloadBtn.setAlignment(Pos.CENTER);
        reloadBtn.setStyle(
            "-fx-font-size:20px;-fx-text-fill:#9cb5ba;" +
            "-fx-background-color:transparent;-fx-cursor:hand;"
        );
        reloadBtn.setOnMouseClicked(e -> loadProfiles());

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
        ScrollPane scroll = new ScrollPane(grid);
        scroll.setFitToWidth(true);
        scroll.setHbarPolicy(ScrollPane.ScrollBarPolicy.NEVER);
        scroll.setStyle(
            "-fx-background-color:#0a1520;-fx-background:#0a1520;" +
            "-fx-border-color:transparent;"
        );
        VBox.setVgrow(scroll, Priority.ALWAYS);

        VBox wrap = new VBox(scroll);
        wrap.setPadding(new Insets(7));
        VBox.setVgrow(wrap, Priority.ALWAYS);

        // estado inicial: carregando
        Label loading = new Label("Carregando perfis…");
        loading.setStyle("-fx-font-size:12px;-fx-text-fill:#4a6478;");
        GridPane.setColumnSpan(loading, 4);
        grid.add(loading, 0, 0);

        return wrap;
    }

    // ── Carregamento de dados ─────────────────────────────────────────────────

    private void loadProfiles() {
        ppNameLabel.setText("Carregando…");
        Thread t = new Thread(() -> {
            try {
                List<HotkeyProfileEntity> loaded = repo.findAllProfiles();
                Platform.runLater(() -> {
                    profiles.clear();
                    profiles.addAll(loaded);
                    if (loaded.isEmpty()) {
                        ppNameLabel.setText("—");
                        renderGrid(List.of());
                    } else {
                        // mantém perfil atual se ainda existir, senão usa o primeiro
                        String keepId = currentProfileId != null &&
                            loaded.stream().anyMatch(p -> p.id().equals(currentProfileId))
                            ? currentProfileId : loaded.get(0).id();
                        selectProfile(keepId);
                    }
                });
            } catch (SQLException ex) {
                Platform.runLater(() -> {
                    ppNameLabel.setText("Erro");
                    renderGrid(List.of());
                });
            }
        }, "hkp-profile-loader");
        t.setDaemon(true);
        t.start();
    }

    private void selectProfile(String id) {
        currentProfileId = id;
        HotkeyProfileEntity prof = profiles.stream()
            .filter(p -> p.id().equals(id)).findFirst().orElse(null);
        if (prof != null) ppNameLabel.setText(prof.name());

        Thread t = new Thread(() -> {
            try {
                Optional<HotkeyProfileEntity> opt = repo.findProfileById(id);
                List<HotkeyButton> buttons = opt.map(HotkeyProfileEntity::buttons)
                    .orElse(List.of());
                Platform.runLater(() -> renderGrid(buttons));
            } catch (SQLException ex) {
                Platform.runLater(() -> renderGrid(List.of()));
            }
        }, "hkp-buttons-loader");
        t.setDaemon(true);
        t.start();
    }

    // ── Renderização do grid ──────────────────────────────────────────────────

    private void renderGrid(List<HotkeyButton> buttons) {
        grid.getChildren().clear();
        if (buttons.isEmpty()) {
            Label empty = new Label("Nenhum botão neste perfil");
            empty.setStyle("-fx-font-size:12px;-fx-text-fill:#4a6478;");
            GridPane.setColumnSpan(empty, 4);
            grid.add(empty, 0, 0);
            return;
        }
        for (int i = 0; i < buttons.size(); i++) {
            StackPane card = makeCard(buttons.get(i), i + 1);
            grid.add(card, i % 4, i / 4);
        }
    }

    // ── hkp-card ─────────────────────────────────────────────────────────────

    private StackPane makeCard(HotkeyButton b, int slot) {
        int pi = Math.max(0, Math.min(b.palette(), PALETTES.length - 1));
        String grad   = PALETTES[pi][0];
        String border = PALETTES[pi][2];

        // gradient overlay (simula ::before com opacity .34)
        Region overlay = new Region();
        overlay.setStyle("-fx-background-color:" + grad + ";");
        overlay.setOpacity(0.34);
        overlay.setMaxWidth(Double.MAX_VALUE);
        overlay.setMaxHeight(Double.MAX_VALUE);

        // ── Top row: slot + cue ──────────────────────────────────────────────
        Label slotLbl = new Label(String.valueOf(slot));
        slotLbl.setStyle(
            "-fx-font-size:9px;-fx-font-weight:800;" +
            "-fx-text-fill:rgba(255,255,255,0.75);" +
            "-fx-background-color:rgba(0,0,0,0.30);" +
            "-fx-background-radius:4;-fx-padding:1 5 1 5;"
        );

        Label cueLbl = new Label("\uD83C\uDFA7");  // 🎧
        cueLbl.setStyle(
            "-fx-font-size:14px;-fx-text-fill:rgba(255,255,255,0.50);-fx-cursor:hand;"
        );
        cueLbl.setPrefWidth(20); cueLbl.setPrefHeight(20);
        cueLbl.setAlignment(Pos.CENTER);

        Region topSpacer = new Region();
        HBox.setHgrow(topSpacer, Priority.ALWAYS);

        HBox topRow = new HBox(4, slotLbl, topSpacer, cueLbl);
        topRow.setAlignment(Pos.TOP_LEFT);

        // ── Name ─────────────────────────────────────────────────────────────
        String name = b.label() != null && !b.label().isBlank() ? b.label()
            : b.trackTitle() != null ? b.trackTitle() : "—";
        Label nameLbl = new Label(name);
        nameLbl.setStyle(
            "-fx-font-size:11px;-fx-font-weight:700;-fx-text-fill:#ffffff;-fx-wrap-text:true;"
        );
        nameLbl.setWrapText(true);

        // ── Sub ──────────────────────────────────────────────────────────────
        String sub = b.subLabel() != null && !b.subLabel().isBlank() ? b.subLabel()
            : b.trackArtist() != null && !b.trackArtist().isBlank() ? b.trackArtist()
            : b.trackType() != null ? b.trackType() : "";
        Label subLbl = new Label(sub);
        subLbl.setStyle("-fx-font-size:10px;-fx-text-fill:rgba(255,255,255,0.55);");

        // ── Time ─────────────────────────────────────────────────────────────
        Label timeLbl = new Label(msToTs(b.durationMs()));
        timeLbl.setStyle(
            "-fx-font-size:10px;-fx-font-weight:800;" +
            "-fx-text-fill:rgba(255,255,255,0.80);-fx-font-family:'Courier New';"
        );

        // ── Inner VBox ────────────────────────────────────────────────────────
        VBox inner = new VBox(2, topRow, nameLbl, subLbl, timeLbl);
        inner.setPadding(new Insets(5, 7, 4, 7));
        VBox.setVgrow(inner, Priority.ALWAYS);

        // ── Progress bar ─────────────────────────────────────────────────────
        Region bar = new Region();
        bar.setPrefHeight(3);
        bar.setMaxWidth(Double.MAX_VALUE);
        bar.setStyle("-fx-background-color:rgba(255,255,255,0.10);");

        VBox contentBox = new VBox(0, inner, bar);
        contentBox.setMaxWidth(Double.MAX_VALUE);
        contentBox.setMaxHeight(Double.MAX_VALUE);

        // ── Card ─────────────────────────────────────────────────────────────
        StackPane card = new StackPane(overlay, contentBox);
        card.setMinHeight(78);
        card.setAlignment(Pos.TOP_LEFT);
        card.setStyle(
            "-fx-background-color:#07111a;" +
            "-fx-border-color:" + border + ";" +
            "-fx-border-width:1 1 1 3;" +
            "-fx-border-radius:10;-fx-background-radius:10;" +
            "-fx-cursor:hand;"
        );

        // ── Hover ────────────────────────────────────────────────────────────
        card.setOnMouseEntered(e -> card.setStyle(
            "-fx-background-color:#0d1820;" +
            "-fx-border-color:" + border + ";" +
            "-fx-border-width:1 1 1 3;" +
            "-fx-border-radius:10;-fx-background-radius:10;" +
            "-fx-cursor:hand;"
        ));
        card.setOnMouseExited(e -> card.setStyle(
            "-fx-background-color:#07111a;" +
            "-fx-border-color:" + border + ";" +
            "-fx-border-width:1 1 1 3;" +
            "-fx-border-radius:10;-fx-background-radius:10;" +
            "-fx-cursor:hand;"
        ));

        // ── Click para tocar ─────────────────────────────────────────────────
        card.setOnMouseClicked(e -> {
            if (onCartPlay != null && b.trackPath() != null && !b.trackPath().isBlank()) {
                onCartPlay.accept(b);
            }
        });

        return card;
    }

    // ── Helper ───────────────────────────────────────────────────────────────

    private static String msToTs(int ms) {
        if (ms <= 0) return "0:00";
        int s = ms / 1000;
        return String.format("%02d:%02d", s / 60, s % 60);
    }
}
