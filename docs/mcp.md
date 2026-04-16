# Valet MCP Server

## What is MCP?

MCP (Model Context Protocol) lets AI coding assistants interact with local tools through a standardized interface. Valet exposes its route management, DNS, and diagnostics capabilities as MCP tools so that AI assistants like Claude Code can manage your local development environment directly.

## Claude Code Configuration

Add the following to your Claude Code MCP configuration:

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

This launches `valetd mcp` as a subprocess that communicates over stdin/stdout using the MCP protocol.

## Available Tools

Valet exposes 12 MCP tools:

| Tool              | Description                                           |
|-------------------|-------------------------------------------------------|
| `list_routes`     | List all configured routes                            |
| `add_route`       | Add a new route (domain -> local service)             |
| `remove_route`    | Remove an existing route                              |
| `update_route`    | Update an existing route's configuration              |
| `get_status`      | Get the current status of the Valet daemon            |
| `list_tlds`       | List all registered TLDs                              |
| `add_tld`         | Register a new TLD with macOS resolver                |
| `remove_tld`      | Unregister a TLD                                      |
| `list_templates`  | List available route templates                        |
| `preview_route`   | Preview what a route would look like before adding it |
| `diagnose_route`  | Diagnose connectivity issues with a route             |
| `trust`           | Trust a route's TLS certificate                       |

## Example Usage

Once configured, you can ask Claude Code things like:

- "Set up myapp.test pointing to localhost:3000"
- "List all my routes"
- "Why isn't api.test resolving?"
- "Add a route for dashboard.test on port 5173"

Claude Code will use the appropriate MCP tools to fulfill these requests.
