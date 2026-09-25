# TRT-2722: Team SLOs and TRT Watcher Canvas in SHIP Status Dash

**Date:** 2026-09-23
**JIRA:** [TRT-2722](https://redhat.atlassian.net/browse/TRT-2722)
**Author:** Stephen Goeddel

Related: [SHIPSTRAT-3](https://redhat.atlassian.net/browse/SHIPSTRAT-3), [TRT-2955](https://redhat.atlassian.net/browse/TRT-2955), [TRT-2433](https://redhat.atlassian.net/browse/TRT-2433), [TRT-2666](https://redhat.atlassian.net/browse/TRT-2666), [TRT-2790](https://redhat.atlassian.net/browse/TRT-2790), [TRT-2609](https://redhat.atlassian.net/browse/TRT-2609)

This is a **dual-repo** plan. SHIP Status Dash is the store, evaluator, and UI. Chai Bot ([openshift-eng/ship-help-bot](https://github.com/openshift-eng/ship-help-bot)) is the producer that already watches payloads and files infra outages. The Slack watcher canvas is on-demand only. Neither repo is a follow-up to the other.

## Problem Statement

TRT-2722 asked whether team SLOs need a high-visibility home-page widget or can reuse existing surfaces. The team page (`/team/:team`, TRT-2433) should be the detailed SLO workspace. The home page gets a small summary widget that bubbles key met/missed status per team and deep-links to that team's SLO section. The full watcher canvas does not live on `/`.

TRT's operational SLO is at least one accepted payload per day on watched amd64 streams. A Slack canvas on `#forum-ocp-release-oversight` still exists; Chai only writes it when someone asks. The live picture moves to ship-status (streams, recent payloads, failed jobs, Jira/outage links). Stop asking Chai to update that canvas. There is no scheduled refresh and no canvas cutover.

ship-status does not poll release-controller or Sippy to populate the SLO. Chai already does that in `PayloadCheckHandler` (`ship_help_bot/tools/_auto/payload_check/`). The bot is responsible for keeping SLO data current. Authorized humans can also add and edit the same records from the team-page UI or by asking Chai in Slack. A missed SLO is an indicator only. Never create an outage because an SLO was missed.

Incident outages on `trt-2955` ([TRT-2955](https://redhat.atlassian.net/browse/TRT-2955)) stay as ship-status outages (`trt-incidents/incidents`, `outage_per_reason`, `exclude_from_main_outage_well`). They should not remain a separate team-page card or home component well. Set `slo_component: true` on that component so list APIs omit it, then show its active outages in the team SLO incidents panel and the home SLO well. Other teams set the same flag on their incident component. Payload rows still link to those outages. They do not replace them.

## Split of responsibilities

| Layer | Owner | Does | Does not |
|-------|--------|------|----------|
| Poll Sippy + release-controller, load payload-agent YAML | Chai `PayloadCheckHandler` (`ocp_payload_ops` / `payload_check_dev`, amd64) | Already lists Accepted/Rejected tags per stream, analysis status, infra jobs | ship-status never grows this poller |
| Payload-agent analysis (TRT-2609) | Existing payload agent | Root-cause YAML/HTML for rejected payloads | Not replaced. Chai consumes the YAML. |
| Infra outages for mapped jobs | Chai `record_payload_infra_outage` (`acting_for=chai-bot`) | Create-or-link Degraded unconfirmed outages, later close from later payloads | Not an SLO miss. Keep this path. |
| TRT incident Jira cards | Chai (`trt_incident_jira` + payload_check revert flow) | File `project=TRT`, labels `trt-incident,ai-generated-jira` | ship-status has no Jira token |
| Incident outages | ship-status `jira_monitor` on `trt-incidents/incidents` | One outage per Jira issue (`outage_per_reason`) | Chai does not copy incidents into `slo_workspace_items` |
| SLO workspace facts | Chai inserts once via authenticated MCP; humans via team page or Slack | Generic rows plus versioned jsonb `details`. TRT maps payloads into `payload_streams` v1. | Scheduled handler does not rewrite an existing tag. LLM does not author the store. |
| SLO met/missed | ship-status, from stored workspace items | For TRT: count `Accepted` in 24h per YAML stream (`group_key`) | Never open/update/resolve an outage for a miss |
| Watcher UI | ship-status `/team/TRT#slo` | Versioned per-team component; recurring-job grouping, correlation, add/edit | Home page is a summary chip only |
| Slack | Chai | Payload-check alerts stay. Stop asking for canvas updates. | Do not scrape canvas HTML. No cutover or pointer rewrite. |

Empty SLO rows are a Chai lag problem (handler missed a tag, MCP write failed), not something ship-status backfills. A stored row that later goes Accepted on the release-controller stays as first written until a human asks Chai to refresh it.

## Current SHIP Status Dash constraints

- [`frontend/src/components/team/TeamPage.tsx`](frontend/src/components/team/TeamPage.tsx) is only a filtered `SubComponentList` (`GET /api/sub-components?team=`). No SLO section, no custom widgets.
- Outages are tied to component/sub-component slugs. Payload tags are ephemeral. They should not become ship-status components.
- [`GET /api/outages/during`](API_ENDPOINTS.md) already supports time-window and `team` filters. That is the join API for overlapping infra/incident outages.
- Jira probing is anonymous (`jira_monitor`). Do not mount a Jira token. Correlation can use issue keys already stored on incident outages (`Reason.Check` and `link_type=jira`).
- [TRT-2790](https://redhat.atlassian.net/browse/TRT-2790) (build02 Sippy pass ratio) was closed as Won't Do specifically because it belonged on this SLO/team page. Revisit it as a later TRT SLO source. The home widget may show its met/missed bit. The metric detail stays on `/team/TRT#slo`.

## Current Chai Bot constraints

- **Auth path already exists (TRT-2666).** Writes go to authenticated MCP behind oauth-proxy. Chai's SA is a `trusted_delegator`. Identity on the dashboard is `X-Acting-For`: `chai-bot` for autonomous work, OrgData kerberos uid when a human asked in Slack. `chai-bot` is already a `user` owner on components so bot-initiated outages authorize. Team SLO writes need the same string on `team_slos.owners`.
- **Discovery already exists.** `ship_help_bot/tools/ship_status/` builds FunctionTools from MCP `tools/list`. New `get_*` tools on the public MCP and new write tools on the auth MCP appear at persona startup. Do not hard-code a second client. Reads stay `get_*` / `list_*` so they route to the public endpoint.
- **`PayloadCheckHandler` is the poller.** `ocp_payload_ops.payload_check_dev` (every 5 minutes, `architectures: ["amd64"]`, `releases: ocp-dev`) already: asks Sippy which releases are in scope, lists terminal tags (Rejected / Accepted) on ci and nightly streams since a Firestore watermark (`scheduled_state/trt_payload_check`), loads payload-agent YAML, and records ship-status infra outages. `patch_manager.payload_check_ga` is the GA/z-stream sibling and is **out of TRT SLO v1**.
- **Infra writes are a wrapper, not raw `create_outage`.** `record_payload_infra_outage` groups jobs by component/sub-component, queries `get_outages_during`, then creates or links with `bot_initiated=True`, `acting_for=chai-bot`. SLO upserts should follow that pattern: a deterministic helper the handler calls, not an LLM turn that invents payload rows.
- **Rejected-centric today.** ship-status infra backfill runs for every Rejected tag with YAML. Accepted tags are observed only far enough to close infra outages. The SLO needs **Accepted and Rejected** (and Ready when the release-controller still shows it) on every configured amd64 stream (ci and nightly) so "1 accepted / 24h" has a complete window.
- **Slack canvas is on-demand only.** `ocp_payload_ops` and `trt_internal` can `update_canvas` on `forum_ocp_release_oversight_canvas` when a human asks. There is no scheduled canvas-refresh job. Do not scrape that canvas, rewrite it as a pointer, or dual-write. Once `/team/TRT#slo` is live, stop asking Chai to update it. Payload check still posts actionable Slack (reverts, force-accept, newly created ship-status outages).
- **Incident filing stays on Chai.** Revert buttons already create TRT Jira incidents. `trt_internal` has `trt_incident_jira` instructions. The ship-status `jira_monitor` turns those issues into `trt-incidents/incidents` outages. SLO workspace tools only **link** an item or job group to that outage.
- **RWS exposure.** Raw outage writes stay coordinator-only. `record_payload_infra_outage` is RWS-exposed only for `ocp_payload_ops`. SLO upsert wrappers should match: handler + that persona, not every workspace worker.
- **Missed SLO must not call `create_outage`.** Instructions need an explicit rule. Infra and incident outages remain the only outage writers.

## Recommended product shape

Treat this as three layers:

1. **Home-page SLO summary** (small, all teams): met/missed chips, one line of context, compact `slo_component` incident rows, link to `/team/{team}#slo`. Not the watcher canvas. Does not light the ship-on-fire logo.
2. **Generic SLO status** on the team page: named objectives, window, met/missed, last evaluation (`id="slo"`), plus an incidents panel when the team owns a `slo_component`.
3. **SLO workspace** (pluggable per team, writable, versioned): generic rows (`kind`, `schema_version`, `item_key`, `group_key`, `outcome`, jsonb `details`). TRT's first extra workspace is the amd64 payload watcher (`kind: payload_streams`, `schema_version: 1`). Chai maps stream/tag/phase into those fields. The team page renders with a versioned component for that team and schema, not a generic jsonb explorer. Other teams omit a workspace, or upsert a different `kind`+version into the same tables, and still get the SLO strip and incidents panel.

```mermaid
flowchart TB
  subgraph sources [Sources Chai already polls]
    Sippy[Sippy releases API]
    RC[Release-controller amd64]
    Agent[Payload-agent YAML TRT-2609]
  end

  subgraph chai [Chai Bot ship-help-bot]
    Handler[PayloadCheckHandler ocp_payload_ops]
    InfraWrap[record_payload_infra_outage]
    SloWrap[upsert SLO workspace items plus links]
    Jira[TRT incident Jira]
    SlackMsg[Slack alerts]
  end

  subgraph shipStatus [SHIP Status Dash]
    YAML[Dashboard YAML SLOs plus owners]
    Store[Persisted SLO workspace]
    Incidents[Incident outages TRT-2955]
    InfraOut[Infra outages]
    WriteAPI["PUT /api/teams/{team}/slo/items"]
    AuthMCP[Authenticated MCP write tools]
    DetailAPI["GET /api/teams/{team}/slo"]
    SummaryAPI["GET /api/teams/slo-summary"]
    TeamPage["/team/TRT#slo"]
    Home[Home SLO widget]
  end

  Sippy --> Handler
  RC --> Handler
  Agent --> Handler
  Handler --> InfraWrap --> InfraOut
  Handler --> SloWrap --> AuthMCP --> WriteAPI --> Store
  Handler --> Jira --> Incidents
  Handler --> SlackMsg
  UI[Team page frontend] --> WriteAPI
  SlackHuman[Slack user via Chai] --> AuthMCP
  YAML --> DetailAPI
  Store --> DetailAPI --> TeamPage
  Store --> SummaryAPI --> Home
  Home -->|"team + #slo"| TeamPage
  Incidents --> DetailAPI
  InfraOut --> DetailAPI
```

### TRT team page layout

Keep the existing sub-component grid below. Add sections above it:

**SLO strip** (all teams with config, `id="slo"`): e.g. "Accepted payload / 24h: 3 of 4 streams meeting target." This is the target of the home-page deep link. Missed SLOs are prominent here as status only. Do not light the ship-on-fire logo. Do not create an outage when an SLO is missed.

**Incidents panel** (any team that owns a `slo_component: true` component): active outages from that component, with Jira links and jump-to outage details. This is where `trt-incidents/incidents` is shown for TRT. Same widget for ART/CRT/DPTP when they mark their incident component. Not a `SubComponentCard` in the grid.

**Watcher canvas** (TRT `payload_streams` v1 workspace, amd64 only):

- One table per stream in YAML `workspace.streams` (amd64 ci and nightly). The mock shows 5.0 and 5.1. When 5.0 GAs, remove those names from YAML. No arm64/multi/ppc/s390x.
- Last N payloads **on the team page** (`recent_payloads`). The store keeps every tag still inside the SLO `window` so evaluation is not limited to those N rows. Home does not list payloads.
- Recurring-job grouping: same blocking job failing on 2+ consecutive payloads in that stream. Computed by ship-status from stored rows so Chai does not have to send a grouping structure.
- Every payload row links to its release-controller page (`details.payload_url`). Rejected (and Ready, when the agent has output) also link to payload-agent analysis HTML (`details.analysis_url`).
- Each failed blocking job can carry a `notes` string (bot or human). That is separate from payload-wide `slo_workspace_items.notes`.
- Other links: Prow (job URL), Jira, ship-status outage details (and any extra URLs attached on `slo_workspace_links`).
- Correlated ship-status objects beside a payload or a failure group:
  - TRT incident outages whose Jira key appears on the group, or whose window overlaps payload evaluation.
  - Infra outages (`GET /api/outages/during`) so "build02 down" explains a GCP job streak. These are the outages Chai already opened via `record_payload_infra_outage`.
  - Explicit outage/Jira links attached by the bot or the UI.

Authorized users on the team page can add a payload, edit phase/jobs/per-job notes/payload notes, and add/remove links. Same protected APIs as MCP. Empty/stale data is a bot-lag problem, not something ship-status backfills from release-controller.

### Visual mockup

Static mock of `/team/TRT` with the SLO strip, incidents panel, amd64 payload streams, and the existing Sippy cards. Sample payload/incident data. Layout is based on the live team page ([ship-status.ci.openshift.org/team/TRT](https://ship-status.ci.openshift.org/team/TRT)) as of 2026-09-25, when that page listed Sippy, Sippy-Auth, and Incidents.

Yellow italic lines labeled **Mock caption** are annotations for these screenshots, not product copy.

![Mock of the TRT team page with SLO strip, incidents panel, amd64 payload streams, and Sippy cards](slo-team-page-mockup.png)

Home page: In Outage stays first. A Team SLOs well under it lists per-team roll-up and, in a nested well labeled with the component and sub-component names, the `slo_component` incident outages. TRT Incidents is not a component well. Payload tables stay on the team page.

![Mock of the home page SLO well with TRT roll-up and incident rows](slo-home-page-mockup.png)

What changed vs today:

- Team header title is the team name (`TRT`), not `TRT Sub Components`.
- New SLO strip (`#slo`) and incidents panel (`#incidents`) above the grid.
- `Incidents` is not a sub-component card (`slo_component: true` on TRT Incidents). Sippy and Sippy-Auth remain.
- Payload streams are tables per amd64 nightly and ci stream (5.0 and 5.1 in the mock). Each payload has a Release controller link. Rejected rows also have a Payload agent link. Failed jobs show Prow links, optional per-job notes, recurring badges, and Jira/outage links.
- Add/edit controls are shown as authorized-user actions. They are not on the public read-only view for anonymous visitors in the real app.

### Home-page SLO widget

Place a compact well on [`frontend/src/components/ComponentStatusList.tsx`](frontend/src/components/ComponentStatusList.tsx) immediately below `UnhealthyWell` (In Outage) and above the component wells. In Outage is always the top well when it has items. Do not put SLOs above it.

Per team in the union of `team_slos` entries and `ship_team` values on any `slo_component` (incident-only teams must appear):

- Team name (existing `TeamChip` color) linking to `/team/{team}#slo`
- If the team has `team_slos`: roll-up (all met / N of M missed), worst-miss hint (e.g. "5.0 nightly: last accepted 32h ago"), stream chips. Click-through goes to `#slo`, not a payload row.
- If the team owns a `slo_component`: a nested well inside that team's SLO block, labeled with the component name and sub-component name (e.g. `TRT Incidents` / `Incidents`). Compact incident rows in that well, not loose under the SLO chips. Link to outage details and `/team/{team}#incidents`.
- A team with only `slo_component` (no `team_slos`) still gets a block: chip plus nested incident well, no payload roll-up.

Rules that keep `/` from becoming the canvas:

- No payload lists, job failures, or correlation on the home widget
- Teams with no SLO config and no `slo_component` are omitted
- Missed SLOs do not put the team in the In Outage well, do not light the ship fire, and do not create an outage.
- Hide the well entirely if no teams have SLOs or `slo_component`s (local/e2e without YAML)
- A `slo_component` is omitted from home `ComponentWell`s. Its outages appear in this SLO well (and the team SLO incidents panel), not as a component card.

### Folding incident components into SLOs

Keep TRT-2955's data model: one ship-status outage per Jira issue, Jira probe, `outage_per_reason`, auto-resolve, existing MCP outage tools. Do not copy incidents into `slo_workspace_items`.

Do not derive hide-from-list from `team_slos`. That couples two configs and is easy to get wrong. Add an explicit component flag:

```yaml
  - name: "TRT Incidents"
    description: "TRT Jira incidents labeled trt-incident"
    ship_team: "TRT"
    slo_component: true
    sub_components:
      - name: "Incidents"
        exclude_from_main_outage_well: true
        monitoring:
          outage_per_reason: true
          auto_resolve: true
```

`slo_component: true` on the **component**:

Wire it through the config contract, not only YAML examples:

- [`pkg/types/config.go`](pkg/types/config.go) `Component`: add `SLOComponent bool` with `json:"slo_component,omitempty" yaml:"slo_component,omitempty"`. Same pattern as `ExcludeFromMainOutageWell` on the sub. YAML load already unmarshals `Component`; without this field the flag is dropped.
- Frontend [`frontend/src/types.ts`](frontend/src/types.ts) `Component`: add `slo_component?: boolean` so clients can ignore flagged rows if a list handler ever leaks one.
- Set `slo_component: true` on TRT Incidents in [`hack/local/dashboard/config.yaml`](hack/local/dashboard/config.yaml) and the production file in `openshift/release`.

Behavior:

- `GET /api/components` omits it, so there is no home `ComponentWell`.
- `GET /api/sub-components` omits its subs, so `/team/TRT` has no Incidents card (Sippy stays).
- It still does not appear in the In Outage well or light the ship (`exclude_from_main_outage_well` stays on the sub as today; `slo_component` does not replace that).
- `GET /api/teams/{team}/slo` and `GET /api/teams/slo-summary` include its active outages for `ship_team`.
- Home SLO well and `TeamSLOIncidents` render those outages. On home, they live in a nested well labeled with the component and sub-component names.

`team_slos` does not need an `incidents:` pointer. Discovery is: components with `slo_component: true` and matching `ship_team`. A team can have a `slo_component` with no payload SLO, or a payload SLO with no `slo_component`.

| Today | After |
|-------|--------|
| `TRT Incidents` is a home component well and a `/team/TRT` card | Hidden from those grids via `slo_component: true` |
| Incidents only excluded from the In Outage well / ship fire | Also excluded from home component wells and the team sub-component list |
| Team page is a flat card grid | SLO section owns the incident list; home SLO well shows the same outages compactly |

Outage create/update/link/triage stays on the existing component APIs and MCP tools. The SLO panel is a view plus deep links. Payload correlation uses these outages first (Jira key, time overlap, and explicit `slo_workspace_links`).

Other teams reuse this with zero TRT UI: add a Jira-monitored incident component, set `slo_component: true` and `ship_team`. Payload workspace remains optional.

### Other teams (same framework, later sources)

Other teams reuse the same SLO page with no TRT-specific UI:

- **Incidents panel first.** Any team can add a Jira-monitored incident component (copy TRT-2955), set `slo_component: true` and `ship_team`. That is enough to get the panel and the home SLO well rows.
- **ART / CRT / DPTP SLOs** later: content cadence, payload creation interval, mass failures (`prometheus` / `time_since_event` sources, or bot upserts into the same workspace tables with a new `(kind, schema_version)`). Each of those teams gets its own versioned workspace component under `frontend/src/components/team/slo/{team}/v{n}/`. They do not reuse `trt/v1`. Chai does not need a TRT-shaped handler for those teams in v1.

## Data sources

| Need | Source | Auth |
|------|--------|------|
| Entire TRT SLO workspace (items, jobs in jsonb, notes, Jira keys) | Chai `PayloadCheckHandler` via MCP, or authorized humans via the team-page UI / Slack | oauth-proxy + HMAC + team SLO owners (`chai-bot` for bot-initiated) |
| Open TRT incidents as outages | Existing `jira_monitor` + `outage_per_reason` (issues Chai already files) | Anonymous Browse |
| Overlapping infra/incident outages | `GET /api/outages/during` (includes Chai's `record_payload_infra_outage` rows) | Public read |
| Slack narrative (reverts, force-accept) | Unchanged payload_check Slack path | Existing |

SHIP Status Dash does not poll release-controller, Sippy, or Slack to fill the SLO. Chai (or a human) must upsert workspace items. Do not scrape the Slack canvas. Once the team page is live, stop asking Chai to update it.

## Config model

Add team-scoped SLO config to dashboard YAML (production file lives in `openshift/release` `core-services/ship-status/`. Local mirror in [`hack/local/dashboard/config.yaml`](hack/local/dashboard/config.yaml)). Named SLOs attach to team. Incident components use `slo_component: true` on the component. `workspace.kind` plus `workspace.schema_version` is the contract Chai and ship-status share. `payload_streams` v1 is TRT's first pair, not the table layout.

Sketch:

```yaml
team_slos:
  - team: TRT
    owners:   # required. Same shape as component owners. Include chai-bot.
      - rover_group: "technical-release-team"
      - user: "chai-bot"   # bot-initiated acting-for, same as TRT-2666
    slos:
      - name: accepted-payload-per-day
        display_name: "1 accepted payload per day"
        source: payload_acceptance
        workspace:
          kind: payload_streams
          schema_version: 1    # required when workspace is set; Chai and the UI must match
          recent_payloads: 5   # team-page list size only; evaluation uses the hardcoded 24h window
          streams:
            - controller: amd64
              name: "5.1.0-0.nightly"
            - controller: amd64
              name: "5.1.0-0.ci"
            - controller: amd64
              name: "5.0.0-0.nightly"
            - controller: amd64
              name: "5.0.0-0.ci"
```

`source` is the extension point. Unknown sources are ignored so other teams can land config before ship-status implements their evaluator.

### Stream lifecycle (drop 5.0, add 5.2)

The watched set is `workspace.streams` in dashboard YAML (`openshift/release`, git-sync). Edit that list when a version GAs or a new nightly opens. No Sippy snapshot API.

**Add 5.2:** PR the two amd64 names onto `workspace.streams`. After git-sync, evaluation and the UI include those tables. Chai starts inserting new 5.2 tags on the next tick. Until the first `Accepted` in `window`, that stream is a miss.

**Drop 5.0:** PR those names off the list. After git-sync they leave the SLO strip, home chips, and payload tables. Chai stops inserting new 5.0 tags. Existing 5.0 rows stay until normal window prune. Do not DELETE them. Do not keep scoring them: leftover 5.0 Rejected tags must not miss the team SLO after we stopped watching.

Do not infer the watched set from leftover `group_key`s in the database. YAML is the list.

### How met/missed is computed

This is not a generic query over jsonb. Ship-status registers evaluators in Go, keyed by `source`. YAML only names the evaluator (`source: payload_acceptance`). Window and target live in that Go code, not YAML: 24h and `min_accepted: 1`. Later teams add `prometheus` or `time_since_event` the same way: new Go code plus a new `source` string, not a new table.

`payload_acceptance` (TRT, this repo):

1. Take `workspace.streams` from YAML (not every `group_key` in the table).
2. For each of those streams, select stored items with `kind=payload_streams`, `group_key` equal to that stream name, and `occurred_at` inside the last 24h.
3. Count items whose `outcome` is `Accepted`.
4. That stream is met if the count is at least 1.
5. The named SLO is met when every YAML stream is met. Names not in YAML are ignored even if rows remain.

Those steps use version-stable columns only (`group_key`, `occurred_at`, `outcome`). Job notes, payload URLs, and recurring-job grouping are display. They do not change met/missed.

Results are computed on read (no evaluation table). They go to the UI on the public GET APIs below. Never open, update, or resolve a ship-status outage because an SLO was missed.

Generic across teams: YAML shape, evaluator registry, result shape, `TeamSLOStatus` strip. Not generic: pretending every team's SLO is `payload_acceptance`.

`schema_version` is an integer on the workspace, not a YAML comment. Ship-status owns the document for each `(kind, schema_version)` (Go types plus a JSON schema in this repo). Chai's upsert wrapper must send that version. A bump is a coordinated change: ship-status validator and renderer first, then Chai producer, then YAML. Do not silently coerce an unknown version.

`owners` is required (same shape as component owners). Include `user: chai-bot` for bot-initiated writes. `IsUserAuthorizedForTeamSLO` uses only this list. Writes never authorize against the public route.

## Backend design

Persist the workspace. Do not poll release-controller. SLO met/missed is computed only from stored rows the bot or UI wrote.

Do not make `slo_payloads(stream, tag, phase)` a first-class schema. Those names are TRT payload-controller vocabulary. ART/CRT/DPTP will not have streams or tags.

Tables (names indicative):

- `slo_workspace_items`: `team`, `kind` (same as YAML `workspace.kind`, e.g. `payload_streams`), `schema_version` (integer, required), `item_key` (opaque unique id within team+kind), optional `group_key` (UI/eval grouping), `occurred_at`, `outcome` (opaque string), `details` jsonb, `notes`, `updated_by`, `updated_at`. Unique `(team, kind, item_key)`.
- `slo_workspace_links`: item id, url, `link_type` (`jira` / `outage` / `other`), optional `outage_id`.

`details` is PostgreSQL jsonb (same as `outage_audit_logs.old` / `.new`). Its shape is defined by `(kind, schema_version)`, not by team name and not by "whatever Chai sent today". Ship-status validates upserts against that document. It does not add typed job columns when Chai grows a field. Bump `schema_version` instead.

`schema_version` lives on the row (and in YAML), not buried only inside jsonb. The frontend must know which renderer to load before it parses `details`.

Write rules:

- Reject upserts with a missing `schema_version`.
- Reject upserts whose `(kind, schema_version)` this ship-status build does not know, or whose `details` fail that version's JSON schema.
- Accept older versions that this build still has a validator and renderer for, so a 24h window can mix v1 and v2 during a rollout.
- Do not rewrite stored `details` in place when bumping. New tags arrive at the new version. Existing rows stay until a human asks Chai to refresh that payload.
- Scheduled Chai writes each `(team, kind, item_key)` once. Later handler ticks skip that key. Replace only when a human asks (Slack refresh skill or team-page edit). Last write then wins, including `jobs[].notes`. Chai is the usual author on the first insert (payload-agent text). Humans can add or edit afterward without the next tick clobbering them.

TRT mapping (producer and `payload_streams` v1 UI only, not the generic table):

| Generic column | TRT v1 value |
|----------------|-----------|
| `kind` | `payload_streams` |
| `schema_version` | `1` |
| `group_key` | stream name (`5.1.0-0.nightly`) |
| `item_key` | payload tag |
| `occurred_at` | tag timestamp |
| `outcome` | `Accepted` / `Rejected` / `Ready` |
| `details` | v1 document: release-controller URL, payload-agent analysis URL, failed blocking jobs with optional per-job notes |

TRT `payload_streams` v1 `details` (indicative, frozen in the JSON schema this repo will ship):

```json
{
  "payload_url": "https://amd64.ocp.releases.ci.openshift.org/releasestream/5.1.0-0.nightly/release/5.1.0-0.nightly-2026-09-25-060000",
  "analysis_url": "https://storage.googleapis.com/test-platform-results/payload-agent/5.1.0-0.nightly-2026-09-24-180000.html",
  "jobs": [
    {
      "name": "periodic-ci-...",
      "url": "https://prow.ci.openshift.org/...",
      "state": "failure",
      "blocking": true,
      "notes": "Same disruption as TRT-4120. Not infra."
    }
  ]
}
```

`payload_url` is required (release-controller page for that tag). `analysis_url` is the payload-agent HTML when present (typical for Rejected). `jobs[].notes` is optional. Payload-wide notes stay on `slo_workspace_items.notes`, not in this document. A field added later is a new `schema_version`, not a quiet extra key on v1.

Idempotent **insert** by `(team, kind, item_key)` on the scheduled path: if the row exists, skip. Persist every stored row whose `occurred_at` still falls inside the SLO `window` (24h for TRT). `recent_payloads` is a team-page UI cap only for `payload_streams`. The home widget does not list workspace rows (roll-up, worst-miss, and compact `slo_component` incident rows). Do not prune stored rows down to N. An in-window `Accepted` outcome must remain available to `payload_acceptance` even if later Rejected tags have pushed it off the visible list. Prune only rows that are outside both the evaluation window and the last-N display set.

**Public read APIs:**

- `GET /api/teams/{team}/slo`: team page. Evaluations from stored items in the evaluator's window (not limited to last N), `incidents`, last-N workspace items for YAML streams, and `workspace.schema_version`. Names removed from YAML are omitted even if rows remain.
- `GET /api/teams/slo-summary`: home widget. One block per team in the union of `team_slos` and `slo_component` `ship_team`s. Roll-up when `team_slos` exists; compact incident rows grouped by `slo_component`. No workspace item lists. Home does not load versioned workspace components.
- `GET /api/components` and `GET /api/sub-components` omit `slo_component: true` components (and their subs).

The evaluator returns display fields (`window`, `target`) so the UI does not hardcode 24h / min 1. Indicative team-page body:

```json
{
  "team": "TRT",
  "workspace": { "kind": "payload_streams", "schema_version": 1, "recent_payloads": 5 },
  "evaluations": [
    {
      "name": "accepted-payload-per-day",
      "display_name": "1 accepted payload per day",
      "source": "payload_acceptance",
      "window": "24h",
      "target": { "min_accepted": 1 },
      "met": false,
      "groups": [
        { "key": "5.1.0-0.nightly", "accepted": 1, "met": true, "last_accepted_at": "2026-09-25T06:00:00Z" },
        { "key": "5.0.0-0.nightly", "accepted": 0, "met": false, "last_accepted_at": "2026-09-23T22:00:00Z" }
      ]
    }
  ],
  "incidents": [],
  "items": []
}
```

`TeamSLOStatus` reads `evaluations`. `PayloadStreamsWorkspace` reads `items` plus `workspace`. Home `GET /api/teams/slo-summary` is the same evaluations rolled up (met count, worst-miss `key` + `last_accepted_at`) plus compact incidents. No payload `items`.

**Protected write APIs** (oauth-proxy + HMAC + `IsUserAuthorizedForTeamSLO`). Used by MCP and the frontend:

- `PUT /api/teams/{team}/slo/items`: create or replace one workspace item (`kind`, `schema_version`, `item_key`, `group_key`, `occurred_at`, `outcome`, `details`, `notes`).
- `PATCH` (or PUT of a subset) for edits.
- `PUT /api/teams/{team}/slo/items/{kind}/{item_key}/links`: attach Jira or outage.
- `DELETE` for item or link mistakes.

`payload_acceptance` is the evaluator named in YAML (see above). Recurring-job grouping and any other `details` parsing stay versioned next to the renderer. Other `source` values do not use this table in v1 (`prometheus` / `time_since_event`).

A later team workspace reuses the same tables with a new `(kind, schema_version)` and its own `details` document. It does not add `stream` / `tag` / `phase` columns. It does ship a new versioned frontend component.

## Chai Bot (ship-help-bot) design

Reuse the TRT-2666 path. Contract lives in this repo (REST + MCP). Producer lives in ship-help-bot.

### MCP contract (this repo)

**Public MCP (read):**

- `get_team_slo(team)`
- `get_team_slo_summary()`

Name them `get_*` so Chai's ship_status discovery routes them to the public endpoint automatically.

**Authenticated MCP (write), `acting_for` required:**

- `upsert_slo_item(team, kind, schema_version, item_key, group_key, occurred_at, outcome, details, notes, acting_for, ...)`
- `add_slo_item_link(team, kind, item_key, url, link_type, outage_id?, acting_for)`
- Existing `create_outage` / `add_outage_link` stay the way to file a TRT incident or infra outage. Workspace tools link to that outage rather than duplicating it.

TRT's handler maps stream/tag/phase/jobs into those arguments and always sets `schema_version` to the value ship-status has published for `payload_streams`. It does not require MCP tools named `stream` / `tag` / `phase`. Other teams upsert the same tools with a different `kind`, `schema_version`, and `details` document. If ship-status rejects the version, treat it like any other failed upsert (Slack a human). Do not guess a fallback schema.

Bot-initiated: `acting_for: chai-bot` (locked, same owner string as TRT-2666). User-initiated in Slack: OrgData uid as today. Frontend writes use the logged-in user (no acting-for). Dashboard audit logs the acting identity (`chai-bot` or the human).

Do not mount a Jira token on ship-status. Chai already has Jira tools. It sends issue keys/URLs in the upsert or as links.

### Producer: extend PayloadCheckHandler, do not add a second poller

Owner persona: **`ocp_payload_ops`** (`payload_check_dev`, amd64, `ocp-dev`). Not `patch_manager` (GA) and not a new scheduled persona.

On each tick, after the existing Sippy / release-controller / YAML load:

**Write each payload once.** The scheduled handler inserts a row the first time it sees a terminal tag that is not already in ship-status. It does not upsert that tag again on later ticks, even if phase, jobs, or payload-agent analysis changed. A human must ask for an update (Slack refresh skill or team-page edit). Adding a Jira or outage **link** after the fact is not a payload rewrite.

1. For each YAML `workspace.streams` name (amd64 ci and nightly), find new terminal tags in the recent window (Accepted, Rejected, and Ready if still listed). Skip streams not in YAML. Skip any `item_key` (tag) that already exists for that team and `kind`. For each new tag, `upsert_slo_item`: `kind=payload_streams`, `schema_version` from the published TRT contract (1 in v1), `group_key=stream`, `item_key=tag`, `outcome=phase`, `occurred_at` from the tag. Always set `details.payload_url` to the release-controller page. Set `details.analysis_url` when payload-agent HTML exists. Put failed blocking jobs (Prow URL, state, `notes` from payload-agent when present) in `details.jobs`.
2. Keep calling `record_payload_infra_outage` for Rejected tags with mapped infra jobs. Unchanged. That path may still run on later ticks; it does not rewrite the SLO payload row.
3. When that wrapper creates or links an outage, also `add_slo_item_link(..., link_type=outage, outage_id=...)` if the payload row exists.
4. When the Slack revert flow files a TRT incident Jira, `add_slo_item_link(..., link_type=jira)` for the affected payload(s). Do not wait for jira_monitor; the incidents panel will catch up.
5. SLO **inserts** on this tick stay best-effort relative to the Firestore watermark: do not hold the watermark forever if ship-status is down (same as today's infra writes). When an insert fails, post a Slack message a human can act on. Include stream, tag, error, and whether the watermark advanced past that tag. There is no automatic failed-tag queue and no automatic post-watermark Ready-to-Accepted rewrite in v1.

Human recovery after a failed or skipped insert (watermark may already have moved), or when a stored row is stale (Ready later Accepted, new analysis, bad notes):

- Ask Chai in Slack to refresh that stream and tag. This is a required **interactive skill** in ship-help-bot: look up the current phase (and jobs) from the same Sippy / release-controller sources `PayloadCheckHandler` already uses, then `upsert_slo_item` **replacing** the row. Independent of the Firestore watermark. The only scheduled-handler exception. The LLM only chooses to invoke it on explicit user intent. It does not invent phase or jobs.
- Or team page add/edit (same protected `PUT` as MCP).

The Slack failure message should point at the interactive skill and link `/team/TRT#slo`. Do not add a second scheduled poller or a failed-tag queue. If nobody asks Chai, the first insert stays as written.

Implement a coordinator-side wrapper (same shape as `record_payload_infra_outage` in `payload_infra.py`): `acting_for=chai-bot`, groups/idempotent, sandbox fake for tests. The LLM must not be the thing that decides payload rows.

Persona-callable tools remain for humans ("add a note on 5.1 nightly 2026-09-23-…", "link TRT-1234 to this payload", "refresh 5.1.0-0.nightly-2026-09-25-060000 on the SLO"). Those use OrgData `acting_for`. Instructions: only on explicit user intent, same as raw `create_outage`.

### Slack canvas

The oversight canvas is on-demand only. No scheduled writer, so no cutover: do not dual-write, do not replace the canvas body with a ship-status URL, do not scrape it. After `/team/TRT#slo` is the live view, stop asking Chai to `update_canvas` for payload/SLO status.

**Alerts stay in Slack.** Revert / force-accept / new infra outage posts from payload_check do not move into the dashboard. Failed SLO inserts also post to Slack (stream, tag, error, watermark status) so a human can ask Chai to refresh that tag.

### Instructions and safety

Update in ship-help-bot (not this repo):

- `ship_help_bot/tools/ship_status/instructions/02_write_tools.md`: new upsert/link tools, `acting_for` rules, **never create an outage because an SLO was missed**.
- `ship_help_bot/tools/_auto/payload_check/README.md` and handler module doc: insert-once SLO steps beside infra backfill. Do not rewrite an existing tag on later ticks.
- `trt_payload_check_handler.md`: mention the ship-status team-page URL when posting; do not treat SLO miss as an incident. Slack failed SLO inserts with stream, tag, error, watermark status, and how to ask Chai to refresh that tag.
- Interactive skill: on explicit user intent, fetch current phase for a named stream and tag and **replace** the row. Independent of the Firestore watermark. Same skill for a failed first insert, a missing payload, or a stale `Ready` row. The scheduled handler does not do this.
- RWS: expose the upsert wrapper only to `ocp_payload_ops`, same as `record_payload_infra_outage`. Keep raw SLO writes off workspace workers.

### What Chai does not do in v1

- Evaluate met/missed (ship-status does that from stored rows).
- Recurring-job grouping (ship-status derives it).
- Poll on behalf of ART/CRT/DPTP SLOs.
- Replace the payload agent.
- Open a ship-status outage per rejected payload or per missed SLO.
- Automatically rewrite a payload the scheduled handler already inserted (phase change, new analysis, Ready-to-Accepted). Humans ask Chai to refresh a specific tag.
- Invent or coerce `schema_version`. If ship-status rejects the document, Slack a human.

## Frontend design

**Team page** ([`frontend/src/components/team/TeamPage.tsx`](frontend/src/components/team/TeamPage.tsx)):

1. Fetch `/api/teams/{team}/slo`. If empty, keep today's page.
2. `TeamSLOStatus` strip with `id="slo"` (generic, all teams) from `evaluations` on that response. Scroll into view when the hash is `#slo`.
3. If the team has a `slo_component`, render `TeamSLOIncidents` (`id="incidents"`) with active outage rows linking to existing details pages.
4. If the team has a workspace, render from a **versioned per-team registry**. Do not parse `details` in a shared generic table.
5. Existing `SubComponentList` unchanged except list APIs no longer return `slo_component` subs.

Workspace registry (indicative):

```
frontend/src/components/team/slo/
  TeamSLOStatus.tsx
  TeamSLOIncidents.tsx
  registry.ts
  unknown/UnknownSLOWorkspace.tsx
  trt/v1/PayloadStreamsWorkspace.tsx
```

`registry.ts` maps `(team, kind, schema_version)` to a React component. TRT v1 is `payload_streams` / `1` → `trt/v1/PayloadStreamsWorkspace`. ART later adds `art/v1/...`. Bumping TRT's `details` shape means `trt/v2/...` plus a ship-status JSON schema `payload_streams` v2. Leave v1 mounted until no displayed rows still use it.

Team page lookup:

- Group displayed items by `schema_version`.
- For each group, load `registry[team][kind][schema_version]`.
- Unknown pair: `UnknownSLOWorkspace` (kind, version, item count). Do not guess fields. Do not crash the rest of the team page.
- When the viewer is authorized, the versioned component owns add/edit for that schema (TRT v1: new payload, edit phase/jobs/per-job notes, RC and analysis URLs, links). Writes include `schema_version`.

Home stays generic. It never imports team workspace components.

Deep links: `/team/TRT#slo` from the home widget, `/team/TRT#stream-5.1.0-0.nightly` within the TRT v1 canvas, outage details and Jira browse URLs from failure groups.

**Home page** ([`frontend/src/components/ComponentStatusList.tsx`](frontend/src/components/ComponentStatusList.tsx)):

1. Fetch `/api/teams/slo-summary` next to the existing components/status polls.
2. If the payload is non-empty, render `TeamSLOSummaryWell` **after** `UnhealthyWell` and before the component wells. In Outage stays the top well. Include teams that have only a `slo_component`.
3. Each team block is a `TeamChip` plus SLO roll-up plus worst-miss hint. Under that, a nested well per `slo_component` labeled with component and sub-component names, containing compact incident rows. "View SLO" navigates to `/team/{team}#slo`. Incident rows link to outage details.

Public route is read-only. All SLO mutations (bot MCP and frontend add/edit) use the protected route and `IsUserAuthorizedForTeamSLO`. `chai-bot` is an owner for bot-initiated MCP. Human UI users must be a rover-group/user owner of that team SLO.

## Correlation with incident outages (TRT-2955)

The SLO incidents panel is the TRT-2955 list, not a second copy. Correlation is then:

- Show those active outages in `TeamSLOIncidents`.
- For each recurring job group, list incident outages whose Jira key is already linked (`add_slo_item_link` or `Reason.Check` match), or whose window overlaps the failing payload span.
- For each payload, also list overlapping Build Farm / Prow outages so infra vs product is visible. Those rows are often ones Chai already created with `record_payload_infra_outage`.
- Click through to existing outage details (triage notes, Slack thread, Jira).

Chai keeps filing incident Jira (then ship-status `jira_monitor`) and infra outages via existing MCP outage tools. Workspace tools only link an item/job group to that outage. Per-job notes stay on `details.jobs[].notes`. Payload-wide notes stay on `slo_workspace_items.notes`. Incident write-up stays on the outage.

## Phasing

Work both repos in this order. ship-status contract first so Chai can integrate against it.

1. **ship-status: config + store + read APIs + SLO strip + home summary.** Persist empty workspace. Team page `id="slo"`. Home widget links to `/team/{team}#slo`. Include `chai-bot` on `team_slos.owners` in local YAML.
2. **ship-status: `slo_component` + incidents panel.** Flag on TRT Incidents, omit from home/team list APIs, `TeamSLOIncidents` plus incident rows on the home SLO well. TRT-2955 stays the outage backend.
3. **ship-status: protected writes + authenticated MCP** for workspace items. `upsert_slo_item` / `add_slo_item_link`, required `schema_version`, JSON schema validation. Wire local e2e with chai-bot SA and `X-Acting-For`. This is the contract Chai consumes.
4. **ship-status: watcher workspace UI** as a versioned per-team registry (`trt/v1` first) plus frontend add/edit. JSON schema for `payload_streams` v1. Unknown versions render the fallback, not a guessed table. Join payload rows to the incidents panel.
5. **Chai: deterministic SLO inserts** in `PayloadCheckHandler` / `payload_infra`-style wrapper. One write per tag unless a human asks to refresh. Send `schema_version: 1` with the v1 details document. Accepted + Rejected (+ Ready) on every configured amd64 stream (ci and nightly). Link infra outages and Jira keys. Slack on insert failure with enough detail for a human to replay. Instructions: never outage-on-SLO-miss; stop asking for Slack canvas updates. Tests around the handler, not wording in an LLM reply.
6. **Other teams** add `slo_component: true` (and later their own `team_slos`) without a `payload_streams` workspace and without a Chai payload handler. If they later push facts, they reuse `slo_workspace_items` with a new `(kind, schema_version)` and a new versioned frontend component.

SHIP Status Dash v1 is complete when Chai can upsert and the team page renders it. Chai v1 is complete when amd64 ci and nightly tags land in ship-status on the existing 5-minute tick without an LLM authoring the rows.

## Explicit non-goals (v1)

- Putting the payload watcher canvas on the home page.
- Lighting the ship-on-fire logo, adding SLO misses to the In Outage well, or creating any outage when an SLO is missed.
- One ship-status outage per rejected payload.
- Copying incident outages into `slo_workspace_items`. Incidents stay outages. The SLO page is the view.
- Polling release-controller, Sippy, or Slack from the dashboard to populate SLO data.
- A second Chai poller, scheduled rewrite of an existing payload row, or using `patch_manager.payload_check_ga` for this SLO.
- Letting an LLM turn be the source of SLO workspace rows.
- Non-amd64 streams (arm64, multi, ppc64le, s390x).
- Scraping, mirroring, dual-writing, or rewriting the Slack canvas as a pointer. It is on-demand only; stop asking for updates.
- Iframe of Sippy or the edge payload-monitor HTML.
- Replacing Sippy component readiness or the payload agent (TRT-2609). Chai remains the producer. ship-status is the store/UI.
- Authenticated Jira search from ship-status pods. Chai or the UI sends keys/URLs.
- Auto-migrating stored `details` between schema versions. New tags use the new version. Existing rows wait for a human refresh.
- One generic jsonb table UI shared by every team. Each team workspace is a versioned component.

## Locked decisions

- Streams: amd64 ci and nightly, named in YAML `workspace.streams`. Edit that list when a version GAs or a new stream opens. Not arm64/multi/ppc/s390x. Not GA/z-stream (`payload_check_ga`). Removed names drop out of eval and UI; leftover rows are not deleted.
- Data plane: Chai (and humans) write everything. ship-status does not poll release-controller. Chai extends `PayloadCheckHandler`, it does not add a parallel poller. The handler inserts each payload tag once. It does not rewrite that row unless a human asks.
- Frontend add/edit is in scope, same APIs as MCP, not bot-only.
- Bot `acting_for` / owner user string: `chai-bot`.
- Missed SLO: status indicator only, never an outage. Infra and incident outage paths stay as they are.
- Incidents: keep `trt-incidents` outages. Set `slo_component: true` on that component so list APIs omit it. Show the outages on the team SLO panel and the home SLO well. Other teams set the same flag.
- Failed SLO insert: do not block the Firestore watermark. Slack the failure (stream, tag, error, whether the watermark advanced). A human asks Chai to refresh that tag, or edits on the team page. No automatic failed-tag queue and no automatic rewrite of existing rows in v1.
- Interactive Chai skill: fetch current phase for a requested stream and tag, then replace the row. Independent of the watermark. Used after a write-failure Slack, a missing payload, or a stale stored row. This is the only Chai rewrite of an existing payload.
- Persist SLO workspace items for the full evaluation `window`. `recent_payloads` is team-page display-only for `payload_streams`. The home widget does not list workspace rows. An in-window `Accepted` outcome is never pruned just because later items filled the last-N list.
- Store schema is generic (`slo_workspace_items` + jsonb `details`). Do not add `stream` / `tag` / `phase` columns. TRT maps those onto `group_key` / `item_key` / `outcome` in the producer and the `payload_streams` v1 UI.
- TRT v1 `details` always includes `payload_url` (release-controller). `analysis_url` when payload-agent HTML exists. Failed jobs may have `notes`. Chai writes notes on the first insert. Humans can add or edit after. Scheduled ticks do not clobber. A human-requested refresh replaces the row (Chai authoritative on that write).
- `owners` is required on `team_slos`. No fallback to component owners.
- `payload_acceptance` is a named Go evaluator selected by YAML `source`. It hardcodes 24h and min 1 `Accepted` per YAML stream. Window and target are returned on the GET APIs for display. It is not a generic jsonb check. Other teams register a different `source`.
- `(kind, schema_version)` is the Chai/ship-status contract. Required on YAML workspace and every stored row. Ship-status owns the JSON schema and a versioned per-team frontend component. Reject unknown versions. Mixed versions in a window render with the matching component, not a migration of old jsonb. Home does not load those components.
- Slack canvas: on-demand only. Stop asking Chai to update it. No cutover. ship-status never scrapes it.
- Recurring-job grouping: ship-status, from stored `details` using the item's `schema_version` (TRT `payload_streams` v1 first).

## Implementation todos

**SHIP Status Dash (this repo)**

- Define `team_slos` YAML (including `chai-bot` owners), public read APIs, persisted workspace store, TeamPage SLO strip, and home-page widget linking to `#slo`.
- Render TRT amd64 `payload_streams` **v1** from persisted upserts only (no release-controller poll): last N **displayed on the team page**, release-controller and payload-agent links, failed jobs with per-job notes, recurring-job grouping, plus frontend add/edit. Evaluation uses the full `window`. Home widget stays roll-up plus compact incident rows, no payload tables.
- Versioned per-team workspace registry (`frontend/src/components/team/slo/{team}/v{n}/`) plus JSON schema per `(kind, schema_version)`. Unknown versions use the fallback component. Bumps add a new version; they do not mutate v1 in place.
- Protected write API plus authenticated MCP (`upsert_slo_item` / `add_slo_item_link`) so producers can upsert generic workspace rows with `schema_version`. TRT maps payloads into the v1 document (`acting-for`, same path as TRT-2666). E2e with chai-bot SA.
- Add `SLOComponent` on `types.Component` in `pkg/types/config.go` (and `slo_component` on the frontend `Component` type). Set it on TRT Incidents in local YAML. Filter list APIs on that field.
- Generic team SLO incidents panel backed by `slo_component: true` (TRT Incidents first). Omit those components from home/team list APIs. List their outages on the home SLO well. Reuse for other teams.
- Add `prometheus` / `time_since_event` sources so ART, CRT, and DPTP can use the same team-page SLO strip. Incidents panel is already generic via YAML.

**Chai Bot (ship-help-bot)**

- Wrapper + `PayloadCheckHandler` **insert-once** for every configured amd64 stream (ci and nightly; Accepted/Rejected/Ready), `acting_for=chai-bot`, `schema_version` matching ship-status `payload_streams` v1. Skip tags that already exist. Do not hold the Firestore watermark on ship-status errors.
- Slack a human when an SLO insert fails: stream, tag, error, watermark status, and how to ask Chai to refresh that tag.
- Interactive skill: on user request, look up a named stream and tag and **replace** the row (watermark-independent). Covers write failures, missing payloads, and stale stored rows. Scheduled ticks never do this.
- After `record_payload_infra_outage` create/link, attach `slo_workspace_links` (`outage`). After incident Jira create, attach `jira` links.
- Persona tools for human Slack edits (OrgData `acting_for`). Instructions: explicit intent only, never outage-on-SLO-miss, RWS exposure matches infra wrapper.
- Stop asking Chai to update the Slack payload canvas once the team page is live. Do not scrape it or rewrite it as a pointer.
