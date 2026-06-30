BINARY  := sosget
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build tidy test clean release-mac release-linux release-windows

build:
	go build -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd/sosget

tidy:
	go mod tidy

test:
	go test ./...

clean:
	rm -f $(BINARY) && rm -rf dist/

# Build for each platform natively (Fyne requires CGo and platform GL libraries)
release-mac:
	mkdir -p dist
	go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-darwin ./cmd/sosget
	@echo "macOS binary: dist/$(BINARY)-darwin"

release-linux:
	mkdir -p dist
	CGO_ENABLED=1 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-linux ./cmd/sosget
	@echo "Linux binary: dist/$(BINARY)-linux"

release-windows:
	mkdir -p dist
	# Requires MinGW: brew install mingw-w64
	CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc GOOS=windows GOARCH=amd64 \
		go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-windows.exe ./cmd/sosget
	@echo "Windows binary: dist/$(BINARY)-windows.exe"
