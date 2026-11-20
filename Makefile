VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
GOBUILD := CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)"
SRC := $(wildcard relay/*.go) relay/go.mod
PREFIX ?= $(HOME)/.local

# Binários do tablet (Linux estático; roda no Android sem NDK).
DEVICE := build/iconia-relay-android-arm build/iconia-relay-android-arm64 \
          build/iconia-relay-android-386 build/iconia-relay-android-amd64

.PHONY: all relay test lint install uninstall clean

all: relay

relay: $(DEVICE) build/iconia-relay-host

build/iconia-relay-android-arm: $(SRC)
	cd relay && GOOS=linux GOARCH=arm GOARM=7 $(GOBUILD) -o ../$@ .
build/iconia-relay-android-arm64: $(SRC)
	cd relay && GOOS=linux GOARCH=arm64 $(GOBUILD) -o ../$@ .
build/iconia-relay-android-386: $(SRC)
	cd relay && GOOS=linux GOARCH=386 $(GOBUILD) -o ../$@ .
build/iconia-relay-android-amd64: $(SRC)
	cd relay && GOOS=linux GOARCH=amd64 $(GOBUILD) -o ../$@ .
build/iconia-relay-host: $(SRC)
	cd relay && $(GOBUILD) -o ../$@ .

test:
	cd relay && go vet ./... && go test -race -count=1 ./...

lint:
	shellcheck bin/iconia

install:
	mkdir -p $(PREFIX)/bin
	ln -sf $(CURDIR)/bin/iconia $(PREFIX)/bin/iconia

uninstall:
	rm -f $(PREFIX)/bin/iconia

clean:
	rm -rf build
