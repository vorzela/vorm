#!/bin/bash
# vorm per-user installer (no root).
# Usage: curl -fsSL https://raw.githubusercontent.com/vorzela/vorm/main/install.sh | bash
set -euo pipefail

CYAN='\033[0;36m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; RED='\033[0;31m'; NC='\033[0m'
info() { echo -e "${CYAN}ℹ ${1}${NC}"; }
ok() { echo -e "${GREEN}✓ ${1}${NC}"; }
warn() { echo -e "${YELLOW}⚠ ${1}${NC}"; }
fail() { echo -e "${RED}✗ ${1}${NC}"; exit 1; }

INSTALL_DIR="${VORM_INSTALL_DIR:-${HOME}/.local/bin}"
VERSION="${VORM_VERSION:-latest}"
BINARY="${INSTALL_DIR}/vorm"

if [ "$(id -u)" -eq 0 ] && [ -z "${VORM_INSTALL_DIR:-}" ] && [ -z "${SUDO_USER:-}" ]; then
  warn "Running as root installs into /root/.local/bin."
  warn "vorm is a per-user tool; prefer running this as your normal user."
fi

command -v go >/dev/null 2>&1 || fail "Go is required. Install Go from https://go.dev/dl/ (no root needed for a user install)."

mkdir -p "$INSTALL_DIR"
info "Installing vorm@${VERSION} into ${INSTALL_DIR} (per-user; no root)"
GOBIN="$INSTALL_DIR" go install "github.com/vorzela/vorm/cmd/vorm@${VERSION}"
chmod +x "$BINARY"

# Add the install directory to the user's shell profile when needed.
if [[ ":$PATH:" != *":${INSTALL_DIR}:"* ]]; then
  PROFILE=""
  case "${SHELL:-}" in
    */bash) PROFILE="$HOME/.bashrc";;
    */zsh)  PROFILE="$HOME/.zshrc";;
  esac
  if [ -z "$PROFILE" ]; then
    for candidate in "$HOME/.bashrc" "$HOME/.zshrc" "$HOME/.profile"; do
      [ -f "$candidate" ] && { PROFILE="$candidate"; break; }
    done
  fi
  [ -n "$PROFILE" ] || PROFILE="$HOME/.bashrc"
  touch "$PROFILE"
  if ! grep -qsF "$INSTALL_DIR" "$PROFILE"; then
    printf '\nexport PATH="%s:$PATH"\n' "$INSTALL_DIR" >> "$PROFILE"
    info "Added ${INSTALL_DIR} to PATH in ${PROFILE}"
  fi
  export PATH="${INSTALL_DIR}:$PATH"
fi

ok "Installed $( ${BINARY} version 2>/dev/null || echo "${BINARY}" )"
info "Binary: ${BINARY}"
info "Restart your shell or run: source ${PROFILE:-$HOME/.bashrc}"
