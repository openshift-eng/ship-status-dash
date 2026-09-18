Extra context for agentic review responses, layered on `/openshift-developer:address-review-pr`.

## Verify and Push

1. Run `make test` and `make lint`.
2. Run `make mcp-test` when MCP code changes.
3. Run `make local-e2e` whenever it is useful to verify full service
   integration. Avoid rerunning it unless relevant changes were made after the
   previous run.
4. For frontend changes, use Playwright MCP tools.
5. Commit fixes referencing the review feedback.
6. Push: `git push fork HEAD` (or `git push origin HEAD`).

## Environment

- PostgreSQL is available at localhost:5433 (user: `postgres`, password:
  `password`, database: `ship_status`).

## Additional Instructions

- Keep mutating API endpoints on the protected route. The public route must
  remain read-only.
- Preserve audit logging for outage modifications.
