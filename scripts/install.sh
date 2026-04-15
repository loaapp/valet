#!/bin/bash
set -e

echo "Installing Valet..."

# Check prerequisites
command -v go >/dev/null 2>&1 || { echo "Go is required. Install from https://go.dev"; exit 1; }
command -v mkcert >/dev/null 2>&1 || { echo "mkcert is required. Run: brew install mkcert && mkcert -install"; exit 1; }

# Build
make daemon cli

# Install binaries to /usr/local/bin
sudo cp bin/valetd /usr/local/bin/valetd
sudo cp bin/valet /usr/local/bin/valet

echo "Installed valetd and valet to /usr/local/bin/"
echo ""
echo "Next steps:"
echo "  1. Start the daemon: valetd"
echo "  2. Register a TLD:   sudo valetd tld add --tld test"
echo "  3. Add a route:      valet add myapp.test localhost:3000"
echo "  4. Visit:            https://myapp.test"
