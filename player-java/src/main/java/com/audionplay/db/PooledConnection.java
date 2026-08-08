package com.audionplay.db;

import java.sql.Connection;

/**
 * Wrapper de {@link Connection} compatível com try-with-resources.
 *
 * Ao sair do bloco {@code try}, {@link #close()} devolve a conexão
 * ao pool em vez de fechá-la de verdade.
 *
 * <pre>{@code
 *   try (PooledConnection pc = Database.pool().borrow()) {
 *       PreparedStatement ps = pc.get().prepareStatement("SELECT ...");
 *       ResultSet rs = ps.executeQuery();
 *       ...
 *   } // conexão devolvida automaticamente
 * }</pre>
 */
public final class PooledConnection implements AutoCloseable {

    private final Connection connection;
    private final ConnectionPool pool;

    PooledConnection(Connection connection, ConnectionPool pool) {
        this.connection = connection;
        this.pool = pool;
    }

    /** Retorna a conexão JDBC subjacente. */
    public Connection get() {
        return connection;
    }

    /** Devolve a conexão ao pool — não fecha de verdade. */
    @Override
    public void close() {
        pool.release(connection);
    }
}
