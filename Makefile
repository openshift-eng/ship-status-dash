.PHONY: run-dashboard e2e test local-dev lint

run-dashboard:
	@./hack/run-dashboard.sh

e2e:
	@./hack/e2e.sh

local-dev:
	@./hack/local/local-dev.sh $(DSN)

test:
	@echo "Running unit tests..."
	# Run tests in all packages except the test package, this is where the e2e tests are located
	@go test $(shell go list ./... | grep -v '/test/') -v

lint:
	@./hack/go-lint.sh --timeout 10m run ./...
	@cd frontend && npm run lint
	@cd frontend && npm audit --omit=dev
