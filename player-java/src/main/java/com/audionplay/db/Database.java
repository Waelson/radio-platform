package com.audionplay.db;

import java.sql.SQLException;

/**
 * Ponto único de acesso ao pool de conexões.
 *
 * Deve ser inicializado uma vez na startup da aplicação:
 * <pre>{@code
 *   Database.init("/caminho/para/library.db");
 * }</pre>
 *
 * E encerrado ao fechar o app:
 * <pre>{@code
 *   Database.close();
 * }</pre>
 */
public final class Database {

    private static final int POOL_SIZE = 5;

    private static ConnectionPool instance;

    private Database() {}

    /**
     * Inicializa o pool apontando para o arquivo SQLite informado.
     * Idempotente: ignora chamadas subsequentes se já inicializado.
     *
     * @throws SQLException se não conseguir abrir as conexões iniciais.
     */
    public static synchronized void init(String dbPath) throws SQLException {
        if (instance != null) return;
        instance = new ConnectionPool(dbPath, POOL_SIZE);
        System.out.println("[DB] Pool inicializado: " + dbPath + " (" + POOL_SIZE + " conexões)");
    }

    /**
     * Retorna o pool de conexões.
     *
     * @throws IllegalStateException se {@link #init} não tiver sido chamado.
     */
    public static ConnectionPool pool() {
        if (instance == null) throw new IllegalStateException("Database.init() não foi chamado.");
        return instance;
    }

    /** Fecha todas as conexões do pool. */
    public static synchronized void close() {
        if (instance != null) {
            instance.close();
            instance = null;
            System.out.println("[DB] Pool encerrado.");
        }
    }
}
