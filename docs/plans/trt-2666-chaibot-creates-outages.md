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

## Current System Architecture

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

### Existing Component-Monitor Auth Flow

The component-monitor follows this flow:

1. Reads SA token from file (`--report-auth-token-file`)
2. Sends `POST /api/component-monitor/report` with `Authorization: Bearer <token>`
3. The oauth-proxy validates the token via `TokenReview` against the K8s API,
  sets `X-Forwarded-User` to the SA name, and signs with HMAC
4. Dashboard auth middleware validates HMAC + user header
5. `ValidateRequest` checks SA is in component owners + monitor name matches
  sub-component config
6. Outages are created/resolved based on probe results

### Existing Chai-bot Integration

Chai-bot runs on a separate OpenShift cluster from SHIP Status. Rather than
calling the REST API directly, it integrates through an external MCP endpoint (`mcp.ship-status.ci.openshift.org/mcp`) using `streamable_http`
transport.
Tools are discovered dynamically via `tools/list` at startup -- if new tools
are added to the MCP server, chai-bot picks them up automatically.

The current MCP endpoint is unauthenticated. For write operations, the MCP
server authenticates to the dashboard API via oauth-proxy using a service
account Bearer token (the same mechanism the component-monitor uses).

Chai-bot can also resolve Slack user IDs to Kerberos `uid` via its OrgData
tool, which is used to verify user authorization via the maintainer-list
endpoint before creating outages.

### Service Account Authorization

The existing outage CRUD endpoints
(`POST/PATCH/DELETE /api/components/{comp}/{sub}/outages`) use
`IsUserAuthorizedForComponent`, which **does not check
`Owner.ServiceAccount`**. Chai-bot's service account (SA) cannot use these
endpoints without auth changes. This is the central design challenge.

## Design Options

There are three distinct problems here.

### Problem 1: How does chai-bot authenticate for write operations?

Since chai-bot communicates via the external MCP endpoint (not directly to the
dashboard API), there are two auth layers to consider:

1. **Outer layer**: chai-bot to the MCP server (over external HTTPS)
2. **Inner layer**: MCP server to the dashboard API (over loopback via
  oauth-proxy)

#### Inner layer: MCP server to dashboard API

The MCP server already documents this pattern for future write operations (see
`cmd/dashboard/README.md`): mounting a service account Bearer token at
`SHIP_STATUS_AUTH_TOKEN_FILE` and sending it to the oauth-proxy at
`127.0.0.1:8443`. This follows the same mechanism the component-monitor uses.
A K8s ServiceAccount for chai-bot in the `ship-status` namespace would be
added to `dashboard-config.yaml` owners as a `service_account` entry, and
the oauth-proxy validates the token via `TokenReview`.

#### Outer layer: chai-bot to the MCP server

The current MCP endpoint is unauthenticated (fine for read-only tools). For write operations, the question is whether the MCP server itself needs to authenticate its callers. **(Updated - 6/23) The MCP server can instead authenticate directly to the dashboard API using the inner layer (SA Bearer token via oauth-proxy), making the options below unnecessary.** 

#### ~~Option A: Bearer token on the MCP endpoint~~

Add a shared secret or API key to the MCP server that chai-bot presents as a
Bearer token on write tool calls. The MCP server validates the token before
proxying writes to the dashboard API.

- Simple to implement -- the MCP server already runs as a Python process and
can check a header
- Adding an auth header to chai-bot's MCP client is a small change
- Requires a shared secret to be provisioned and rotated

#### ~~Option B: Route the MCP endpoint through oauth-proxy for writes~~

Add a second, protected MCP route (e.g.,
`protected.mcp.ship-status.ci.openshift.org`) that goes through the existing
oauth-proxy. Chai-bot would present a K8s SA Bearer token to this route.

- Reuses the existing oauth-proxy infrastructure
- Requires chai-bot to have a K8s SA token for the app.ci cluster -- it
already has kubeconfigs for app.ci mounted for other tools, so this is
feasible
- More infrastructure setup (new Route, oauth-proxy config)

#### ~~Option C: Unauthenticated MCP endpoint, rely on dashboard-level auth~~

Skip MCP-level auth entirely. Write tools accept an `on_behalf_of` parameter,
and the MCP server passes it to the dashboard API as `X-On-Behalf-Of`. The
dashboard still checks whether the on-behalf-of user is authorized for the
component.

- Simplest option -- no new auth on the MCP layer
- Anyone who can reach the MCP endpoint could call write tools and claim to
be any user. The dashboard would reject unauthorized users, but the caller
could impersonate an authorized one.
- Acceptable only if the MCP endpoint is network-restricted (e.g., only
reachable from trusted clusters)

**(Updated - 6/23) We should follow what was done for the component-monitor
by creating a service account for chai-bot and authenticating to the dashboard
API via oauth-proxy using a Bearer token (the same mechanism the
component-monitor already uses with `POST /api/component-monitor/report`).
This avoids adding a new auth layer on the MCP server -- the MCP server
proxies write requests to the dashboard API with the SA Bearer token through
oauth-proxy, and read-only tools continue to use the public endpoint as they
do today.**

### Problem 2: How does chai-bot verify a user is authorized?

When a user says "create an outage on Sippy", chai-bot needs to confirm they're
a Sippy maintainer. Owners are defined as `rover_group`s (not individual users),
and group membership is resolved via the OpenShift Groups API cache in the
dashboard backend.

#### ~~Option A: On-behalf-of delegation~~

The first option would be to have chai-bot pass the requesting user's identity
with the outage creation request. The SHIP Status API itself checks group
membership using its existing `GroupMembershipCache` and
`IsUserAuthorizedForComponent`.

The flow:

1. User in Slack: "create an outage on Sippy"
2. Chai-bot resolves Slack user to Kerberos `uid` via OrgData
3. Chai-bot calls the `create_outage` MCP tool with `on_behalf_of: <uid>`
4. MCP server passes the identity to the dashboard API as `X-On-Behalf-Of`
5. Dashboard runs `IsUserAuthorizedForComponent` against the on-behalf-of user
6. Outage is created and attributed to the requesting user (not the MCP SA)

For autonomous detection, chai-bot omits the `on_behalf_of` parameter and the
MCP server acts under its own SA identity, creating an unconfirmed outage.

- Authorization stays in the SHIP Status system where it already lives
- Leverages the existing `GroupMembershipCache` -- no new group resolution needed
- The API must only trust `X-On-Behalf-Of` from explicitly allowed SAs
- Audit logs would record both the SA and the delegated user

#### Option B: New maintainer-list endpoint

We could add a new authenticated endpoint
(`GET /api/components/{slug}/maintainers`) that expands `rover_group`s to
individual users. Chai-bot would query this endpoint to check if the requesting
user is a maintainer before making the outage creation request.

- The data is already available -- `GroupMembershipCache.GetGroupMembers`
exists in `groups.go` and returns the member list for a given group
- Chai-bot gets a clear yes/no answer before acting
- Authorization responsibility is split -- chai-bot checks membership
client-side, and the dashboard trusts the SA as a recognized owner
- Also useful as a general-purpose endpoint (e.g., for "who owns this
component?" queries)

#### ~~Option C: Chai-bot queries OpenShift Groups API directly~~

Another option would be having chai-bot resolve Slack user to Kerberos ID to
OpenShift Group membership itself, then compare against `dashboard-config.yaml`
to determine if the user is a component owner.

- No SHIP Status API changes needed -- chai-bot handles everything
- Duplicates the authorization logic entirely outside the SHIP Status system
- Requires chai-bot to have its own OpenShift API access and group caching
- Risk of drift -- if the dashboard's `GroupMembershipCache` and chai-bot's
cache disagree on group membership, behavior becomes inconsistent

#### ~~Option D: Trust chai-bot entirely~~

We could also add chai-bot's SA as an owner on all components in
`dashboard-config.yaml` and let it handle access control internally in its own
logic.

- Simplest API-side implementation -- no new auth concepts or endpoints
- All authorization lives in chai-bot, not in the data system
- If chai-bot has a bug or misconfiguration, it could create outages on any
component for any user with no server-side guardrails
- Violates the principle that the system of record should enforce its own
access control

**(Updated - 6/23) We should have a separate maintainer-list endpoint. The
data is already available via `GroupMembershipCache.GetGroupMembers`, and
exposing it as an endpoint lets chai-bot check authorization before acting.
Chai-bot's SA is added as a trusted owner on all components (following the
same pattern as the component-monitor), so the dashboard trusts chai-bot to
have performed the maintainer check before creating the outage.**

### Problem 3: How does a SA use the outage CRUD endpoints?

The existing outage endpoints use `IsUserAuthorizedForComponent`, which only
checks `Owner.User` and `Owner.RoverGroup`. Even if chai-bot's SA is listed
as a `service_account` owner in the config, this function won't recognize it
and the request will be rejected.

#### Option A: Extend `IsUserAuthorizedForComponent` to check service accounts

The first option would be to add a `ServiceAccount` check to the existing
authorization function so SA identities can use the same endpoints as human
users.

- Minimal code change -- add one `if` clause to check `Owner.ServiceAccount`
in `IsUserAuthorizedForComponent`
- All existing outage endpoints (create, update, delete, triage notes, links)
would work for SAs immediately with no other changes
- Blurs the boundary between human and machine authorization -- the function
was deliberately designed to exclude SAs
- Making it accept SAs for all operations removes the ability to enforce
different constraints on bot actions (e.g., forcing unconfirmed severity
for autonomous outages)

#### ~~Option B: Create a dedicated endpoint for chai-bot~~

We could give chai-bot its own endpoint (e.g., `POST /api/chai-bot/outage`),
similar to how component-monitor has `POST /api/component-monitor/report`.

- Clean separation of concerns -- human endpoints stay unchanged
- Can have its own validation logic (e.g., require `X-On-Behalf-Of` for
user-initiated actions, force unconfirmed for bot-initiated)
- Duplicates the outage creation logic that already exists in the handlers
(`CreateOutageJSON`, `UpdateOutageJSON`, etc.)
- Requires maintaining a parallel API surface for what is essentially the
same set of operations

#### ~~Option C: Delegation middleware for trusted service accounts~~

Another option would be to add middleware that, for trusted SAs like the MCP
server's, processes `X-On-Behalf-Of` and replaces the request context user with the
delegated identity before the request reaches existing handlers.

- Existing handlers work unchanged -- they see a normal user identity
- Authorization and audit logging work exactly as they do for human users
- The middleware layer handles the SA trust and identity delegation
- Requires careful implementation -- must only activate for explicitly
trusted SAs, not all service accounts
- For bot-initiated actions (no `X-On-Behalf-Of`),
`IsUserAuthorizedForComponent` would be extended to accept
`service_account` owners, but only for creating unconfirmed outages

**(Updated - 6/23) We will probably need to extend** `IsUserAuthorizedForComponent` **to authorize SAs, following the pattern that** `ValidateRequest` **already uses for the component-monitor. Adding a** `service_account` **check to the existing function should be a minimal code change, and since chai-bot's SA is added as an owner on all components, the existing endpoints should work immediately. Chai-bot handles authorization client-side via the maintainer-list endpoint before creating outages, so the dashboard only needs to verify the SA is a recognized owner. An alternative would be to create a separate** `IsServiceAccountAuthorizedForComponent` **function and call both in the handlers, which keeps human and SA authorization separate but requires updating every handler that needs SA access.**

## Solution Overview

### Two Modes of Operation

**User-initiated** (maintainer-verified):

1. User asks chai-bot in Slack to create an outage
2. Chai-bot resolves the user's Slack ID to Kerberos `uid` via OrgData
3. Chai-bot calls the maintainer-list endpoint to verify the user is authorized
  for the target component
4. If authorized, chai-bot calls `create_outage` on the SHIP Status MCP server
5. MCP server calls the dashboard API with its SA Bearer token via oauth-proxy
6. `IsUserAuthorizedForComponent` (extended) checks `service_account` owners
  and confirms the SA is a recognized owner
7. Outage is created and attributed to chai-bot's SA. The requesting user's
  identity is recorded in the outage description or triage notes for
   traceability.
8. Audit log records the SA that created the outage

**Bot-initiated** (autonomous detection):

1. Chai-bot detects an issue on its own
2. Chai-bot calls `create_outage` on the SHIP Status MCP server (no user
  involved)
3. MCP server calls the dashboard API with its SA Bearer token via oauth-proxy
4. `IsUserAuthorizedForComponent` (extended) checks `service_account` owners
5. Outage is created as unconfirmed with severity `Suspected`
6. Team is notified via Slack

### Key Constraints

- Chai-bot must never create confirmed outages autonomously -- only
unconfirmed/Suspected
- Chai-bot must check the maintainer-list endpoint before creating user-initiated
outages to verify the requesting user is authorized
- Duplicate detection: check for existing active outages before creating
- Slack notification deduplication: check if alerting is already active

## Implementation Plan

### Phase 1: Service Account and Auth Infrastructure

**Kubernetes:**

- Create a ServiceAccount for chai-bot in the `ship-status` namespace (or
reuse the existing pod SA if appropriate) and mount a projected token
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

### Phase 2: Maintainer-List Endpoint and Write Tools

**Maintainer-list endpoint (`cmd/dashboard/handlers.go`):**

- Add `GET /api/components/{componentName}/maintainers` (protected endpoint)
- Expand each component's `rover_group` owners via
`GroupMembershipCache.GetGroupMembers` and return the list of individual
users authorized to manage the component
- Chai-bot calls this endpoint before creating outages to verify the
requesting user is a maintainer

**MCP write tools (in `mcp/`):**

The MCP server already has plumbing for authenticated writes:
`SHIP_STATUS_AUTH_TOKEN_FILE` for the SA Bearer token and
`SHIP_STATUS_PROTECTED_API_URL` pointing at the oauth-proxy on
`127.0.0.1:8443`. New write tools are added here. Chai-bot discovers them
automatically via `tools/list` at startup.

- `create_outage` -- create an outage on a sub-component. Accepts severity
and description.
- `resolve_outage` -- set end_time on an active outage
- `add_triage_note` -- add a triage note to an existing outage
- `check_maintainers` -- call the maintainer-list endpoint to verify a user
is authorized for a component
- `get_component_status` -- query current status (already exists as read-only)
- `list_active_outages` -- list active outages for a component/sub-component

**Bot-initiated safeguards:**

- When chai-bot creates outages autonomously (no user request), force severity
to `Suspected` and `ConfirmedAt` to null
- Set `discovered_from: "chai-bot"` to distinguish from community reports and
component-monitor
- Check for existing active outages on the same sub-component before creating.
If one exists, return the existing outage instead of creating a duplicate.
This is especially important for bot-initiated outages, where multiple probe
failures could trigger rapid-fire creation attempts.

### Phase 3: Slack Notification Integration

**File: `pkg/outage/slack_report.go`**

- Before sending a notification, check if the sub-component already has an
active outage with Slack threads
- If not already alerted, notify the component's configured Slack channel
- Include context about what chai-bot detected and a link to the dashboard

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
| Maintainer list returns members      | Component with `rover_group` owners    | Expanded list of individual users returned    |
| Maintainer list unknown component    | Non-existent component slug            | 404 Not Found                                 |


**MCP write tools (`mcp/`):**


| Test Case                         | Input                                 | Expected                           |
| --------------------------------- | ------------------------------------- | ---------------------------------- |
| Write with valid SA token         | Valid SA auth + `create_outage` call  | Tool executes, calls dashboard API |
| Write without SA token            | No auth + `create_outage` call        | Rejected by oauth-proxy            |
| Read tools remain unauthenticated | No auth + `get_component_status` call | Tool executes normally             |
| Check maintainers returns list    | `check_maintainers` call              | Returns list of authorized users   |


### E2E Tests

**File: `test/e2e/dashboard_test.go`**


| Test Case                   | Input                                            | Expected                                     |
| --------------------------- | ------------------------------------------------ | -------------------------------------------- |
| SA creates outage as owner  | Chai-bot SA creates outage on owned component    | Outage created, audit trail recorded         |
| SA rejected on unowned comp | SA creates outage on component it doesn't own    | 403                                          |
| Bot-initiated flow          | SA creates outage autonomously                   | Suspected severity, unconfirmed              |
| Maintainer list endpoint    | Query maintainers for component with rover_group | Returns expanded member list                 |
| Outage lifecycle via SA     | Create, add triage note, resolve                 | All operations succeed, audit trail complete |


### Regression

The existing 60+ e2e test cases and all unit tests must continue to pass. The
only change to `IsUserAuthorizedForComponent` is adding a `service_account`
check, which does not affect existing human user authorization flows.

## Open Questions

1. **Scope of MCP tools.** Should chai-bot also update outage severity, or
  only create and resolve? Should it manage outage links?
  > **A:** Chai-bot should be able to do anything a human can do through
  > the web UI (update severity, add triage notes, manage links, etc.). Need
  > to look into the full set of operations and whether any are too
  > complicated to expose as MCP tools.

2. **Bot-initiated severity.** Should bot-initiated outages always be
  `Suspected`, or should chai-bot be able to set `Degraded`/`Down` when it
   has high-confidence signals (e.g., confirmed by multiple probes)?
  > **A:** Always `Suspected`.

(Will add more here as they come up during implementation.)