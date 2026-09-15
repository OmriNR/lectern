#!/usr/bin/env bash
#
# Builds lectern and installs it on your PATH so it can be run from any
# directory. Safe to re-run to rebuild/update an existing install.
#
# Run locally from a checkout:
#   ./install.sh
#
# Or install directly from GitHub without cloning first:
#   curl -fsSL https://raw.githubusercontent.com/OmriNR/lectern/main/install.sh | bash
set -euo pipefail

BINARY_NAME="lectern"
INSTALL_DIR="${LECTERN_INSTALL_DIR:-$HOME/.local/bin}"
REPO_URL="https://github.com/OmriNR/lectern.git"

if ! command -v go >/dev/null 2>&1; then
    echo "error: Go is not installed or not on PATH. Install Go from https://go.dev/dl/ first." >&2
    exit 1
fi

# When run from a local checkout, build straight from it. When piped from
# curl (no local file to point at), fetch a fresh copy to build instead.
SCRIPT_DIR=""
if [ -n "${BASH_SOURCE:-}" ] && [ -f "${BASH_SOURCE[0]}" ]; then
    SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
fi

CLONE_DIR=""
cleanup() {
    if [ -n "$CLONE_DIR" ]; then
        rm -rf "$CLONE_DIR"
    fi
}
trap cleanup EXIT

if [ -n "$SCRIPT_DIR" ] && [ -f "$SCRIPT_DIR/go.mod" ]; then
    SRC_DIR="$SCRIPT_DIR"
else
    if ! command -v git >/dev/null 2>&1; then
        echo "error: git is not installed or not on PATH." >&2
        exit 1
    fi
    CLONE_DIR="$(mktemp -d)"
    echo "Fetching lectern source from ${REPO_URL}..."
    git clone --depth 1 "$REPO_URL" "$CLONE_DIR" >/dev/null
    SRC_DIR="$CLONE_DIR"
fi

echo "Building ${BINARY_NAME}..."
mkdir -p "$INSTALL_DIR"
(cd "$SRC_DIR" && go build -o "$INSTALL_DIR/$BINARY_NAME" .)
chmod +x "$INSTALL_DIR/$BINARY_NAME"

echo "Installed ${BINARY_NAME} to ${INSTALL_DIR}/${BINARY_NAME}"

case ":$PATH:" in
    *":$INSTALL_DIR:"*)
        echo "You're all set. Run '${BINARY_NAME}' from any directory."
        ;;
    *)
        SHELL_RC="$HOME/.bashrc"
        case "${SHELL:-}" in
            */zsh) SHELL_RC="$HOME/.zshrc" ;;
        esac

        PATH_LINE="export PATH=\"$INSTALL_DIR:\$PATH\""
        if [ -f "$SHELL_RC" ] && grep -qxF "$PATH_LINE" "$SHELL_RC"; then
            echo "You're all set. Restart your shell (or 'source \"$SHELL_RC\"') to run '${BINARY_NAME}' from any directory."
        else
            echo "$PATH_LINE" >> "$SHELL_RC"
            echo "Added ${INSTALL_DIR} to PATH in ${SHELL_RC}."
            echo "Restart your shell (or run: source \"$SHELL_RC\") to run '${BINARY_NAME}' from any directory."
        fi
        ;;
esac
