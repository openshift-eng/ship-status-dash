---
description: "MCP server (ship-status) for SHIP Dashboard REST API tools"
applyTo: "mcp/**"
---

MCP server **`ship-status`** wraps the dashboard REST API for AI agents. Code lives in `mcp/api_client.py` and `mcp/api_server.py`.

- Local config: [`.mcp.json`](../../.mcp.json) → `mcp/run.sh`
- Env: `SHIP_STATUS_PUBLIC_API_URL`, `SHIP_STATUS_PROTECTED_API_URL`
- Tests: `mcp/.venv/bin/pytest mcp/` (install `mcp/requirements-dev.txt` first)

There are two MCP server modes, controlled by `MCP_MODE`:

- **Public** (`MCP_MODE=public`): Read-only tools, accepts unauthenticated traffic. This instance is stateless and MUST NOT hold credentials.
- **Authenticated** (`MCP_MODE=authenticated`): Write tools, sits behind oauth-proxy. This instance holds its own SA token to call the dashboard's protected API. Only authenticated callers can reach it.

See `security.instructions.md` for the full auth model and credential placement rules.

Do not add dev workflow tools here -- use **`ship-status-dev`** (`ship-status-dev/`).
