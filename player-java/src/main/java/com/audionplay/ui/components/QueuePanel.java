package com.audionplay.ui.components;

import com.audionplay.db.entity.TrackEntity;
import com.audionplay.db.entity.TrackEntity.TrackType;
import com.audionplay.ui.Theme;
import javafx.geometry.Insets;
import javafx.geometry.Pos;
import javafx.scene.control.Label;
import javafx.scene.control.ScrollPane;
import javafx.scene.control.TextField;
import javafx.scene.layout.*;

import java.util.ArrayList;
import java.util.List;
import java.util.Optional;
import java.util.function.Consumer;

import static com.audionplay.ui.Theme.*;

/**
 * Painel lateral da fila de reprodução — visual fiel ao .queue-section do player.html.
 *
 * Layout:
 *   [queue-section card: header | toolbar | info-bar | scroll (cresce)]
 *   [next-break-card]
 */
public class QueuePanel extends VBox {

    private final VBox  list;
    private final Label countLabel;
    private final Label durationLabel;

    private int  totalItems = 0;
    private long totalMs    = 0;

    private final List<TrackEntity>   entities       = new ArrayList<>();
    private Consumer<TrackEntity> onPlay;
    private Runnable              onQueueChanged;

    public QueuePanel() {
        super(10); // gap entre queue card e break card
        setPrefWidth(422); setMinWidth(369); setMaxWidth(465);
        setPadding(new Insets(10));
        setStyle("-fx-background-color:" + BG_MAIN + ";");

        list          = new VBox(6);
        countLabel    = new Label("0 ITENS");
        countLabel.setStyle("-fx-font-size:11px;-fx-font-weight:bold;-fx-text-fill:#88a0b5;");
        durationLabel = new Label("");
        durationLabel.setStyle("-fx-font-size:11px;-fx-font-weight:bold;-fx-text-fill:#88a0b5;-fx-border-color:rgba(255,255,255,0.10);-fx-border-width:0 0 0 1;-fx-padding:0 0 0 8;");

        // ── Queue section card ─────────────────────────────────────────────────
        VBox queueCard = new VBox(0);
        queueCard.setStyle(
            "-fx-background-color:#0e1b28;" +
            "-fx-border-color:#20384c;-fx-border-width:1;" +
            "-fx-border-radius:14;-fx-background-radius:14;"
        );
        VBox.setVgrow(queueCard, Priority.ALWAYS);

        ScrollPane scroll = buildScrollPane();
        VBox.setVgrow(scroll, Priority.ALWAYS);

        queueCard.getChildren().addAll(
            buildHeader(),
            buildToolbar(),
            buildInfoBar(),
            scroll
        );

        getChildren().addAll(queueCard, buildBreakCard());
    }

    // ── API pública ───────────────────────────────────────────────────────────

    public void setOnPlay(Consumer<TrackEntity> h) { this.onPlay = h; }
    public void setOnQueueChanged(Runnable h)      { this.onQueueChanged = h; }

    public int size() { return entities.size(); }

    public Optional<TrackEntity> peekFirst() {
        return entities.isEmpty() ? Optional.empty() : Optional.of(entities.get(0));
    }

    public Optional<TrackEntity> peekSecond() {
        return entities.size() >= 2 ? Optional.of(entities.get(1)) : Optional.empty();
    }

    public Optional<TrackEntity> pollFirst() {
        if (entities.isEmpty()) return Optional.empty();
        entities.remove(0);
        if (!list.getChildren().isEmpty()) list.getChildren().remove(0);
        totalItems = Math.max(0, totalItems - 1);
        totalMs    = entities.stream().mapToLong(TrackEntity::durationMs).sum();
        updateStats();
        fireQueueChanged();
        return entities.isEmpty() ? Optional.empty() : Optional.of(entities.get(0));
    }

    public void addTrack(TrackEntity entity) {
        String dur = Theme.formatTime(entity.durationMs() / 1000.0);
        QueueCard card;
        if (entity.type() == TrackType.MUSIC) {
            card = QueueCard.music(entity.title(), entity.artist(), "--:--", dur, false);
        } else if (entity.type() == TrackType.VINHETA || entity.type() == TrackType.JINGLE) {
            card = QueueCard.vinheta(entity.title(), "--:--", dur);
        } else {
            card = QueueCard.vinheta(entity.title(), "--:--", dur);
        }
        card.setOnPlay(() -> { if (onPlay != null) onPlay.accept(entity); });
        list.getChildren().add(card);
        entities.add(entity);

        totalItems++;
        totalMs += entity.durationMs();
        updateStats();
        fireQueueChanged();
    }

    // ══════════════════════════════════════════════════════════════════════════
    //  LAYOUT
    // ══════════════════════════════════════════════════════════════════════════

    /** queue-header */
    private HBox buildHeader() {
        Label title = new Label("FILA DE REPRODUÇÃO");
        title.setStyle(
            "-fx-font-size:11px;-fx-font-weight:bold;-fx-text-fill:#bdd0df;"
        );
        HBox hdr = new HBox(title);
        hdr.setAlignment(Pos.CENTER_LEFT);
        hdr.setMinHeight(42);
        hdr.setPadding(new Insets(0, 12, 0, 12));
        hdr.setStyle(
            "-fx-background-color:#0a1621;" +
            "-fx-border-color:#20384c;-fx-border-width:0 0 1 0;" +
            "-fx-background-radius:14 14 0 0;"
        );
        return hdr;
    }

    /** queue-toolbar: busca + botão ↻ */
    private HBox buildToolbar() {
        TextField search = new TextField();
        search.setPromptText("Buscar na fila...");
        search.setPrefHeight(32);
        search.setStyle(
            "-fx-background-color:#071019;" +
            "-fx-border-color:#20384c;-fx-border-width:1;" +
            "-fx-border-radius:8;-fx-background-radius:8;" +
            "-fx-text-fill:#eef7ff;-fx-prompt-text-fill:#5f788e;" +
            "-fx-padding:0 10 0 10;-fx-font-size:12px;"
        );
        HBox.setHgrow(search, Priority.ALWAYS);

        Label refreshBtn = new Label("↻");
        refreshBtn.setPrefWidth(40); refreshBtn.setPrefHeight(32);
        refreshBtn.setAlignment(Pos.CENTER);
        refreshBtn.setStyle(
            "-fx-font-size:16px;-fx-text-fill:#9cb5ba;" +
            "-fx-background-color:transparent;" +
            "-fx-border-color:rgba(220,239,242,0.12);-fx-border-width:1;" +
            "-fx-border-radius:6;-fx-background-radius:6;" +
            "-fx-cursor:hand;"
        );

        HBox bar = new HBox(7, search, refreshBtn);
        bar.setAlignment(Pos.CENTER_LEFT);
        bar.setPadding(new Insets(8));
        bar.setStyle("-fx-border-color:#20384c;-fx-border-width:0 0 1 0;");
        return bar;
    }

    /** queue-info-bar: count | duration | ∅ | ⏰ */
    private HBox buildInfoBar() {
        Label clearBtn = sqBtn("∅");
        Label horaBtn  = sqBtn("⏰");

        Region spacer = new Region();
        HBox.setHgrow(spacer, Priority.ALWAYS);

        HBox bar = new HBox(10, countLabel, durationLabel, spacer, clearBtn, horaBtn);
        bar.setAlignment(Pos.CENTER_LEFT);
        bar.setPadding(new Insets(5, 10, 5, 10));
        bar.setMinHeight(34);
        bar.setStyle("-fx-border-color:#20384c;-fx-border-width:0 0 1 0;");
        return bar;
    }

    private ScrollPane buildScrollPane() {
        list.setPadding(new Insets(5, 5, 5, 5));
        list.setStyle("-fx-background-color:#0e1b28;");

        ScrollPane scroll = new ScrollPane(list);
        scroll.setFitToWidth(true);
        scroll.setHbarPolicy(ScrollPane.ScrollBarPolicy.NEVER);
        scroll.setStyle(
            "-fx-background-color:#0e1b28;-fx-background:#0e1b28;" +
            "-fx-border-color:transparent;"
        );
        return scroll;
    }

    /** next-break-card — card separado abaixo do queue-section */
    private VBox buildBreakCard() {
        // ícone amber 42×42
        Label icon = new Label("◫");
        icon.setStyle("-fx-font-size:19px;-fx-text-fill:#ffbf4b;");
        icon.setAlignment(Pos.CENTER);
        StackPane iconBox = new StackPane(icon);
        iconBox.setPrefWidth(42); iconBox.setPrefHeight(42);
        iconBox.setMinWidth(42);  iconBox.setMaxWidth(42);
        iconBox.setStyle(
            "-fx-background-color:rgba(255,191,75,0.12);" +
            "-fx-background-radius:10;"
        );

        // info
        Label mainLbl = new Label("Próximo break");
        mainLbl.setStyle("-fx-font-size:12px;-fx-font-weight:bold;-fx-text-fill:#eef7ff;");
        Label subLbl  = new Label("Nenhum break agendado");
        subLbl.setStyle("-fx-font-size:10px;-fx-text-fill:#88a0b5;");
        VBox info = new VBox(4, mainLbl, subLbl);
        info.setAlignment(Pos.CENTER_LEFT);
        HBox.setHgrow(info, Priority.ALWAYS);

        // countdown
        Label countdown = new Label("—");
        countdown.setStyle("-fx-font-size:22px;-fx-font-weight:bold;-fx-text-fill:#ffbf4b;");

        // inner amber card
        HBox inner = new HBox(10, iconBox, info, countdown);
        inner.setAlignment(Pos.CENTER_LEFT);
        inner.setPadding(new Insets(10));
        inner.setStyle(
            "-fx-background-color:rgba(255,191,75,0.06);" +
            "-fx-border-color:rgba(255,191,75,0.22);-fx-border-width:1;" +
            "-fx-border-radius:11;-fx-background-radius:11;"
        );
        inner.setOpacity(0.4); // .empty state

        // outer card
        VBox outer = new VBox(inner);
        outer.setPadding(new Insets(10));
        outer.setStyle(
            "-fx-background-color:#0e1b28;" +
            "-fx-border-color:#20384c;-fx-border-width:1;" +
            "-fx-border-radius:14;-fx-background-radius:14;"
        );
        return outer;
    }

    // ══════════════════════════════════════════════════════════════════════════
    //  HELPERS
    // ══════════════════════════════════════════════════════════════════════════

    private static Label sqBtn(String text) {
        Label lbl = new Label(text);
        lbl.setPrefWidth(32); lbl.setPrefHeight(32);
        lbl.setAlignment(Pos.CENTER);
        lbl.setStyle(
            "-fx-font-size:16px;-fx-text-fill:#9cb5ba;" +
            "-fx-background-color:transparent;" +
            "-fx-border-color:rgba(220,239,242,0.12);-fx-border-width:1;" +
            "-fx-border-radius:6;-fx-background-radius:6;" +
            "-fx-cursor:hand;"
        );
        return lbl;
    }

    private void updateStats() {
        countLabel.setText(totalItems + (totalItems == 1 ? " ITEM" : " ITENS"));
        if (totalMs <= 0) {
            durationLabel.setText("");
            return;
        }
        long s   = totalMs / 1000;
        long h   = s / 3600; long m = (s % 3600) / 60; long sec = s % 60;
        String fmt = h > 0
            ? String.format("⏱ %d:%02d:%02d", h, m, sec)
            : String.format("⏱ %02d:%02d", m, sec);
        durationLabel.setText(fmt);
    }

    private void fireQueueChanged() {
        if (onQueueChanged != null) onQueueChanged.run();
    }
}
