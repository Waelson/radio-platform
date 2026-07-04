# 02 — Modelo de Processo

## Processo principal

O Playout Engine roda como um processo separado da UI:

```text
playout-engine
```

A UI não deve embutir o Engine no mesmo processo.

## Responsabilidades do processo

O processo `playout-engine` é responsável por:

- Inicializar configuração.
- Abrir API HTTP local.
- Inicializar event bus.
- Inicializar output de áudio.
- Gerenciar fila em memória.
- Executar áudio.
- Publicar status e eventos.
- Expor health checks.

## Mono-instância

A primeira versão deve impedir múltiplas instâncias concorrentes usando a mesma configuração.

Estratégias por plataforma:

- macOS/Linux: lock file em diretório configurável.
- Windows: named mutex ou lock file.

MVP aceitável:

- lock file com PID.
- se arquivo existir, validar se o processo ainda está vivo.

## Inicialização

Sequência recomendada:

1. Ler configuração.
2. Configurar logger.
3. Validar diretórios.
4. Verificar lock de instância.
5. Inicializar state manager.
6. Inicializar event bus.
7. Inicializar audio output.
8. Inicializar playback manager.
9. Inicializar API server.
10. Publicar evento `EngineStarted`.

## Shutdown

O Engine deve tratar:

- SIGINT.
- SIGTERM.
- Ctrl+C.
- Encerramento via endpoint `/v1/admin/shutdown` se habilitado.

Sequência:

1. Parar de aceitar novos comandos.
2. Publicar `EngineStopping`.
3. Parar playback com fade opcional curto.
4. Fechar decoder ativo.
5. Fechar output device.
6. Fechar conexões WebSocket.
7. Remover lock file.
8. Publicar logs finais.

## Relação com UI

A UI pode:

- iniciar o processo, se ele não estiver rodando;
- conectar a `http://127.0.0.1:<port>`;
- obter status via `/v1/status`;
- receber eventos via `/v1/events`.

A UI não pode:

- ler memória interna;
- acessar mixer;
- abrir dispositivo de áudio;
- acessar diretamente a fila interna;
- tocar áudio.

## Diagrama de isolamento

```text
┌─────────────────────────────┐
│ UI Process                  │
│ - Rendering                 │
│ - User input                │
│ - REST/WebSocket client     │
└──────────────┬──────────────┘
               │ localhost/LAN
               ▼
┌─────────────────────────────┐
│ Playout Engine Process       │
│ - API                        │
│ - State                      │
│ - Queue                      │
│ - Playback                   │
│ - Mixer                      │
│ - Output                     │
└──────────────┬──────────────┘
               │ OS audio API
               ▼
┌─────────────────────────────┐
│ Audio Device                │
└─────────────────────────────┘
```

## Performance

O processo de playout deve priorizar estabilidade de áudio. Operações pesadas devem ser evitadas no Engine.

Não realizar no Engine:

- upload de arquivos;
- normalização LUFS pesada;
- reindexação de biblioteca;
- conversão de biblioteca em lote;
- análise de waveform de arquivos grandes.

Essas tarefas pertencem a um serviço de ingestão ou aplicação separada.
