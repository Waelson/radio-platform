# Plano: Job de build Linux no GitHub Actions

## Contexto

O workflow `.github/workflows/playout.yml` já possui:

- `test` — roda `go test ./internal/...` no `ubuntu-latest` (sem artefato)
- `build-windows-wasapi` — compila `playout-engine.exe` no `windows-latest` e faz upload do artefato

Falta um job que produza o binário **Linux** (`playout-engine`) e faça upload como artefato.

---

## Decisão: duas variantes de build Linux

O Linux tem dois drivers de saída de áudio relevantes:

| Variante | Build tags | CGO | Dependência extra | Uso |
|---|---|---|---|---|
| **headless** | `cli` | Não | Nenhuma | Testes, integração CI, servidor sem áudio |
| **portaudio** | `portaudio cli` | Sim | `libportaudio-dev` | Estação de rádio real |

Ambas serão produzidas no mesmo job para maximizar cobertura com um único runner.

---

## Implementação

### Arquivo modificado

`.github/workflows/playout.yml` — adicionar job `build-linux`.

### Job: `build-linux`

```yaml
build-linux:
  name: Build (Linux)
  runs-on: ubuntu-latest
  defaults:
    run:
      working-directory: playout
  steps:
    - uses: actions/checkout@v4

    - uses: actions/setup-go@v5
      with:
        go-version-file: playout/go.mod
        cache-dependency-path: playout/go.sum

    # ── Variante 1: headless (CGO_ENABLED=0, sem driver de áudio real) ──────────
    - name: Build playout-engine (headless / cli)
      env:
        CGO_ENABLED: "0"
        GOARCH: amd64
        GOOS: linux
      run: |
        go build \
          -tags "cli" \
          -ldflags "-X main.Version=${{ github.sha }}" \
          -o playout-engine-headless \
          ./cmd/playout-engine

    - name: Verify headless binary
      run: |
        size=$(stat -c%s playout-engine-headless)
        echo "playout-engine-headless: ${size} bytes"
        [ "$size" -gt 0 ]

    # ── Variante 2: portaudio (CGO_ENABLED=1, driver PortAudio real) ────────────
    - name: Install libportaudio-dev
      run: sudo apt-get install -y libportaudio2 libportaudio-dev

    - name: Build playout-engine (portaudio / cli)
      env:
        CGO_ENABLED: "1"
        GOARCH: amd64
        GOOS: linux
      run: |
        go build \
          -tags "portaudio cli" \
          -ldflags "-X main.Version=${{ github.sha }}" \
          -o playout-engine-portaudio \
          ./cmd/playout-engine

    - name: Verify portaudio binary
      run: |
        size=$(stat -c%s playout-engine-portaudio)
        echo "playout-engine-portaudio: ${size} bytes"
        [ "$size" -gt 0 ]

    # ── Upload de ambos os artefatos ─────────────────────────────────────────────
    - name: Upload headless artifact
      uses: actions/upload-artifact@v4
      with:
        name: playout-engine-linux-headless-${{ github.sha }}
        path: playout/playout-engine-headless
        retention-days: 7

    - name: Upload portaudio artifact
      uses: actions/upload-artifact@v4
      with:
        name: playout-engine-linux-portaudio-${{ github.sha }}
        path: playout/playout-engine-portaudio
        retention-days: 7
```

---

## Arquivo resultante — estrutura completa de jobs

```
playout.yml
├── test                    (ubuntu-latest, CGO=0, ./internal/...)
├── build-windows-wasapi    (windows-latest, CGO=1, -tags "wasapi cli")
└── build-linux             (ubuntu-latest)
    ├── variante headless   (CGO=0, -tags "cli")
    └── variante portaudio  (CGO=1, -tags "portaudio cli")
```

---

## Verificação pós-implementação

```bash
# Após push, verificar na aba Actions:
# - job "Build (Linux)" passa
# - dois artefatos são publicados:
#   playout-engine-linux-headless-<sha>
#   playout-engine-linux-portaudio-<sha>
```

---

## Notas

- O job `build-linux` é independente de `test` e `build-windows-wasapi` — roda em paralelo.
- `apt-get install` é necessário apenas para a variante portaudio; a variante headless usa `CGO_ENABLED=0` e não precisa de libs externas.
- `GOOS=linux GOARCH=amd64` é explícito por clareza, mas o runner ubuntu-latest já usa esses valores por padrão.
- A flag `cli` desabilita systray/webview em ambas as variantes, tornando os binários adequados para uso em servidor headless.
