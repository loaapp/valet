# Valet

> Trusted HTTPS for local development. DNS management. AI-powered configuration.

![Dashboard](docs/images/dashboard.png)

## What is Valet?

Valet gives you trusted HTTPS on custom domain names for your local development services — with zero manual configuration. Add a route, get a certificate, hit it in the browser. No more `localhost:3000`.

Manage everything from a native macOS app, the command line, or an AI assistant that can set up your entire dev environment in one conversation.

## Features

### Trusted HTTPS on Any Domain

Route traffic from custom domains to your local services with automatic mkcert certificates. No browser warnings, no self-signed cert hassles.

![Routes](docs/images/routes.png)

### Real-Time Metrics Dashboard

Watch your traffic in real-time with a live Chart.js chart, per-route statistics, and summary metrics. All data persists in SQLite across restarts.

### DNS Management

Register TLDs or override real domains — Valet runs a local DNS server that resolves your routes to localhost and forwards everything else to upstream DNS.

![TLDs & DNS Entries](docs/images/tlds.png)

Register a TLD with one command:

![TLD Registration](docs/images/tld-add.png)

### HTTP & DNS Log Viewer

See every request and DNS query flowing through Valet with live-updating log tables. Filter by route, toggle auto-scroll, clear when needed.

![HTTP Logs](docs/images/logs.png)

![DNS Logs](docs/images/dns-logs.png)

### AI Assistant

An in-app AI assistant powered by ADK that can manage routes, diagnose issues, and configure your entire setup through natural language. Works with Ollama, or any OpenAI-compatible endpoint.

### MCP Server for Claude Code

Valet exposes an MCP server so Claude Code can manage your proxy configuration directly from the terminal.

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

### And more...

- Route templates (SPA+API, WebSocket, CORS, load-balanced)
- 4 themes (macOS Dark, macOS Light, Nord, Rose Pine)
- A/CNAME record support for DNS entries
- Advanced routing with path rules, headers, compression
- Input validation (Zod + Go)
- Caddy config preview before saving

## Install

Download the latest DMG from [Releases](https://github.com/loaapp/valet/releases).

Or build from source:

```bash
make build
```

See [Building from Source](docs/building.md) for prerequisites and details.

## Quick Start

```bash
# Start the daemon
valetd

# Register a TLD with DNS resolver (one-time, requires sudo)
sudo valetd tld add --tld test

# Add a route
valet add myapp.test localhost:3000

# Visit https://myapp.test in your browser
```

## Documentation

- [Building from Source](docs/building.md)
- [DNS Configuration](docs/dns.md)
- [MCP Integration](docs/mcp.md)
- [Contributing](docs/contributing.md)

## License

MIT License. See [LICENSE.md](LICENSE.md) for details.
