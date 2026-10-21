# yc-platform-mcp

MCP server that exposes the yc-platform microservice OpenAPI documents to Claude and Cursor. Each service keeps its contract at `services/<name>/api/openapi.yaml` in the platform repo. This server loads those documents (copied here as `specs/<service>.yaml`) and answers questions about endpoints, schemas, auth, and example requests.

The default transport is stdio, which Claude Desktop and Cursor both speak. `MCP_TRANSPORT=http` switches the same server to the streamable HTTP transport.

## Prerequisites

- Go 1.26 or newer. `github.com/pb33f/libopenapi` v0.41.6 requires Go 1.26, and the MCP Go SDK v1.8.0 requires Go 1.25. This module was planned against a Go 1.22 floor; those libraries raise it. The `go` line in `go.mod` is `1.26.0`.
- The specs in `specs/`, or your own OpenAPI 3.0/3.1 file or directory.

## Build

```bash
make build
```

The binary is `./bin/yc-platform-mcp`.

## Configure

`OPENAPI_SPEC_PATH` is a file or a directory. A directory loads every `.json`, `.yaml`, and `.yml` file in that directory (not nested folders). The default is `./specs`. The service name is the file name (`yc-product.yaml` becomes `yc-product`). A file named `openapi.yaml` uses its parent directory name instead.

Copy `.env.example` to `.env` for local runs. The server logs to stderr. Stdout is reserved for MCP messages.

## Claude Desktop

Build the binary, then merge `claude_desktop_config.example.json` into Claude Desktop's config (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS). The example points at this checkout:

```json
{
  "mcpServers": {
    "ecommerce-docs": {
      "command": "/Users/slaghuis/code/services/yc-platform-mcp/bin/yc-platform-mcp",
      "env": {
        "OPENAPI_SPEC_PATH": "/Users/slaghuis/code/services/yc-platform-mcp/specs"
      }
    }
  }
}
```

Restart Claude Desktop after saving. A single spec works too: set `OPENAPI_SPEC_PATH` to that file.

## Cursor Desktop

Use the same command and environment. In Cursor, open Settings → MCP and add a server, or put the `mcpServers` object in `~/.cursor/mcp.json`. Point `command` at `bin/yc-platform-mcp` and `OPENAPI_SPEC_PATH` at the `specs` directory.

## Run locally

```bash
make run
```

That builds the binary and starts stdio mode with `./specs`. The process waits for an MCP client on stdin. To check that the documents load before a client connects:

```bash
make build
OPENAPI_SPEC_PATH=./specs ./bin/yc-platform-mcp
```

A missing or malformed spec logs the error to stderr and exits 1.

Set `MCP_TRANSPORT=http` and `MCP_HTTP_ADDR=127.0.0.1:8080` when you want streamable HTTP instead of stdio.

## Test

```bash
make test
```

`make lint` runs `gofmt` and `go vet`. `make tidy` refreshes `go.mod`. `make clean` removes `bin/` and the coverage profile.

## Tools

| Tool | What it answers |
|---|---|
| `list_services` | Microservices, tags, versions, endpoint counts |
| `search_endpoints` | Operations matching a keyword, optionally one service or tag |
| `get_endpoint_details` | Parameters, body, responses, and security for one operation |
| `list_schemas` | Schema names and short descriptions |
| `get_schema` | A schema with resolved `$ref` properties |
| `get_auth_info` | Security schemes and global requirements |
| `get_example_request` | A curl, Go, or JavaScript request (`curl` is the default) |
| `list_error_responses` | 4xx, 5xx, and default responses |
| `get_api_overview` | Title, version, description, base URL, endpoint count |

Resources: `api://overview`, `api://endpoints`, `api://schemas`.

## Questions Claude can answer

- Which services are in yc-platform, and how many endpoints does yc-cart have?
- How do I add an item to a cart? Show the request body and a curl example.
- What fields are on the Product schema in yc-product?
- What does yc-pricing return when a price evaluation fails?
- Which operations mention inventory?
