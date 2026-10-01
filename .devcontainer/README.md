# Ship Status Dashboard — Dev Container

This is the recommended development environment. See [DEVELOPMENT.md](../DEVELOPMENT.md) for the full guide; manual (non-container) setup is documented there as optional.

## Quick Start

Use the `/ship-status-dev-setup` slash command in Claude Code or Cursor to set up automatically.

## Prerequisites

- **Podman v4+** or **Docker** with Compose
- **devcontainer CLI**: `npm install -g @devcontainers/cli`

## Services

PostgreSQL is published by `init-services.sh`. The devcontainer publishes 3030, 8180, 8443, and 9090 on the host loopback (`127.0.0.1`). The browser opens the mock oauth proxy at `http://localhost:8443` directly, so editor port forwarding is turned off for these ports. Recreate the devcontainer after changing the published ports.

| Service | Container | Port | Notes |
|---------|-----------|------|-------|
| PostgreSQL | ship-status-postgres | 5433 | Auto-started by init-services.sh (host port 5433 → container 5432) |
| Dashboard API | devcontainer | 8180 | Published on the host. Start with `/ship-status-dev-serve` |
| Mock OAuth Proxy | devcontainer | 8443 | Published on the host. Started alongside the dashboard |
| Vite Dev Server | devcontainer | 3030 | Published on the host. Start with `/ship-status-dev-frontend` |
| Prometheus | devcontainer | 9090 | Published on the host. Started by `/ship-status-dev-app` |

## Manual Setup

### macOS (Podman)

```bash
podman machine init   # first time only
podman machine start
devcontainer up --workspace-folder . --docker-path podman
```

### macOS (Docker Desktop)

```bash
devcontainer up --workspace-folder .
```

### Linux (Podman)

```bash
systemctl --user enable --now podman.socket
devcontainer up --workspace-folder . --docker-path podman
```

### Linux (Docker)

```bash
devcontainer up --workspace-folder .
```

## Environment

Copy `.devcontainer/.env.example` to `.devcontainer/.env` and fill in any blank values.

The image includes **uv** / **uvx** and a pinned **apm-cli** so `make apm` and `make verify-apm` work without extra setup. MCP Python venvs for `mcp/` and `ship-status-dev/` are created in `post-create.sh`.

## GitHub CLI Authentication

The GitHub CLI (`gh`) is included in the image. Its configuration is mounted read-only from the host's `~/.config/gh`. By default, `gh auth login` may store tokens in the host's secure credential store, such as macOS Keychain or a Linux keyring. The devcontainer cannot access credentials stored there. To make the token available through the mounted configuration, authenticate on the host using file-based storage:

```bash
gh auth login --insecure-storage
```

This stores the token in plain text in `~/.config/gh/hosts.yml`. The read-only mount prevents changes from the devcontainer, but processes in the devcontainer can still read and exfiltrate the token. Mount credentials only in trusted workspaces, use a least-privilege GitHub identity, protect the file, and do not commit it.

## GCP Authentication

GCP credentials are mounted read-only from the host's `~/.config/gcloud`. Authenticate on the host:

```bash
gcloud auth application-default login
```

## Cleanup

### Podman

```bash
devcontainer down --workspace-folder .
podman stop ship-status-postgres && podman rm ship-status-postgres
podman network rm ship-status-net
```

### Docker

```bash
devcontainer down --workspace-folder .
docker stop ship-status-postgres && docker rm ship-status-postgres
docker network rm ship-status-net
```
