---
description: "Run frontend BDD tests only"
---

# Ship Status dev — bdd

Run frontend BDD tests directly:

```bash
make bdd
```

This installs frontend dependencies (`npm ci`) and runs Playwright BDD tests against a mocked API (no backend required). Tests use feature files in `frontend/bdd/features/` with step definitions in `frontend/bdd/steps/`.

## Presenting results

After the run completes, read the full output and present every test with its result and timing. Format as a list:

- ✓ TestName (1.23s)
- ✗ TestName (0.45s)

Show all tests, not just failures. Include the total pass/fail count and overall duration at the end.
