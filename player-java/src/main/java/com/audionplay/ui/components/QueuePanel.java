package com.audionplay.ui.components;

import com.audionplay.db.entity.TrackEntity;
import com.audionplay.db.entity.TrackEntity.TrackType;
import java.util.ArrayList;
import java.util.List;
import java.util.Optional;
import java.util.function.Consumer;
import com.audionplay.ui.Theme;
import javafx.geometry.Insets;
import javafx.geometry.Pos;
import javafx.scene.control.Label;
import javafx.scene.control.ScrollPane;
import javafx.scene.control.TextField;
import javafx.scene.layout.HBox;
import javafx.scene.layout.Priority;
import javafx.scene.layout.VBox;

import static com.audionplay.ui.Theme.*;

/**
 * Painel lateral esquerdo com a fila de reprodução.
 */
public class QueuePanel extends VBox {

    private final VBox  list;
    private final Label countLabel;
    private final Label durationLabel;

    private int   totalItems = 0;
    private long  totalMs    = 0;

    private final List<TrackEntity>   entities = new ArrayList<>();
    private Consumer<TrackEntity> onPlay;
    private Runnable              onQueueChanged;

    public QueuePanel() {
        super(0);
        setPrefWidth(422); setMinWidth(369); setMaxWidth(465);
        setStyle("-fx-background-color:" + BG_PANEL + ";-fx-border-color:" + BORDER + ";-fx-border-width:0 1 0 0;");

        list          = new VBox(6);
        countLabel    = Theme.lbl("0 ITENS",  "-fx-font-size:11px;-fx-font-weight:bold;-fx-text-fill:" + TEXT_PRI + ";");
        durationLabel = Theme.lbl("⏱ 00:00", "-fx-font-size:11px;-fx-text-fill:" + TEXT_SEC + ";");

        ScrollPane scroll = buildScrollPane();
        VBox.setVgrow(scroll, Priority.ALWAYS);

        getChildren().addAll(
            buildHeader(),
            buildSearch(),
            buildStats(),
            scroll,
            buildFooter()
        );
    }

    public void setOnPlay(Consumer<TrackEntity> handler)        { this.onPlay = handler; }
    public void setOnQueueChanged(Runnable handler)             { this.onQueueChanged = handler; }

    public int size()  { return entities.size(); }

    /** Retorna a primeira faixa da fila sem removê-la. */
    public Optional<TrackEntity> peekFirst() {
        return entities.isEmpty() ? Optional.empty() : Optional.of(entities.get(0));
    }

    /** Retorna a segunda faixa da fila sem removê-la. */
    public Optional<TrackEntity> peekSecond() {
        return entities.size() >= 2 ? Optional.of(entities.get(1)) : Optional.empty();
    }

    /** Remove e retorna a primeira faixa da fila. */
    public Optional<TrackEntity> pollFirst() {
        if (entities.isEmpty()) return Optional.empty();
        entities.remove(0);
        if (!list.getChildren().isEmpty()) list.getChildren().remove(0);
        totalItems = Math.max(0, totalItems - 1);
        totalMs = entities.stream().mapToLong(TrackEntity::durationMs).sum();
        updateStats();
        fireQueueChanged();
        return entities.isEmpty() ? Optional.empty() : Optional.of(entities.get(0));
    }

    /** Adiciona uma faixa do catálogo ao final da fila. */
    public void addTrack(TrackEntity entity) {
        String dur = Theme.formatTime(entity.durationMs() / 1000.0);
        QueueCard card;
        if (entity.type() == TrackType.MUSIC) {
            card = QueueCard.music(entity.title(), entity.artist(), "--:--", dur, false);
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

    private void fireQueueChanged() {
        if (onQueueChanged != null) onQueueChanged.run();
    }

    private void updateStats() {
        countLabel.setText(totalItems + (totalItems == 1 ? " ITEM" : " ITENS"));
        long s = totalMs / 1000;
        long h = s / 3600; long m = (s % 3600) / 60; long sec = s % 60;
        String fmt = h > 0
            ? String.format("⏱ %d:%02d:%02d", h, m, sec)
            : String.format("⏱ %02d:%02d", m, sec);
        durationLabel.setText(fmt);
    }

    private HBox buildHeader() {
        HBox hdr = new HBox(Theme.lbl(
            "FILA DE REPRODUÇÃO",
            "-fx-font-size:10px;-fx-font-weight:bold;-fx-text-fill:" + TEXT_SEC + ";"
        ));
        hdr.setPadding(new Insets(16, 16, 10, 16));
        return hdr;
    }

    private HBox buildSearch() {
        TextField tf = new TextField();
        tf.setPromptText("Buscar na fila...");
        tf.setStyle(
            "-fx-background-color:#12151E;-fx-border-color:" + BORDER + ";" +
            "-fx-border-radius:6;-fx-background-radius:6;" +
            "-fx-text-fill:" + TEXT_PRI + ";-fx-prompt-text-fill:" + TEXT_MUT + ";" +
            "-fx-padding:7 10 7 10;"
        );
        HBox.setHgrow(tf, Priority.ALWAYS);

        HBox search = new HBox(8, tf, Theme.lbl("↻", "-fx-font-size:15px;-fx-text-fill:" + TEXT_SEC + ";"));
        search.setPadding(new Insets(0, 12, 10, 12));
        search.setAlignment(Pos.CENTER_LEFT);
        return search;
    }

    private HBox buildStats() {
        HBox stats = new HBox(8);
        stats.setPadding(new Insets(0, 12, 10, 12));
        stats.setAlignment(Pos.CENTER_LEFT);
        stats.getChildren().addAll(
            countLabel,
            durationLabel,
            Theme.hSpacer(),
            Theme.lbl("⊘", "-fx-font-size:13px;-fx-text-fill:" + TEXT_SEC + ";"),
            Theme.lbl("⏲", "-fx-font-size:13px;-fx-text-fill:" + TEXT_SEC + ";")
        );
        return stats;
    }

    private ScrollPane buildScrollPane() {
        list.setPadding(new Insets(0, 8, 8, 8));

        ScrollPane scroll = new ScrollPane(list);
        scroll.setFitToWidth(true);
        scroll.setHbarPolicy(ScrollPane.ScrollBarPolicy.NEVER);
        scroll.setStyle("-fx-background-color:transparent;-fx-background:transparent;-fx-border-color:transparent;");
        return scroll;
    }

    private HBox buildFooter() {
        HBox footer = new HBox(10,
            Theme.lbl("⏸", "-fx-text-fill:" + TEXT_SEC + ";-fx-font-size:14px;"),
            new VBox(2,
                Theme.lbl("Próximo break",         "-fx-font-size:12px;-fx-font-weight:bold;-fx-text-fill:" + TEXT_SEC + ";"),
                Theme.lbl("Nenhum break agendado", "-fx-font-size:10px;-fx-text-fill:" + TEXT_MUT + ";")
            )
        );
        footer.setPadding(new Insets(30, 16, 30, 16));
        footer.setAlignment(Pos.CENTER_LEFT);
        footer.setStyle("-fx-background-color:" + BG_PANEL + ";-fx-border-color:" + BORDER + ";-fx-border-width:1 0 0 0;");
        return footer;
    }
}
