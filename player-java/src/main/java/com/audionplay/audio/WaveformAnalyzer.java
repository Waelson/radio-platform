package com.audionplay.audio;

import java.util.function.Consumer;

/**
 * Contrato para análise de waveform de um arquivo de áudio.
 *
 * A análise é assíncrona — o resultado é entregue via callback
 * na FX thread para que a UI possa desenhar imediatamente.
 */
public interface WaveformAnalyzer {

    /**
     * Analisa o arquivo e entrega um array de picos normalizados [0.0, 1.0].
     *
     * @param filePath  caminho absoluto do arquivo
     * @param numBars   número de barras desejadas na waveform
     * @param onReady   callback chamado na FX thread com o array de picos
     * @param onError   callback chamado na FX thread em caso de falha
     */
    void analyze(String filePath, int numBars,
                 Consumer<double[]> onReady,
                 Consumer<String> onError);
}
