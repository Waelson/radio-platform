package com.audionplay.ui.components;

import com.audionplay.horacerta.HoraCertaConfig;
import javafx.geometry.Insets;
import javafx.geometry.Pos;
import javafx.scene.Scene;
import javafx.scene.control.*;
import javafx.scene.layout.*;
import javafx.scene.paint.Color;
import javafx.stage.*;

import java.io.File;
import java.util.ArrayList;
import java.util.List;

/**
 * Dialog de configuração da Hora Certa.
 *
 * Campos compatíveis com HoraCertaConfig do playout Go:
 *   hours_dir, minutes_dir, hour_pattern, minute_pattern, gain_db
 *
 * Campos adicionais (player-java):
 *   fires_at_minutes — lista de minutos da hora em que o scheduler dispara
 */
public class HoraCertaConfigDialog {

    private final Window  owner;
    private Runnable      onSaved;

    public HoraCertaConfigDialog(Window owner) { this.owner = owner; }

    public void setOnSaved(Runnable onSaved) { this.onSaved = onSaved; }

    public void show() {
        HoraCertaConfig current = HoraCertaConfig.load();

        Stage dialog = new Stage();
        dialog.initOwner(owner);
        dialog.initModality(Modality.APPLICATION_MODAL);
        dialog.setTitle("Configurar Hora Certa");
        dialog.setResizable(false);

        // ── Campos ────────────────────────────────────────────────────────────
        CheckBox enabledCb = new CheckBox("Habilitado");
        enabledCb.setSelected(current.enabled());
        enabledCb.setStyle("-fx-text-fill:#d8e6ef;-fx-font-size:13px;");

        TextField hoursDirFld = styledField(current.hoursDir(),    "Ex.: /RadioFlow/horacerta/horas");
        TextField minDirFld   = styledField(current.minutesDir(),  "Deixar vazio = mesma pasta de horas");
        TextField hourPatFld  = styledField(current.hourPattern(),  "HRS{HH}.mp3");
        TextField minPatFld   = styledField(current.minutePattern(),"MIN{MM}.mp3");
        TextField gainFld     = styledField(String.valueOf(current.gainDb()), "0.0");
        TextField firesFld    = styledField(minutesToString(current.firesAtMinutes()),
                                             "Ex.: 0  ou  0,30");

        Button browseHours = browseBtn(hoursDirFld, dialog);
        Button browseMin   = browseBtn(minDirFld,   dialog);

        // ── Grid ──────────────────────────────────────────────────────────────
        GridPane grid = new GridPane();
        grid.setHgap(10); grid.setVgap(11);
        grid.setPadding(new Insets(20));
        grid.setStyle("-fx-background-color:#0a1520;");

        ColumnConstraints c0 = new ColumnConstraints();
        c0.setMinWidth(140); c0.setPrefWidth(145);
        ColumnConstraints c1 = new ColumnConstraints();
        c1.setHgrow(Priority.ALWAYS); c1.setMinWidth(200);
        ColumnConstraints c2 = new ColumnConstraints();
        c2.setPrefWidth(60);
        grid.getColumnConstraints().addAll(c0, c1, c2);

        int row = 0;
        grid.add(enabledCb, 0, row++, 3, 1);
        addRow(grid, row++, "Pasta Horas:",       hoursDirFld, browseHours);
        addRow(grid, row++, "Pasta Minutos:",      minDirFld,   browseMin);
        addRow(grid, row++, "Padrão Hora:",        hourPatFld,  null);
        addRow(grid, row++, "Padrão Minuto:",      minPatFld,   null);
        addRow(grid, row++, "Gain (dB):",          gainFld,     null);
        addRow(grid, row++, "Disparar no(s) min:", firesFld,    null);

        Label hint = new Label(
            "Separe múltiplos minutos por vírgula. Ex.: 0,30 = topo da hora e meia hora.");
        hint.setStyle("-fx-font-size:10px;-fx-text-fill:#4a6478;");
        hint.setWrapText(true);
        grid.add(hint, 0, row++, 3, 1);

        Label hint2 = new Label(
            "Padrões: {HH} = hora (00–23), {MM} = minuto (00–59).\n" +
            "MIN00 é opcional — se ausente às XX:00, toca apenas o arquivo de hora.");
        hint2.setStyle("-fx-font-size:10px;-fx-text-fill:#4a6478;");
        hint2.setWrapText(true);
        grid.add(hint2, 0, row, 3, 1);

        // ── Botões ────────────────────────────────────────────────────────────
        Button ok     = actionBtn("Salvar",    "#00bfe9", "#00141b");
        Button cancel = actionBtn("Cancelar",  "#1c2e3e", "#d7e3ec");

        ok.setOnAction(e -> {
            HoraCertaConfig saved = new HoraCertaConfig(
                enabledCb.isSelected(),
                hoursDirFld.getText().trim(),
                minDirFld.getText().trim(),
                hourPatFld.getText().trim().isEmpty() ? "HRS{HH}.mp3" : hourPatFld.getText().trim(),
                minPatFld.getText().trim().isEmpty()  ? "MIN{MM}.mp3" : minPatFld.getText().trim(),
                parseDouble(gainFld.getText()),
                parseMinutes(firesFld.getText())
            );
            saved.save();
            if (onSaved != null) onSaved.run();
            dialog.close();
        });
        cancel.setOnAction(e -> dialog.close());

        HBox buttons = new HBox(8, ok, cancel);
        buttons.setAlignment(Pos.CENTER_RIGHT);
        buttons.setPadding(new Insets(4, 20, 16, 20));
        buttons.setStyle("-fx-background-color:#0a1520;");

        VBox root = new VBox(0, grid, buttons);
        root.setStyle("-fx-background-color:#0a1520;");

        Scene scene = new Scene(root, 530, 390);
        scene.setFill(Color.web("#0a1520"));
        dialog.setScene(scene);
        dialog.showAndWait();
    }

    // ── Helpers ───────────────────────────────────────────────────────────────

    private static void addRow(GridPane g, int row, String label, TextField fld, Button btn) {
        Label lbl = new Label(label);
        lbl.setStyle("-fx-font-size:12px;-fx-text-fill:#88a0b5;");
        g.add(lbl, 0, row);
        if (btn != null) {
            g.add(fld, 1, row);
            g.add(btn, 2, row);
        } else {
            GridPane.setColumnSpan(fld, 2);
            g.add(fld, 1, row);
        }
    }

    private static TextField styledField(String value, String prompt) {
        TextField f = new TextField(value != null ? value : "");
        f.setPromptText(prompt);
        f.setStyle(
            "-fx-background-color:#071019;-fx-border-color:#20384c;-fx-border-width:1;" +
            "-fx-border-radius:6;-fx-background-radius:6;" +
            "-fx-text-fill:#eef7ff;-fx-prompt-text-fill:#4a6478;-fx-font-size:12px;");
        f.setPrefHeight(30);
        return f;
    }

    private static Button browseBtn(TextField target, Stage owner) {
        Button btn = new Button("...");
        btn.setStyle(
            "-fx-background-color:#071019;-fx-border-color:#20384c;-fx-border-width:1;" +
            "-fx-border-radius:6;-fx-background-radius:6;-fx-text-fill:#88a0b5;" +
            "-fx-font-size:12px;-fx-cursor:hand;");
        btn.setPrefHeight(30); btn.setPrefWidth(50);
        btn.setOnAction(e -> {
            DirectoryChooser chooser = new DirectoryChooser();
            chooser.setTitle("Selecionar pasta");
            String cur = target.getText().trim();
            if (!cur.isEmpty()) {
                File f = new File(cur);
                if (f.isDirectory()) chooser.setInitialDirectory(f);
            }
            File dir = chooser.showDialog(owner);
            if (dir != null) target.setText(dir.getAbsolutePath());
        });
        return btn;
    }

    private static Button actionBtn(String label, String bg, String fg) {
        Button btn = new Button(label);
        btn.setStyle(
            "-fx-background-color:" + bg + ";-fx-text-fill:" + fg + ";" +
            "-fx-border-radius:8;-fx-background-radius:8;-fx-font-size:13px;" +
            "-fx-font-weight:bold;-fx-padding:6 18 6 18;-fx-cursor:hand;");
        return btn;
    }

    private static String minutesToString(List<Integer> mins) {
        if (mins == null || mins.isEmpty()) return "0";
        StringBuilder sb = new StringBuilder();
        for (int i = 0; i < mins.size(); i++) {
            if (i > 0) sb.append(",");
            sb.append(mins.get(i));
        }
        return sb.toString();
    }

    private static List<Integer> parseMinutes(String text) {
        List<Integer> list = new ArrayList<>();
        if (text == null || text.isBlank()) { list.add(0); return list; }
        for (String s : text.split(",")) {
            try {
                int v = Integer.parseInt(s.trim());
                if (v >= 0 && v < 60) list.add(v);
            } catch (NumberFormatException ignored) {}
        }
        if (list.isEmpty()) list.add(0);
        return list;
    }

    private static double parseDouble(String s) {
        try { return Double.parseDouble(s.trim().replace(',', '.')); }
        catch (Exception e) { return 0.0; }
    }
}
