.PHONY: build e2e test local-dashboard-dev local-component-monitor-dev lint npm build-dashboard build-frontend build-component-monitor component-monitor-dry-run

build: build-frontend build-dashboard

local-e2e:
	@./test/e2e/scripts/local-e2e.sh

test:
	@echo "Running unit tests..."
	@gotestsum -- ./pkg/... ./cmd/... -v

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
	REACT_APP_PUBLIC_DOMAIN=https://ship-status.ci.openshift.org \
	REACT_APP_PROTECTED_DOMAIN=https://protected.ship-status.ci.openshift.org \
	npm run build

build-component-monitor:
	@go build -mod=vendor -o component-monitor ./cmd/component-monitor

component-monitor-dry-run:
	@./hack/component-monitor-dry-run/create-job.sh