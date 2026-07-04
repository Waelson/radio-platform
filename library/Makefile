BIN     := library-service
CMD     := ./cmd/library-service
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build test vet clean

build:
	CGO_ENABLED=1 go build -ldflags "-X main.Version=$(VERSION)" -o $(BIN) $(CMD)

test:
	CGO_ENABLED=1 go test ./...

vet:
	go vet ./...

clean:
	rm -f $(BIN)
