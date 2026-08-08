package com.audionplay.ui.components;

import com.audionplay.db.entity.TrackEntity;
import com.audionplay.db.entity.TrackEntity.TrackType;
import com.audionplay.db.repository.TrackRepository;
import com.audionplay.ui.Theme;
import javafx.application.Platform;
import javafx.collections.FXCollections;
import javafx.collections.ObservableList;
import javafx.collections.transformation.FilteredList;
import javafx.geometry.Insets;
import javafx.geometry.Pos;
import javafx.scene.control.*;
import javafx.scene.layout.*;

import java.sql.SQLException;
import java.util.List;
import java.util.function.Consumer;

import static com.audionplay.ui.Theme.*;

/**
 * Tela de catálogo de músicas.
 *
 * Carrega faixas do {@link TrackRepository} de forma assíncrona e exibe
 * em uma TableView com busca textual e filtro por tipo.
 *
 * Quando o usuário clica em "Tocar Agora", dispara {@link #setOnPlay}.
 */
public class CatalogPanel extends VBox {

    // ── Dados ─────────────────────────────────────────────────────────────────
    private final TrackRepository repo = new TrackRepository();
    private final ObservableList<TrackEntity> masterList  = FXCollections.observableArrayList();
    private final FilteredList<TrackEntity>   filteredList = new FilteredList<>(masterList, t -> true);

    private TrackType activeType = null;
    private String    searchTerm = "";

    // ── Componentes com estado ────────────────────────────────────────────────
    private Label          statsLabel;
    private Label          statusLabel;
    private TableView<TrackEntity> table;
    private VBox           detailBar;
    private Label          detailTitle;
    private Label          detailMeta;
    private Button         playBtn;
    private Button         activeTypeBtn;

    // ── Callbacks ─────────────────────────────────────────────────────────────
    private Consumer<TrackEntity> onPlay;
    private Consumer<TrackEntity> onAddToQueue;

    public CatalogPanel() {
        super(0);
        setStyle("-fx-background-color:" + BG_MAIN + ";");
        VBox.setVgrow(this, Priority.ALWAYS);

        getChildren().addAll(
            buildHeader(),
            buildToolbar(),
            buildTable(),
            buildDetailBar()
        );

        loadAsync();
    }

    public void setOnPlay(Consumer<TrackEntity> handler)       { this.onPlay = handler; }
    public void setOnAddToQueue(Consumer<TrackEntity> handler) { this.onAddToQueue = handler; }

    /** Recarrega o catálogo do banco. */
    public void reload() { loadAsync(); }

    // ══════════════════════════════════════════════════════════════════════════
    //  LAYOUT
    // ══════════════════════════════════════════════════════════════════════════

    private HBox buildHeader() {
        statsLabel = Theme.lbl("", "-fx-font-size:11px;-fx-text-fill:" + TEXT_SEC + ";");

        Button reloadBtn = new Button("↻  Recarregar");
        reloadBtn.setStyle(
            "-fx-background-color:transparent;-fx-text-fill:" + BLUE + ";" +
            "-fx-font-size:12px;-fx-cursor:hand;-fx-padding:0;"
        );
        reloadBtn.setOnAction(e -> reload());

        HBox hdr = new HBox(12,
            Theme.lbl("CATÁLOGO DE MÚSICAS",
                "-fx-font-size:13px;-fx-font-weight:bold;-fx-text-fill:" + TEXT_PRI + ";"),
            Theme.hSpacer(),
            statsLabel,
            reloadBtn
        );
        hdr.setAlignment(Pos.CENTER_LEFT);
        hdr.setPadding(new Insets(16, 20, 10, 20));
        hdr.setStyle("-fx-border-color:" + BORDER + ";-fx-border-width:0 0 1 0;");
        return hdr;
    }

    private VBox buildToolbar() {
        // ── Busca ──────────────────────────────────────────────────────────────
        TextField search = new TextField();
        search.setPromptText("Buscar por título, artista ou álbum...");
        search.setStyle(
            "-fx-background-color:#12151E;-fx-border-color:" + BORDER + ";" +
            "-fx-border-radius:6;-fx-background-radius:6;" +
            "-fx-text-fill:" + TEXT_PRI + ";-fx-prompt-text-fill:" + TEXT_MUT + ";" +
            "-fx-padding:8 12 8 12;"
        );
        HBox.setHgrow(search, Priority.ALWAYS);
        search.textProperty().addListener((o, ov, nv) -> {
            searchTerm = nv == null ? "" : nv.trim().toLowerCase();
            updateFilter();
        });

        // ── Filtros de tipo ────────────────────────────────────────────────────
        HBox filters = new HBox(6);
        filters.setAlignment(Pos.CENTER_LEFT);

        Button btnAll = typeFilterBtn("Todos", null);
        activeTypeBtn = btnAll;
        setTypeActive(btnAll);

        filters.getChildren().addAll(
            btnAll,
            typeFilterBtn("Música",  TrackType.MUSIC),
            typeFilterBtn("Vinheta", TrackType.VINHETA),
            typeFilterBtn("Jingle",  TrackType.JINGLE),
            typeFilterBtn("Spot",    TrackType.SPOT),
            typeFilterBtn("Efeitos", TrackType.EFEITOS)
        );

        // ── Status de carregamento ─────────────────────────────────────────────
        statusLabel = Theme.lbl("Carregando catálogo...", "-fx-font-size:10px;-fx-text-fill:" + TEXT_MUT + ";");

        HBox row1 = new HBox(10, search);
        row1.setAlignment(Pos.CENTER_LEFT);

        HBox row2 = new HBox();
        row2.setAlignment(Pos.CENTER_LEFT);
        row2.getChildren().addAll(filters, Theme.hSpacer(), statusLabel);

        VBox toolbar = new VBox(8, row1, row2);
        toolbar.setPadding(new Insets(10, 20, 10, 20));
        toolbar.setStyle("-fx-border-color:" + BORDER + ";-fx-border-width:0 0 1 0;");
        return toolbar;
    }

    private VBox buildTable() {
        table = new TableView<>(filteredList);
        table.setColumnResizePolicy(TableView.CONSTRAINED_RESIZE_POLICY_FLEX_LAST_COLUMN);
        table.setStyle(
            "-fx-base:#1A1D27;-fx-background:#1A1D27;" +
            "-fx-control-inner-background:#1A1D27;" +
            "-fx-control-inner-background-alt:#13161E;" +
            "-fx-accent:" + BLUE + ";" +
            "-fx-selection-bar:#252A40;" +
            "-fx-selection-bar-non-focused:#1E2235;" +
            "-fx-table-header-border-color:" + BORDER + ";" +
            "-fx-font-size:13px;"
        );
        table.setPlaceholder(Theme.lbl("Nenhuma faixa encontrada",
            "-fx-font-size:13px;-fx-text-fill:" + TEXT_MUT + ";"));

        table.getColumns().addAll(
            colIndex(),
            colText("Título",  200, true,  t -> t.title()),
            colText("Artista", 140, true,  t -> t.artist()),
            colText("Álbum",   120, true,  t -> t.album()),
            colType(),
            colDuration()
        );

        // Mostra barra de detalhes ao selecionar
        table.getSelectionModel().selectedItemProperty().addListener((o, ov, track) -> {
            if (track != null) updateDetailBar(track);
            detailBar.setVisible(track != null);
            detailBar.setManaged(track != null);
        });

        // Duplo clique → adicionar à fila
        table.setRowFactory(tv -> {
            TableRow<TrackEntity> row = new TableRow<>();
            row.setOnMouseClicked(e -> {
                if (e.getClickCount() == 2 && !row.isEmpty()) fireAddToQueue(row.getItem());
            });
            return row;
        });

        VBox.setVgrow(table, Priority.ALWAYS);
        VBox wrapper = new VBox(table);
        VBox.setVgrow(wrapper, Priority.ALWAYS);
        return wrapper;
    }

    private VBox buildDetailBar() {
        detailTitle = Theme.lbl("", "-fx-font-size:14px;-fx-font-weight:bold;-fx-text-fill:" + TEXT_PRI + ";");
        detailMeta  = Theme.lbl("", "-fx-font-size:11px;-fx-text-fill:" + TEXT_SEC + ";");

        playBtn = new Button("▶  Tocar Agora");
        playBtn.setStyle(
            "-fx-background-color:" + BLUE + ";-fx-text-fill:white;" +
            "-fx-font-size:13px;-fx-font-weight:bold;" +
            "-fx-background-radius:7;-fx-padding:8 20 8 20;-fx-cursor:hand;"
        );
        playBtn.setOnAction(e -> {
            TrackEntity selected = table.getSelectionModel().getSelectedItem();
            if (selected != null) firePlay(selected);
        });

        VBox info = new VBox(3, detailTitle, detailMeta);
        info.setAlignment(Pos.CENTER_LEFT);
        HBox.setHgrow(info, Priority.ALWAYS);

        detailBar = new VBox();
        HBox content = new HBox(16, info, playBtn);
        content.setAlignment(Pos.CENTER_LEFT);
        detailBar.getChildren().add(content);
        detailBar.setPadding(new Insets(12, 20, 12, 20));
        detailBar.setStyle(
            "-fx-background-color:" + BG_PANEL + ";" +
            "-fx-border-color:" + BORDER + ";-fx-border-width:1 0 0 0;"
        );
        detailBar.setVisible(false);
        detailBar.setManaged(false);
        return detailBar;
    }

    // ══════════════════════════════════════════════════════════════════════════
    //  COLUNAS
    // ══════════════════════════════════════════════════════════════════════════

    @SuppressWarnings("unchecked")
    private TableColumn<TrackEntity, Integer> colIndex() {
        TableColumn<TrackEntity, Integer> col = new TableColumn<>("#");
        col.setPrefWidth(48); col.setMinWidth(48); col.setMaxWidth(48);
        col.setResizable(false);
        col.setCellFactory(tc -> new TableCell<>() {
            @Override
            protected void updateItem(Integer item, boolean empty) {
                super.updateItem(item, empty);
                setText(empty ? null : String.valueOf(getIndex() + 1));
                setStyle("-fx-text-fill:" + TEXT_MUT + ";-fx-alignment:CENTER-RIGHT;-fx-padding:0 8 0 0;");
            }
        });
        return col;
    }

    @FunctionalInterface
    private interface TrackGetter { String get(TrackEntity t); }

    @SuppressWarnings("unchecked")
    private TableColumn<TrackEntity, String> colText(String header, int prefWidth,
                                                      boolean grows, TrackGetter getter) {
        TableColumn<TrackEntity, String> col = new TableColumn<>(header);
        if (grows) {
            col.setPrefWidth(prefWidth);
            col.setMinWidth(80);
        } else {
            col.setPrefWidth(prefWidth);
            col.setMinWidth(prefWidth);
            col.setMaxWidth(prefWidth);
            col.setResizable(false);
        }
        col.setCellValueFactory(data -> {
            String v = getter.get(data.getValue());
            return new javafx.beans.property.SimpleStringProperty(v == null ? "" : v);
        });
        col.setCellFactory(tc -> new TableCell<>() {
            @Override
            protected void updateItem(String item, boolean empty) {
                super.updateItem(item, empty);
                setText(empty || item == null ? null : item);
                setStyle("-fx-text-fill:" + TEXT_PRI + ";-fx-padding:0 8 0 8;");
            }
        });
        return col;
    }

    @SuppressWarnings("unchecked")
    private TableColumn<TrackEntity, TrackType> colType() {
        TableColumn<TrackEntity, TrackType> col = new TableColumn<>("Tipo");
        col.setPrefWidth(90); col.setMinWidth(90); col.setMaxWidth(90);
        col.setResizable(false);
        col.setCellValueFactory(data ->
            new javafx.beans.property.SimpleObjectProperty<>(data.getValue().type()));
        col.setCellFactory(tc -> new TableCell<>() {
            @Override
            protected void updateItem(TrackType type, boolean empty) {
                super.updateItem(type, empty);
                if (empty || type == null) { setText(null); setStyle(""); return; }
                String color = typeColor(type);
                setText(typeLabel(type));
                setStyle(
                    "-fx-text-fill:" + color + ";" +
                    "-fx-font-size:10px;-fx-font-weight:bold;" +
                    "-fx-alignment:CENTER;-fx-padding:0 4 0 4;"
                );
            }
        });
        return col;
    }

    @SuppressWarnings("unchecked")
    private TableColumn<TrackEntity, Integer> colDuration() {
        TableColumn<TrackEntity, Integer> col = new TableColumn<>("Duração");
        col.setPrefWidth(80); col.setMinWidth(80); col.setMaxWidth(80);
        col.setResizable(false);
        col.setCellValueFactory(data ->
            new javafx.beans.property.SimpleIntegerProperty(data.getValue().durationMs()).asObject());
        col.setCellFactory(tc -> new TableCell<>() {
            @Override
            protected void updateItem(Integer ms, boolean empty) {
                super.updateItem(ms, empty);
                setText(empty || ms == null ? null : formatMs(ms));
                setStyle("-fx-text-fill:" + TEXT_SEC + ";-fx-alignment:CENTER-RIGHT;-fx-padding:0 12 0 0;");
            }
        });
        return col;
    }

    // ══════════════════════════════════════════════════════════════════════════
    //  FILTROS
    // ══════════════════════════════════════════════════════════════════════════

    private Button typeFilterBtn(String label, TrackType type) {
        Button btn = new Button(label);
        setTypeInactive(btn);
        btn.setOnAction(e -> {
            if (activeTypeBtn != null) setTypeInactive(activeTypeBtn);
            setTypeActive(btn);
            activeTypeBtn = btn;
            activeType = type;
            updateFilter();
        });
        return btn;
    }

    private void updateFilter() {
        filteredList.setPredicate(track -> {
            boolean typeOk = (activeType == null) || (track.type() == activeType);
            boolean textOk = searchTerm.isEmpty()
                || track.title().toLowerCase().contains(searchTerm)
                || track.artist().toLowerCase().contains(searchTerm)
                || (track.album() != null && track.album().toLowerCase().contains(searchTerm));
            return typeOk && textOk;
        });
        updateStats();
    }

    private void updateStats() {
        long total = filteredList.size();
        long totalMs = filteredList.stream().mapToLong(TrackEntity::durationMs).sum();
        statsLabel.setText(total + " faixas · " + formatMs((int) totalMs));
    }

    private void updateDetailBar(TrackEntity t) {
        detailTitle.setText(t.title());
        StringBuilder meta = new StringBuilder();
        if (t.artist() != null && !t.artist().isBlank()) meta.append(t.artist()).append("  ·  ");
        meta.append(formatMs(t.durationMs())).append("  ·  ").append(typeLabel(t.type()));
        if (t.loudnessLufs() != null)
            meta.append(String.format("  ·  %.1f LUFS", t.loudnessLufs()));
        detailMeta.setText(meta.toString());
    }

    private static void setTypeActive(Button btn) {
        btn.setStyle(
            "-fx-background-color:" + BLUE + ";-fx-text-fill:white;" +
            "-fx-font-size:12px;-fx-background-radius:6;" +
            "-fx-padding:5 14 5 14;-fx-cursor:hand;"
        );
    }

    private static void setTypeInactive(Button btn) {
        btn.setStyle(
            "-fx-background-color:#14172280;-fx-text-fill:" + TEXT_SEC + ";" +
            "-fx-font-size:12px;-fx-background-radius:6;" +
            "-fx-border-color:" + BORDER + ";-fx-border-radius:6;-fx-border-width:1;" +
            "-fx-padding:5 14 5 14;-fx-cursor:hand;"
        );
    }

    // ══════════════════════════════════════════════════════════════════════════
    //  CARREGAMENTO ASSÍNCRONO
    // ══════════════════════════════════════════════════════════════════════════

    private void loadAsync() {
        statusLabel.setText("Carregando catálogo...");
        masterList.clear();

        Thread t = new Thread(() -> {
            try {
                List<TrackEntity> tracks = repo.findAll();
                Platform.runLater(() -> {
                    masterList.setAll(tracks);
                    updateFilter();
                    statusLabel.setText("");
                });
            } catch (SQLException e) {
                Platform.runLater(() ->
                    statusLabel.setText("Erro ao carregar: " + e.getMessage()));
            }
        }, "catalog-loader");
        t.setDaemon(true);
        t.start();
    }

    private void firePlay(TrackEntity track) {
        if (onPlay != null) onPlay.accept(track);
    }

    private void fireAddToQueue(TrackEntity track) {
        if (onAddToQueue != null) onAddToQueue.accept(track);
    }

    // ══════════════════════════════════════════════════════════════════════════
    //  HELPERS
    // ══════════════════════════════════════════════════════════════════════════

    private static String formatMs(int ms) {
        int s   = ms / 1000;
        int h   = s / 3600;
        int m   = (s % 3600) / 60;
        int sec = s % 60;
        return h > 0
            ? String.format("%d:%02d:%02d", h, m, sec)
            : String.format("%d:%02d", m, sec);
    }

    private static String typeLabel(TrackType type) {
        return switch (type) {
            case MUSIC   -> "Música";
            case VINHETA -> "Vinheta";
            case JINGLE  -> "Jingle";
            case SPOT    -> "Spot";
            case EFEITOS -> "Efeitos";
        };
    }

    private static String typeColor(TrackType type) {
        return switch (type) {
            case MUSIC   -> ACC_MUS;   // teal
            case VINHETA -> ACC_JIN;   // roxo
            case JINGLE  -> "#CE93D8"; // lilás
            case SPOT    -> "#FFB74D"; // laranja
            case EFEITOS -> TEXT_SEC;  // cinza
        };
    }
}
