BIN_DIR ?= .bin
BIN := $(BIN_DIR)/noops
INSTALL_DIR ?= $(HOME)/.local/bin
INSTALL_BIN := $(INSTALL_DIR)/noops

.PHONY: build install install-cli test

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN) ./cmd/noops

install: install-cli

install-cli:
	mkdir -p $(INSTALL_DIR)
	go build -o $(INSTALL_BIN) ./cmd/noops

test:
	go test ./...
