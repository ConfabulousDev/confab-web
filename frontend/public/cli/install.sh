#!/bin/bash
set -e

# Confab CLI Installer
# Usage: curl -fsSL https://confabulous.dev/cli/install.sh | bash

BINARY_NAME="confab"
GITHUB_REPO="ConfabulousDev/confab"
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

    echo "${os}_${arch}"
}

# Download a file using curl or wget
download() {
    local url="$1"
    local output="$2"
    local auth_header=""

    if [ -n "$CONFAB_GITHUB_TOKEN" ]; then
        auth_header="Authorization: Bearer $CONFAB_GITHUB_TOKEN"
    fi

    if command -v curl >/dev/null 2>&1; then
        if [ -n "$auth_header" ]; then
            curl -fsSL -H "$auth_header" "$url" -o "$output"
        else
            curl -fsSL "$url" -o "$output"
        fi
    elif command -v wget >/dev/null 2>&1; then
        if [ -n "$auth_header" ]; then
            wget -q --header="$auth_header" "$url" -O "$output"
        else
            wget -q "$url" -O "$output"
        fi
    else
        echo "Error: curl or wget is required"
        exit 1
    fi
}

# Fetch content to stdout
fetch() {
    local url="$1"
    local auth_header=""

    if [ -n "$CONFAB_GITHUB_TOKEN" ]; then
        auth_header="Authorization: Bearer $CONFAB_GITHUB_TOKEN"
    fi

    if command -v curl >/dev/null 2>&1; then
        if [ -n "$auth_header" ]; then
            curl -fsSL -H "$auth_header" "$url"
        else
            curl -fsSL "$url"
        fi
    elif command -v wget >/dev/null 2>&1; then
        if [ -n "$auth_header" ]; then
            wget -qO- --header="$auth_header" "$url"
        else
            wget -qO- "$url"
        fi
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
    local platform version archive_name archive_url checksum tmp_dir tmp_file

    platform="$(detect_platform)"
    echo "Installing confab for ${platform}..."

    # Get the latest version from GitHub releases
    echo "Fetching latest version..."
    version="$(fetch "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"tag_name": *"v?([^"]+)".*/\1/')"
    if [ -z "$version" ]; then
        echo "Error: Failed to determine latest version"
        exit 1
    fi
    echo "Latest version: ${version}"

    # Create temp directory
    tmp_dir="$(mktemp -d)"
    tmp_file="${tmp_dir}/${BINARY_NAME}"
    trap 'rm -rf "$tmp_dir"' EXIT

    # Download archive from GitHub releases
    archive_name="${BINARY_NAME}_${version}_${platform}.tar.gz"
    archive_url="${RELEASES_URL}/download/v${version}/${archive_name}"
    echo "Downloading ${archive_url}..."
    download "$archive_url" "${tmp_dir}/${archive_name}"

    # Download checksums file and extract checksum for our archive
    checksums_url="${RELEASES_URL}/download/v${version}/checksums.txt"
    checksum="$(fetch "$checksums_url" | grep "${archive_name}" | cut -d' ' -f1)"

    if ! echo "$checksum" | grep -qE '^[a-fA-F0-9]{64}$'; then
        echo "Error: Failed to get checksum for ${archive_name}"
        exit 1
    fi

    # Verify checksum
    echo "Verifying checksum..."
    verify_checksum "${tmp_dir}/${archive_name}" "$checksum"

    # Extract the binary from the archive
    echo "Extracting..."
    tar -xzf "${tmp_dir}/${archive_name}" -C "$tmp_dir"

    # Run the binary's install command
    chmod +x "$tmp_file"
    "$tmp_file" install
}

main "$@"
