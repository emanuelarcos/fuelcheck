#!/bin/sh
set -e

# fuelcheck installer
# Usage: curl -fsSL https://raw.githubusercontent.com/emanuelarcos/fuelcheck/main/install.sh | sh

REPO="emanuelarcos/fuelcheck"
BINARY="fuelcheck"
INSTALL_DIR="/usr/local/bin"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
DIM='\033[0;90m'
BOLD='\033[1m'
RESET='\033[0m'

info() {
    printf "${CYAN}==>${RESET} %s\n" "$1"
}

success() {
    printf "${GREEN}==>${RESET} %s\n" "$1"
}

error() {
    printf "${RED}error:${RESET} %s\n" "$1" >&2
    exit 1
}

# Detect OS
detect_os() {
    case "$(uname -s)" in
        Linux*)  echo "linux" ;;
        Darwin*) echo "darwin" ;;
        *)       error "Unsupported OS: $(uname -s). Only Linux and macOS are supported." ;;
    esac
}

# Detect architecture
detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)  echo "amd64" ;;
        arm64|aarch64) echo "arm64" ;;
        *)             error "Unsupported architecture: $(uname -m). Only amd64 and arm64 are supported." ;;
    esac
}

# Get latest release tag from GitHub API
get_latest_version() {
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | \
            grep '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/'
    elif command -v wget >/dev/null 2>&1; then
        wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | \
            grep '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/'
    else
        error "curl or wget is required to download fuelcheck."
    fi
}

# Download a URL to a file
download() {
    url="$1"
    output="$2"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$url" -o "$output"
    elif command -v wget >/dev/null 2>&1; then
        wget -qO "$output" "$url"
    fi
}

main() {
    printf "${CYAN}"
    printf '  (  ┏━╸╻ ╻┏━╸╻  ┏━╸╻ ╻┏━╸┏━╸╻┏\n'
    printf ' )\\) ┣╸ ┃ ┃┣╸ ┃  ┃  ┣━┫┣╸ ┃  ┣┻┓\n'
    printf '((_) ╹  ┗━┛┗━╸┗━╸┗━╸╹ ╹┗━╸┗━╸╹ ╹\n'
    printf "${RESET}"
    printf "${DIM}            installer${RESET}\n"
    echo ""

    OS=$(detect_os)
    ARCH=$(detect_arch)

    info "Detected platform: ${BOLD}${OS}/${ARCH}${RESET}"

    # Get version (from arg or latest)
    VERSION="${1:-}"
    if [ -z "$VERSION" ]; then
        info "Fetching latest release..."
        VERSION=$(get_latest_version)
        if [ -z "$VERSION" ]; then
            error "Could not determine latest version. Check https://github.com/${REPO}/releases"
        fi
    fi

    info "Installing fuelcheck ${BOLD}${VERSION}${RESET}"

    # Build download URL
    # GoReleaser naming convention: fuelcheck_<os>_<arch>.tar.gz
    ARCHIVE="${BINARY}_${OS}_${ARCH}.tar.gz"
    URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"

    # Download to temp dir
    TMP_DIR=$(mktemp -d)
    trap 'rm -rf "$TMP_DIR"' EXIT

    info "Downloading ${DIM}${URL}${RESET}"
    download "$URL" "${TMP_DIR}/${ARCHIVE}" || error "Download failed. Check that version ${VERSION} exists at:\n  https://github.com/${REPO}/releases"

    # Extract
    info "Extracting..."
    tar -xzf "${TMP_DIR}/${ARCHIVE}" -C "$TMP_DIR"

    # Find the binary (goreleaser puts it at root of the tar)
    if [ ! -f "${TMP_DIR}/${BINARY}" ]; then
        error "Binary not found in archive. The release may be corrupted."
    fi

    chmod +x "${TMP_DIR}/${BINARY}"

    # Install
    if [ -w "$INSTALL_DIR" ]; then
        mv "${TMP_DIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
    else
        info "Need sudo to install to ${INSTALL_DIR}"
        sudo mv "${TMP_DIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
    fi

    # Verify
    if command -v fuelcheck >/dev/null 2>&1; then
        INSTALLED_VERSION=$(fuelcheck --version 2>&1 | awk '{print $NF}')
        echo ""
        success "fuelcheck ${BOLD}${INSTALLED_VERSION}${RESET} installed to ${INSTALL_DIR}/${BINARY}"
        echo ""
        printf "${DIM}  Run 'fuelcheck' to get started.${RESET}\n"
        printf "${DIM}  Run 'fuelcheck --help' for usage info.${RESET}\n"
    else
        echo ""
        success "fuelcheck installed to ${INSTALL_DIR}/${BINARY}"
        printf "\n${DIM}  Make sure ${INSTALL_DIR} is in your PATH.${RESET}\n"
    fi
}

main "$@"
