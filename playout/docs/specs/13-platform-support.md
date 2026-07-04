# 13 — Suporte a Plataformas

## Plataformas alvo

- macOS
- Linux
- Windows

## Estratégia

O core do Engine deve ser independente de plataforma.

Código específico fica em:

```text
/internal/platform
/internal/audio/output/<driver>
```

## macOS

### Áudio

- Primeira opção: PortAudio usando CoreAudio.
- Futuro: adapter CoreAudio nativo.

### Instalação dev

```bash
brew install go
brew install ffmpeg
brew install portaudio
```

### Observações

- macOS pode solicitar permissão de microfone/áudio dependendo da biblioteca usada.
- Para output comum, normalmente não há permissão especial.

## Linux

### Áudio

- PortAudio usando ALSA/PulseAudio/JACK.
- Futuro: ALSA nativo.

### Instalação dev Debian/Ubuntu

```bash
sudo apt-get update
sudo apt-get install -y golang ffmpeg portaudio19-dev
```

### Observações

- Permissões de usuário para dispositivos de áudio podem ser necessárias.
- Em rádio, preferir ambiente controlado e desativar sleep/hibernation.

## Windows

### Áudio

- PortAudio usando WASAPI/DirectSound.
- Futuro: WASAPI nativo.

### Instalação dev

- Instalar Go.
- Instalar FFmpeg e adicionar ao PATH.
- Preparar toolchain C se PortAudio binding exigir CGO.

## Build tags

Se necessário, usar build tags:

```go
//go:build darwin
//go:build linux
//go:build windows
```

## CGO

A primeira versão pode depender de CGO se usar PortAudio.

Mitigação:

- Isolar output adapter.
- Permitir `NullOutput` sem CGO para testes.
- Ter build/test do core sem output real.

## FFmpeg

O decoder inicial pode exigir `ffmpeg` disponível no PATH.

Validação no startup:

```bash
ffmpeg -version
```

Se não existir:

- retornar erro claro;
- permitir modo sem decoder apenas para testes se configurado.

## Caminhos de arquivo

Usar `filepath` do Go.

Não assumir separador `/`.

Receber paths absolutos ou paths normalizados.

## Sinais

- macOS/Linux: SIGINT/SIGTERM.
- Windows: Ctrl+C e eventos de console.

Usar `os/signal` e `context`.
