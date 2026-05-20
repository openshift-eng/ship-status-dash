.PHONY: build e2e test mcp-test local-dashboard-dev local-component-monitor-dev lint npm build-dashboard build-frontend build-component-monitor component-monitor-dry-run apm verify-apm

build: build-frontend build-dashboard

local-e2e:
	@./test/e2e/scripts/local-e2e.sh

test:
	@echo "Running unit tests..."
	@gotestsum -- ./pkg/... ./cmd/... -v

mcp-test:
	@cd mcp && (test -x .venv/bin/pytest || (python3.12 -m venv .venv 2>/dev/null || python3 -m venv .venv) && .venv/bin/pip install -q -r requirements-dev.txt) && .venv/bin/pytest test_api_client.py -q

lint: npm
	@./hack/go-lint.sh --timeout 10m run ./...
	@cd frontend && npm run lint
	@cd frontend && npm audit --omit=dev

npm:
	@cd frontend && npm ci --no-audit --ignore-scripts

build-dashboard:
	@go build -mod=vendor -o dashboard ./cmd/dashboard

build-frontend: npm
	@cd frontend && \
	VITE_PUBLIC_DOMAIN=https://ship-status.ci.openshift.org \
	VITE_PROTECTED_DOMAIN=https://protected.ship-status.ci.openshift.org \
	npm run build

build-component-monitor:
	@go build -mod=vendor -o component-monitor ./cmd/component-monitor

component-monitor-dry-run:
	@./hack/component-monitor-dry-run/create-job.sh

apm:
	@command -v uvx >/dev/null || (echo "uvx not found; install uv (see .devcontainer/Dockerfile)" >&2 && exit 1)
	uvx --from apm-cli@0.11.0 apm install
	uvx --from apm-cli@0.11.0 apm compile

verify-apm: apm
	@if ! git diff --quiet HEAD -- .apm apm.lock.yaml .claude .cursor .gemini .opencode AGENTS.md CLAUDE.md GEMINI.md frontend/AGENTS.md frontend/CLAUDE.md mcp/AGENTS.md mcp/CLAUDE.md ship-status-dev/AGENTS.md ship-status-dev/CLAUDE.md; then \
		echo "ERROR: Generated APM files are out of date. Run 'make apm' and commit the results."; \
		git diff --stat HEAD -- .apm apm.lock.yaml .claude .cursor .gemini .opencode AGENTS.md CLAUDE.md GEMINI.md frontend/AGENTS.md frontend/CLAUDE.md mcp/AGENTS.md mcp/CLAUDE.md ship-status-dev/AGENTS.md ship-status-dev/CLAUDE.md; \
		exit 1; \
	fi