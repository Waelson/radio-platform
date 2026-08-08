package com.audionplay.ui.components;

import com.audionplay.db.entity.TrackEntity;
import com.audionplay.db.entity.TrackEntity.TrackType;
import com.audionplay.db.repository.TrackRepository;
import javafx.application.Platform;
import javafx.beans.property.SimpleObjectProperty;
import javafx.beans.property.SimpleStringProperty;
import javafx.collections.FXCollections;
import javafx.collections.ObservableList;
import javafx.collections.transformation.FilteredList;
import javafx.geometry.Insets;
import javafx.geometry.Pos;
import javafx.scene.control.*;
import javafx.scene.effect.DropShadow;
import javafx.scene.layout.*;
import javafx.scene.paint.Color;
import javafx.scene.shape.Circle;

import java.sql.SQLException;
import java.util.*;
import java.util.function.Consumer;
import java.util.stream.Collectors;

/**
 * Tela de catálogo — visual fiel ao cat-view / cat-library-panel do player.html.
 *
 * Layout: header | filter-bar | tabela (cresce) | paginação
 * Colunas: Título · Artista · Álbum · Tipo (badge) · Categoria · Duração · Cue In · Cue Out · Ações
 */
public class CatalogPanel extends VBox {

    private static final int PAGE_SIZE = 50;

    // ── Dados ─────────────────────────────────────────────────────────────────
    private final TrackRepository             repo         = new TrackRepository();
    private final ObservableList<TrackEntity> masterList   = FXCollections.observableArrayList();
    private final FilteredList<TrackEntity>   filteredList = new FilteredList<>(masterList, t -> true);
    private final ObservableList<TrackEntity> pagedList    = FXCollections.observableArrayList();

    private TrackType activeType     = null;
    private String    searchTerm     = "";
    private String    activeCategory = null;
    private int       currentPage    = 1;

    // ── Componentes com estado ────────────────────────────────────────────────
    private Label                  countChipLabel;
    private Label                  statusLabel;
    private TableView<TrackEntity> table;
    private ComboBox<String>       categoryCombo;
    private Button                 prevBtn;
    private Button                 nextBtn;
    private Label                  pageInfoLabel;
    private HBox                   paginationBar;

    // ── Callbacks ─────────────────────────────────────────────────────────────
    private Consumer<TrackEntity> onPlay;
    private Consumer<TrackEntity> onAddToQueue;

    public CatalogPanel() {
        super(10);
        setStyle("-fx-background-color:#071019;");
        setPadding(new Insets(10));
        VBox.setVgrow(this, Priority.ALWAYS);

        VBox card = new VBox(0);
        card.setStyle(
            "-fx-background-color:#0e1b28;" +
            "-fx-border-color:#20384c;-fx-border-width:1;" +
            "-fx-border-radius:14;-fx-background-radius:14;"
        );
        VBox.setVgrow(card, Priority.ALWAYS);
        card.getChildren().addAll(
            buildHeader(),
            buildFilterBar(),
            buildTable(),
            buildPagination()
        );

        getChildren().add(card);
        loadAsync();
    }

    public void setOnPlay(Consumer<TrackEntity> h)       { this.onPlay = h; }
    public void setOnAddToQueue(Consumer<TrackEntity> h) { this.onAddToQueue = h; }
    public void reload() { loadAsync(); }

    // ══════════════════════════════════════════════════════════════════════════
    //  LAYOUT
    // ══════════════════════════════════════════════════════════════════════════

    private HBox buildHeader() {
        Label title = new Label("CONSULTAR CATÁLOGO");
        title.setStyle("-fx-font-size:11px;-fx-font-weight:bold;-fx-text-fill:#bdd0df;");

        // chip verde com dot
        Circle dot = new Circle(3.5, Color.web("#36d399"));
        DropShadow glow = new DropShadow(10, Color.web("#36d399"));
        glow.setSpread(0.4);
        dot.setEffect(glow);

        countChipLabel = new Label("—");
        countChipLabel.setStyle("-fx-font-size:11px;-fx-text-fill:#88a0b5;");

        HBox chip = new HBox(7, dot, countChipLabel);
        chip.setAlignment(Pos.CENTER);
        chip.setPadding(new Insets(0, 10, 0, 10));
        chip.setMinHeight(30);
        chip.setStyle(
            "-fx-background-color:rgba(14,27,40,0.78);" +
            "-fx-border-color:#20384c;-fx-border-width:1;" +
            "-fx-border-radius:999;-fx-background-radius:999;"
        );

        Button reloadBtn = new Button("↻  Recarregar");
        reloadBtn.setStyle(
            "-fx-background-color:#0e1b28;" +
            "-fx-border-color:#20384c;-fx-border-width:1;" +
            "-fx-border-radius:7;-fx-background-radius:7;" +
            "-fx-text-fill:#88a0b5;-fx-font-size:10px;" +
            "-fx-padding:4 10 4 10;-fx-cursor:hand;"
        );
        reloadBtn.setOnAction(e -> reload());

        Region spacer = new Region();
        HBox.setHgrow(spacer, Priority.ALWAYS);

        HBox hdr = new HBox(9, title, chip, spacer, reloadBtn);
        hdr.setAlignment(Pos.CENTER_LEFT);
        hdr.setMinHeight(44);
        hdr.setPadding(new Insets(0, 14, 0, 14));
        hdr.setStyle(
            "-fx-background-color:rgba(10,22,33,0.7);" +
            "-fx-border-color:#20384c;-fx-border-width:0 0 1 0;"
        );
        return hdr;
    }

    private HBox buildFilterBar() {
        // busca
        TextField search = new TextField();
        search.setPromptText("Título ou artista...");
        search.setStyle(
            "-fx-background-color:#091521;" +
            "-fx-border-color:#20384c;-fx-border-width:1;" +
            "-fx-border-radius:8;-fx-background-radius:8;" +
            "-fx-text-fill:#eef7ff;-fx-prompt-text-fill:#5f788e;" +
            "-fx-padding:0 10 0 10;-fx-font-size:12px;"
        );
        search.setPrefHeight(32);
        HBox.setHgrow(search, Priority.ALWAYS);
        search.textProperty().addListener((o, ov, nv) -> {
            searchTerm = nv == null ? "" : nv.trim().toLowerCase();
            applyFilter();
        });

        // combo tipo
        ComboBox<String> typeCombo = new ComboBox<>();
        typeCombo.getItems().addAll(
            "Todos os tipos", "Música", "Vinheta", "Jingle", "Spot", "Efeitos"
        );
        typeCombo.setValue("Todos os tipos");
        styleCombo(typeCombo);
        typeCombo.setPrefWidth(148); typeCombo.setMinWidth(148); typeCombo.setMaxWidth(148);
        typeCombo.setPrefHeight(32);
        typeCombo.valueProperty().addListener((o, ov, nv) -> {
            activeType = switch (nv == null ? "" : nv) {
                case "Música"   -> TrackType.MUSIC;
                case "Vinheta"  -> TrackType.VINHETA;
                case "Jingle"   -> TrackType.JINGLE;
                case "Spot"     -> TrackType.SPOT;
                case "Efeitos"  -> TrackType.EFEITOS;
                default         -> null;
            };
            boolean isMusic = activeType == TrackType.MUSIC;
            categoryCombo.setDisable(!isMusic);
            if (!isMusic) { categoryCombo.setValue("Todas as categorias"); activeCategory = null; }
            applyFilter();
        });

        // combo categoria
        categoryCombo = new ComboBox<>();
        categoryCombo.getItems().add("Todas as categorias");
        categoryCombo.setValue("Todas as categorias");
        categoryCombo.setDisable(true);
        styleCombo(categoryCombo);
        categoryCombo.setPrefWidth(148); categoryCombo.setMinWidth(148); categoryCombo.setMaxWidth(148);
        categoryCombo.setPrefHeight(32);
        categoryCombo.valueProperty().addListener((o, ov, nv) -> {
            activeCategory = (nv == null || nv.equals("Todas as categorias")) ? null : nv;
            applyFilter();
        });

        // status
        statusLabel = new Label("Carregando...");
        statusLabel.setStyle("-fx-font-size:10px;-fx-text-fill:#5f788e;");

        HBox bar = new HBox(8, search, typeCombo, categoryCombo, statusLabel);
        bar.setAlignment(Pos.CENTER_LEFT);
        bar.setPadding(new Insets(10, 14, 10, 14));
        bar.setStyle("-fx-border-color:#20384c;-fx-border-width:0 0 1 0;");
        return bar;
    }

    private VBox buildTable() {
        table = new TableView<>(pagedList);
        table.setColumnResizePolicy(TableView.CONSTRAINED_RESIZE_POLICY_FLEX_LAST_COLUMN);
        table.setStyle(
            "-fx-base:#0b1925;-fx-background:#0b1925;" +
            "-fx-control-inner-background:#0e1b28;" +
            "-fx-control-inner-background-alt:#0e1b28;" +
            "-fx-accent:#00d4ff;" +
            "-fx-selection-bar:rgba(0,212,255,0.09);" +
            "-fx-selection-bar-non-focused:rgba(0,212,255,0.05);" +
            "-fx-table-header-border-color:#20384c;" +
            "-fx-font-size:11px;"
        );
        table.setPlaceholder(new Label("Nenhuma faixa encontrada") {{
            setStyle("-fx-font-size:12px;-fx-text-fill:#5f788e;");
        }});
        table.setRowFactory(tv -> {
            TableRow<TrackEntity> row = new TableRow<>();
            row.setOnMouseClicked(e -> {
                if (e.getClickCount() == 2 && !row.isEmpty()) fireAddToQueue(row.getItem());
            });
            return row;
        });

        table.getColumns().addAll(
            colText("Título",    0.25, true,  t -> t.title().isBlank() ? t.path() : t.title()),
            colText("Artista",   0.18, true,  t -> nvl(t.artist())),
            colText("Álbum",     0.12, true,  t -> nvl(t.album())),
            colTypeBadge(),
            colText("Categoria", 0.08, false, t -> nvl(t.category())),
            colDuration(),
            colMs("Cue In",  t -> t.cueInMs()),
            colMs("Cue Out", t -> t.cueOutMs()),
            colActions()
        );

        VBox.setVgrow(table, Priority.ALWAYS);
        VBox wrapper = new VBox(table);
        VBox.setVgrow(wrapper, Priority.ALWAYS);
        return wrapper;
    }

    private HBox buildPagination() {
        prevBtn = new Button("← Anterior");
        nextBtn = new Button("Próxima →");
        pageInfoLabel = new Label("Página 1 de 1");
        pageInfoLabel.setStyle("-fx-font-size:11px;-fx-text-fill:#5f788e;");

        String btnStyle =
            "-fx-background-color:#0e1b28;" +
            "-fx-border-color:#20384c;-fx-border-width:1;" +
            "-fx-border-radius:7;-fx-background-radius:7;" +
            "-fx-text-fill:#88a0b5;-fx-font-size:11px;" +
            "-fx-padding:0 12 0 12;-fx-pref-height:27;-fx-cursor:hand;";
        prevBtn.setStyle(btnStyle);
        nextBtn.setStyle(btnStyle);

        prevBtn.setOnAction(e -> goToPage(currentPage - 1));
        nextBtn.setOnAction(e -> goToPage(currentPage + 1));
        prevBtn.setDisable(true);
        nextBtn.setDisable(true);

        paginationBar = new HBox(14, prevBtn, pageInfoLabel, nextBtn);
        paginationBar.setAlignment(Pos.CENTER);
        paginationBar.setPadding(new Insets(8, 14, 8, 14));
        paginationBar.setStyle("-fx-border-color:#20384c;-fx-border-width:1 0 0 0;");
        return paginationBar;
    }

    // ══════════════════════════════════════════════════════════════════════════
    //  COLUNAS
    // ══════════════════════════════════════════════════════════════════════════

    @FunctionalInterface
    private interface TrackGetter<T> { T get(TrackEntity t); }

    @SuppressWarnings("unchecked")
    private TableColumn<TrackEntity, String> colText(String header, double pct,
                                                      boolean grows, TrackGetter<String> getter) {
        TableColumn<TrackEntity, String> col = new TableColumn<>(header);
        col.setCellValueFactory(d -> new SimpleStringProperty(getter.get(d.getValue())));
        col.setPrefWidth(pct * 1000);
        if (!grows) { col.setMinWidth(pct * 300); col.setMaxWidth(pct * 1500); }
        col.setCellFactory(tc -> new TableCell<>() {
            @Override
            protected void updateItem(String item, boolean empty) {
                super.updateItem(item, empty);
                setText(empty || item == null ? null : item);
                setStyle("-fx-text-fill:#eef7ff;-fx-padding:8 10 8 10;");
            }
        });
        return col;
    }

    @SuppressWarnings("unchecked")
    private TableColumn<TrackEntity, TrackType> colTypeBadge() {
        TableColumn<TrackEntity, TrackType> col = new TableColumn<>("Tipo");
        col.setCellValueFactory(d -> new SimpleObjectProperty<>(d.getValue().type()));
        col.setPrefWidth(80); col.setMinWidth(80); col.setMaxWidth(80);
        col.setResizable(false);
        col.setCellFactory(tc -> new TableCell<>() {
            private final Label badge = new Label();
            private final HBox  box   = new HBox(badge);
            { box.setAlignment(Pos.CENTER_LEFT); box.setPadding(new Insets(8, 10, 8, 10)); }
            @Override
            protected void updateItem(TrackType type, boolean empty) {
                super.updateItem(type, empty);
                if (empty || type == null) { setGraphic(null); return; }
                badge.setText(typeLabel(type));
                badge.setStyle(
                    "-fx-font-size:10px;-fx-font-weight:bold;" +
                    "-fx-padding:2 6 2 6;-fx-background-radius:3;" +
                    "-fx-background-color:" + typeBg(type) + ";" +
                    "-fx-text-fill:" + typeColor(type) + ";"
                );
                setGraphic(box);
                setText(null);
                setStyle("-fx-padding:0;");
            }
        });
        return col;
    }

    @SuppressWarnings("unchecked")
    private TableColumn<TrackEntity, Integer> colDuration() {
        TableColumn<TrackEntity, Integer> col = new TableColumn<>("Duração");
        col.setCellValueFactory(d -> new javafx.beans.property.SimpleIntegerProperty(d.getValue().durationMs()).asObject());
        col.setPrefWidth(80); col.setMinWidth(80); col.setMaxWidth(80);
        col.setResizable(false);
        col.setCellFactory(tc -> new TableCell<>() {
            @Override
            protected void updateItem(Integer ms, boolean empty) {
                super.updateItem(ms, empty);
                setText(empty || ms == null ? null : msToTime(ms));
                setStyle("-fx-text-fill:#eef7ff;-fx-padding:8 10 8 10;");
            }
        });
        return col;
    }

    @SuppressWarnings("unchecked")
    private TableColumn<TrackEntity, Integer> colMs(String header, TrackGetter<Integer> getter) {
        TableColumn<TrackEntity, Integer> col = new TableColumn<>(header);
        col.setCellValueFactory(d -> new SimpleObjectProperty<>(getter.get(d.getValue())));
        col.setPrefWidth(70); col.setMinWidth(70); col.setMaxWidth(70);
        col.setResizable(false);
        col.setCellFactory(tc -> new TableCell<>() {
            @Override
            protected void updateItem(Integer ms, boolean empty) {
                super.updateItem(ms, empty);
                setText(empty ? null : (ms == null ? "—" : msToTime(ms)));
                setStyle("-fx-text-fill:#5f788e;-fx-alignment:CENTER;-fx-padding:8 10 8 10;");
            }
        });
        return col;
    }

    @SuppressWarnings("unchecked")
    private TableColumn<TrackEntity, TrackEntity> colActions() {
        TableColumn<TrackEntity, TrackEntity> col = new TableColumn<>("Ações");
        col.setCellValueFactory(d -> new SimpleObjectProperty<>(d.getValue()));
        col.setPrefWidth(100); col.setMinWidth(100); col.setMaxWidth(100);
        col.setResizable(false);
        col.setCellFactory(tc -> new TableCell<>() {
            private final Button cueBtn   = iconBtn("🎧");
            private final Button cutBtn   = iconBtn("✂");
            private final Button queueBtn = iconBtn("↓");
            private final HBox   box      = new HBox(1, cueBtn, cutBtn, queueBtn);
            {
                box.setAlignment(Pos.CENTER_LEFT);
                box.setPadding(new Insets(0, 4, 0, 4));
                cueBtn.setOnAction(e -> {
                    TrackEntity t = getItem();
                    if (t != null) firePlay(t);
                });
                queueBtn.setOnAction(e -> {
                    TrackEntity t = getItem();
                    if (t != null) fireAddToQueue(t);
                });
            }
            @Override
            protected void updateItem(TrackEntity t, boolean empty) {
                super.updateItem(t, empty);
                setGraphic(empty ? null : box);
                setText(null);
                setStyle("-fx-padding:0;");
            }
        });
        return col;
    }

    // ══════════════════════════════════════════════════════════════════════════
    //  FILTRO E PAGINAÇÃO
    // ══════════════════════════════════════════════════════════════════════════

    private void applyFilter() {
        filteredList.setPredicate(t -> {
            boolean typeOk = activeType == null || t.type() == activeType;
            boolean catOk  = activeCategory == null ||
                (t.category() != null && t.category().equals(activeCategory));
            boolean textOk = searchTerm.isEmpty()
                || t.title().toLowerCase().contains(searchTerm)
                || t.artist().toLowerCase().contains(searchTerm)
                || (t.album() != null && t.album().toLowerCase().contains(searchTerm));
            return typeOk && catOk && textOk;
        });
        currentPage = 1;
        refreshPage();
    }

    private void refreshPage() {
        int total      = filteredList.size();
        int totalPages = Math.max(1, (int) Math.ceil((double) total / PAGE_SIZE));
        currentPage    = Math.max(1, Math.min(currentPage, totalPages));

        int from = (currentPage - 1) * PAGE_SIZE;
        int to   = Math.min(from + PAGE_SIZE, total);

        pagedList.setAll(filteredList.subList(from, to));

        long totalMs = filteredList.stream().mapToLong(TrackEntity::durationMs).sum();
        countChipLabel.setText(total + " faixas · " + msToTime((int) totalMs));

        pageInfoLabel.setText("Página " + currentPage + " de " + totalPages);
        prevBtn.setDisable(currentPage <= 1);
        nextBtn.setDisable(currentPage >= totalPages);
        paginationBar.setVisible(totalPages > 1 || total == 0);
    }

    private void goToPage(int page) {
        currentPage = page;
        refreshPage();
    }

    // ══════════════════════════════════════════════════════════════════════════
    //  CARREGAMENTO ASSÍNCRONO
    // ══════════════════════════════════════════════════════════════════════════

    private void loadAsync() {
        statusLabel.setText("Carregando...");
        masterList.clear();

        Thread t = new Thread(() -> {
            try {
                var tracks = repo.findAll();
                Platform.runLater(() -> {
                    masterList.setAll(tracks);
                    // popula categorias
                    List<String> cats = tracks.stream()
                        .map(TrackEntity::category)
                        .filter(c -> c != null && !c.isBlank())
                        .distinct()
                        .sorted()
                        .collect(Collectors.toList());
                    categoryCombo.getItems().setAll("Todas as categorias");
                    categoryCombo.getItems().addAll(cats);
                    applyFilter();
                    statusLabel.setText("");
                });
            } catch (SQLException ex) {
                Platform.runLater(() -> statusLabel.setText("Erro: " + ex.getMessage()));
            }
        }, "catalog-loader");
        t.setDaemon(true);
        t.start();
    }

    private void firePlay(TrackEntity t)       { if (onPlay       != null) onPlay.accept(t); }
    private void fireAddToQueue(TrackEntity t) { if (onAddToQueue != null) onAddToQueue.accept(t); }

    // ══════════════════════════════════════════════════════════════════════════
    //  HELPERS DE ESTILO
    // ══════════════════════════════════════════════════════════════════════════

    private static void styleCombo(ComboBox<?> c) {
        c.setStyle(
            "-fx-background-color:#091521;" +
            "-fx-border-color:#20384c;-fx-border-width:1;" +
            "-fx-border-radius:8;-fx-background-radius:8;" +
            "-fx-text-fill:#eef7ff;-fx-font-size:12px;" +
            "-fx-prompt-text-fill:#5f788e;"
        );
    }

    private static Button iconBtn(String icon) {
        Button b = new Button(icon);
        b.setPrefWidth(28);
        b.setPrefHeight(28);
        b.setMinWidth(28);
        b.setMinHeight(28);
        b.setMaxWidth(28);
        b.setMaxHeight(28);
        String base =
            "-fx-background-color:transparent;" +
            "-fx-border-color:transparent;" +
            "-fx-background-radius:4;-fx-border-radius:4;" +
            "-fx-text-fill:#5f788e;-fx-font-size:15px;" +
            "-fx-alignment:center;-fx-cursor:hand;-fx-padding:0;";
        String hover =
            "-fx-background-color:rgba(255,255,255,0.07);" +
            "-fx-border-color:transparent;" +
            "-fx-background-radius:4;-fx-border-radius:4;" +
            "-fx-text-fill:#eef7ff;-fx-font-size:15px;" +
            "-fx-alignment:center;-fx-cursor:hand;-fx-padding:0;";
        b.setStyle(base);
        b.setOnMouseEntered(e -> b.setStyle(hover));
        b.setOnMouseExited(e -> b.setStyle(base));
        return b;
    }

    // ══════════════════════════════════════════════════════════════════════════
    //  HELPERS DE TIPO
    // ══════════════════════════════════════════════════════════════════════════

    private static String typeLabel(TrackType t) {
        return switch (t) {
            case MUSIC   -> "MUSIC";
            case VINHETA -> "VINHETA";
            case JINGLE  -> "JINGLE";
            case SPOT    -> "SPOT";
            case EFEITOS -> "EFEITOS";
        };
    }

    private static String typeBg(TrackType t) {
        return switch (t) {
            case MUSIC   -> "rgba(0,188,212,0.15)";
            case VINHETA -> "rgba(156,39,176,0.2)";
            case JINGLE  -> "rgba(76,175,80,0.2)";
            case SPOT    -> "rgba(255,152,0,0.2)";
            case EFEITOS -> "rgba(244,67,54,0.15)";
        };
    }

    private static String typeColor(TrackType t) {
        return switch (t) {
            case MUSIC   -> "#00d4ff";
            case VINHETA -> "#ce93d8";
            case JINGLE  -> "#a5d6a7";
            case SPOT    -> "#ffcc80";
            case EFEITOS -> "#ef9a9a";
        };
    }

    // ══════════════════════════════════════════════════════════════════════════
    //  FORMATAÇÃO DE TEMPO
    // ══════════════════════════════════════════════════════════════════════════

    private static String msToTime(int ms) {
        int s   = ms / 1000;
        int h   = s / 3600;
        int m   = (s % 3600) / 60;
        int sec = s % 60;
        return h > 0
            ? String.format("%d:%02d:%02d", h, m, sec)
            : String.format("%02d:%02d", m, sec);
    }

    private static String nvl(String s) {
        return (s == null || s.isBlank()) ? "—" : s;
    }
}
