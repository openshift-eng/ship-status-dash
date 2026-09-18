#!/bin/bash
set -euo pipefail

# Ship Status Dashboard environment setup for TRT agentic CI workflows.
# Called by the generic workflow after the workspace has been initialized.

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

export SHIP_STATUS_DSN="postgres://postgres:password@localhost:5433/ship_status?sslmode=disable"

echo "Starting services..."
"${REPO_ROOT}/.devcontainer/init-services.sh"

echo "Running post-create setup..."
"${REPO_ROOT}/.devcontainer/post-create.sh"
