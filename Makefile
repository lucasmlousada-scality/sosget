BINARY   := sosget
MODULE   := github.com/lucasmlousada-scality/sosget
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X main.version=$(VERSION)

.PHONY: build tidy lint clean release

build:
	go build -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd/sosget

tidy:
	go mod tidy

lint:
	golangci-lint run ./...

clean:
	rm -f $(BINARY) dist/

release:
	GOOS=linux   GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64   ./cmd/sosget
	GOOS=darwin  GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-darwin-amd64  ./cmd/sosget
	GOOS=darwin  GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-darwin-arm64  ./cmd/sosget
	GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-windows-amd64.exe ./cmd/sosget
	@echo "Binaries in dist/"
