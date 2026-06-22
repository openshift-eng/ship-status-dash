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

Chai-bot runs on a separate OpenShift cluster from SHIP Status. It already has
read-only access to SHIP Status via an external MCP endpoint
(`mcp.ship-status.ci.openshift.org/mcp`) using `streamable_http` transport.
Tools are discovered dynamically via `tools/list` at startup -- if new tools
are added to the MCP server, chai-bot picks them up automatically.

The current read-only MCP endpoint is unauthenticated. For write operations,
authentication will need to be added to the MCP connection.

Chai-bot can also resolve Slack user IDs to Kerberos `uid` via its OrgData
tool, which is used for the on-behalf-of identity mapping described later.

### Service Account Authorization

The existing outage CRUD endpoints
(`POST/PATCH/DELETE /api/components/{comp}/{sub}/outages`) use
`IsUserAuthorizedForComponent`, which **does not check
`Owner.ServiceAccount`**. The MCP server's service account (SA) cannot use
these endpoints without auth changes. This is the central design challenge.

## Design Options

There are really two distinct auth problems here, and it helps to think about
them separately.

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
A K8s ServiceAccount for the MCP server in the `ship-status` namespace would
be added to `dashboard-config.yaml` owners as a `service_account` entry, and
the oauth-proxy validates the token via `TokenReview`.

#### Outer layer: chai-bot to the MCP server

The current MCP endpoint is unauthenticated (fine for read-only tools). For
write operations, the MCP server needs to authenticate callers so it can trust
on-behalf-of user claims.

#### Option A: Bearer token on the MCP endpoint

Add a shared secret or API key to the MCP server that chai-bot presents as a
Bearer token on write tool calls. The MCP server validates the token before
proxying writes to the dashboard API.

- Simple to implement -- the MCP server already runs as a Python process and
  can check a header
- Adding an auth header to chai-bot's MCP client is a small change
- Requires a shared secret to be provisioned and rotated

#### Option B: Route the MCP endpoint through oauth-proxy for writes

Add a second, protected MCP route (e.g.,
`protected.mcp.ship-status.ci.openshift.org`) that goes through the existing
oauth-proxy. Chai-bot would present a K8s SA Bearer token to this route.

- Reuses the existing oauth-proxy infrastructure
- Requires chai-bot to have a K8s SA token for the app.ci cluster -- it
  already has kubeconfigs for app.ci mounted for other tools, so this is
  feasible
- More infrastructure setup (new Route, oauth-proxy config)

#### Option C: Unauthenticated MCP endpoint, rely on dashboard-level auth

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

A Bearer token on the MCP endpoint (Option A) makes the most sense because
it's straightforward and avoids the infrastructure overhead of a second
oauth-proxy route. The inner layer (MCP server to dashboard API) follows the
existing component-monitor pattern with a K8s service account token.

### Problem 2: How does chai-bot verify a user is authorized?

When a user says "create an outage on Sippy", chai-bot needs to confirm they're
a Sippy maintainer. Owners are defined as `rover_group`s (not individual users),
and group membership is resolved via the OpenShift Groups API cache in the
dashboard backend.

#### Option A: On-behalf-of delegation

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
- Splits authorization responsibility -- chai-bot checks membership
  client-side, then the API either re-checks (redundant) or trusts chai-bot
  and skips its own check (weaker security)
- Exposes the full member list over the API, which may be a concern
- Still viable as a complementary endpoint (e.g., for "who are the admins?"
  queries), but probably shouldn't be the primary mechanism

#### Option C: Chai-bot queries OpenShift Groups API directly

Another option would be having chai-bot resolve Slack user to Kerberos ID to
OpenShift Group membership itself, then compare against `dashboard-config.yaml`
to determine if the user is a component owner.

- No SHIP Status API changes needed -- chai-bot handles everything
- Duplicates the authorization logic entirely outside the SHIP Status system
- Requires chai-bot to have its own OpenShift API access and group caching
- Risk of drift -- if the dashboard's `GroupMembershipCache` and chai-bot's
  cache disagree on group membership, behavior becomes inconsistent

#### Option D: Trust chai-bot entirely

We could also add chai-bot's SA as an owner on all components in
`dashboard-config.yaml` and let it handle access control internally in its own
logic.

- Simplest API-side implementation -- no new auth concepts or endpoints
- All authorization lives in chai-bot, not in the data system
- If chai-bot has a bug or misconfiguration, it could create outages on any
  component for any user with no server-side guardrails
- Violates the principle that the system of record should enforce its own
  access control

On-behalf-of delegation (Option A) makes the most sense because
`GroupMembershipCache` and `IsUserAuthorizedForComponent` already perform the
exact check needed. Delegation reuses them directly without duplicating group
resolution logic, and keeps authorization in the system that owns the data.

### Problem 3: How does a SA use the outage CRUD endpoints?

The existing outage endpoints use `IsUserAuthorizedForComponent`, which skips
`Owner.ServiceAccount`. This is a gap -- the MCP server's SA can't use them
as-is.

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

#### Option B: Create a dedicated endpoint for chai-bot

We could give chai-bot its own endpoint (e.g., `POST /api/chai-bot/outage`),
similar to how component-monitor has `POST /api/component-monitor/report`.

- Clean separation of concerns -- human endpoints stay unchanged
- Can have its own validation logic (e.g., require `X-On-Behalf-Of` for
  user-initiated actions, force unconfirmed for bot-initiated)
- Duplicates the outage creation logic that already exists in the handlers
  (`CreateOutageJSON`, `UpdateOutageJSON`, etc.)
- Requires maintaining a parallel API surface for what is essentially the
  same set of operations

#### Option C: Delegation middleware for trusted service accounts

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

Delegation middleware (Option C) makes the most sense because the auth
middleware (`auth.go`) already sets the context user from `X-Forwarded-User`.
Adding a delegation step that substitutes the on-behalf-of user for trusted SAs
keeps all handler code untouched, and `IsUserAuthorizedForComponent` runs
against the delegated user identity without any modification to the existing
authorization logic.

## Solution Overview

### Two Modes of Operation

**User-initiated** (on-behalf-of):

1. User asks chai-bot in Slack to create an outage
2. Chai-bot resolves the user's Slack ID to Kerberos `uid` via OrgData
3. Chai-bot calls `create_outage` on the SHIP Status MCP server, passing the
   Kerberos `uid` as the `on_behalf_of` parameter
4. MCP server authenticates chai-bot, then calls the dashboard API with its SA
   token via oauth-proxy, including `X-On-Behalf-Of: <uid>`
5. Delegation middleware sees `X-On-Behalf-Of` from a trusted SA, replaces
   the context user with the delegated identity
6. `IsUserAuthorizedForComponent` checks the delegated user's `rover_group`
   membership
7. Outage is created and attributed to the delegated user. Confirmation status
   follows normal rules (based on `RequiresConfirmation`), same as a direct
   UI action by that user.
8. Audit log records both the MCP server SA and the delegated user

**Bot-initiated** (autonomous detection):

1. Chai-bot detects an issue on its own
2. Chai-bot calls `create_outage` on the SHIP Status MCP server with no
   `on_behalf_of` parameter
3. MCP server authenticates chai-bot, then calls the dashboard API with its SA
   token via oauth-proxy, no `X-On-Behalf-Of` header
4. No delegation -- SA acts as itself
5. `IsUserAuthorizedForComponent` (extended) checks `service_account` owners
6. Outage is created as unconfirmed with severity `Suspected`
7. Team is notified via Slack

### Key Constraints

- Chai-bot must never create confirmed outages autonomously -- only
  unconfirmed/Suspected
- `X-On-Behalf-Of` is only trusted from explicitly allowed service accounts
  (configured in code or config)
- Duplicate detection: check for existing active outages before creating
- Slack notification deduplication: check if alerting is already active

## Implementation Plan

### Phase 1: Service Account and Auth Infrastructure

**MCP endpoint auth (`mcp/`):**

- Add Bearer token authentication to the MCP server for write tool calls
- Generate a shared secret, provision it to both the MCP server (as an env var)
  and to chai-bot's SHIP Status MCP client
- Read-only tools remain unauthenticated

**Kubernetes (for the MCP server's SA token to the dashboard API):**

- The MCP server needs a ServiceAccount in the `ship-status` namespace to
  authenticate to the dashboard API via oauth-proxy. This may already exist
  for the pod; if not, create one and mount a projected token.
- Ensure the oauth-proxy SA has `system:auth-delegator` ClusterRole to perform
  `TokenReview` for the MCP server's SA token

**Config (`dashboard-config.yaml` in `openshift/release`):**

- Add the MCP server's `service_account` to the owners list of every
  component, since it needs to create and manage outages for all components
  on behalf of chai-bot. This follows the existing per-component ownership
  model and requires no dashboard code changes.
- Alternatively, the dashboard could support a global "trusted delegating SA"
  concept -- a top-level config key listing SAs that can delegate on behalf of
  any user, separate from per-component ownership. This would avoid the
  maintenance burden of updating every component's owners list whenever a new
  component is added. The per-component approach is simpler to start with, but
  if the config grows unwieldy, a global key may be worth adding later.

**Auth middleware (`cmd/dashboard/auth.go`):**

- After HMAC validation, check if the authenticated user is a trusted SA AND
  `X-On-Behalf-Of` header is present
- If so, set the context user to the on-behalf-of value instead of the SA name
- Store the original SA identity in a separate context key (e.g.,
  `DelegatingServiceAccount`) for audit logging
- If the SA is not trusted or no header is present, proceed as today

**Audit logging (`pkg/types/models.go`, `pkg/outage/outage_manager.go`):**

- In the existing audit log write paths, also check the context for
  `DelegatingServiceAccount`
- If present, write both identities to the `outage_audit_logs` row: the `User`
  column records the delegated user (the person who requested the action), and
  a new `ActingServiceAccount` column records the SA that carried the request
- For bot-initiated actions (no delegation), `User` is the SA itself and
  `ActingServiceAccount` is empty

**Authorization (`cmd/dashboard/handlers.go`):**

- Extend `IsUserAuthorizedForComponent` (or add a variant) to accept
  `service_account` owners for bot-initiated actions

### Phase 2: Write Tools for Outage Management

The MCP server (`mcp/`) already has plumbing for authenticated writes:
`SHIP_STATUS_AUTH_TOKEN_FILE` for the SA Bearer token and
`SHIP_STATUS_PROTECTED_API_URL` pointing at the oauth-proxy on `127.0.0.1:8443`.
New write tools are added here. Chai-bot discovers them automatically via
`tools/list` at startup.

**New MCP tools (in `mcp/`):**

- `create_outage` -- create an outage on a sub-component. Accepts severity,
  description, and an optional `on_behalf_of` parameter (Kerberos `uid`). The
  MCP server translates `on_behalf_of` into the `X-On-Behalf-Of` header when
  calling the dashboard API.
- `resolve_outage` -- set end_time on an active outage
- `add_triage_note` -- add a triage note to an existing outage
- `get_component_status` -- query current status (already exists as read-only)
- `list_active_outages` -- list active outages for a component/sub-component

**Bot-initiated safeguards:**

- When no `on_behalf_of` is provided (bot acting autonomously), force severity
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

### Phase 4: Chai-bot Integration (could be a separate issue)

The following changes are on the chai-bot side, so they could either be
included as part of this work or tracked as a separate issue. User-initiated write tools (Phase 2) are only usable end-to-end once identity resolution is in place here.

**MCP client auth:**

Chai-bot's SHIP Status MCP client currently connects without authentication.
Once the MCP server requires a Bearer token for write tools (Phase 1), the
client needs to be updated to send the shared secret.

**User identity resolution:**

Chai-bot can already resolve Slack user IDs to Kerberos `uid` via its OrgData
tool. This is the identity that gets passed as the `on_behalf_of` parameter to
write MCP tools. Resolving the requesting user's identity at session init and
storing it in session state is likely the better approach over enabling full
individual employee lookups on the outage-creating persona, since the persona
only needs to know who is talking to it, not perform arbitrary lookups.

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
- The resolved `uid` is not authorized for the target component (403 from the
  dashboard) -- inform the user they are not a maintainer of that component
- Identity resolution is temporarily unavailable -- inform the user and
  suggest trying again later

## Test Plan

### Unit Tests

**MCP write tools (`mcp/`):**

| Test Case                         | Input                                 | Expected                                         |
| --------------------------------- | ------------------------------------- | ------------------------------------------------ |
| Write with valid Bearer token     | Valid auth + `create_outage` call     | Tool executes, calls dashboard API               |
| Write without Bearer token        | No auth + `create_outage` call        | Rejected before reaching dashboard API           |
| on_behalf_of translated to header | `on_behalf_of: "user"` parameter      | Dashboard API called with `X-On-Behalf-Of: user` |
| No on_behalf_of omits header      | No `on_behalf_of` parameter           | Dashboard API called without `X-On-Behalf-Of`    |
| Read tools remain unauthenticated | No auth + `get_component_status` call | Tool executes normally                           |

**File: `cmd/dashboard/auth_test.go`**

| Test Case                    | Input                                    | Expected                               |
| ---------------------------- | ---------------------------------------- | -------------------------------------- |
| Delegation from trusted SA   | Trusted SA + `X-On-Behalf-Of` header     | Context user is the delegated identity |
| Delegation from untrusted SA | Non-trusted SA + `X-On-Behalf-Of` header | Header ignored, SA's own identity used |
| No delegation header         | Trusted SA, no `X-On-Behalf-Of`          | SA acts as itself                      |

**File: `cmd/dashboard/handlers_test.go`**

| Test Case                            | Input                                          | Expected                                               |
| ------------------------------------ | ---------------------------------------------- | ------------------------------------------------------ |
| Delegated user authorized            | On-behalf-of user in component's `rover_group` | 201 Created                                            |
| Delegated user unauthorized          | On-behalf-of user NOT in `rover_group`         | 403 Forbidden                                          |
| Bot-initiated creates unconfirmed    | SA acts as itself, no on-behalf-of             | Outage created as Suspected, unconfirmed               |
| Bot-initiated blocked from confirmed | SA acts as itself, severity=Down               | Rejected or forced to Suspected                        |
| Duplicate detection                  | Active outage exists on sub-component          | Returns existing outage, no duplicate created          |
| Audit log records delegation         | On-behalf-of request creates outage            | Audit log contains both SA identity and delegated user |

### E2E Tests

**File: `test/e2e/dashboard_test.go`**

| Test Case                        | Input                                                          | Expected                                     |
| -------------------------------- | -------------------------------------------------------------- | -------------------------------------------- |
| Full user-initiated flow         | MCP server SA creates outage with on-behalf-of authorized user | Outage attributed to delegated user          |
| Full bot-initiated flow          | MCP server SA creates outage without on-behalf-of              | Suspected severity, unconfirmed              |
| Unauthorized delegation rejected | On-behalf-of for user not in rover_group                       | 403                                          |
| Outage lifecycle via SA          | Create, add triage note, resolve                               | All operations succeed, audit trail complete |

### Regression

The existing 60+ e2e test cases and all unit tests must continue to pass. The
delegation middleware only activates for explicitly trusted SAs with the
`X-On-Behalf-Of` header, so all existing OAuth and component-monitor flows
should be unaffected.

## Open Questions

1. **Scope of MCP tools.** Should chai-bot also update outage severity, or
   only create and resolve? Should it manage outage links?

2. **Bot-initiated severity.** Should bot-initiated outages always be
   `Suspected`, or should chai-bot be able to set `Degraded`/`Down` when it
   has high-confidence signals (e.g., confirmed by multiple probes)?

(Will add more here as they come up during implementation.)
