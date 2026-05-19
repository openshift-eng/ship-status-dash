# Ship Status Dashboard — Architecture

High-level dataflow for the Ship Status and Availability Dashboard. For component-specific details, see [`cmd/dashboard/README.md`](cmd/dashboard/README.md) and [`cmd/component-monitor/README.md`](cmd/component-monitor/README.md).

## System overview

```mermaid
flowchart TB
    subgraph Users["Users & operators"]
        Browser["Browser (React SPA)"]
        Operator["Operators / on-call"]
    end

    subgraph Ingress["Ingress (production)"]
        PublicIngress["Public ingress"]
        ProtectedIngress["Protected ingress + OAuth proxy"]
    end

    subgraph Dashboard["cmd/dashboard"]
        API["HTTP API (gorilla/mux)"]
        AuthMW["HMAC auth middleware<br/>(protected routes)"]
        ReportProc["ComponentMonitorReportProcessor"]
        AbsentChecker["AbsentReportChecker<br/>(background)"]
        OutageMgr["DBOutageManager"]
        ConfigMgr["Config manager<br/>(YAML, hot-reload)"]
        GroupCache["Group membership cache"]
        SPA["Static SPA handler"]
    end

    subgraph Monitor["cmd/component-monitor"]
        Orch["ProbeOrchestrator"]
        HTTPProber["HTTP prober"]
        PromProber["Prometheus prober"]
        JUnitProber["JUnit prober"]
        SystemdProber["Systemd prober"]
        ReportClient["ReportClient"]
    end

    subgraph Data["Persistence & config"]
        PG[("PostgreSQL")]
        DashYAML["Dashboard YAML config"]
        MonYAML["Monitor YAML config"]
    end

    subgraph External["External targets & integrations"]
        HTTPTargets["Monitored HTTP endpoints"]
        Prom["Prometheus"]
        GCS["GCS / Prow JUnit artifacts"]
        DBus["systemd (D-Bus)"]
        K8s["Kubernetes API"]
        Slack["Slack API"]
    end

    Browser --> PublicIngress
    Browser --> ProtectedIngress
    PublicIngress --> API
    ProtectedIngress --> AuthMW --> API
    API --> SPA
    API --> OutageMgr
    ReportProc --> OutageMgr
    AbsentChecker --> OutageMgr
    OutageMgr --> PG
    ReportProc --> PG
    ConfigMgr --> DashYAML
    GroupCache --> K8s
    Dashboard --> ConfigMgr

    Orch --> HTTPProber & PromProber & JUnitProber & SystemdProber
    HTTPProber --> HTTPTargets
    PromProber --> Prom
    JUnitProber --> GCS
    SystemdProber --> DBus
    Prom -.->|via kubeconfig / in-cluster| K8s
    Orch --> ReportClient
    ReportClient -->|POST /api/component-monitor/report<br/>Bearer + HMAC| ProtectedIngress
    ProtectedIngress --> ReportProc
    OutageMgr --> Slack
    Operator --> Slack

    MonYAML --> Monitor
    DashYAML --> Dashboard
```

## Components

| Component | Location | Role |
|-----------|----------|------|
| Dashboard API | `cmd/dashboard` | REST API, outage management, Slack notifications, absent-report watchdog |
| Frontend | `frontend/` | React SPA (served as static assets by the dashboard in production) |
| Component monitor | `cmd/component-monitor` | Periodic probes and status reports to the dashboard API |
| Database | PostgreSQL | Outages, audit logs, report pings, Slack thread metadata |
| Migrations | `cmd/migrate` | Schema migrations |

## Main data paths

1. **Read path (users)** — Browser → public ingress → dashboard API → PostgreSQL / in-memory config → JSON responses for status and outage views.
2. **Write path (operators)** — Browser → protected ingress (OAuth + HMAC) → dashboard API → outage manager → PostgreSQL (+ audit logs, Slack).
3. **Monitor path** — Probe orchestrator → external targets → merged report → protected ingress → report processor → pings and auto-created/resolved outages in PostgreSQL.
