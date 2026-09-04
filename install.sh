#!/bin/sh
# ploi-tui installer — Linux and macOS (amd64/arm64).
#
# Install (latest release):
#   curl -sSfL https://raw.githubusercontent.com/pdaether/ploi-tui/main/install.sh | bash
#
# Update to the latest release: re-run the same command.
#
# Options (environment variables):
#   INSTALL_DIR  target directory (default: /usr/local/bin, falls back to
#                ~/.local/bin when not writable or sudo is unavailable)
#
# Dependencies: curl, tar. No checksum verification in v1 (kept tiny on purpose).
set -eu

REPO="pdaether/ploi-tui"
BINARY="ploi-tui"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

fail() {
  echo "install: $*" >&2
  exit 1
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "need '$1' (install it and re-run)"
}

need_cmd uname
need_cmd curl
need_cmd tar
need_cmd mktemp
need_cmd chmod

# --- Detect OS (matches GoReleaser archive names: Linux / Darwin) ---
OS="$(uname -s)"
case "$OS" in
  Linux) OS="Linux" ;;
  Darwin) OS="Darwin" ;;
  *) fail "unsupported OS: $(uname -s) (only Linux and macOS are supported)" ;;
esac

# --- Detect arch (matches GoReleaser archive names: x86_64 / aarch64) ---
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64 | amd64) ARCH="x86_64" ;;
  arm64 | aarch64) ARCH="aarch64" ;;
  *) fail "unsupported architecture: $(uname -m) (only x86_64 and aarch64/arm64 are supported)" ;;
esac

ARCHIVE="${BINARY}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/latest/download/${ARCHIVE}"

echo "Installing ${BINARY} (${OS}/${ARCH})..."
echo "  from: ${URL}"
echo "  to:   ${INSTALL_DIR}/${BINARY}"
echo "To update later, re-run this same command."
echo

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT INT TERM

echo "Downloading ${ARCHIVE}..."
if ! curl -sSfL "$URL" -o "$TMPDIR/$ARCHIVE"; then
  fail "download failed.

  Possible reasons:
  - No release published yet (check https://github.com/${REPO}/releases)
  - No network access to github.com
  - Platform not covered (only: Linux/Darwin x x86_64/aarch64)"
fi

echo "Extracting..."
tar -xzf "$TMPDIR/$ARCHIVE" -C "$TMPDIR"
[ -f "$TMPDIR/$BINARY" ] || fail "archive did not contain '${BINARY}' (got: $(ls "$TMPDIR"))"
chmod +x "$TMPDIR/$BINARY"

# --- Pick install target: preferred dir, else ~/.local/bin fallback ---
TARGET_DIR="$INSTALL_DIR"
if [ ! -d "$TARGET_DIR" ]; then
  echo "Creating $TARGET_DIR..."
  if ! mkdir -p "$TARGET_DIR" 2>/dev/null; then
    echo "Cannot create $TARGET_DIR, falling back to ~/.local/bin."
    TARGET_DIR="$HOME/.local/bin"
    mkdir -p "$TARGET_DIR"
  fi
fi

install_file() {
  # $1 = source, $2 = destination
  if mv "$1" "$2" 2>/dev/null; then
    return 0
  fi
  if command -v sudo >/dev/null 2>&1 && sudo mv "$1" "$2"; then
    return 0
  fi
  return 1
}

if ! install_file "$TMPDIR/$BINARY" "$TARGET_DIR/$BINARY"; then
  echo "$TARGET_DIR is not writable and sudo did not work."
  echo "Falling back to ~/.local/bin."
  TARGET_DIR="$HOME/.local/bin"
  mkdir -p "$TARGET_DIR"
  install_file "$TMPDIR/$BINARY" "$TARGET_DIR/$BINARY" \
    || fail "could not install to $TARGET_DIR (check permissions)"
fi

# --- macOS Gatekeeper hint (binary is not notarized in v1) ---
if [ "$OS" = "Darwin" ] && command -v xattr >/dev/null 2>&1; then
  xattr -d com.apple.quarantine "$TARGET_DIR/$BINARY" 2>/dev/null || true
fi

echo
echo "Installed to $TARGET_DIR/$BINARY"

# --- PATH hint ---
case ":$PATH:" in
  *":$TARGET_DIR:"*) ;;
  *)
    echo "Note: $TARGET_DIR is not on your PATH."
    echo "Add it, e.g.: export PATH=\"\$TARGET_DIR:\$PATH\""
    ;;
esac

if [ "$OS" = "Darwin" ]; then
  echo "Note (macOS): the binary is not notarized. If macOS blocks it,"
  echo "right-click it in Finder, choose Open, and confirm once."
fi

# --- Smoke test ---
if "$TARGET_DIR/$BINARY" version 2>/dev/null; then
  echo "Done. Run '${BINARY}' to launch, '${BINARY} connect' to set up your API token."
else
  echo "Installed, but smoke test ('${BINARY} version') failed — try running it directly:"
  echo "  $TARGET_DIR/$BINARY version"
fi
