# 00 — Visão Geral

## Objetivo

Desenvolver um **Playout Engine** em Go para automação de rádio local FM/AM.

O Engine deve ser responsável por executar áudio de forma contínua, previsível e resiliente, sem depender da interface gráfica. A UI será apenas uma camada de operação e visualização.

## Problema

Em uma automação de rádio, a interface pode travar, fechar ou reiniciar sem que o áudio no ar seja afetado. O componente que toca áudio precisa ser isolado, simples, observável e capaz de sobreviver a falhas locais.

## Escopo inicial

A primeira versão deve suportar:

- Execução como processo separado da UI.
- API HTTP local para comandos.
- WebSocket para eventos e status em tempo real.
- Fila em memória.
- Reprodução de arquivos de áudio por path absoluto.
- Comandos básicos: play, pause, resume, stop, skip, enqueue, clear queue.
- Status: estado atual, item tocando, posição, duração, tamanho da fila.
- Crossfade básico entre músicas.
- Panic mode com áudio de segurança.
- Audio health: nível, silêncio e buffer.
- Execução em macOS, Linux e Windows.

## Fora do escopo inicial

A primeira versão **não** deve implementar:

- UI gráfica.
- Banco de dados.
- Cadastro de músicas.
- Gestão completa de biblioteca.
- Scheduler comercial avançado.
- Redundância main/backup.
- Streaming web.
- Integração com transmissor.
- Normalização LUFS completa.
- Beat matching avançado.

Esses itens podem ser adicionados depois sem alterar o núcleo do Engine.

## Princípios

1. **A UI nunca toca áudio.**
2. **O Engine não depende da UI para continuar rodando.**
3. **O Engine recebe itens já resolvidos.**
4. **O Engine deve ser pequeno, testável e orientado a mensagens.**
5. **Toda ação deve gerar evento auditável.**
6. **Estado deve ser consultável a qualquer momento.**
7. **Áudio no ar tem prioridade sobre qualquer outra atividade.**

## Terminologia

| Termo | Significado |
|---|---|
| Engine | Processo Go responsável por tocar áudio. |
| UI | Interface de operação em React, Angular, desktop ou web. |
| Asset | Unidade de áudio: música, spot, vinheta, bed, efeito. |
| Queue Item | Item resolvido a ser executado pelo Engine. |
| Playback | Execução de áudio no pipeline. |
| Crossfade | Transição em que o áudio atual sai enquanto o próximo entra. |
| Ducking | Redução temporária do volume de um canal para outro se destacar. |
| Hot Button | Botão de disparo instantâneo de áudio. |
| Panic | Estado de emergência em que o Engine toca um áudio seguro. |
| Audio Health | Indicadores técnicos do áudio: nível, silêncio e buffer. |

## Modelo mental

```text
UI
 │
 │ REST commands / WebSocket events
 ▼
Playout Engine
 │
 │ decoder → mixer → output
 ▼
Audio Device
```

A UI envia intenções. O Engine valida, executa e publica eventos.
