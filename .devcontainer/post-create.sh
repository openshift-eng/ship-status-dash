#!/bin/bash
set -eu

echo "==> Installing Go IDE tools..."
go install golang.org/x/tools/gopls@v0.18.1
go install github.com/go-delve/delve/cmd/dlv@v1.24.2
go install honnef.co/go/tools/cmd/staticcheck@v0.6.1

echo "==> Downloading Go module dependencies..."
go mod download

echo "==> Installing frontend dependencies..."
make npm

echo "==> Removing legacy Vite cache (host/container use separate node_modules/.vite-* dirs)..."
rm -rf frontend/node_modules/.vite

echo "==> Setting up MCP server venvs (ship-status + ship-status-dev)..."
for mcp_dir in mcp ship-status-dev; do
  rm -rf "${mcp_dir}/.venv"
  python3.12 -m venv "${mcp_dir}/.venv"
  "${mcp_dir}/.venv/bin/pip" install --upgrade pip -q
  "${mcp_dir}/.venv/bin/pip" install -r "${mcp_dir}/requirements-dev.txt" -q
done

echo "==> Pinning APM CLI (used by: make apm)..."
uv tool install apm-cli==0.11.0

echo "==> Running database migrations..."
SHIP_STATUS_DSN="${SHIP_STATUS_DSN:-postgres://postgres:password@localhost:5433/ship_status?sslmode=disable}"
go run ./cmd/migrate --dsn "$SHIP_STATUS_DSN"

echo "==> Dev environment ready."
