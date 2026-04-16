# Valet DNS

## How It Works

Valet runs a local DNS server on **port 15353**. It resolves domains that have active routes to `127.0.0.1` and forwards everything else to an upstream resolver (default: `8.8.8.8`). This means only domains you have explicitly configured in Valet resolve locally — all other DNS traffic passes through normally.

## Registering TLDs

A TLD (top-level domain) tells macOS to send DNS queries for that domain suffix to Valet's DNS server.

```bash
sudo valetd tld add --tld test
```

This creates a resolver file at `/etc/resolver/test` containing:

```
nameserver 127.0.0.1
port 15353
```

macOS reads `/etc/resolver/<domain>` files and routes matching DNS queries to the specified nameserver. After adding a TLD, any `*.test` lookup will hit Valet's DNS server.

### Custom Domains

You can also register real domain suffixes for local development:

```bash
sudo valetd tld add --tld example.com
```

This creates `/etc/resolver/example.com` so that `*.example.com` resolves through Valet locally.

## DNS Entries

Once a TLD is registered, you create DNS entries by adding routes (via the GUI or API). Each route maps a subdomain within a TLD to a local service.

For example, adding a route for `myapp.test` pointing to `localhost:3000` creates a DNS entry that resolves `myapp.test` to `127.0.0.1`.

### A Records vs CNAME Records

- **A records** map a domain directly to an IP address (e.g., `myapp.test` -> `127.0.0.1`). This is the default for Valet routes.
- **CNAME records** map a domain to another domain name. Valet supports these for advanced configurations where you need alias-style resolution.

## Route-Aware DNS

Valet's DNS server is route-aware:

- If a queried domain matches an active route, it responds with `127.0.0.1`.
- If a queried domain falls under a registered TLD but has no route, the query is forwarded to the upstream DNS server (`8.8.8.8`).

This prevents Valet from hijacking domains you haven't explicitly configured.

## macOS Limitations

The `/etc/resolver` mechanism in macOS only supports a **single subdomain level**. This means:

- `myapp.test` works (one level under the `.test` TLD).
- `api.myapp.test` also works (macOS resolver matches the TLD suffix).
- However, more complex resolver hierarchies are not supported — you cannot have different resolver behavior for `api.myapp.test` vs `web.myapp.test` at the resolver file level. Valet handles this at the application layer by matching the full domain against registered routes.
