#!/usr/bin/env sh
# Installs the dooray-mcp binary for macOS or Linux into a directory on PATH.
#
#   curl -fsSL https://raw.githubusercontent.com/minseoky/dooray-mcp-go/main/scripts/install.sh | sh
#
# Environment:
#   DOORAY_MCP_VERSION  release tag to install, default: latest
#   DOORAY_MCP_BIN_DIR  install directory, default: $HOME/.local/bin

set -eu

REPO="minseoky/dooray-mcp-go"
BIN_DIR="${DOORAY_MCP_BIN_DIR:-$HOME/.local/bin}"
VERSION="${DOORAY_MCP_VERSION:-latest}"

case "$(uname -s)" in
  Darwin) OS="darwin" ;;
  Linux)  OS="linux" ;;
  *) echo "unsupported operating system: $(uname -s)" >&2; exit 1 ;;
esac

case "$(uname -m)" in
  arm64|aarch64) ARCH="arm64" ;;
  x86_64|amd64)  ARCH="amd64" ;;
  *) echo "unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

ASSET="dooray-mcp_${OS}_${ARCH}.tar.gz"
if [ "$VERSION" = "latest" ]; then
  URL="https://github.com/${REPO}/releases/latest/download/${ASSET}"
else
  URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

echo "downloading ${URL}"
curl -fsSL "$URL" -o "$TMP_DIR/$ASSET"
tar xzf "$TMP_DIR/$ASSET" -C "$TMP_DIR"

mkdir -p "$BIN_DIR"
mv "$TMP_DIR/dooray-mcp_${OS}_${ARCH}" "$BIN_DIR/dooray-mcp"
chmod +x "$BIN_DIR/dooray-mcp"

# Gatekeeper flags downloaded binaries on macOS; clearing the quarantine
# attribute keeps the MCP client from failing to spawn it.
if [ "$OS" = "darwin" ] && command -v xattr >/dev/null 2>&1; then
  xattr -d com.apple.quarantine "$BIN_DIR/dooray-mcp" 2>/dev/null || true
fi

echo "installed: $BIN_DIR/dooray-mcp"
case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) echo "warning: $BIN_DIR is not on PATH; add it to your shell profile." >&2 ;;
esac
