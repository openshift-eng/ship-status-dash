# TRT-2666: Chai-bot Creates Outages

**Date:** 2026-06-18
**JIRA:** [TRT-2666](https://redhat.atlassian.net/browse/TRT-2666)
**Author:** Arnav Meduri

## Problem Statement

Component maintainers currently have to manually create outages in the SHIP
Status Dashboard through the web UI. This means navigating to the correct
sub-component page, filling in severity/description, and managing the outage
lifecycle (updating severity, adding triage notes, attaching links, resolving, etc.).
Chai-bot currently has no way to create or manage outages in the dashboard.

The goal is to enable chai-bot to:

1. Create outages on behalf of authorized component maintainers (user-initiated)
2. Create unconfirmed outages autonomously when it discovers issues (bot-initiated)
3. Manage the full outage lifecycle via MCP tools

## Current Architecture

### Auth Model

The dashboard pod runs on app.ci with two ingress routes:

- **Public** (`ship-status.ci.openshift.org`) -- unauthenticated read-only
endpoints
- **Protected** (`protected.ship-status.ci.openshift.org`) -- routes through
an oauth-proxy sidecar that validates the caller's token via TokenReview,
sets `X-Forwarded-User`, and signs the request with HMAC
(`GAP-Signature`). The dashboard auth middleware validates the HMAC
signature and extracts the user identity.

### Owner Model

Owners are defined per-component in `dashboard-config.yaml`:

```yaml
owners:
  - rover_group: "test-platform-ci-admins"  # OpenShift Group
  - service_account: "system:serviceaccount:ship-status:component-monitor"
  - user: "developer"                       # Dev/testing only
```

Authorization depends on the type of caller:

- **Human users** are checked by `IsUserAuthorizedForComponent`
(`handlers.go`), which matches the authenticated identity against
`Owner.User` (exact string) or `Owner.RoverGroup` (resolved to individual
members via `GroupMembershipCache` and the OpenShift Groups API). This
function does **not** check `Owner.ServiceAccount`.
- **Service accounts** are checked by `ValidateRequest`
(`component_monitoring_report.go`), which matches
`Owner.ServiceAccount`. This is only used for the component-monitor's
dedicated endpoint (`POST /api/component-monitor/report`).

These paths are completely separate -- a service account cannot pass human
authorization checks, and vice versa.

### Chai-bot (Current State)

Chai-bot runs on a separate cluster (mp-plus) and connects to the dashboard's
MCP endpoint (`mcp.ship-status.ci.openshift.org/mcp`) using `streamable_http`
transport. The MCP endpoint is unauthenticated and currently serves read
tools only (status queries, outage lookups). Chai-bot can resolve Slack user
IDs to Kerberos `uid` via its OrgData tool.

## Proposed Architecture

### Overview

The MCP endpoint is publicly accessible, which is fine for read operations but
means write tools cannot be added to it without exposing unauthenticated write
access. The proposed design splits MCP into two servers:

- A **read-only MCP** (the existing server with mutating tools removed)
remains publicly accessible for status queries and outage lookups.
- An **authenticated MCP** sits behind an oauth-proxy and serves write tools
only. Any caller who can authenticate via the oauth-proxy (chai-bot, other
service accounts, or human users) can reach it. Authorization is enforced
per-request — the dashboard returns 403 if the `acting-for` identity is not
a maintainer of the target component.

The authenticated MCP has its own dedicated SA on app.ci. When it needs to
call the dashboard's protected API, it sends its SA Bearer token through the
dashboard's oauth-proxy, which validates it and signs the request with HMAC.
The MCP cannot simply forward headers from the incoming request because HMAC
covers the full request (method, path, body, headers) — a different request
would invalidate the signature. Instead, the MCP makes its own authenticated
request with `acting-for` in the body, where it is covered by the HMAC
signature.

### Request Flow

There are two protected subdomains, each with its own oauth-proxy:

- `protected-mcp.ship-status.ci.openshift.org` — authenticated MCP (write tools)
- `protected.ship-status.ci.openshift.org` — dashboard API (existing)

Since oauth-proxy only supports [path-based routing](https://github.com/openshift/oauth-proxy/#upstream-configuration)
(not hostname-based), each subdomain requires its own oauth-proxy container,
Route, and Service. Both run in the same pod on app.ci, so the MCP SA only
needs to exist in one cluster.

```
Chai (mp-plus)
  | Bearer: <chai-bot SA token>
  v
oauth-proxy @ protected-mcp.ship-status.ci.openshift.org
  | validates chai-bot token, sets X-Forwarded-User, HMAC signs
  v
Authenticated MCP
  | reads acting-for from tool arguments
  | Bearer: <MCP SA token>
  v
oauth-proxy @ protected.ship-status.ci.openshift.org
  | validates MCP SA token, sets X-Forwarded-User, HMAC signs
  v
Dashboard
  | validates HMAC
  | sees X-Forwarded-User = MCP SA (the authenticated caller)
  | reads acting-for from request body
  | checks acting-for user is authorized for component
  | records discovered_from: "mcp" + acting-for identity in audit log
  v
authorized or 403
```

### acting-for Identity

Every write tool on the authenticated MCP accepts an `acting-for` argument:

- **User-initiated:** Chai-bot resolves the Slack user's Kerberos `uid` and
sends it as `acting-for`. The dashboard checks if that user is authorized
for the target component.
- **Bot-initiated:** Chai-bot sends `"chai-bot"` as `acting-for` when acting
autonomously (e.g., detecting an outage on its own).

The `acting-for` value is included in the request body to the dashboard. Since
HMAC covers the full request body, it cannot be tampered with in transit.
A new `discovered_from` value of `"mcp"` tells the dashboard to record the
`acting-for` identity in audit logs.

### Authorization

Authorization is enforced server-side as part of each write request. The
dashboard sees `X-Forwarded-User` as the MCP SA (proving the request came
from the trusted MCP), and reads `acting-for` from the request body to
determine who the action is on behalf of. It then checks whether the
`acting-for` identity is authorized for the target component via
`IsUserAuthorizedForComponent`. If not, the dashboard returns 403 and the MCP
passes it back to chai-bot.

This means:

- Chai-bot does not check authorization client-side
- No maintainer-list endpoint or `check_maintainers` tool is needed
- Chai-bot receives a 4xx when the user lacks permissions and communicates
that to the user

### Two Modes of Operation

**User-initiated:**

1. User asks chai-bot in Slack to create an outage
2. Chai-bot resolves the user's Slack ID to Kerberos `uid` via OrgData
3. Chai-bot calls `create_outage` on the authenticated MCP with
  `acting-for: "<uid>"` and its SA Bearer token
4. oauth-proxy validates chai-bot's token, forwards to the MCP
5. MCP calls the dashboard's oauth-proxy with its own SA token, including
  `acting-for` in the request body
6. Dashboard validates HMAC, checks if the `acting-for` user is authorized,
  creates outage or returns 403
7. Audit log records the `acting-for` identity with `discovered_from: "mcp"`

**Bot-initiated:**

1. Chai-bot detects an issue on its own
2. Chai-bot calls `create_outage` with `acting-for: "chai-bot"`
3. Same flow -- dashboard checks if `"chai-bot"` is authorized
4. Outage created as unconfirmed with severity `Suspected`
5. Slack notification sent via `pkg/outage/slack_report.go`

### Safeguards

These are enforced in the authenticated MCP before calling the dashboard:

- Bot-initiated outages (`acting-for: "chai-bot"`) are always forced to
severity `Suspected` with `ConfirmedAt` null
- Before creating an outage, check for an existing active outage on the same
sub-component. If one exists, return it instead of creating a duplicate.

## Implementation Plan

### Phase 1: Two MCPs and Auth Infrastructure

**Read-only MCP:**

- Remove mutating tools from the existing MCP server (stays public,
unauthenticated)

**Authenticated MCP:**

- Deploy the authenticated MCP as a new container in the dashboard pod
(e.g., port 8091)
- Add a new oauth-proxy container (`oauth-proxy-mcp`) configured with
`--upstream=http://localhost:8091` for `protected-mcp.ship-status.ci.openshift.org`
- Add a new Route and Service for the `protected-mcp` subdomain
- Create a dedicated SA for the MCP to use when calling the dashboard
through the dashboard's oauth-proxy (same cluster, app.ci only)

**Dashboard changes (`cmd/dashboard/`):**

- Add a `trusted_delegators` field to `DashboardConfig`:
  ```yaml
  trusted_delegators:
    - "system:serviceaccount:ship-status:mcp-server"
  ```
- When a request has `discovered_from: "mcp"`:
  1. Check if `X-Forwarded-User` is in `config.TrustedDelegators`. If not,
     return 403.
  2. Read `acting-for` from the request body.
  3. Run `IsUserAuthorizedForComponent` against the `acting-for` identity.
     If not authorized, return 403.
  4. Create the outage, recording `acting-for` as the user in audit logs.
- The MCP SA is never a component owner. It only needs to be in
  `trusted_delegators`.

**Chai-bot SA provisioning:**

- Chai-bot SA on app.ci is set up in openshift/release#81417 (SA, RBAC)
- Store the token in Bitwarden
- Provision the secret to chai-bot's namespace on mp-plus
- Chai mounts the token and includes it in requests to the authenticated MCP

**Component ownership:**

- Add `"chai-bot"` as a `user` on all components in
`dashboard-config.yaml` for bot-initiated outages

### Phase 2: Write Tools

**Authenticated MCP tools:**

The write tools are already implemented (merged in PR #120) on the current
public MCP. Phase 2 moves them to the authenticated MCP and adds `acting-for`
to each tool. The MCP calls the dashboard's oauth-proxy with its SA Bearer
token, including `acting-for` in the request body. The oauth-proxy sets
`X-Forwarded-User` to the MCP SA and signs with HMAC. The dashboard checks
authorization against the `acting-for` identity.

Existing write tools to move to the authenticated MCP:

- `create_outage` -- create an outage on a sub-component
- `update_outage` -- update severity, description, end_time, or start_time
(resolving an outage is done by setting `end_time`)
- `delete_outage` -- permanently delete an outage created in error
- `add_triage_note` / `update_triage_note` -- manage triage notes
- `add_outage_link` / `update_outage_link` / `delete_outage_link` -- manage
outage links

**MCP server (**`mcp/api_server.py`**,** `mcp/api_client.py`**):**

- Middleware reads `Authorization` from headers (confirms oauth-proxy auth)
- Reads `acting-for` from tool arguments
- Calls the dashboard's oauth-proxy with the MCP SA Bearer token, including
`acting-for` in the request body
- Enforces safeguards before calling the dashboard (bot-initiated severity
override, duplicate detection)
- Passes through 4xx errors to the caller

Outages created via MCP go through the same API as the web UI, so Slack
notifications (`pkg/outage/slack_report.go`) work automatically.

### Phase 3: Chai-bot Integration

**Client configuration (**`ship_help_bot/tools/ship_status/client.py`**):**

- Connect to both MCP servers (read-only for queries, authenticated for
writes)
- `extra_headers` callback sends SA Bearer token to the authenticated MCP

**Identity resolution:** Chai-bot resolves Slack user IDs to Kerberos `uid`
via OrgData and sends it as `acting-for` on user-initiated requests.

**Prompt instructions:** Persona guidance for when to offer outage creation,
confirming intent before acting, explaining auth failures, and when to act
autonomously.

**Error handling:**

- 403 from the MCP -- inform user they are not a maintainer
- Slack user cannot be resolved -- suggest the dashboard UI
- MCP/dashboard unavailable -- suggest trying again later

## Test Plan

### Unit Tests

**Dashboard auth middleware (`cmd/dashboard/`):**

Tests trusted caller verification, acting-for authorization, and audit
logging for MCP-originated requests.


| Test Case                          | Input                                         | Expected                |
| ---------------------------------- | --------------------------------------------- | ----------------------- |
| Trusted MCP SA + authorized user   | MCP SA caller, acting-for is component owner  | 201 Created             |
| Trusted MCP SA + unauthorized user | MCP SA caller, acting-for not in owners       | 403 Forbidden           |
| Unknown SA rejected                | Non-MCP SA caller with discovered_from: "mcp" | 403 Forbidden           |
| Audit log records acting-for       | MCP SA caller, acting-for: "jdoe"             | Audit log user = "jdoe" |


**MCP (**`mcp/`**):**

Tests the authenticated MCP's tool logic, safeguards, and error passthrough.


| Test Case                         | Input                                 | Expected                         |
| --------------------------------- | ------------------------------------- | -------------------------------- |
| Missing acting-for rejected       | Valid token, no acting-for argument   | Error returned to caller         |
| Bot-initiated forced to Suspected | acting-for: "chai-bot", severity=Down | Severity overridden to Suspected |
| Duplicate outage returns existing | Active outage on same sub-component   | Existing outage returned         |
| Dashboard 403 passed through      | acting-for user not authorized        | 403 returned to caller           |


### E2E Tests

`test/e2e/dashboard_test.go`**:**

End-to-end tests through the full auth chain (oauth-proxy → MCP → dashboard).


| Test Case             | Input                                 | Expected                               |
| --------------------- | ------------------------------------- | -------------------------------------- |
| User-initiated outage | acting-for authorized user via MCP    | Created, audit records acting-for user |
| Unauthorized user     | acting-for not in owners              | 403                                    |
| Bot-initiated flow    | acting-for: "chai-bot", severity=Down | Suspected, unconfirmed                 |
| Outage lifecycle      | Create, triage note, resolve via MCP  | All succeed, audit trail complete      |


## Open Questions

1. **Token provisioning to mp-plus.** The chai-bot SA token will be stored
   in Bitwarden. What is the best way to get it provisioned to chai-bot's
   namespace on mp-plus? 

### Answered

1. **Authenticated MCP deployment.** oauth-proxy only supports path-based
  routing, so a separate subdomain requires its own oauth-proxy instance.
  > **A:** Use a dedicated subdomain (`protected-mcp.ship-status.ci.openshift.org`)
  > with its own oauth-proxy container.
2. **Chai-bot autonomous authorization.** `"chai-bot"` needs to pass
  `IsUserAuthorizedForComponent` for bot-initiated outages.
  > **A:** Add `"chai-bot"` as a `user` on every component in
  > `dashboard-config.yaml`.
3. **Scope of MCP tools.** Should chai-bot also update outage severity, or
  only create and resolve? Should it manage outage links?
  > **A:** Chai-bot should be able to do anything a human can do through the
  > web UI. The full set of tools is implemented in PR #120.
4. **Bot-initiated severity.** Should bot-initiated outages always be
  `Suspected`, or should chai-bot be able to set `Degraded`/`Down` with
   high-confidence signals?
  > **A:** Always `Suspected`.
5. **Secret store.** Which secret store should hold the chai-bot SA token?
  > **A:** Bitwarden.

