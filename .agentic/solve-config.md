## Write Tests for New Code

You MUST write tests for any new functionality you introduce. PRs that add new
code without corresponding tests are incomplete.

### Go (backend)

- Write unit tests for new exported functions and non-trivial logic.
- Use table-driven tests with descriptive case names. Search the same package
  for existing test patterns before writing new ones.
- Place test files next to the code they test.

### React (frontend)

Add or update BDD coverage for user-visible behavior under `frontend/`.
Follow the existing Playwright test patterns.

### MCP servers

Add pytest coverage for changes under `mcp/` or `ship-status-dev/`.

## Build, Test, and Verify

1. Run `make test` to verify your changes work.
2. Run `make mcp-test` when MCP code changes.
3. Run `make lint` to check for linting issues.

## Test Locally

Use the `ship-status-dev` MCP tools:

- `dashboard_serve` starts the API and mock OAuth proxy.
- `frontend_start` starts the React frontend.
- `run_migrate` applies database migrations.
- `component_monitor_start` runs the component monitor. Start the dashboard with
  `dashboard_serve` first, and ensure a local `prometheus` binary is available
  on `PATH`.
- `run_tests` runs lint and then unit tests. If lint fails, it skips the test
  phase. Fix the lint failure and rerun `run_tests` to complete verification.

For frontend changes, use Playwright MCP tools.

Run `make local-e2e` whenever it is useful to verify full service integration.
Avoid rerunning it unless relevant changes were made after the previous run.

## Environment

PostgreSQL is available at localhost:5433 (user: `postgres`, password:
`password`, database: `ship_status`).
