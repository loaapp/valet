# Building Valet from Source

## Prerequisites

- **Go 1.26+** — [go.dev/dl](https://go.dev/dl/)
- **Wails v2** — `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- **Node.js 22+** — [nodejs.org](https://nodejs.org/) or `brew install node`
- **mkcert** — `brew install mkcert && mkcert -install` (local TLS certificates)

## Monorepo Structure

Valet is a Go workspace with three modules:

| Directory   | Description                        |
|-------------|------------------------------------|
| `pkg/`      | Shared library code (client, models, etc.) |
| `valetd/`   | Daemon (`valetd`) and CLI (`valet`) |
| `valetapp/` | Wails GUI application              |

The `go.work` file at the repo root links all modules so they resolve locally during development.

## Makefile Targets

| Target       | Description                                      |
|--------------|--------------------------------------------------|
| `make build` | Build everything (daemon, CLI, and app)           |
| `make daemon`| Build `valetd` daemon to `bin/valetd`             |
| `make cli`   | Build `valet` CLI to `bin/valet`                  |
| `make app`   | Build Valet.app with bundled daemon and CLI        |
| `make dev`   | Run Wails dev mode with hot-reload                |
| `make dmg`   | Build app and create DMG installer                |
| `make clean` | Remove all build artifacts                        |
| `make vet`   | Run `go vet` across all modules                   |

## Building Step by Step

1. Clone the repository:
   ```bash
   git clone https://github.com/loaapp/valet.git
   cd valet
   ```

2. Install Go dependencies (handled automatically by the Go workspace):
   ```bash
   go work sync
   ```

3. Install frontend dependencies:
   ```bash
   cd valetapp/frontend && npm install && cd ../..
   ```

4. Build everything:
   ```bash
   make build
   ```

   This produces:
   - `bin/valetd` — the daemon
   - `bin/valet` — the CLI
   - `valetapp/build/bin/Valet.app` — the GUI app (with daemon and CLI bundled inside)

5. To create a distributable DMG:
   ```bash
   brew install create-dmg
   make dmg
   ```

## Go Workspace

The `go.work` file at the repo root links all three modules (`pkg`, `valetd`, `valetapp`). This means:

- Local changes to `pkg/` are immediately visible to `valetd/` and `valetapp/` without publishing.
- `go vet`, `go test`, and other tools work across module boundaries.
- IDE support (gopls) understands the full workspace.
