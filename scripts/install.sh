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
CHECKSUM_FILE="SHA256SUMS"
if [ "$VERSION" = "latest" ]; then
  BASE_URL="https://github.com/${REPO}/releases/latest/download"
else
  BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

echo "downloading ${BASE_URL}/${ASSET}"
curl -fsSL "${BASE_URL}/${ASSET}" -o "$TMP_DIR/$ASSET"

# The release publishes SHA256SUMS alongside the archives. A mismatch means the
# download was corrupted or tampered with, so installation must not continue.
echo "downloading ${BASE_URL}/${CHECKSUM_FILE}"
if ! curl -fsSL "${BASE_URL}/${CHECKSUM_FILE}" -o "$TMP_DIR/$CHECKSUM_FILE"; then
  echo "error: could not download ${CHECKSUM_FILE}; refusing to install unverified files." >&2
  exit 1
fi

EXPECTED="$(awk -v asset="$ASSET" '$2 == asset || $2 == "*" asset { print $1 }' "$TMP_DIR/$CHECKSUM_FILE" | head -n 1)"
if [ -z "$EXPECTED" ]; then
  echo "error: ${CHECKSUM_FILE} has no entry for ${ASSET}; refusing to install." >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL="$(sha256sum "$TMP_DIR/$ASSET" | awk '{ print $1 }')"
elif command -v shasum >/dev/null 2>&1; then
  ACTUAL="$(shasum -a 256 "$TMP_DIR/$ASSET" | awk '{ print $1 }')"
else
  echo "error: neither sha256sum nor shasum is available; cannot verify the download." >&2
  exit 1
fi

if [ "$EXPECTED" != "$ACTUAL" ]; then
  echo "error: checksum mismatch for ${ASSET}" >&2
  echo "  expected: ${EXPECTED}" >&2
  echo "  actual:   ${ACTUAL}" >&2
  exit 1
fi
echo "checksum verified: ${ACTUAL}"

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

# Registration is deliberately a separate step. A process that writes an
# executable and then launches it looks like a dropper running its payload, so
# this script never starts the binary it just installed.
if [ -n "${DOORAY_TOKEN:-}" ]; then
  echo ""
  echo "To finish, register it with Claude Desktop by running:"
  echo "  \"$BIN_DIR/dooray-mcp\" register --token \"\$DOORAY_TOKEN\""
else
  echo ""
  echo "To finish, register it with Claude Desktop by running:"
  echo "  \"$BIN_DIR/dooray-mcp\" register --token \"{personal-token}\""
fi
