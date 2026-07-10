# TRT-2666: Chai-bot Creates Outages

**Date:** 2026-06-18
**JIRA:** [TRT-2666](https://redhat.atlassian.net/browse/TRT-2666)
**Author:** Arnav Meduri

## Problem Statement

Component maintainers currently have to manually create outages in the SHIP
Status Dashboard through the web UI. This means navigating to the correct
sub-component page, filling in severity/description, and managing the outage
lifecycle (updating severity, adding triage notes, attaching links, resolving).
Chai-bot currently has no way to create or manage outages in the dashboard.

The goal is to enable chai-bot to:

1. Create outages on behalf of authorized component maintainers (user-initiated)
2. Create unconfirmed outages autonomously when it discovers issues (bot-initiated)
3. Manage the full outage lifecycle via MCP tools

## System Architecture

### Dual-Ingress Auth Model

The pod runs three containers: the dashboard (port 8080) and two sidecar
containers -- an oauth-proxy (port 8443) and the MCP server. Because they
share a pod, the sidecars can reach the dashboard on loopback
(`127.0.0.1:8080`). Two ingress hostnames route to the dashboard and oauth-proxy:

- **Public** (`ship-status.ci.openshift.org`) -- routes to the dashboard
directly on port 8080, serving unauthenticated read-only endpoints
- **Protected** (`protected.ship-status.ci.openshift.org`) -- routes to the
oauth-proxy on port 8443, which authenticates the client, sets
`X-Forwarded-User`, signs the request with HMAC (`GAP-Signature`), and
proxies to the dashboard on port 8080

Inside the dashboard, routes are registered as either public or protected
(`server.go`). Protected routes run through HMAC auth middleware
(`cmd/dashboard/auth.go`), which validates the signature and extracts
`X-Forwarded-User` into the request context. The middleware does not
distinguish between OAuth users and service accounts -- both arrive as a
string. The distinction happens downstream in authorization.

### Two Separate Authorization Paths

**Human users** -- checked via `IsUserAuthorizedForComponent` (`handlers.go`):

- Matches `Owner.User` (exact string match, used for dev/testing only)
- Matches `Owner.RoverGroup` via `GroupMembershipCache`, which resolves group
membership through the OpenShift Groups API (`pkg/auth/groups.go`). The
`rover_group` names correspond to OpenShift `Group` resources synced from
Rover/LDAP upstream -- the dashboard queries the Groups API, not Rover/LDAP
directly. Results are cached in-memory at startup and reloaded when the
config file content changes.
- Does **not** check `Owner.ServiceAccount`

**Service accounts** -- checked via `ValidateRequest`
(`component_monitoring_report.go`):

- Only used for the component-monitor's dedicated endpoint
(`POST /api/component-monitor/report`)
- Matches `Owner.ServiceAccount` (exact string match)

These are completely separate paths -- a service account identity cannot pass
human authorization checks, and vice versa.

### Owner Model

Owners are defined per-component in `dashboard-config.yaml` as a list of
entries, each with one of three optional fields:

```yaml
owners:
  - rover_group: "test-platform-ci-admins" # Production: OpenShift Group
  - service_account: "system:serviceaccount:ship-status:component-monitor"
  - user: "developer" # Dev/testing only
```

### Component-Monitor Auth Flow

The component-monitor follows this flow:

1. Reads SA token from file (`--report-auth-token-file`)
2. Sends `POST /api/component-monitor/report` with `Authorization: Bearer <token>`
3. The oauth-proxy validates the token via `TokenReview` against the K8s API,
  sets `X-Forwarded-User` to the SA name, and signs with HMAC
4. Dashboard auth middleware validates HMAC + user header
5. `ValidateRequest` checks SA is in component owners + monitor name matches
  sub-component config
6. Outages are created/resolved based on probe results

### Chai-bot Integration

Chai-bot runs on a separate OpenShift cluster (mp-plus) from the dashboard
(app.ci). Rather than calling the REST API directly, it integrates through an
external MCP endpoint (`mcp.ship-status.ci.openshift.org/mcp`) using
`streamable_http` transport. Tools are discovered dynamically via `tools/list`
at startup -- if new tools are added to the MCP server, chai-bot picks them up
automatically.

The MCP endpoint is unauthenticated for read operations (read tools are
intentionally public). Currently, only read tools exist. This design adds
write tools with authentication described below.

Chai-bot can also resolve Slack user IDs to Kerberos `uid` via its OrgData
tool, which is used to verify user authorization before creating outages.

### MCP Write Authentication (New)

Since the MCP endpoint is publicly reachable, write operations require the
caller to authenticate. The MCP server acts as a stateless passthrough -- it
holds no credentials itself. The flow:

```
Chai (mp-plus)
  |
  | Authorization: Bearer <chai-bot SA token>
  v
MCP server (mcp.ship-status.ci.openshift.org:8090)
  |
  | extracts token, forwards unchanged
  v
oauth-proxy (127.0.0.1:8443)
  |
  | TokenReview + SAR on app.ci
  | sets X-Forwarded-User + GAP-Signature
  v
dashboard (127.0.0.1:8080)
  |
  | verifies HMAC, checks component owners
  v
request authorized (or rejected)
```

The chai-bot SA token is stored in Bitwarden and provisioned to Chai's pod on
mp-plus as a mounted secret. A dedicated ServiceAccount (`chai-bot`) in the
`ship-status` namespace on app.ci is set up in openshift/release#81417 (SA,
RBAC, and dashboard-config owner entries).

### Service Account Authorization

The existing outage CRUD endpoints
(`POST/PATCH/DELETE /api/components/{comp}/{sub}/outages`) use
`IsUserAuthorizedForComponent`, which **does not check
`Owner.ServiceAccount`**. Chai-bot's service account (SA) cannot use these
endpoints without auth changes. This is the central design challenge.

## Design Decisions

### How does chai-bot authenticate for write operations?

The MCP server acts as a stateless passthrough. Chai-bot includes its SA
Bearer token in the `Authorization` header when calling MCP tools. The MCP
server extracts the token and forwards it to the oauth-proxy at
`127.0.0.1:8443`, which validates it via TokenReview. This follows the same
validation mechanism the component-monitor uses, but with the token coming
from the caller rather than being stored on the MCP server.

This approach was chosen because the MCP endpoint is publicly accessible. If
the MCP server held its own token and used it unconditionally, any caller who
could reach the endpoint would have their write requests authenticated as
chai-bot, bypassing the dashboard's authorization model entirely.

### How does chai-bot verify a user is authorized?

A **protected** endpoint (`GET /api/components/{slug}/maintainers`) expands
each component's `rover_group` owners to individual Kerberos IDs. Chai-bot
calls it (via its SA Bearer token through oauth-proxy) to verify the
requesting user is a maintainer before creating an outage.

- The data is already available -- `GroupMembershipCache.GetGroupMembers`
exists in `groups.go` and returns the member list for a given group
- Chai-bot gets a clear yes/no answer before acting
- Authorization responsibility is split -- chai-bot checks membership
client-side, and the dashboard trusts the SA as a recognized owner
- Also useful as a general-purpose endpoint (e.g., for "who owns this
component?" queries)

### How does a SA use the outage CRUD endpoints?

`IsUserAuthorizedForComponent` is extended to also check
`Owner.ServiceAccount`, following the pattern that `ValidateRequest` already
uses for the component-monitor. This is a minimal change -- add a loop over
owners checking the `ServiceAccount` field against the authenticated user
identity.

Chai-bot's SA is added as an owner on all components in
`dashboard-config.yaml`, so the existing endpoints work immediately. Chai-bot
handles authorization client-side via the protected maintainer-list endpoint
before creating outages, so the dashboard only needs to verify the SA is a
recognized owner.

## Solution Overview

### Two Modes of Operation

**User-initiated** (maintainer-verified):

1. User asks chai-bot in Slack to create an outage
2. Chai-bot resolves the user's Slack ID to Kerberos `uid` via OrgData
3. Chai-bot calls the protected maintainer-list endpoint to verify the user is
  authorized for the target component
4. If authorized, chai-bot calls `create_outage` on the SHIP Status MCP server,
  including its SA Bearer token in the `Authorization` header
5. MCP server forwards the token to the oauth-proxy at `127.0.0.1:8443`
6. `IsUserAuthorizedForComponent` (extended) checks `service_account` owners
  and confirms the SA is a recognized owner
7. Outage is created and attributed to chai-bot's SA. The requesting user's
  identity is recorded in the outage description or triage notes for
   traceability.
8. Audit log records the SA that created the outage

**Bot-initiated** (autonomous detection):

1. Chai-bot detects an issue on its own
2. Chai-bot calls `create_outage` on the SHIP Status MCP server, including its
  SA Bearer token in the `Authorization` header
3. MCP server forwards the token to the oauth-proxy at `127.0.0.1:8443`
4. `IsUserAuthorizedForComponent` (extended) checks `service_account` owners
5. Outage is created as unconfirmed with severity `Suspected`
6. Slack notification is sent automatically by the existing outage reporting
   path (`pkg/outage/slack_report.go`)

### Requirements

- Chai-bot must never create confirmed outages autonomously -- only
  unconfirmed/Suspected
- Chai-bot must check the protected maintainer-list endpoint before creating
  user-initiated outages to verify the requesting user is authorized
- Before creating an outage, check for an existing active outage on the same
  sub-component and return it instead of creating a duplicate

Several other pieces chai-bot needs already exist and are reused unchanged:

- Protected-route auth (oauth-proxy + HMAC) and the component-monitor SA pattern
- MCP read tools in `mcp/api_server.py` -- use `get_infrastructure_status` for
  component status and `get_component_outages` for active outages
- Slack reporting in `pkg/outage/slack_report.go`, which runs when outages are
  created or updated through the API and threads updates on the same alert

## Implementation Plan

### Phase 1: Service Account and Auth Infrastructure

**Kubernetes:**

- Create a dedicated ServiceAccount for chai-bot in the `ship-status`
  namespace and mount a projected token
- Ensure the oauth-proxy SA has `system:auth-delegator` ClusterRole to perform
`TokenReview` for the new SA token

**Config (`dashboard-config.yaml` in `openshift/release`):**

- Add chai-bot's `service_account` to the owners list of every component,
following the same pattern used for the component-monitor SA. Each component
gets a new entry like:
`- service_account: "system:serviceaccount:ship-status:chai-bot"`

**Authorization (`cmd/dashboard/handlers.go`):**

- Extend `IsUserAuthorizedForComponent` to also check `Owner.ServiceAccount`,
reusing the same logic that `ValidateRequest` already uses in
`component_monitoring_report.go`. This is a minimal change -- add a loop
over owners checking the `ServiceAccount` field against the authenticated
user identity.

**Token provisioning:**

- Store the chai-bot SA token in Bitwarden and configure `ci-secret-generator` to
sync it to Chai's namespace on mp-plus
- Chai's pod mounts the token as a file and reads it at runtime to include in
requests to the MCP server
- The MCP sidecar needs `SHIP_STATUS_PROTECTED_API_URL=http://127.0.0.1:8443/api`
set (removed by the security revert in openshift/release#81712). Do NOT
re-add any token mount to the MCP sidecar.

### Phase 2: Maintainer-List Endpoint and Write Tools

**Maintainer-list endpoint (`cmd/dashboard/handlers.go`):**

Add `GET /api/components/{componentName}/maintainers` as a **protected**
endpoint (`protected: true` in `server.go` -- oauth-proxy + HMAC auth, not on
the public ingress). It expands each component's `rover_group` owners via
`GroupMembershipCache.GetGroupMembers` and returns the list of individual
users authorized to manage the component. Chai-bot calls this endpoint before
creating outages to verify the requesting user is a maintainer.

**MCP write tools (in `mcp/`):**

The MCP server uses `SHIP_STATUS_PROTECTED_API_URL` pointing at the
oauth-proxy on `127.0.0.1:8443` for write operations. The Bearer token is
extracted from the caller's `Authorization` header and forwarded with each
protected request. Status and outage queries use the existing read tools
(`get_infrastructure_status`, `get_component_outages`) via the public API.
This phase adds write tools; chai-bot discovers them automatically via
`tools/list` at startup.

- `create_outage` -- create an outage on a sub-component. Accepts severity
and description.
- `resolve_outage` -- set end_time on an active outage
- `add_triage_note` -- add a triage note to an existing outage
- `check_maintainers` -- call the protected maintainer-list endpoint to verify
a user is authorized for a component

**MCP server auth implementation (`mcp/api_server.py`, `mcp/api_client.py`):**

- Add FastMCP middleware that reads `Authorization` from incoming HTTP headers
  using `get_http_headers(include={"authorization"})` (authorization is
  stripped by default) and stores the Bearer token in a `ContextVar`
- `DashboardClient.protected_request()` uses the context token when available,
  falling back to `SHIP_STATUS_AUTH_TOKEN_FILE` for local dev only

**Chai-bot MCP client (`ship_help_bot/tools/ship_status/client.py`):**

- Add `extra_headers` callback to `StreamableHttpMcpClient` that reads the
  token from a mounted file and returns `{"Authorization": "Bearer <token>"}`
- Follows the same pattern as the Product Pages MCP client

**Bot-initiated safeguards:**

- When chai-bot creates outages autonomously (no user request), force severity
to `Suspected` and `ConfirmedAt` to null
- Set `discovered_from: "chai-bot"` to distinguish from community reports and
component-monitor
- Check for existing active outages on the same sub-component before creating.
If one exists, return the existing outage instead of creating a duplicate.
This is especially important for bot-initiated outages, where multiple probe
failures could trigger rapid-fire creation attempts.

### Phase 3: Slack Notifications

Outages chai-bot creates or updates go through the same API as the web UI.
`pkg/outage/slack_report.go` handles notifications and threads updates on the
existing alert. The duplicate-outage check in Phase 2 is sufficient here.

### Phase 4: Chai-bot Integration

The following changes are on the chai-bot side.

**User identity resolution:**

Chai-bot can already resolve Slack user IDs to Kerberos `uid` via its OrgData
tool. Before creating an outage on a user's behalf, chai-bot resolves the
user's identity and calls the `check_maintainers` tool to verify they are
authorized for the target component.

**Persona prompt instructions:**

The persona(s) with access to write tools will need prompt guidance on:

- When to offer outage creation vs. just answering a question
- Confirming intent before acting ("You want me to create a Degraded outage on
Sippy sub-component X?")
- How to explain authorization failures to the user
- When to autonomously create outages (bot-initiated) vs. wait for a human

**Error handling:**

Chai-bot should handle these failure cases gracefully:

- Slack user cannot be resolved to a Kerberos `uid` (not everyone is in the
org data) -- inform the user and suggest using the dashboard UI directly
- The resolved `uid` is not in the maintainer list for the target component --
inform the user they are not a maintainer of that component
- Identity resolution is temporarily unavailable -- inform the user and
suggest trying again later

## Test Plan

### Unit Tests

**File: `cmd/dashboard/handlers_test.go`**


| Test Case                            | Input                                  | Expected                                      |
| ------------------------------------ | -------------------------------------- | --------------------------------------------- |
| SA authorized as owner               | Chai-bot SA listed in component owners | 201 Created                                   |
| SA not authorized                    | SA not in component owners             | 403 Forbidden                                 |
| Bot-initiated creates unconfirmed    | SA creates outage autonomously         | Outage created as Suspected, unconfirmed      |
| Bot-initiated blocked from confirmed | SA creates outage with severity=Down   | Rejected or forced to Suspected               |
| Duplicate detection                  | Active outage exists on sub-component  | Returns existing outage, no duplicate created |
| Maintainer list returns members      | Protected request with SA token; component with `rover_group` owners | Expanded list of individual users returned    |
| Maintainer list requires auth        | Unauthenticated request to maintainer-list endpoint                  | 401 Unauthorized                              |
| Maintainer list unknown component    | Non-existent component slug                                          | 404 Not Found                                 |


**MCP write tools (`mcp/`):**


| Test Case                         | Input                                 | Expected                           |
| --------------------------------- | ------------------------------------- | ---------------------------------- |
| Write with valid SA token         | Caller includes valid Bearer token    | Tool executes, calls dashboard API |
| Write without token               | No Authorization header               | Rejected (no token to forward)     |
| Read tools remain unauthenticated | No auth + `get_infrastructure_status` call | Tool executes normally             |
| Check maintainers returns list    | `check_maintainers` call with valid token | Returns list of authorized users   |


### E2E Tests

**File: `test/e2e/dashboard_test.go`**


| Test Case                   | Input                                            | Expected                                     |
| --------------------------- | ------------------------------------------------ | -------------------------------------------- |
| SA creates outage as owner  | Chai-bot SA creates outage on owned component    | Outage created, audit trail recorded         |
| SA rejected on unowned comp | SA creates outage on component it doesn't own    | 403                                          |
| Bot-initiated flow          | SA creates outage autonomously                   | Suspected severity, unconfirmed              |
| Maintainer list endpoint (auth)     | Protected route with SA token; component with `rover_group` | Returns expanded member list                 |
| Maintainer list endpoint (no auth)  | Unauthenticated request to maintainer-list endpoint         | 401 Unauthorized                             |
| Outage lifecycle via SA     | Create, add triage note, resolve                 | All operations succeed, audit trail complete |


### Regression

The existing 60+ e2e test cases and all unit tests must continue to pass. The
only change to `IsUserAuthorizedForComponent` is adding a `service_account`
check, which does not affect existing human user authorization flows.

## Open Questions

1. **Scope of MCP tools.** Should chai-bot also update outage severity, or
  only create and resolve? Should it manage outage links?
  > **A:** Chai-bot should be able to do anything a human can do through the web UI (update severity, add triage notes, manage links, etc.) -- need to look into the full set of operations and whether any are too complicated to expose as MCP tools.
2. **Bot-initiated severity.** Should bot-initiated outages always be
  `Suspected`, or should chai-bot be able to set `Degraded`/`Down` when it
   has high-confidence signals (e.g., confirmed by multiple probes)?
  > **A:** Always `Suspected`.
3. **Secret store.** Which secret store (Vault or Bitwarden) should hold the
  chai-bot SA token for provisioning to Chai's pod?
  > **A:** Bitwarden (TRT collection). The token will be synced to chai-bot's namespace via `ci-secret-generator`.
