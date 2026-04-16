# Contributing to Valet

## Workflow

1. Fork the repository.
2. Create a feature branch from `main`.
3. Make your changes and open a pull request.

## Development Setup

Run the Wails app in dev mode (hot-reload for the Svelte frontend):

```bash
make dev
```

The daemon needs to run separately for the app to connect to it:

```bash
make daemon
bin/valetd
```

## Code Style

- **Go**: Follow standard Go conventions. Run `make vet` before submitting.
- **Svelte**: The frontend uses Svelte 5 with runes (`$state`, `$derived`, `$effect`). Prefer runes over legacy reactive statements.

## Commit Messages

Use imperative mood and describe the "why," not just the "what":

- Good: "Fix DNS resolver leak when TLD is removed"
- Avoid: "Updated dns.go"
