# Valet

Local development reverse proxy with trusted HTTPS, DNS management, and an AI assistant.

## Features

- Embedded Caddy reverse proxy with zero-downtime config reloads
- Trusted local HTTPS via mkcert (no browser warnings)
- Local DNS server with custom domain/TLD support
- Route templates (SPA+API, WebSocket, CORS, load-balanced)
- Desktop GUI (Wails + Svelte 5) with theme support
- AI assistant powered by ADK (Ollama, OpenAI-compatible)
- MCP server for Claude Code integration
- Real-time metrics dashboard with Chart.js
- DNS query and HTTP access log viewer
- A/CNAME record support for DNS entries

## Prerequisites

- Go 1.26+
- Wails v2 (for the desktop app)
- Node.js (for the frontend build)
- mkcert (`brew install mkcert && mkcert -install`)

## Quick Start

```bash
# Build everything
make build

# Start the daemon
bin/valetd

# Register a TLD with DNS resolver
sudo bin/valetd tld add --tld test

# Add a route
bin/valet add myapp.test localhost:3000

# Open https://myapp.test in your browser
```

## Architecture

This is a monorepo with three modules:

- **`pkg/`** — shared libraries (Caddy manager, DNS server, mkcert integration, config store)
- **`valetd/`** — daemon (`valetd`) and CLI (`valet`) binaries, REST API, MCP server, AI assistant
- **`valetapp/`** — desktop GUI built with Wails + Svelte 5

The daemon manages an embedded Caddy reverse proxy and a DNS server. Configuration is stored in `~/.valet/valet.db` (SQLite) and certificates live in `~/.valet/certs/`.

## GUI

Run the desktop app in development mode:

```bash
make dev
```

This launches the Wails dev server with hot-reload for the Svelte frontend.

## MCP Integration

Valet includes an MCP server so Claude Code can manage routes, TLDs, and DNS records directly. Add this to your Claude Code MCP config:

```json
{
  "mcpServers": {
    "valet": {
      "command": "valetd",
      "args": ["mcp"]
    }
  }
}
```

## DNS

Valet runs a local DNS server on port 15353. When you register a TLD (e.g., `test`), Valet installs a macOS resolver file (`/etc/resolver/test`) that directs lookups for `*.test` to the local DNS server.

**macOS limitation:** the resolver system only supports a single subdomain level. `myapp.test` works; `api.myapp.test` does not resolve through `/etc/resolver`.

## License

MIT License. See [LICENSE.md](LICENSE.md) for details.
