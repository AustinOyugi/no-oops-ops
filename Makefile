BIN_DIR ?= .bin
BIN := $(BIN_DIR)/noops
INSTALL_DIR ?= $(HOME)/.local/bin
INSTALL_BIN := $(INSTALL_DIR)/noops

.PHONY: build install install-cli release-check release-snapshot test

build:
	mkdir -p $(BIN_DIR)
	go build -a -o $(BIN) ./cmd/noops

install: build
	mkdir -p $(INSTALL_DIR)
	install -m 0755 $(BIN) $(INSTALL_BIN)

install-cli: install

test:
	go test ./...

release-check:
	goreleaser check

release-snapshot:
	goreleaser release --snapshot --clean
