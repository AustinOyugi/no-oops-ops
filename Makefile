BIN_DIR ?= .bin
BIN := $(BIN_DIR)/noops
INSTALL_DIR ?= $(HOME)/.local/bin
INSTALL_BIN := $(INSTALL_DIR)/noops
VERSION ?= 0.0.1

.PHONY: build install install-cli uninstall release-check release-snapshot test

build: test
	mkdir -p $(BIN_DIR)
	go build \
      -ldflags "-X github.com/AustinOyugi/no-oops-ops/internal/config.Version=$(VERSION)" \
      -a \
      -o $(BIN) \
      ./cmd/noops

install: build
	mkdir -p $(INSTALL_DIR)
	install -m 0755 $(BIN) $(INSTALL_BIN)

install-cli: install

uninstall: build
	$(BIN) uninstall $(UNINSTALL_ARGS)
	rm -f $(BIN)

test:
	go test ./...

release-check:
	goreleaser check

release-snapshot:
	goreleaser release --snapshot --clean
