package com.audionplay.horacerta;

import javafx.application.Platform;

import java.time.LocalTime;
import java.util.List;
import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.ScheduledFuture;
import java.util.concurrent.TimeUnit;
import java.util.function.BiConsumer;
import java.util.function.Consumer;

/**
 * Agenda disparos de hora-certa usando um ScheduledExecutorService.
 *
 * Alinha o primeiro disparo ao início do próximo minuto e repete a cada minuto exato.
 * Verifica se o minuto atual está na lista {@link HoraCertaConfig#firesAtMinutes()} e,
 * se sim, resolve os caminhos de arquivo e notifica o callback na thread JavaFX.
 *
 * Compatível com o modo AFTER_CURRENT do scheduler Go: a inserção na fila
 * e o momento de início do disparo são responsabilidade do chamador.
 */
public class HoraCertaScheduler {

    private final ScheduledExecutorService executor =
        Executors.newSingleThreadScheduledExecutor(r -> {
            Thread t = new Thread(r, "hora-certa-scheduler");
            t.setDaemon(true);
            return t;
        });

    private volatile HoraCertaConfig  config;
    private volatile HoraCertaResolver resolver;
    private ScheduledFuture<?>         task;

    /**
     * Callback invocado na thread JavaFX ao disparar:
     *   (paths resolvidos, horário do disparo)
     */
    private BiConsumer<List<String>, LocalTime> onFire;

    /** Callback de erro invocado na thread JavaFX. */
    private Consumer<String> onError;

    // ── API pública ───────────────────────────────────────────────────────────

    public void setOnFire(BiConsumer<List<String>, LocalTime> cb) { this.onFire  = cb; }
    public void setOnError(Consumer<String> cb)                   { this.onError = cb; }

    /**
     * Aplica a nova configuração e (re)inicia o scheduler se estiver habilitado.
     * Pode ser chamado a qualquer momento — o tick anterior é cancelado.
     */
    public void applyConfig(HoraCertaConfig cfg) {
        this.config   = cfg;
        this.resolver = new HoraCertaResolver(cfg);
        if (cfg.enabled()) {
            start();
        } else {
            stop();
        }
    }

    /**
     * Inicia o scheduler, alinhando o primeiro tick ao início do próximo minuto.
     */
    public void start() {
        if (task != null && !task.isDone()) task.cancel(false);

        long now      = System.currentTimeMillis();
        long nextMin  = ((now / 60_000L) + 1) * 60_000L;
        long delayMs  = nextMin - now;

        task = executor.scheduleAtFixedRate(
            this::tick, delayMs, 60_000L, TimeUnit.MILLISECONDS);

        System.out.println("[HoraCerta] Scheduler iniciado — próximo tick em "
            + delayMs / 1000.0 + "s");
    }

    public void stop() {
        if (task != null) { task.cancel(false); task = null; }
    }

    public void shutdown() {
        stop();
        executor.shutdownNow();
    }

    // ── Lógica interna ────────────────────────────────────────────────────────

    private void tick() {
        HoraCertaConfig cfg = this.config;
        if (cfg == null || !cfg.enabled()) return;

        LocalTime now = LocalTime.now();
        if (!cfg.firesAtMinutes().contains(now.getMinute())) return;

        System.out.printf("[HoraCerta] Disparo às %02d:%02d%n", now.getHour(), now.getMinute());

        HoraCertaResolver res = this.resolver;
        try {
            List<String> paths = res.resolve(now);
            BiConsumer<List<String>, LocalTime> cb = onFire;
            if (cb != null) Platform.runLater(() -> cb.accept(paths, now));
        } catch (Exception e) {
            System.err.println("[HoraCerta] Erro ao resolver: " + e.getMessage());
            Consumer<String> errCb = onError;
            if (errCb != null) Platform.runLater(() -> errCb.accept(e.getMessage()));
        }
    }
}
