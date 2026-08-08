package com.audionplay.db;

import java.sql.Connection;
import java.sql.DriverManager;
import java.sql.SQLException;
import java.util.concurrent.LinkedBlockingDeque;
import java.util.concurrent.TimeUnit;

/**
 * Pool fixo de conexões JDBC para SQLite.
 *
 * Funcionamento:
 *  - Na inicialização, abre {@code size} conexões reais e as mantém em uma fila.
 *  - {@link #acquire()} bloqueia até {@code ACQUIRE_TIMEOUT_MS} ms aguardando
 *    uma conexão ficar disponível; lança {@link SQLException} em caso de timeout.
 *  - {@link #release(Connection)} devolve a conexão ao pool (não a fecha).
 *  - Use {@link PooledConnection} via try-with-resources para garantir devolução
 *    automática ao pool sem precisar chamar release() manualmente.
 *  - {@link #close()} encerra todas as conexões — chamar ao finalizar o app.
 *
 * SQLite suporta múltiplas conexões leitoras simultâneas (WAL mode);
 * escrita serializa automaticamente. Um pool de 5 conexões é suficiente
 * para o padrão de uso do player (leituras de catálogo + UI thread).
 */
public class ConnectionPool implements AutoCloseable {

    private static final long ACQUIRE_TIMEOUT_MS = 5_000;

    private final String jdbcUrl;
    private final LinkedBlockingDeque<Connection> pool;
    private volatile boolean closed = false;

    public ConnectionPool(String dbPath, int size) throws SQLException {
        this.jdbcUrl = "jdbc:sqlite:" + dbPath;
        this.pool = new LinkedBlockingDeque<>(size);

        try {
            Class.forName("org.sqlite.JDBC");
        } catch (ClassNotFoundException e) {
            throw new SQLException("Driver sqlite-jdbc não encontrado no classpath", e);
        }

        for (int i = 0; i < size; i++) {
            pool.push(openConnection());
        }
    }

    // ── API pública ───────────────────────────────────────────────────────────

    /**
     * Retorna uma conexão do pool.
     * Bloqueia até {@code ACQUIRE_TIMEOUT_MS} ms.
     *
     * @throws SQLException se o pool estiver fechado ou o timeout expirar.
     */
    public Connection acquire() throws SQLException {
        if (closed) throw new SQLException("Pool já foi fechado.");
        try {
            Connection conn = pool.pollFirst(ACQUIRE_TIMEOUT_MS, TimeUnit.MILLISECONDS);
            if (conn == null) throw new SQLException("Timeout ao aguardar conexão do pool.");
            if (conn.isClosed()) conn = openConnection(); // reconecta se necessário
            return conn;
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            throw new SQLException("Thread interrompida ao aguardar conexão.", e);
        }
    }

    /**
     * Devolve a conexão ao pool.
     * Se o pool já estiver fechado, fecha a conexão diretamente.
     */
    public void release(Connection conn) {
        if (conn == null) return;
        if (closed) {
            closeQuietly(conn);
            return;
        }
        pool.offerLast(conn);
    }

    /**
     * Obtém uma {@link PooledConnection} via try-with-resources.
     * A conexão é devolvida ao pool automaticamente ao sair do bloco.
     *
     * <pre>{@code
     *   try (PooledConnection pc = pool.borrow()) {
     *       PreparedStatement ps = pc.get().prepareStatement("SELECT ...");
     *       ...
     *   }
     * }</pre>
     */
    public PooledConnection borrow() throws SQLException {
        return new PooledConnection(acquire(), this);
    }

    /** Encerra todas as conexões do pool. */
    @Override
    public void close() {
        closed = true;
        Connection conn;
        while ((conn = pool.pollFirst()) != null) {
            closeQuietly(conn);
        }
    }

    public int available() { return pool.size(); }

    // ── Interno ───────────────────────────────────────────────────────────────

    private Connection openConnection() throws SQLException {
        Connection conn = DriverManager.getConnection(jdbcUrl);
        conn.setAutoCommit(true);
        // WAL mode: leitores não bloqueiam escritores e vice-versa
        try (var st = conn.createStatement()) {
            st.execute("PRAGMA journal_mode=WAL");
            st.execute("PRAGMA foreign_keys=ON");
            st.execute("PRAGMA busy_timeout=3000");
        }
        return conn;
    }

    private static void closeQuietly(Connection conn) {
        try { conn.close(); } catch (SQLException ignored) {}
    }
}
