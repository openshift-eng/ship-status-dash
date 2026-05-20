---
description: "MCP server (ship-status) for SHIP Dashboard REST API tools"
applyTo: "mcp/**"
---

MCP server **`ship-status`** wraps the public (and future protected) dashboard REST API for AI agents. Code lives in `mcp/api_client.py` and `mcp/api_server.py`.

- Local config: [`.mcp.json`](../../.mcp.json) → `mcp/run.sh`
- Env: `SHIP_STATUS_PUBLIC_API_URL`, `SHIP_STATUS_PROTECTED_API_URL`, `SHIP_STATUS_AUTH_TOKEN_FILE` (writes, later)
- Tests: `mcp/.venv/bin/pytest mcp/` (install `mcp/requirements-dev.txt` first)

Do not add dev workflow tools here — use **`ship-status-dev`** (`ship-status-dev/`).
