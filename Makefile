.PHONY: build e2e test local-dev lint npm build-dashboard build-frontend

build: build-frontend build-dashboard

e2e:
	@./hack/e2e.sh

local-dev:
	@./hack/local/local-dev.sh $(DSN)

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
