---
description: "MCP server (ship-status) for SHIP Dashboard REST API tools"
applyTo: "mcp/**"
---

MCP server **`ship-status`** wraps the dashboard REST API for AI agents. Code lives in `mcp/api_client.py` and `mcp/api_server.py`.

- Local config: [`.mcp.json`](../../.mcp.json) → `mcp/run.sh`
- Env: `SHIP_STATUS_PUBLIC_API_URL`, `SHIP_STATUS_PROTECTED_API_URL`
- Tests: `mcp/.venv/bin/pytest mcp/` (install `mcp/requirements-dev.txt` first)

The MCP server is a stateless protocol adapter. It MUST NOT hold or store authentication credentials (tokens, API keys, secrets). For write operations, the caller provides a bearer token which the MCP server forwards unmodified to the dashboard's protected API. The dashboard validates the token via oauth-proxy and enforces authorization. See `security.instructions.md` for the full auth model.

Do not add dev workflow tools here -- use **`ship-status-dev`** (`ship-status-dev/`).
