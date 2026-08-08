package com.audionplay.ui.components;

import com.audionplay.ui.Theme;
import javafx.geometry.Insets;
import javafx.geometry.Pos;
import javafx.scene.control.Label;
import javafx.scene.control.Slider;
import javafx.scene.layout.*;

import java.util.function.Consumer;

import static com.audionplay.ui.Theme.*;

/**
 * Painel de volume — visual fiel ao or-card "Volume" do player.html.
 */
public class VolumePanel extends VBox {

    private Consumer<Float> onProgramVolumeChange;
    private Consumer<Float> onPreviewVolumeChange;
    private Consumer<Float> onBotoneiraVolumeChange;

    public VolumePanel() {
        super(0);
        setStyle(OR_CARD);

        HBox hdr = Theme.orCardHeader("Volume");

        // ── Vol body ──────────────────────────────────────────────────────────
        VBox body = new VBox(2);
        body.setPadding(new Insets(9, 12, 9, 12));
        body.getChildren().addAll(
            volRow("Programa",  1.00, v -> { if (onProgramVolumeChange  != null) onProgramVolumeChange.accept(v);  }),
            volRow("Preview",   1.00, v -> { if (onPreviewVolumeChange   != null) onPreviewVolumeChange.accept(v);   }),
            volRow("Botoneira", 1.00, v -> { if (onBotoneiraVolumeChange != null) onBotoneiraVolumeChange.accept(v); })
        );

        getChildren().addAll(hdr, body);
    }

    public void setOnProgramVolumeChange(Consumer<Float> cb)   { this.onProgramVolumeChange   = cb; }
    public void setOnPreviewVolumeChange(Consumer<Float> cb)   { this.onPreviewVolumeChange   = cb; }
    public void setOnBotoneiraVolumeChange(Consumer<Float> cb) { this.onBotoneiraVolumeChange = cb; }

    // ── Builder ───────────────────────────────────────────────────────────────

    private HBox volRow(String label, double val, Consumer<Float> onChange) {
        Label lbl = new Label(label);
        lbl.setStyle("-fx-font-size:10px;-fx-font-weight:700;-fx-text-fill:#88a0b5;");
        lbl.setMinWidth(66);

        Slider slider = new Slider(0, 1, val);
        slider.setStyle(
            "-fx-control-inner-background:#061018;" +
            "-fx-accent:#36d399;" +
            "-fx-pref-height:4;"
        );
        HBox.setHgrow(slider, Priority.ALWAYS);

        Label pct = new Label((int)(val * 100) + "%");
        pct.setStyle("-fx-font-size:10px;-fx-font-weight:600;-fx-text-fill:#88a0b5;");
        pct.setMinWidth(32);
        pct.setAlignment(Pos.CENTER_RIGHT);

        slider.valueProperty().addListener((o, ov, nv) -> {
            pct.setText((int)(nv.doubleValue() * 100) + "%");
            onChange.accept(nv.floatValue());
        });

        HBox row = new HBox(8, lbl, slider, pct);
        row.setAlignment(Pos.CENTER);
        row.setMinHeight(32);
        return row;
    }
}
