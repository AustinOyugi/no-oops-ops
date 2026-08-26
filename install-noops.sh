#!/usr/bin/env bash
set -euo pipefail

# Usage: ./install-noops.sh [vMAJOR.MINOR.PATCH]
#        GITHUB_TOKEN=... ./install-noops.sh --snapshot <latest|run-id/artifacts/artifact-id>
VERSION="${1:-v0.0.1}"
REPOSITORY="AustinOyugi/no-oops-ops"
SNAPSHOT_REF=""

if [[ "${1:-}" == "--snapshot" ]]; then
  if [[ $# -ne 2 || ( "$2" != "latest" && ! "$2" =~ ^[0-9]+/artifacts/[0-9]+$ ) ]]; then
    echo "Usage: GITHUB_TOKEN=... $0 --snapshot <latest|run-id/artifacts/artifact-id>" >&2
    exit 1
  fi
  SNAPSHOT_REF="$2"
fi

if [[ -z "$SNAPSHOT_REF" && ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
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
TEMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TEMP_DIR"' EXIT

if [[ -n "$SNAPSHOT_REF" ]]; then
  if [[ -z "${GITHUB_TOKEN:-}" ]]; then
    echo "Snapshot installation requires GITHUB_TOKEN with Actions read access." >&2
    exit 1
  fi
  if [[ "$SNAPSHOT_REF" == "latest" ]]; then
    ARTIFACT_ID="$(curl --fail --location --silent --show-error -H "Authorization: Bearer $GITHUB_TOKEN" \
      "https://api.github.com/repos/${REPOSITORY}/actions/artifacts?per_page=1" \
      | sed -n 's/^[[:space:]]*"id": \([0-9][0-9]*\),$/\1/p' | head -n 1)"
    if [[ -z "$ARTIFACT_ID" ]]; then
      echo "No GitHub Actions snapshot artifact is available." >&2
      exit 1
    fi
  else
    ARTIFACT_ID="${SNAPSHOT_REF##*/}"
  fi
  curl --fail --location --silent --show-error -H "Authorization: Bearer $GITHUB_TOKEN" \
    --output "$TEMP_DIR/snapshot.zip" \
    "https://api.github.com/repos/${REPOSITORY}/actions/artifacts/${ARTIFACT_ID}/zip"
  if command -v unzip >/dev/null; then
    unzip -q "$TEMP_DIR/snapshot.zip" -d "$TEMP_DIR/artifact"
  elif command -v python3 >/dev/null; then
    python3 -c 'import sys, zipfile; zipfile.ZipFile(sys.argv[1]).extractall(sys.argv[2])' \
      "$TEMP_DIR/snapshot.zip" "$TEMP_DIR/artifact"
  else
    echo "Snapshot installation requires unzip or python3." >&2
    exit 1
  fi
  ARCHIVE_PATH="$(find "$TEMP_DIR/artifact" -name "$ARCHIVE" -type f -print -quit)"
  CHECKSUM_PATH="$(find "$TEMP_DIR/artifact" -name checksums.txt -type f -print -quit)"
  [[ -n "$ARCHIVE_PATH" && -n "$CHECKSUM_PATH" ]] || { echo "Snapshot artifact is missing $ARCHIVE or checksums.txt." >&2; exit 1; }
  mkdir "$TEMP_DIR/payload"
  cp "$ARCHIVE_PATH" "$TEMP_DIR/payload/$ARCHIVE"
  cp "$CHECKSUM_PATH" "$TEMP_DIR/payload/checksums.txt"
  DOWNLOAD_DIR="$TEMP_DIR/payload"
else
  DOWNLOAD_URL="https://github.com/${REPOSITORY}/releases/download/${VERSION}"
  curl --fail --location --silent --show-error --output "$TEMP_DIR/$ARCHIVE" "$DOWNLOAD_URL/$ARCHIVE"
  curl --fail --location --silent --show-error --output "$TEMP_DIR/checksums.txt" "$DOWNLOAD_URL/checksums.txt"
  DOWNLOAD_DIR="$TEMP_DIR"
fi

(
  cd "$DOWNLOAD_DIR"
  sha256sum --ignore-missing --check checksums.txt
)

tar --extract --gzip --file "$DOWNLOAD_DIR/$ARCHIVE" --directory "$TEMP_DIR"
sudo install --mode=0755 "$TEMP_DIR/noops" /usr/local/bin/noops
noops --version
