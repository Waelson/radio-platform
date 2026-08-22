package com.audionplay;

import com.audionplay.db.Database;
import com.audionplay.ui.MainWindow;
import javafx.application.Application;
import javafx.geometry.Rectangle2D;
import javafx.scene.Scene;
import javafx.stage.Screen;
import javafx.stage.Stage;

import java.nio.file.Path;
import java.nio.file.Paths;
import java.sql.SQLException;

/**
 * Entry point da aplicação.
 * Responsabilidade única: criar o Stage, inicializar o banco e delegar para MainWindow.
 */
public class MainApp extends Application {

    @Override
    public void start(Stage stage) throws Exception {
        initDatabase();

        MainWindow window = new MainWindow();
        javafx.scene.layout.StackPane root = window.build(stage);

        Rectangle2D screen = Screen.getPrimary().getVisualBounds();
        double initW = Math.min(1440, screen.getWidth());
        double initH = Math.min(900,  screen.getHeight());

        Scene scene = new Scene(root, initW, initH);
        stage.setTitle("Audion Play — Broadcast Suite");
        stage.setScene(scene);
        stage.setMinWidth(1100);
        stage.setMinHeight(680);
        stage.setOnCloseRequest(e -> Database.close());
        stage.show();
    }

    private void initDatabase() throws SQLException {
        // Procura library.db ao lado do jar, depois na raiz do projeto (dev)
        Path jarDir = Paths.get(
            getClass().getProtectionDomain().getCodeSource().getLocation().getPath()
        ).getParent();

        Path dbPath = jarDir.resolve("library.db");
        if (!dbPath.toFile().exists()) {
            // Fallback para desenvolvimento via Maven
            dbPath = Paths.get(System.getProperty("user.dir"), "library.db");
        }

        Database.init(dbPath.toAbsolutePath().toString());
    }

    public static void main(String[] args) {
        launch(args);
    }
}
