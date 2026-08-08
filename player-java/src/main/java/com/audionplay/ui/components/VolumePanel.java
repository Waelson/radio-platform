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
 * Painel de sliders de volume (PROGRAMA, PREVIEW, BOTONEIRA).
 */
public class VolumePanel extends VBox {

    private Consumer<Float> onProgramVolumeChange;

    public VolumePanel() {
        super(10);
        setStyle(
            "-fx-background-color:#14172280;" +
            "-fx-background-radius:8;-fx-padding:12;" +
            "-fx-border-color:" + BORDER + ";" +
            "-fx-border-radius:8;-fx-border-width:1;"
        );
        getChildren().addAll(
            Theme.lbl("VOLUME", "-fx-font-size:10px;-fx-font-weight:bold;-fx-text-fill:" + TEXT_SEC + ";"),
            volRow("PROGRAMA",  0.31, true),
            volRow("PREVIEW",   0.22, false),
            volRow("BOTONEIRA", 1.00, false)
        );
    }

    /** Registra callback chamado quando o slider PROGRAMA muda. */
    public void setOnProgramVolumeChange(Consumer<Float> cb) {
        this.onProgramVolumeChange = cb;
    }

    private HBox volRow(String label, double val, boolean isProgram) {
        Label l = Theme.lbl(label, "-fx-font-size:10px;-fx-text-fill:" + TEXT_SEC + ";");
        l.setMinWidth(72);

        Slider s = new Slider(0, 1, val);
        s.setStyle("-fx-control-inner-background:#2A2E42;");
        HBox.setHgrow(s, Priority.ALWAYS);

        Label v = Theme.lbl((int)(val * 100) + "%", "-fx-font-size:10px;-fx-text-fill:" + TEXT_PRI + ";");
        v.setMinWidth(30);
        v.setAlignment(Pos.CENTER_RIGHT);

        if (isProgram) {
            s.valueProperty().addListener((o, ov, nv) -> {
                if (onProgramVolumeChange != null) onProgramVolumeChange.accept(nv.floatValue());
            });
        }

        HBox row = new HBox(8, l, s, v);
        row.setAlignment(Pos.CENTER);
        return row;
    }
}
