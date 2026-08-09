package com.audionplay.ui.components;

import com.audionplay.db.entity.*;
import com.audionplay.scheduler.PlaylistGenerator;
import com.audionplay.scheduler.PlaylistGenerator.GeneratedItem;
import com.audionplay.scheduler.PlaylistGenerator.GenerateResult;
import com.audionplay.db.entity.TrackEntity.TrackType;
import com.audionplay.db.repository.CategoryRepository;
import com.audionplay.db.repository.ClockRepository;
import com.audionplay.db.repository.SeparationRuleRepository;
import javafx.application.Platform;
import javafx.geometry.Insets;
import javafx.geometry.Pos;
import javafx.scene.Node;
import javafx.scene.control.*;
import javafx.scene.input.*;
import javafx.scene.layout.*;
import javafx.scene.SnapshotParameters;
import javafx.scene.paint.Color;
import javafx.stage.Popup;

import java.time.LocalDate;
import java.time.LocalDateTime;
import java.util.*;
import java.util.function.Consumer;

/**
 * Tela de Rotação — fiel ao player/player.html#view-rotacao.
 *
 * 4 abas: Clocks | Grade | Regras | Playlist
 */
public class RotacaoPanel extends VBox {

    // ── Paleta ────────────────────────────────────────────────────────────────
    private static final String BG_MAIN  = "#071019";
    private static final String BG_CARD  = "#0a1a27";
    private static final String BORDER   = "#20384c";
    private static final String CYAN     = "#20e6ff";
    private static final String TEXT_DIM = "#88a0b5";
    private static final String TEXT_HI  = "rgba(220,239,242,0.9)";

    private static final String[] DAYS = {"Dom","Seg","Ter","Qua","Qui","Sex","Sáb"};
    private static final String[] SLOT_TYPES = {"CATEGORY","JINGLE","SPOT","VINHETA","HORA_CERTA","FIXED"};
    private static final Map<String,String> TYPE_COLOR = Map.of(
        "HORA_CERTA","#4a9eff","VINHETA","#4ac98a","CATEGORY","#20e6ff",
        "JINGLE","#f59e0b","SPOT","#f97316","FIXED","#94a3b8"
    );
    private static final Map<String,String> TYPE_LBL = Map.of(
        "HORA_CERTA","HC","VINHETA","VH","CATEGORY","CAT",
        "JINGLE","JGL","SPOT","SPT","FIXED","FIX"
    );
    private static final String[] GRID_PALETTE = {
        "#4a9eff","#f59e0b","#4ac98a","#f97316","#a78bfa",
        "#f472b6","#34d399","#fb7185","#60a5fa","#fbbf24"
    };

    // ── Repositórios ──────────────────────────────────────────────────────────
    private final ClockRepository          clockRepo  = new ClockRepository();
    private final SeparationRuleRepository rulesRepo  = new SeparationRuleRepository();
    private final CategoryRepository       catRepo    = new CategoryRepository();

    // ── Estado ────────────────────────────────────────────────────────────────
    private List<ClockEntity>        clocks     = new ArrayList<>();
    private List<CategoryEntity>     categories = new ArrayList<>();
    private List<ClockScheduleEntry> gridData   = new ArrayList<>();
    private List<SeparationRuleEntity> rules    = new ArrayList<>();

    // ── Containers das abas ───────────────────────────────────────────────────
    private VBox    paneClocks, paneGrade, paneRegras, paneGerar;
    private VBox    clockListVBox, rulesListVBox, gerarResultVBox, gerarWarnVBox;
    private Button  gerarBtn, enqueueBtn;
    private Label   gerarSummaryLbl;
    private VBox    gerarSummaryBox;
    private Button[] tabBtns;
    private String  currentPane = "clocks";

    // ── Grade 24×7 ────────────────────────────────────────────────────────────
    private Button[][] gridBtns;  // [hour][day]
    private Map<String,String> clockColorMap = new HashMap<>();

    // ── DnD de slots ─────────────────────────────────────────────────────────
    private String draggedSlotId;
    private String draggedClockId;

    // ── Gerar (Playlist) ──────────────────────────────────────────────────────
    private final PlaylistGenerator playlistGenerator = new PlaylistGenerator();
    private List<GeneratedItem> gerarItems = new ArrayList<>();
    private Consumer<List<TrackEntity>> onEnqueue;
    private Consumer<TrackEntity>       onCue;

    // ── Popup grade ───────────────────────────────────────────────────────────
    private Popup gridPopup;
    private int   pickerWeekday, pickerHour;

    // ─────────────────────────────────────────────────────────────────────────
    public RotacaoPanel() {
        super(0);
        setStyle("-fx-background-color:" + BG_MAIN + ";");
        VBox.setVgrow(this, Priority.ALWAYS);
        buildUI();
    }

    public void setOnEnqueue(Consumer<List<TrackEntity>> cb) { this.onEnqueue = cb; }
    public void setOnCue(Consumer<TrackEntity> cb)           { this.onCue = cb; }

    // ── Construção da UI ──────────────────────────────────────────────────────

    private void buildUI() {
        // Header
        HBox header = new HBox();
        header.setAlignment(Pos.CENTER_LEFT);
        header.setPadding(new Insets(10, 14, 10, 14));
        header.setStyle("-fx-border-color:transparent transparent " + BORDER + " transparent;-fx-border-width:0 0 1 0;");
        Label title = new Label("Rotação");
        title.setStyle("-fx-font-size:13px;-fx-font-weight:800;-fx-text-fill:" + TEXT_HI + ";");
        header.getChildren().add(title);

        // Sub-nav tabs
        tabBtns = new Button[4];
        String[] labels = {"Clocks","Grade","Regras","Playlist"};
        String[] panes  = {"clocks","grade","regras","gerar"};
        HBox subnav = new HBox(0);
        subnav.setStyle("-fx-border-color:transparent transparent " + BORDER + " transparent;-fx-border-width:0 0 1 0;");
        for (int i = 0; i < 4; i++) {
            final String pane = panes[i];
            Button btn = new Button(labels[i]);
            applyTabInactive(btn);
            btn.setOnAction(e -> switchPane(pane));
            tabBtns[i] = btn;
            subnav.getChildren().add(btn);
        }

        // Panes
        paneClocks = buildClocksPane();
        paneGrade  = buildGradePane();
        paneRegras = buildRegrasPane();
        paneGerar  = buildGerarPane();

        StackPane content = new StackPane(paneClocks, paneGrade, paneRegras, paneGerar);
        VBox.setVgrow(content, Priority.ALWAYS);

        getChildren().addAll(header, subnav, content);
        switchPane("clocks");
    }

    private void switchPane(String pane) {
        currentPane = pane;
        String[] panes = {"clocks","grade","regras","gerar"};
        Node[]   nodes = {paneClocks, paneGrade, paneRegras, paneGerar};
        for (int i = 0; i < panes.length; i++) {
            boolean active = panes[i].equals(pane);
            nodes[i].setVisible(active);
            nodes[i].setManaged(active);
            if (active) applyTabActive(tabBtns[i]);
            else        applyTabInactive(tabBtns[i]);
        }
        // Carrega dados para a aba selecionada
        switch (pane) {
            case "clocks" -> { loadCategories(); loadClocks(); }
            case "grade"  -> { loadCategories(); loadClocks(); loadGrid(); }
            case "regras" -> loadRules();
            case "gerar"  -> {}
        }
    }

    // ── Estilos de tab ────────────────────────────────────────────────────────

    private static void applyTabActive(Button btn) {
        btn.setStyle(
            "-fx-background-color:transparent;-fx-border-color:transparent transparent " + CYAN + " transparent;" +
            "-fx-border-width:0 0 2 0;-fx-text-fill:" + CYAN + ";-fx-font-size:11px;-fx-font-weight:700;" +
            "-fx-padding:7 14 7 14;-fx-cursor:hand;"
        );
    }

    private static void applyTabInactive(Button btn) {
        btn.setStyle(
            "-fx-background-color:transparent;-fx-border-color:transparent;" +
            "-fx-border-width:0 0 2 0;-fx-text-fill:" + TEXT_DIM + ";-fx-font-size:11px;-fx-font-weight:700;" +
            "-fx-padding:7 14 7 14;-fx-cursor:hand;"
        );
        btn.setOnMouseEntered(e -> btn.setStyle(btn.getStyle().replace("-fx-text-fill:" + TEXT_DIM, "-fx-text-fill:#c8c8c8")));
        btn.setOnMouseExited(e  -> btn.setStyle(btn.getStyle().replace("-fx-text-fill:#c8c8c8", "-fx-text-fill:" + TEXT_DIM)));
    }

    // ═════════════════════════════════════════════════════════════════════════
    // PANE: CLOCKS
    // ═════════════════════════════════════════════════════════════════════════

    private VBox buildClocksPane() {
        VBox pane = new VBox(0);
        VBox.setVgrow(pane, Priority.ALWAYS);

        // Action bar
        TextField newClockField = new TextField();
        newClockField.setPromptText("Nome do clock (ex: Manhã Adulto)");
        newClockField.setStyle(inputStyle());
        HBox.setHgrow(newClockField, Priority.ALWAYS);

        HBox newClockForm = new HBox(6, newClockField, smBtn("Criar", null), smBtn("Cancelar", null));
        newClockForm.setAlignment(Pos.CENTER_LEFT);
        newClockForm.setPadding(new Insets(6, 10, 6, 10));
        newClockForm.setStyle("-fx-border-color:transparent transparent " + BORDER + " transparent;-fx-border-width:0 0 1 0;");
        newClockForm.setVisible(false);
        newClockForm.setManaged(false);

        Button btnCreate = (Button) newClockForm.getChildren().get(1);
        Button btnCancel = (Button) newClockForm.getChildren().get(2);
        btnCancel.setOnAction(e -> { newClockForm.setVisible(false); newClockForm.setManaged(false); newClockField.clear(); });
        btnCreate.setOnAction(e -> {
            String name = newClockField.getText().trim();
            if (name.isBlank()) return;
            new Thread(() -> {
                try {
                    clockRepo.create(name);
                    Platform.runLater(() -> { newClockField.clear(); newClockForm.setVisible(false); newClockForm.setManaged(false); loadClocks(); });
                } catch (Exception ex) { Platform.runLater(() -> Toast.show(null, "Erro: " + ex.getMessage())); }
            }).start();
        });

        Button btnNew = smBtn("+ Novo clock", () -> { newClockForm.setVisible(true); newClockForm.setManaged(true); newClockField.requestFocus(); });
        Button btnRefresh = smBtn("↺ Atualizar", this::loadClocks);

        HBox actionBar = actionBar(btnRefresh, btnNew);

        clockListVBox = new VBox(6);
        clockListVBox.setPadding(new Insets(10, 12, 10, 12));

        ScrollPane scroll = scroll(clockListVBox);
        VBox.setVgrow(scroll, Priority.ALWAYS);

        pane.getChildren().addAll(actionBar, newClockForm, scroll);
        return pane;
    }

    private void loadCategories() {
        new Thread(() -> {
            try { List<CategoryEntity> cats = catRepo.findAll(); Platform.runLater(() -> categories = cats); }
            catch (Exception e) { /* silencioso */ }
        }).start();
    }

    private void loadClocks() {
        new Thread(() -> {
            try {
                List<ClockEntity> list = clockRepo.findAll();
                Platform.runLater(() -> { clocks = list; renderClocks(); });
            } catch (Exception e) {
                Platform.runLater(() -> { clockListVBox.getChildren().setAll(statusLabel("Erro ao carregar clocks")); });
            }
        }).start();
    }

    private void renderClocks() {
        clockListVBox.getChildren().clear();
        if (clocks.isEmpty()) {
            clockListVBox.getChildren().add(statusLabel("Nenhum clock. Crie um clicando em \"+ Novo clock\"."));
            return;
        }
        for (ClockEntity clock : clocks) clockListVBox.getChildren().add(buildClockCard(clock));
    }

    private VBox buildClockCard(ClockEntity clock) {
        // Header
        Label nameLabel  = new Label(clock.name().toUpperCase());
        nameLabel.setStyle("-fx-font-size:12px;-fx-font-weight:700;-fx-text-fill:" + TEXT_HI + ";");
        HBox.setHgrow(nameLabel, Priority.ALWAYS);

        Label countLabel = new Label(clock.slotCount() + " slots");
        countLabel.setStyle("-fx-font-size:10px;-fx-text-fill:" + TEXT_DIM + ";");

        Label hintTotal  = new Label(clock.totalHintMs() > 0 ? "~" + fmtHint(clock.totalHintMs()) : "");
        hintTotal.setStyle("-fx-font-size:10px;-fx-text-fill:" + TEXT_DIM + ";");

        Button delBtn = new Button("✕");
        delBtn.setStyle("-fx-background-color:transparent;-fx-border-color:transparent;-fx-text-fill:" + TEXT_DIM + ";-fx-cursor:hand;-fx-font-size:12px;-fx-padding:0 4 0 4;");
        delBtn.setOnMouseEntered(e -> delBtn.setStyle(delBtn.getStyle().replace("-fx-text-fill:" + TEXT_DIM, "-fx-text-fill:#ef4444")));
        delBtn.setOnMouseExited(e  -> delBtn.setStyle(delBtn.getStyle().replace("-fx-text-fill:#ef4444", "-fx-text-fill:" + TEXT_DIM)));

        HBox header = new HBox(8, nameLabel, hintTotal, countLabel, delBtn);
        header.setAlignment(Pos.CENTER_LEFT);
        header.setPadding(new Insets(8, 10, 8, 10));
        header.setCursor(javafx.scene.Cursor.HAND);
        header.setStyle("-fx-background-color:transparent;");
        header.setOnMouseEntered(e -> header.setStyle("-fx-background-color:rgba(255,255,255,0.03);"));
        header.setOnMouseExited(e  -> header.setStyle("-fx-background-color:transparent;"));

        // Body (expandível)
        VBox slotsContainer = new VBox(3);
        slotsContainer.setPadding(new Insets(6, 10, 6, 10));

        // Add-slot form
        HBox addSlotForm = buildAddSlotForm(clock.id(), slotsContainer, countLabel, hintTotal);

        VBox body = new VBox(0, slotsContainer, addSlotForm);
        body.setStyle("-fx-border-color:" + BORDER + " transparent transparent transparent;-fx-border-width:1 0 0 0;");
        body.setVisible(false);
        body.setManaged(false);

        // Toggle expansão
        final boolean[] expanded = {false};
        Runnable toggle = () -> {
            expanded[0] = !expanded[0];
            body.setVisible(expanded[0]);
            body.setManaged(expanded[0]);
            if (expanded[0]) expandClock(clock.id(), slotsContainer, countLabel, hintTotal);
        };
        header.setOnMouseClicked(e -> toggle.run());

        // Delete
        delBtn.setOnAction(e -> {
            if (!new Alert(Alert.AlertType.CONFIRMATION, "Remover clock \"" + clock.name() + "\"?\nIsso também remove todos os slots.", ButtonType.OK, ButtonType.CANCEL)
                    .showAndWait().filter(r -> r == ButtonType.OK).isPresent()) return;
            new Thread(() -> {
                try { clockRepo.delete(clock.id()); Platform.runLater(this::loadClocks); }
                catch (Exception ex) { Platform.runLater(() -> Toast.show(null, "Erro: " + ex.getMessage())); }
            }).start();
        });

        VBox card = new VBox(0, header, body);
        card.setStyle("-fx-background-color:" + BG_CARD + ";-fx-border-color:" + BORDER + ";-fx-border-width:1;-fx-border-radius:8;-fx-background-radius:8;");
        return card;
    }

    private HBox buildAddSlotForm(String clockId, VBox slotsContainer, Label countLabel, Label hintTotal) {
        ComboBox<String> typeCombo = new ComboBox<>();
        typeCombo.getItems().addAll(SLOT_TYPES);
        typeCombo.setValue("CATEGORY");
        typeCombo.setStyle(selectStyle() + "-fx-font-size:11px;");
        typeCombo.setPrefWidth(140);

        ComboBox<String> catCombo = new ComboBox<>();
        catCombo.setStyle(selectStyle() + "-fx-font-size:11px;");
        catCombo.setPrefWidth(130);
        refreshCatCombo(catCombo);

        TextField hintField = new TextField("360");
        hintField.setStyle(inputStyle() + "-fx-pref-width:60px;-fx-max-width:70px;");
        hintField.setPromptText("seg");

        // Defaults por tipo
        Map<String,Integer> defaults = Map.of("CATEGORY",360,"HORA_CERTA",3,"VINHETA",12,"JINGLE",30,"SPOT",30);
        typeCombo.setOnAction(e -> {
            String t = typeCombo.getValue();
            catCombo.setVisible("CATEGORY".equals(t));
            catCombo.setManaged("CATEGORY".equals(t));
            hintField.setText(String.valueOf(defaults.getOrDefault(t, 30)));
        });

        Button addBtn = smBtn("+ Adicionar slot", () -> {
            String type = typeCombo.getValue();
            String catId = "CATEGORY".equals(type) ? getCatId(catCombo.getValue()) : null;
            long hint = 0;
            try { hint = Long.parseLong(hintField.getText().trim()) * 1000L; } catch (NumberFormatException ignored) {}
            final long finalHint = hint;
            new Thread(() -> {
                try {
                    clockRepo.addSlot(clockId, type, catId, finalHint);
                    Platform.runLater(() -> expandClock(clockId, slotsContainer, countLabel, hintTotal));
                } catch (Exception ex) { Platform.runLater(() -> Toast.show(null, "Erro: " + ex.getMessage())); }
            }).start();
        });

        HBox form = new HBox(5, typeCombo, catCombo, hintField, addBtn);
        form.setAlignment(Pos.CENTER_LEFT);
        form.setPadding(new Insets(6, 10, 6, 10));
        form.setStyle("-fx-border-color:" + BORDER + " transparent transparent transparent;-fx-border-width:1 0 0 0;");
        return form;
    }

    private void expandClock(String clockId, VBox slotsContainer, Label countLabel, Label hintTotal) {
        new Thread(() -> {
            try {
                List<ClockSlotEntity> slots = clockRepo.findSlots(clockId);
                Platform.runLater(() -> {
                    slotsContainer.getChildren().clear();
                    if (slots.isEmpty()) {
                        slotsContainer.getChildren().add(dimLabel("Nenhum slot. Adicione um abaixo.", 10));
                    } else {
                        List<String> ids = slots.stream().map(ClockSlotEntity::id).toList();
                        for (ClockSlotEntity slot : slots)
                            slotsContainer.getChildren().add(buildSlotRow(slot, clockId, slotsContainer, ids, countLabel, hintTotal));
                    }
                    countLabel.setText(slots.size() + " slots");
                    long total = slots.stream().mapToLong(ClockSlotEntity::durationHintMs).sum();
                    hintTotal.setText(total > 0 ? "~" + fmtHint(total) : "");
                });
            } catch (Exception e) {
                Platform.runLater(() -> slotsContainer.getChildren().setAll(dimLabel("Erro ao carregar slots", 10)));
            }
        }).start();
    }

    private HBox buildSlotRow(ClockSlotEntity slot, String clockId, VBox container,
                              List<String> allIds, Label countLabel, Label hintTotal) {
        // Drag handle
        Label handle = new Label("⠿");
        handle.setStyle("-fx-font-size:13px;-fx-text-fill:" + TEXT_DIM + ";-fx-cursor:hand;-fx-padding:0 4 0 0;");
        handle.setMinWidth(14);

        // Position
        Label posLbl = new Label(String.valueOf(slot.position()));
        posLbl.setStyle("-fx-font-size:12px;-fx-font-weight:700;-fx-text-fill:" + TEXT_DIM + ";-fx-min-width:16px;-fx-alignment:center-right;");

        // Type badge
        String typeColor = TYPE_COLOR.getOrDefault(slot.slotType(), "#888");
        Label typeLbl = new Label(TYPE_LBL.getOrDefault(slot.slotType(), slot.slotType().substring(0, 3)));
        typeLbl.setStyle("-fx-font-size:11px;-fx-font-weight:800;-fx-text-fill:" + typeColor + ";-fx-min-width:26px;-fx-alignment:center;");

        // Category / slot type label
        String catTxt = (slot.categoryName() != null && !slot.categoryName().isBlank())
            ? slot.categoryName() : slot.slotType();
        Label catLbl = new Label(catTxt);
        catLbl.setStyle("-fx-font-size:12px;-fx-text-fill:" + TEXT_HI + ";");
        HBox.setHgrow(catLbl, Priority.ALWAYS);

        // Hint label (clicável para editar)
        Label hintLbl = new Label(fmtHint(slot.durationHintMs()));
        hintLbl.setStyle("-fx-font-size:11px;-fx-font-weight:700;-fx-text-fill:" + TEXT_DIM + ";-fx-cursor:hand;-fx-padding:1 3 1 3;");
        hintLbl.setOnMouseClicked(e -> editSlotHint(hintLbl, slot, clockId, container, allIds, countLabel, hintTotal));

        // Delete button
        Button delBtn = new Button("✕");
        delBtn.setStyle("-fx-background-color:transparent;-fx-border-color:transparent;-fx-text-fill:" + TEXT_DIM + ";-fx-cursor:hand;-fx-font-size:13px;-fx-padding:0 3 0 3;");
        delBtn.setOnMouseEntered(e -> delBtn.setStyle(delBtn.getStyle().replace(TEXT_DIM, "#ef4444")));
        delBtn.setOnMouseExited(e  -> delBtn.setStyle(delBtn.getStyle().replace("#ef4444", TEXT_DIM)));
        delBtn.setOnAction(e -> {
            new Thread(() -> {
                try {
                    clockRepo.deleteSlot(slot.id());
                    Platform.runLater(() -> expandClock(clockId, container, countLabel, hintTotal));
                } catch (Exception ex) { Platform.runLater(() -> Toast.show(null, "Erro: " + ex.getMessage())); }
            }).start();
        });

        HBox row = new HBox(6, handle, posLbl, typeLbl, catLbl, hintLbl, delBtn);
        row.setAlignment(Pos.CENTER_LEFT);
        row.setPadding(new Insets(5, 5, 5, 5));
        row.setStyle(
            "-fx-background-color:rgba(32,56,76,0.18);" +
            "-fx-border-color:" + typeColor + " transparent transparent transparent;" +
            "-fx-border-width:0 0 0 3;" +
            "-fx-background-radius:5;-fx-border-radius:5;"
        );
        row.setOnMouseEntered(e -> row.setStyle(row.getStyle().replace("rgba(32,56,76,0.18)", "rgba(32,56,76,0.38)")));
        row.setOnMouseExited(e  -> row.setStyle(row.getStyle().replace("rgba(32,56,76,0.38)", "rgba(32,56,76,0.18)")));

        // DnD para reordenação
        attachSlotDnd(handle, row, slot, clockId, container, countLabel, hintTotal);

        return row;
    }

    private void editSlotHint(Label hintLbl, ClockSlotEntity slot, String clockId, VBox container,
                              List<String> allIds, Label countLabel, Label hintTotal) {
        long curSec = slot.durationHintMs() / 1000;
        TextField tf = new TextField(curSec > 0 ? String.valueOf(curSec) : "");
        tf.setStyle(inputStyle() + "-fx-max-width:60px;");
        tf.setPromptText("seg");
        HBox parent = (HBox) hintLbl.getParent();
        int idx = parent.getChildren().indexOf(hintLbl);
        parent.getChildren().set(idx, tf);
        tf.requestFocus(); tf.selectAll();

        Runnable commit = () -> {
            long hintMs = 0;
            try { hintMs = Long.parseLong(tf.getText().trim()) * 1000L; } catch (NumberFormatException ignored) {}
            final long finalHint = hintMs;
            new Thread(() -> {
                try {
                    clockRepo.updateSlotHint(slot.id(), finalHint);
                    Platform.runLater(() -> expandClock(clockId, container, countLabel, hintTotal));
                } catch (Exception e) { Platform.runLater(() -> expandClock(clockId, container, countLabel, hintTotal)); }
            }).start();
        };
        tf.setOnAction(e -> commit.run());
        tf.focusedProperty().addListener((ob, ov, nv) -> { if (!nv) commit.run(); });
    }

    private void attachSlotDnd(Label handle, HBox row, ClockSlotEntity slot, String clockId,
                               VBox container, Label countLabel, Label hintTotal) {
        handle.setOnDragDetected(e -> {
            draggedSlotId  = slot.id();
            draggedClockId = clockId;
            Dragboard db = row.startDragAndDrop(TransferMode.MOVE);
            ClipboardContent cc = new ClipboardContent();
            cc.putString(slot.id());
            db.setContent(cc);
            SnapshotParameters sp = new SnapshotParameters();
            sp.setFill(Color.TRANSPARENT);
            db.setDragView(row.snapshot(sp, null), e.getX(), e.getY());
            e.consume();
        });
        row.setOnDragOver(e -> {
            if (draggedSlotId != null && !draggedSlotId.equals(slot.id()))
                e.acceptTransferModes(TransferMode.MOVE);
            e.consume();
        });
        row.setOnDragEntered(e -> { if (draggedSlotId != null && !draggedSlotId.equals(slot.id())) row.setOpacity(0.5); e.consume(); });
        row.setOnDragExited(e  -> { row.setOpacity(1.0); e.consume(); });
        row.setOnDragDropped(e -> {
            row.setOpacity(1.0);
            if (draggedSlotId == null || draggedSlotId.equals(slot.id())) { e.setDropCompleted(false); return; }
            // Reorder in container
            List<String> orderedIds = new ArrayList<>();
            for (Node n : container.getChildren()) {
                if (n instanceof HBox r) {
                    Object ud = r.getUserData();
                    if (ud instanceof String sid) orderedIds.add(sid);
                }
            }
            int fromIdx = orderedIds.indexOf(draggedSlotId);
            int toIdx   = orderedIds.indexOf(slot.id());
            if (fromIdx >= 0 && toIdx >= 0) {
                orderedIds.remove(fromIdx);
                orderedIds.add(fromIdx < toIdx ? toIdx - 1 : toIdx, draggedSlotId);
                new Thread(() -> {
                    try {
                        clockRepo.reorderSlots(clockId, orderedIds);
                        Platform.runLater(() -> expandClock(clockId, container, countLabel, hintTotal));
                    } catch (Exception ex) { Platform.runLater(() -> Toast.show(null, "Erro ao reordenar")); }
                }).start();
            }
            draggedSlotId = null; draggedClockId = null;
            e.setDropCompleted(true); e.consume();
        });
        row.setOnDragDone(e -> { draggedSlotId = null; draggedClockId = null; e.consume(); });
        row.setUserData(slot.id());
    }

    // ═════════════════════════════════════════════════════════════════════════
    // PANE: GRADE 24×7
    // ═════════════════════════════════════════════════════════════════════════

    private VBox buildGradePane() {
        VBox pane = new VBox(0);
        VBox.setVgrow(pane, Priority.ALWAYS);

        Button btnRefresh = smBtn("↺ Atualizar", this::loadGrid);
        Label hint = dimLabel("Clique em uma célula para atribuir um clock", 11);
        HBox actionBar = actionBar(btnRefresh, hint);

        // Grid
        gridBtns = new Button[24][7];
        VBox gridVBox = buildGridVBox();

        ScrollPane scroll = new ScrollPane(gridVBox);
        scroll.setFitToWidth(true);
        scroll.setStyle("-fx-background-color:transparent;-fx-background:transparent;-fx-border-color:transparent;");
        VBox.setVgrow(scroll, Priority.ALWAYS);

        pane.getChildren().addAll(actionBar, scroll);

        // Popup picker
        gridPopup = new Popup();
        gridPopup.setAutoHide(true);

        return pane;
    }

    private VBox buildGridVBox() {
        VBox grid = new VBox(0);
        grid.setPadding(new Insets(10));
        grid.setStyle("-fx-background-color:" + BG_MAIN + ";");

        // Header row
        HBox headerRow = new HBox(0);
        Label hourHead = new Label("Hora");
        hourHead.setMinWidth(72); hourHead.setPrefWidth(72);
        hourHead.setStyle(gridThStyle(false));
        headerRow.getChildren().add(hourHead);
        int todayWeekday = java.time.LocalDate.now().getDayOfWeek().getValue() % 7; // 0=Dom
        for (int d = 0; d < 7; d++) {
            Label dayHead = new Label(DAYS[d]);
            dayHead.setStyle(gridThStyle(d == todayWeekday));
            HBox.setHgrow(dayHead, Priority.ALWAYS);
            dayHead.setMaxWidth(Double.MAX_VALUE);

            // Botão limpar dia
            Button clearBtn = new Button("🗑");
            clearBtn.setStyle("-fx-background-color:transparent;-fx-border-color:transparent;-fx-text-fill:" + TEXT_DIM + ";-fx-cursor:hand;-fx-font-size:10px;-fx-padding:0;");
            clearBtn.setVisible(false);
            final int day = d;
            clearBtn.setOnAction(e -> {
                if (!new Alert(Alert.AlertType.CONFIRMATION, "Limpar todos os clocks de " + DAYS[day] + "?",
                        ButtonType.OK, ButtonType.CANCEL).showAndWait().filter(r -> r == ButtonType.OK).isPresent()) return;
                new Thread(() -> { try { clockRepo.clearDay(day); Platform.runLater(this::loadGrid); } catch (Exception ex) {} }).start();
            });

            HBox th = new HBox(4, dayHead, clearBtn);
            th.setAlignment(Pos.CENTER);
            th.setStyle(gridThStyle(d == todayWeekday));
            th.setOnMouseEntered(e -> clearBtn.setVisible(true));
            th.setOnMouseExited(e  -> clearBtn.setVisible(false));
            HBox.setHgrow(th, Priority.ALWAYS);
            headerRow.getChildren().add(th);
        }
        grid.getChildren().add(headerRow);

        // Period labels
        Map<Integer,String> periods = Map.of(0,"🌙 Madrugada · 00–05h", 6,"🌅 Manhã · 06–11h",
                                             12,"☀ Tarde · 12–17h", 18,"🌆 Noite · 18–23h");
        int currentHour = java.time.LocalTime.now().getHour();

        for (int h = 0; h < 24; h++) {
            if (periods.containsKey(h)) {
                Label sep = new Label(periods.get(h));
                sep.setMaxWidth(Double.MAX_VALUE);
                sep.setStyle(
                    "-fx-background-color:rgba(255,255,255,0.025);" +
                    "-fx-padding:5 10 5 10;-fx-font-size:10px;-fx-font-weight:800;" +
                    "-fx-text-fill:" + TEXT_DIM + ";"
                );
                grid.getChildren().add(sep);
            }

            HBox row = new HBox(0);
            String rowStyle = h == currentHour
                ? "-fx-background-color:rgba(32,230,255,0.03);" : "";

            Label hourCell = new Label(String.format("%02d", h) + "h");
            hourCell.setMinWidth(72); hourCell.setPrefWidth(72);
            hourCell.setStyle(
                "-fx-padding:4 8 4 8;-fx-font-size:13px;-fx-font-weight:800;" +
                "-fx-text-fill:" + (h == currentHour ? CYAN : TEXT_DIM) + ";" +
                "-fx-alignment:center;-fx-background-color:rgba(255,255,255,0.02);" +
                "-fx-border-color:" + BORDER + ";-fx-border-width:0 1 1 0;"
            );
            row.getChildren().add(hourCell);
            row.setStyle(rowStyle);

            for (int d = 0; d < 7; d++) {
                final int fh = h, fd = d;
                Button cell = new Button("—");
                cell.setMaxWidth(Double.MAX_VALUE);
                cell.setStyle(gridCellStyle(null, null));
                HBox.setHgrow(cell, Priority.ALWAYS);
                cell.setOnAction(e -> showGridPicker(cell, fd, fh));
                gridBtns[h][d] = cell;

                StackPane td = new StackPane(cell);
                td.setStyle("-fx-border-color:" + BORDER + ";-fx-border-width:0 1 1 0;");
                HBox.setHgrow(td, Priority.ALWAYS);
                row.getChildren().add(td);
            }
            grid.getChildren().add(row);
        }
        return grid;
    }

    private void loadGrid() {
        new Thread(() -> {
            try {
                List<ClockScheduleEntry> data = clockRepo.getGrid();
                Platform.runLater(() -> { gridData = data; renderGrid(); });
            } catch (Exception e) { /* silencioso */ }
        }).start();
    }

    private void renderGrid() {
        // Mapeia clockId → cor
        clockColorMap.clear();
        int ci = 0;
        for (ClockScheduleEntry e : gridData) {
            if (e.clockId() != null && !clockColorMap.containsKey(e.clockId()))
                clockColorMap.put(e.clockId(), GRID_PALETTE[ci++ % GRID_PALETTE.length]);
        }
        // Limpa todos os botões
        for (int h = 0; h < 24; h++)
            for (int d = 0; d < 7; d++)
                if (gridBtns[h][d] != null) {
                    gridBtns[h][d].setText("—");
                    gridBtns[h][d].setStyle(gridCellStyle(null, null));
                }
        // Preenche com dados
        for (ClockScheduleEntry e : gridData) {
            Button btn = gridBtns[e.hour()][e.weekday()];
            if (btn != null) {
                String color = clockColorMap.get(e.clockId());
                btn.setText(e.clockName() != null ? e.clockName().toUpperCase() : "?");
                btn.setStyle(gridCellStyle(color, e.clockId()));
            }
        }
    }

    private void showGridPicker(Button cell, int weekday, int hour) {
        if (clocks.isEmpty()) { Toast.show(null, "Crie um clock primeiro."); return; }
        gridPopup.getContent().clear();

        VBox pickerBox = new VBox(0);
        pickerBox.setStyle("-fx-background-color:#0c1e2d;-fx-border-color:" + CYAN + ";-fx-border-width:1;-fx-border-radius:8;-fx-background-radius:8;-fx-padding:4 0 4 0;");
        pickerBox.setMinWidth(160);

        Label dayHourLbl = new Label(DAYS[weekday] + " " + String.format("%02d", hour) + "h");
        dayHourLbl.setStyle("-fx-font-size:9px;-fx-font-weight:700;-fx-text-fill:" + TEXT_DIM + ";-fx-padding:4 12 6 12;");
        pickerBox.getChildren().add(dayHourLbl);

        Button clearOpt = new Button("— Limpar");
        clearOpt.setStyle(pickerOptStyle(true));
        clearOpt.setMaxWidth(Double.MAX_VALUE);
        clearOpt.setOnAction(e -> {
            gridPopup.hide();
            new Thread(() -> { try { clockRepo.setGridCell(weekday, hour, null); Platform.runLater(this::loadGrid); } catch (Exception ex) {} }).start();
        });
        pickerBox.getChildren().add(clearOpt);

        for (ClockEntity c : clocks) {
            Button opt = new Button(c.name());
            opt.setStyle(pickerOptStyle(false));
            opt.setMaxWidth(Double.MAX_VALUE);
            opt.setOnAction(e -> {
                gridPopup.hide();
                new Thread(() -> { try { clockRepo.setGridCell(weekday, hour, c.id()); Platform.runLater(this::loadGrid); } catch (Exception ex) {} }).start();
            });
            pickerBox.getChildren().add(opt);
        }

        gridPopup.getContent().add(pickerBox);
        javafx.geometry.Bounds bounds = cell.localToScreen(cell.getBoundsInLocal());
        gridPopup.show(cell.getScene().getWindow(), bounds.getMinX(), bounds.getMaxY() + 4);
    }

    // ═════════════════════════════════════════════════════════════════════════
    // PANE: REGRAS DE SEPARAÇÃO
    // ═════════════════════════════════════════════════════════════════════════

    private VBox buildRegrasPane() {
        VBox pane = new VBox(0);
        VBox.setVgrow(pane, Priority.ALWAYS);

        // New rule form
        String[] fieldKeys   = {"artist","title","album","category"};
        String[] fieldLabels = {"🎤 Artista","🎵 Título","💿 Álbum","🗂 Categoria"};
        ComboBox<String> fieldCombo = new ComboBox<>();
        fieldCombo.getItems().addAll(fieldLabels);
        fieldCombo.setValue(fieldLabels[0]);
        fieldCombo.setStyle(selectStyle());

        TextField minField = new TextField("60");
        minField.setStyle(inputStyle() + "-fx-max-width:60px;");
        minField.setPromptText("min");

        Label dupWarn = new Label("");
        dupWarn.setStyle("-fx-font-size:10px;-fx-text-fill:#f0c040;");
        dupWarn.setVisible(false);

        Button createRuleBtn = smBtn("Criar", null);
        Button cancelRuleBtn = smBtn("Cancelar", null);

        HBox newRuleForm = new HBox(6, fieldCombo, minField,
            dimLabel("minutos", 10), createRuleBtn, cancelRuleBtn, dupWarn);
        newRuleForm.setAlignment(Pos.CENTER_LEFT);
        newRuleForm.setPadding(new Insets(6, 10, 6, 10));
        newRuleForm.setStyle("-fx-border-color:transparent transparent " + BORDER + " transparent;-fx-border-width:0 0 1 0;");
        newRuleForm.setVisible(false);
        newRuleForm.setManaged(false);

        // Verifica duplicata ao trocar campo
        fieldCombo.setOnAction(e -> {
            int fi = fieldCombo.getSelectionModel().getSelectedIndex();
            String fieldKey = fieldKeys[Math.max(0, fi)];
            boolean dup = rules.stream().anyMatch(r -> r.field().equals(fieldKey));
            dupWarn.setText(dup ? "⚠ Já existe uma regra para " + fieldLabels[Math.max(0,fi)] : "");
            dupWarn.setVisible(dup);
            createRuleBtn.setDisable(dup);
        });

        cancelRuleBtn.setOnAction(e -> { newRuleForm.setVisible(false); newRuleForm.setManaged(false); });
        createRuleBtn.setOnAction(e -> {
            int fi = fieldCombo.getSelectionModel().getSelectedIndex();
            String fieldKey = fieldKeys[Math.max(0, fi)];
            int min = 60;
            try { min = Integer.parseInt(minField.getText().trim()); } catch (NumberFormatException ignored) {}
            final int finalMin = min;
            new Thread(() -> {
                try {
                    rulesRepo.create(fieldKey, finalMin);
                    Platform.runLater(() -> { newRuleForm.setVisible(false); newRuleForm.setManaged(false); loadRules(); });
                } catch (Exception ex) { Platform.runLater(() -> Toast.show(null, "Erro: " + ex.getMessage())); }
            }).start();
        });

        Button btnRefresh = smBtn("↺ Atualizar", this::loadRules);
        Button btnNew = smBtn("+ Nova regra", () -> { newRuleForm.setVisible(true); newRuleForm.setManaged(true); });
        HBox actionBar = actionBar(btnRefresh, btnNew);

        rulesListVBox = new VBox(6);
        rulesListVBox.setPadding(new Insets(10, 12, 10, 12));

        ScrollPane scroll = scroll(rulesListVBox);
        VBox.setVgrow(scroll, Priority.ALWAYS);

        pane.getChildren().addAll(actionBar, newRuleForm, scroll);
        return pane;
    }

    private void loadRules() {
        new Thread(() -> {
            try {
                List<SeparationRuleEntity> list = rulesRepo.findAll();
                Platform.runLater(() -> { rules = list; renderRules(); });
            } catch (Exception e) {
                Platform.runLater(() -> rulesListVBox.getChildren().setAll(statusLabel("Erro ao carregar regras")));
            }
        }).start();
    }

    private void renderRules() {
        rulesListVBox.getChildren().clear();
        if (rules.isEmpty()) {
            rulesListVBox.getChildren().add(statusLabel("Nenhuma regra. Clique em \"+ Nova regra\"."));
            return;
        }
        Map<String,String> icons  = Map.of("artist","🎤","title","🎵","album","💿","category","🗂");
        Map<String,String> labels = Map.of("artist","Artista","title","Título","album","Álbum","category","Categoria");
        for (SeparationRuleEntity rule : rules) {
            Label icon  = new Label(icons.getOrDefault(rule.field(), "•"));
            icon.setStyle("-fx-font-size:13px;");
            Label field = new Label(labels.getOrDefault(rule.field(), rule.field()));
            field.setStyle("-fx-font-size:12px;-fx-font-weight:700;-fx-text-fill:" + TEXT_HI + ";");
            HBox.setHgrow(field, Priority.ALWAYS);

            // Separação editável
            Label minLbl = new Label("separação mín.:");
            minLbl.setStyle("-fx-font-size:11px;-fx-text-fill:" + TEXT_DIM + ";");
            Label minVal = new Label(fmtMinutes(rule.minSepMinutes()));
            minVal.setStyle("-fx-font-size:11px;-fx-font-weight:700;-fx-text-fill:" + TEXT_HI + ";-fx-cursor:hand;");
            minVal.setOnMouseClicked(e -> editRuleMin(minVal, rule));

            Button delBtn = smBtn("✕ Remover", () -> {
                if (!new Alert(Alert.AlertType.CONFIRMATION, "Remover esta regra de separação?",
                        ButtonType.OK, ButtonType.CANCEL).showAndWait().filter(r -> r == ButtonType.OK).isPresent()) return;
                new Thread(() -> {
                    try { rulesRepo.delete(rule.id()); Platform.runLater(this::loadRules); }
                    catch (Exception ex) { Platform.runLater(() -> Toast.show(null, "Erro: " + ex.getMessage())); }
                }).start();
            });
            delBtn.setStyle(delBtn.getStyle().replace("-fx-text-fill:" + TEXT_DIM, "-fx-text-fill:#ef4444") + "-fx-text-fill:#ef4444;");

            HBox row = new HBox(8, icon, field, minLbl, minVal, delBtn);
            row.setAlignment(Pos.CENTER_LEFT);
            row.setPadding(new Insets(8, 10, 8, 10));
            row.setStyle("-fx-background-color:" + BG_CARD + ";-fx-border-color:" + BORDER + ";-fx-border-width:1;-fx-border-radius:8;-fx-background-radius:8;");
            rulesListVBox.getChildren().add(row);
        }
    }

    private void editRuleMin(Label minVal, SeparationRuleEntity rule) {
        TextField tf = new TextField(String.valueOf(rule.minSepMinutes()));
        tf.setStyle(inputStyle() + "-fx-max-width:60px;");
        HBox parent = (HBox) minVal.getParent();
        int idx = parent.getChildren().indexOf(minVal);
        parent.getChildren().set(idx, tf);
        tf.requestFocus(); tf.selectAll();
        Runnable commit = () -> {
            int min = rule.minSepMinutes();
            try { min = Integer.parseInt(tf.getText().trim()); } catch (NumberFormatException ignored) {}
            final int finalMin = min;
            new Thread(() -> {
                try { rulesRepo.update(rule.id(), finalMin); Platform.runLater(this::loadRules); }
                catch (Exception e) { Platform.runLater(this::loadRules); }
            }).start();
        };
        tf.setOnAction(e -> commit.run());
        tf.focusedProperty().addListener((ob, ov, nv) -> { if (!nv) commit.run(); });
    }

    // ═════════════════════════════════════════════════════════════════════════
    // PANE: GERAR PLAYLIST
    // ═════════════════════════════════════════════════════════════════════════

    private VBox buildGerarPane() {
        VBox pane = new VBox(0);
        VBox.setVgrow(pane, Priority.ALWAYS);

        // Form de geração
        DatePicker datePicker = new DatePicker(LocalDate.now());
        datePicker.setStyle("-fx-background-color:#071019;-fx-pref-width:140px;");

        ComboBox<String> hourCombo = new ComboBox<>();
        for (int h = 0; h < 24; h++) hourCombo.getItems().add(String.format("%02d:00", h));
        hourCombo.setValue(String.format("%02d:00", java.time.LocalTime.now().getHour()));
        hourCombo.setStyle(selectStyle() + "-fx-pref-width:90px;");

        Spinner<Integer> hoursSpinner = new Spinner<>(1, 24, 1);
        hoursSpinner.setStyle("-fx-pref-width:80px;");

        Label previewLbl = new Label("");
        previewLbl.setStyle("-fx-font-size:11px;-fx-font-weight:700;-fx-text-fill:" + CYAN + ";");

        Runnable updatePreview = () -> {
            LocalDate date = datePicker.getValue();
            int hour = hourCombo.getSelectionModel().getSelectedIndex();
            int hours = hoursSpinner.getValue();
            if (date == null) { previewLbl.setText(""); return; }
            String[] dayNames = {"Dom","Seg","Ter","Qua","Qui","Sex","Sáb"};
            String dayName = dayNames[date.getDayOfWeek().getValue() % 7];
            previewLbl.setText(String.format("%s %02d/%02d · %02d:00 → %02d:00",
                dayName, date.getDayOfMonth(), date.getMonthValue(), hour, (hour + hours) % 24));
        };
        datePicker.setOnAction(e -> updatePreview.run());
        hourCombo.setOnAction(e -> updatePreview.run());
        hoursSpinner.valueProperty().addListener((ob,ov,nv) -> updatePreview.run());
        updatePreview.run();

        gerarBtn    = smBtn("Gerar Playlist", null);
        enqueueBtn  = smBtn("Enviar para o Player", null);
        gerarBtn.setStyle(gerarBtn.getStyle() + "-fx-background-color:#123b40;-fx-text-fill:#bdfcff;-fx-border-color:rgba(43,216,222,0.55);");
        enqueueBtn.setStyle(enqueueBtn.getStyle() + "-fx-background-color:#123b40;-fx-text-fill:#bdfcff;-fx-border-color:rgba(43,216,222,0.55);");
        enqueueBtn.setVisible(false);

        gerarSummaryLbl = new Label("");
        gerarSummaryLbl.setStyle("-fx-font-size:11px;-fx-text-fill:" + TEXT_DIM + ";-fx-padding:4 10 4 10;");
        gerarSummaryBox = new VBox(gerarSummaryLbl);
        gerarSummaryBox.setVisible(false);

        gerarResultVBox = new VBox(4);
        gerarResultVBox.setPadding(new Insets(8, 10, 8, 10));
        gerarResultVBox.getChildren().add(statusLabel("Configure e clique em Gerar"));

        gerarWarnVBox = new VBox(2);
        gerarWarnVBox.setPadding(new Insets(0, 10, 8, 10));
        gerarWarnVBox.setVisible(false);

        VBox genForm = new VBox(6,
            new HBox(8, dimLabel("Dia:", 10), datePicker),
            new HBox(8, dimLabel("Hora:", 10), hourCombo),
            new HBox(8, dimLabel("Horas:", 10), hoursSpinner),
            previewLbl,
            new HBox(8, gerarBtn, enqueueBtn)
        );
        genForm.setPadding(new Insets(8, 10, 8, 10));
        genForm.setStyle("-fx-border-color:transparent transparent " + BORDER + " transparent;-fx-border-width:0 0 1 0;");

        gerarBtn.setOnAction(e -> {
            LocalDate date = datePicker.getValue();
            int hour  = hourCombo.getSelectionModel().getSelectedIndex();
            int hours2 = hoursSpinner.getValue();
            if (date == null) return;
            LocalDateTime from = LocalDateTime.of(date, java.time.LocalTime.of(hour, 0));
            generatePlaylist(from, hours2);
        });

        enqueueBtn.setOnAction(e -> enqueueGenerated());

        ScrollPane scroll = scroll(gerarResultVBox);
        VBox.setVgrow(scroll, Priority.ALWAYS);

        pane.getChildren().addAll(genForm, gerarSummaryBox, gerarWarnVBox, scroll);
        return pane;
    }

    private void generatePlaylist(LocalDateTime from, int hours) {
        gerarBtn.setDisable(true);
        enqueueBtn.setVisible(false);
        gerarItems.clear();
        gerarResultVBox.getChildren().setAll(statusLabel("Gerando playlist…"));
        gerarSummaryBox.setVisible(false);
        gerarWarnVBox.getChildren().clear();
        gerarWarnVBox.setVisible(false);

        new Thread(() -> {
            try {
                GenerateResult result = playlistGenerator.generate(from, hours);
                Platform.runLater(() -> {
                    gerarBtn.setDisable(false);
                    gerarItems = new ArrayList<>(result.items());
                    if (gerarItems.isEmpty()) {
                        gerarResultVBox.getChildren().setAll(
                            statusLabel("Nenhum item gerado. Verifique a grade e as categorias."));
                    } else {
                        renderGerarItems();
                        enqueueBtn.setVisible(true);
                        long totalMs  = gerarItems.stream().mapToLong(i -> i.track().durationMs()).sum();
                        long totalSec = totalMs / 1000;
                        String dur = String.format("%02d:%02d:%02d",
                            totalSec/3600, (totalSec%3600)/60, totalSec%60);
                        gerarSummaryLbl.setText(gerarItems.size() + " faixas · " + dur);
                        gerarSummaryBox.setVisible(true);
                    }
                    if (!result.warnings().isEmpty()) {
                        gerarWarnVBox.getChildren().clear();
                        for (String w : result.warnings()) {
                            Label wl = new Label("⚠ " + w);
                            wl.setStyle("-fx-font-size:10px;-fx-text-fill:#f0c040;");
                            gerarWarnVBox.getChildren().add(wl);
                        }
                        gerarWarnVBox.setVisible(true);
                    }
                });
            } catch (Exception e) {
                Platform.runLater(() -> {
                    gerarBtn.setDisable(false);
                    gerarResultVBox.getChildren().setAll(statusLabel("Erro ao gerar: " + e.getMessage()));
                });
            }
        }).start();
    }

    private void renderGerarItems() {
        gerarResultVBox.getChildren().clear();
        for (int i = 0; i < gerarItems.size(); i++) {
            GeneratedItem item = gerarItems.get(i);
            String slotType  = item.slotType() != null ? item.slotType().toUpperCase() : "MUSIC";
            String badgeColor = TYPE_COLOR.getOrDefault(slotType, "#888");
            String badgeLbl   = TYPE_LBL.getOrDefault(slotType, slotType.length() > 3 ? slotType.substring(0,3) : slotType);

            Label badge = new Label(badgeLbl);
            badge.setStyle("-fx-font-size:10px;-fx-font-weight:800;-fx-text-fill:" + badgeColor + ";-fx-min-width:28px;-fx-alignment:center;");

            Label meta = new Label(String.format("%02dh · Slot %d · %s%s", item.hour(), item.position(),
                item.clockName() != null ? item.clockName() : "",
                item.categoryName() != null ? " · " + item.categoryName() : ""));
            meta.setStyle("-fx-font-size:10px;-fx-text-fill:" + TEXT_DIM + ";");

            boolean isHC = "HORA_CERTA".equals(slotType);
            String trackTitle = isHC ? "Hora Certa — sinal automático"
                : (item.track().title() != null && !item.track().title().isBlank()
                    ? item.track().title() : item.track().path());
            Label trackLbl = new Label(trackTitle);
            trackLbl.setStyle("-fx-font-size:12px;-fx-font-weight:700;-fx-text-fill:" + TEXT_HI + ";");
            Label artistLbl = new Label(item.track().artist() != null ? item.track().artist() : "");
            artistLbl.setStyle("-fx-font-size:10px;-fx-text-fill:" + TEXT_DIM + ";");

            VBox info = new VBox(1, meta, trackLbl, artistLbl);
            HBox.setHgrow(info, Priority.ALWAYS);

            long durSec = item.track().durationMs() / 1000;
            String durStr = durSec > 0 ? String.format("%dm%02ds", durSec/60, durSec%60) : "";
            Label durLbl = new Label(durStr);
            durLbl.setStyle("-fx-font-size:11px;-fx-text-fill:" + TEXT_DIM + ";");

            final int fi = i;
            Button cueBtn = new Button("🎧");
            cueBtn.setStyle("-fx-background-color:transparent;-fx-border-color:transparent;-fx-cursor:hand;-fx-font-size:13px;-fx-padding:0 4 0 4;");
            if (isHC) { cueBtn.setDisable(true); cueBtn.setStyle(cueBtn.getStyle() + "-fx-opacity:0.3;"); }
            else { cueBtn.setOnAction(e -> { if (onCue != null) onCue.accept(gerarItems.get(fi).track()); }); }

            Region sep1 = new Region();
            sep1.setMinWidth(1); sep1.setPrefWidth(1); sep1.setStyle("-fx-background-color:rgba(255,255,255,0.15);");
            Region sep2 = new Region();
            sep2.setMinWidth(1); sep2.setPrefWidth(1); sep2.setStyle("-fx-background-color:rgba(255,255,255,0.15);");

            HBox row = new HBox(8, badge, sep1, info, durLbl, sep2, cueBtn);
            row.setAlignment(Pos.CENTER_LEFT);
            row.setPadding(new Insets(6, 8, 6, 8));
            row.setStyle(
                "-fx-background-color:rgba(255,255,255,0.02);-fx-border-radius:6;-fx-background-radius:6;" +
                (isHC ? "-fx-opacity:0.7;" : "")
            );
            gerarResultVBox.getChildren().add(row);
        }
    }

    private void enqueueGenerated() {
        if (gerarItems.isEmpty() || onEnqueue == null) return;
        List<TrackEntity> entities = gerarItems.stream().map(GeneratedItem::track).toList();
        onEnqueue.accept(entities);
        enqueueBtn.setVisible(false);
        gerarItems.clear();
        gerarResultVBox.getChildren().setAll(statusLabel("Configure e clique em Gerar"));
        gerarSummaryBox.setVisible(false);
        gerarWarnVBox.setVisible(false);
    }

    // ═════════════════════════════════════════════════════════════════════════
    // HELPERS
    // ═════════════════════════════════════════════════════════════════════════

    private Button smBtn(String label, Runnable action) {
        Button btn = new Button(label);
        btn.setStyle(
            "-fx-background-color:rgba(255,255,255,0.05);-fx-border-color:" + BORDER + ";-fx-border-radius:5;" +
            "-fx-background-radius:5;-fx-text-fill:" + TEXT_DIM + ";-fx-font-size:10px;-fx-padding:4 9 4 9;" +
            "-fx-cursor:hand;"
        );
        btn.setOnMouseEntered(e -> btn.setStyle(btn.getStyle().replace(TEXT_DIM, "#c8c8c8")));
        btn.setOnMouseExited(e  -> btn.setStyle(btn.getStyle().replace("#c8c8c8", TEXT_DIM)));
        if (action != null) btn.setOnAction(e -> action.run());
        return btn;
    }

    private HBox actionBar(Node... nodes) {
        HBox bar = new HBox(6);
        bar.setAlignment(Pos.CENTER_LEFT);
        bar.setPadding(new Insets(6, 10, 6, 10));
        bar.setStyle("-fx-border-color:transparent transparent " + BORDER + " transparent;-fx-border-width:0 0 1 0;");
        bar.getChildren().addAll(nodes);
        return bar;
    }

    private ScrollPane scroll(VBox content) {
        ScrollPane sp = new ScrollPane(content);
        sp.setFitToWidth(true);
        sp.setStyle("-fx-background-color:transparent;-fx-background:transparent;-fx-border-color:transparent;");
        return sp;
    }

    private Label statusLabel(String text) {
        Label l = new Label(text);
        l.setStyle("-fx-font-size:12px;-fx-text-fill:" + TEXT_DIM + ";-fx-padding:20 0 20 0;");
        return l;
    }

    private Label dimLabel(String text, double size) {
        Label l = new Label(text);
        l.setStyle("-fx-font-size:" + (int)size + "px;-fx-text-fill:" + TEXT_DIM + ";");
        return l;
    }

    private String inputStyle() {
        return "-fx-background-color:#071019;-fx-border-color:" + BORDER + ";-fx-border-radius:5;" +
               "-fx-background-radius:5;-fx-text-fill:" + TEXT_HI + ";-fx-font-size:12px;" +
               "-fx-padding:4 7 4 7;";
    }

    private String selectStyle() {
        return "-fx-background-color:#071019;-fx-border-color:" + BORDER + ";-fx-border-radius:5;" +
               "-fx-background-radius:5;-fx-text-fill:" + TEXT_HI + ";-fx-font-size:12px;" +
               "-fx-padding:3 7 3 7;";
    }

    private String gridThStyle(boolean isToday) {
        return "-fx-padding:6 4 6 4;-fx-font-size:10px;-fx-font-weight:700;" +
               "-fx-text-fill:" + (isToday ? CYAN : TEXT_DIM) + ";" +
               "-fx-background-color:" + (isToday ? "rgba(32,230,255,0.10)" : BG_CARD) + ";" +
               "-fx-border-color:" + BORDER + ";-fx-border-width:0 1 1 0;-fx-alignment:center;";
    }

    private String gridCellStyle(String color, String clockId) {
        if (color == null) {
            return "-fx-background-color:transparent;-fx-border-color:transparent transparent transparent transparent;" +
                   "-fx-border-width:0 0 0 3;-fx-text-fill:rgba(255,255,255,0.15);-fx-font-size:10px;" +
                   "-fx-padding:5 6 5 6;-fx-cursor:hand;" +
                   "-fx-alignment:center-left;";
        }
        return "-fx-background-color:" + color + "1a;-fx-border-color:" + color + " transparent transparent transparent;" +
               "-fx-border-width:0 0 0 3;-fx-text-fill:" + color + ";-fx-font-size:10px;-fx-font-weight:700;" +
               "-fx-padding:5 6 5 6;-fx-cursor:hand;" +
               "-fx-alignment:center-left;";
    }

    private String pickerOptStyle(boolean clear) {
        return "-fx-background-color:transparent;-fx-border-color:transparent;-fx-cursor:hand;" +
               "-fx-text-fill:" + (clear ? TEXT_DIM : TEXT_HI) + ";" +
               "-fx-font-size:11px;-fx-padding:7 14 7 14;-fx-alignment:center-left;";
    }

    private static String fmtHint(long ms) {
        if (ms <= 0) return "—";
        long sec = ms / 1000;
        long m = sec / 60, s = sec % 60;
        if (m == 0) return s + "s";
        if (s == 0) return m + "m";
        return m + "m" + s + "s";
    }

    private static String fmtMinutes(int min) {
        int h = min / 60, m = min % 60;
        if (h == 0) return min + " min";
        if (m == 0) return h + "h";
        return h + "h " + m + "min";
    }

    private void refreshCatCombo(ComboBox<String> combo) {
        combo.getItems().clear();
        for (CategoryEntity c : categories) combo.getItems().add(c.name());
        if (!combo.getItems().isEmpty()) combo.setValue(combo.getItems().get(0));
    }

    private String getCatId(String catName) {
        if (catName == null) return null;
        return categories.stream().filter(c -> c.name().equals(catName))
                .findFirst().map(CategoryEntity::id).orElse(null);
    }
}
