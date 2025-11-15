#!/bin/bash
set -e

BIN_DIR="$HOME/.local/bin"
BIN_PATH="$BIN_DIR/confab"

echo "=== Installing Confab ==="
echo

# Build the binary
echo "Building confab..."
go build -o confab

# Install to ~/.local/bin
echo "Installing to $BIN_DIR..."
mkdir -p "$BIN_DIR"
cp confab "$BIN_PATH"
chmod +x "$BIN_PATH"

echo "✓ Confab installed to $BIN_PATH"
echo

# Check if ~/.local/bin is in PATH
if [[ ":$PATH:" != *":$BIN_DIR:"* ]]; then
  echo "⚠️  Warning: $BIN_DIR is not in your PATH"
  echo "Add this to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
  echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
  echo
fi

# Run init
echo "Initializing confab..."
"$BIN_PATH" init

echo
echo "=== Installation Complete ==="
echo "Confab is now ready to capture your Claude Code sessions."
