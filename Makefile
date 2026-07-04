.PHONY: build-playout build-library build-player test-all dist-mac clean

build-playout:
	$(MAKE) -C playout build-coreaudio

build-library:
	$(MAKE) -C library build

build-player:
	cd player && npm run build

test-all:
	cd playout  && go test ./...
	cd library  && go test ./...

dist-mac:
	$(MAKE) -C playout dist-mac

clean:
	$(MAKE) -C playout clean
	$(MAKE) -C library clean
