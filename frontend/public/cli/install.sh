#!/bin/bash
set -e

# Confab CLI Installer
# Usage: curl -fsSL https://confabulous.dev/cli/install.sh | bash

BINARY_NAME="confab"
GITHUB_REPO="ConfabulousDev/confab-cli"
RELEASES_URL="https://github.com/${GITHUB_REPO}/releases"

# Detect OS and architecture
detect_platform() {
    local os arch

    os="$(uname -s)"
    arch="$(uname -m)"

    case "$os" in
        Darwin) os="darwin" ;;
        Linux) os="linux" ;;
        *)
            echo "Error: Unsupported operating system: $os"
            exit 1
            ;;
    esac

    case "$arch" in
        x86_64|amd64) arch="amd64" ;;
        arm64|aarch64) arch="arm64" ;;
        *)
            echo "Error: Unsupported architecture: $arch"
            exit 1
            ;;
    esac

    echo "${os}-${arch}"
}

# Download a file using curl or wget
download() {
    local url="$1"
    local output="$2"

    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$url" -o "$output"
    elif command -v wget >/dev/null 2>&1; then
        wget -q "$url" -O "$output"
    else
        echo "Error: curl or wget is required"
        exit 1
    fi
}

# Fetch content to stdout
fetch() {
    local url="$1"

    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$url"
    elif command -v wget >/dev/null 2>&1; then
        wget -qO- "$url"
    else
        echo "Error: curl or wget is required"
        exit 1
    fi
}

# Verify SHA256 checksum
verify_checksum() {
    local file="$1"
    local expected="$2"
    local actual

    if command -v sha256sum >/dev/null 2>&1; then
        actual="$(sha256sum "$file" | cut -d' ' -f1)"
    elif command -v shasum >/dev/null 2>&1; then
        actual="$(shasum -a 256 "$file" | cut -d' ' -f1)"
    else
        echo "Warning: No checksum tool found, skipping verification"
        return 0
    fi

    if [ "$actual" != "$expected" ]; then
        echo "Error: Checksum verification failed"
        echo "  Expected: $expected"
        echo "  Actual:   $actual"
        return 1
    fi
}

main() {
    local platform binary_url checksum_url checksum tmp_dir tmp_file

    platform="$(detect_platform)"
    echo "Installing confab for ${platform}..."

    # Create temp directory
    tmp_dir="$(mktemp -d)"
    tmp_file="${tmp_dir}/${BINARY_NAME}"
    trap 'rm -rf "$tmp_dir"' EXIT

    # Download binary from GitHub releases
    binary_url="${RELEASES_URL}/latest/download/${BINARY_NAME}-${platform}"
    echo "Downloading ${binary_url}..."
    download "$binary_url" "$tmp_file"

    # Download checksums file and extract checksum for our binary
    checksums_url="${RELEASES_URL}/latest/download/checksums.txt"
    binary_name="${BINARY_NAME}-${platform}"
    checksum="$(fetch "$checksums_url" | grep "${binary_name}$" | cut -d' ' -f1)"

    if ! echo "$checksum" | grep -qE '^[a-fA-F0-9]{64}$'; then
        echo "Error: Failed to get checksum for ${binary_name}"
        exit 1
    fi

    # Verify checksum
    echo "Verifying checksum..."
    verify_checksum "$tmp_file" "$checksum"

    # Run the binary's install command
    chmod +x "$tmp_file"
    "$tmp_file" install
}

main "$@"
