BIN_DIR ?= .bin
BIN := $(BIN_DIR)/noops
INSTALL_DIR ?= $(HOME)/.local/bin
INSTALL_BIN := $(INSTALL_DIR)/noops
VERSION ?= 0.0.1

.PHONY: build install install-cli uninstall release release-check release-snapshot test

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
	go clean -testcache && go test -v -count=1 ./...

release-check:
	goreleaser check

release-snapshot:
	goreleaser release --snapshot --clean

# Creates and pushes a release tag. The GitHub release workflow publishes the
# binaries and checksums after the tag reaches origin.
TAG ?= v$(VERSION)
release: test release-check
	@printf '%s\n' "$(TAG)" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$$' || { echo "TAG must be vMAJOR.MINOR.PATCH (for example: v0.0.1)"; exit 1; }
	@test -z "$$(git status --porcelain)" || { echo "working tree must be clean before releasing"; exit 1; }
	@git rev-parse -q --verify "refs/tags/$(TAG)" >/dev/null && { echo "tag $(TAG) already exists"; exit 1; } || true
	git tag -a "$(TAG)" -m "No Oops Ops $(TAG)"
	git push origin "$(TAG)"
