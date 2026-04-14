# Valet

Local development reverse proxy manager. Provides trusted HTTPS on custom
domain names routed to local services, all on port 443.

## Features

- **Embedded Caddy** reverse proxy with zero-downtime config reloads
- **mkcert** integration for locally-trusted SSL certificates
- **Local DNS** server for managed TLDs (e.g., `*.test`)
- **`/etc/hosts`** management for arbitrary domain names
- **REST API** for programmatic control
- **CLI** for quick route management

## Quick Start

```bash
# Build
go build -o bin/valetd ./cmd/valetd
go build -o bin/valet ./cmd/valet

# Start the daemon
valet up

# One-time trust setup (configures DNS resolver, requires sudo)
valet trust

# Register a TLD for automatic DNS
valet tld add test

# Add a route
valet add myapp.test localhost:3000

# Visit https://myapp.test in your browser
```

## Architecture

Valet consists of two binaries:

- **`valetd`** — daemon that runs an embedded Caddy reverse proxy, DNS server,
  and REST API
- **`valet`** — CLI client that communicates with `valetd` via REST API

Configuration is stored in `~/.valet/valet.db` (SQLite). Certificates are
stored in `~/.valet/certs/`.

## License

MIT License. See [LICENSE.md](LICENSE.md) for details.
