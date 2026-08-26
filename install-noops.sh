#!/usr/bin/env bash
set -euo pipefail

# Usage: ./install-noops.sh [vMAJOR.MINOR.PATCH]
VERSION="${1:-v0.0.1}"
REPOSITORY="AustinOyugi/no-oops-ops"

if [[ ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Version must use vMAJOR.MINOR.PATCH (for example: v0.0.1)." >&2
  exit 1
fi

case "$(uname -s)" in
  Linux) ;;
  *)
    echo "This installer supports Linux servers only." >&2
    exit 1
    ;;
esac

case "$(uname -m)" in
  x86_64|amd64) ARCH="x86_64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "Unsupported CPU architecture: $(uname -m)." >&2
    exit 1
    ;;
esac

ARCHIVE="noops_Linux_${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/${REPOSITORY}/releases/download/${VERSION}"
TEMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TEMP_DIR"' EXIT

curl --fail --location --silent --show-error \
  --output "$TEMP_DIR/$ARCHIVE" \
  "$DOWNLOAD_URL/$ARCHIVE"
curl --fail --location --silent --show-error \
  --output "$TEMP_DIR/checksums.txt" \
  "$DOWNLOAD_URL/checksums.txt"

(
  cd "$TEMP_DIR"
  sha256sum --ignore-missing --check checksums.txt
)

tar --extract --gzip --file "$TEMP_DIR/$ARCHIVE" --directory "$TEMP_DIR"
sudo install --mode=0755 "$TEMP_DIR/noops" /usr/local/bin/noops
noops --version
