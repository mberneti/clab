#!/usr/bin/env bash
set -euo pipefail

REPO="mberneti/clab"
INSTALL_DIR="${CLAB_INSTALL_DIR:-/usr/local/bin}"

# Detect OS and ARCH
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac
case "$OS" in
  linux|darwin) ;;
  mingw*|msys*|cygwin*) OS="windows" ;;
  *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

# Resolve latest release tag
echo "Fetching latest release..."
TAG="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')"

if [ -z "$TAG" ]; then
  echo "Could not determine latest release tag. Set CLAB_VERSION to install a specific version."
  exit 1
fi

TAG="${CLAB_VERSION:-$TAG}"
ARCHIVE="clab_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${TAG}/${ARCHIVE}"

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

echo "Downloading ${TAG} for ${OS}/${ARCH}..."
curl -fsSL "$URL" -o "$TMPDIR/$ARCHIVE"
tar -xzf "$TMPDIR/$ARCHIVE" -C "$TMPDIR"

EXTRACTED="$TMPDIR/clab_${OS}_${ARCH}"

# Install — try without sudo first, fall back if needed
install_bin() {
  local src="$1"
  local name
  name="$(basename "$src")"
  if [ -w "$INSTALL_DIR" ]; then
    cp "$src" "$INSTALL_DIR/$name"
  else
    echo "  (needs sudo for $INSTALL_DIR)"
    sudo cp "$src" "$INSTALL_DIR/$name"
  fi
  chmod +x "$INSTALL_DIR/$name"
  echo "  Installed $INSTALL_DIR/$name"
}

echo "Installing to $INSTALL_DIR ..."
for bin in "$EXTRACTED"/clab-*; do
  install_bin "$bin"
done

echo ""
echo "clab ${TAG} installed:"
echo "  clab-fetch-diff"
echo "  clab-lint-rules"
echo "  clab-post-comments"
echo ""
echo "Verify: clab-fetch-diff --help"
