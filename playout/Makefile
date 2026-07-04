BIN        := engine
CMD        := ./cmd/playout-engine
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS    := -ldflags "-X main.Version=$(VERSION)"

APP_NAME   := Playout
APP_BUNDLE := dist/$(APP_NAME).app
ICON_SRC   := cmd/playout-engine/assets/icon.png
PLIST_TMPL := cmd/playout-engine/Info.plist.tmpl

.PHONY: build build-portaudio build-coreaudio dist-mac test test-race vet clean run run-portaudio help

## build: compile without PortAudio (pure Go, no CGO)
build:
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BIN) $(CMD)

## build-portaudio: compile with PortAudio output support (requires libportaudio)
build-portaudio:
	go build -tags portaudio $(LDFLAGS) -o $(BIN) $(CMD)

## build-coreaudio: compile with CoreAudio (macOS only)
build-coreaudio:
	go build -tags coreaudio $(LDFLAGS) -o playout-engine $(CMD)

## dist-mac: build macOS .app bundle in dist/
## O binário é o CFBundleExecutable direto (sem shell script wrapper),
## necessário para que NSApplication/systray inicialize corretamente.
dist-mac: build-coreaudio
	@echo "→ Criando bundle: $(APP_BUNDLE)"
	@rm -rf "$(APP_BUNDLE)"
	@mkdir -p "$(APP_BUNDLE)/Contents/MacOS"
	@mkdir -p "$(APP_BUNDLE)/Contents/Resources"
	@# Binário — é o CFBundleExecutable direto
	@cp playout-engine "$(APP_BUNDLE)/Contents/MacOS/playout-engine"
	@# Info.plist (substitui {{VERSION}})
	@sed "s/{{VERSION}}/$(VERSION)/g" "$(PLIST_TMPL)" > "$(APP_BUNDLE)/Contents/Info.plist"
	@# Ícone .icns (requer sips + iconutil — nativo macOS)
	@rm -rf /tmp/AppIcon.iconset
	@mkdir /tmp/AppIcon.iconset
	@sips -z 16  16  "$(ICON_SRC)" --out /tmp/AppIcon.iconset/icon_16x16.png      2>/dev/null
	@sips -z 32  32  "$(ICON_SRC)" --out /tmp/AppIcon.iconset/icon_16x16@2x.png   2>/dev/null
	@sips -z 32  32  "$(ICON_SRC)" --out /tmp/AppIcon.iconset/icon_32x32.png      2>/dev/null
	@sips -z 64  64  "$(ICON_SRC)" --out /tmp/AppIcon.iconset/icon_32x32@2x.png   2>/dev/null
	@sips -z 64  64  "$(ICON_SRC)" --out /tmp/AppIcon.iconset/icon_128x128.png    2>/dev/null
	@sips -z 64  64  "$(ICON_SRC)" --out /tmp/AppIcon.iconset/icon_128x128@2x.png 2>/dev/null
	@iconutil -c icns /tmp/AppIcon.iconset -o "$(APP_BUNDLE)/Contents/Resources/AppIcon.icns"
	@rm -rf /tmp/AppIcon.iconset
	@echo "✓ Bundle pronto: $(APP_BUNDLE)"
	@echo "  Arraste para /Applications ou clique 2x para testar."

## test: run all unit tests (no CGO required)
test:
	CGO_ENABLED=0 go test ./...

## test-race: run all tests with race detector
test-race:
	go test -race ./...

## vet: run go vet
vet:
	go vet ./...

## clean: remove compiled binary
clean:
	rm -f $(BIN)

## run: build (pure Go) and start the engine
run: build
	./$(BIN)

## run-portaudio: build with PortAudio and start the engine
run-portaudio: build-portaudio
	./$(BIN)

## help: list available targets
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'
