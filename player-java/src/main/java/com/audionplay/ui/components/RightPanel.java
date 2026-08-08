package com.audionplay.ui.components;

import javafx.geometry.Insets;
import javafx.scene.layout.VBox;

import static com.audionplay.ui.Theme.*;

/**
 * Painel direito: Modo de Operação, VU Meter, Volume, Agendamentos, Streaming.
 *
 * Expõe VuMeterPanel e VolumePanel para wiring de callbacks em MainWindow.
 */
public class RightPanel extends VBox {

    private final VuMeterPanel vuMeter;
    private final VolumePanel volume;

    public RightPanel() {
        super(10);
        setPrefWidth(405); setMinWidth(369); setMaxWidth(438);
        setPadding(new Insets(10));
        setStyle("-fx-background-color:" + BG_MAIN + ";");

        vuMeter = new VuMeterPanel();
        volume  = new VolumePanel();

        getChildren().addAll(
            new OpModePanel(),
            vuMeter,
            volume,
            new SchedulingPanel(),
            new StreamingPanel()
        );
    }

    public VuMeterPanel getVuMeter() { return vuMeter; }
    public VolumePanel  getVolume()  { return volume;  }
}
