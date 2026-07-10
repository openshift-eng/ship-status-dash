---
description: "Authentication, authorization, and credential handling rules for Ship Status Dashboard"
applyTo: "**"
---

### Authentication model

The dashboard uses a dual-ingress architecture:

* **Public route** (`ship-status.ci.openshift.org`, port 8080) -- read-only API, no authentication required.
* **Protected route** (`protected.ship-status.ci.openshift.org`, port 8443) -- routes through an oauth-proxy that authenticates callers via Kubernetes `TokenReview`, sets `X-Forwarded-User`, and signs requests with an HMAC `GAP-Signature` header before proxying to the dashboard on loopback.

The dashboard verifies the HMAC signature on protected requests and checks authorization against `Owner.User`, `Owner.Group`, and `Owner.ServiceAccount` fields in the component configuration.

### Credential placement

Services that proxy or relay requests (including the MCP server) MUST NOT hold authentication credentials. The caller supplies their own bearer token; the proxy forwards it unmodified to the dashboard's protected API. The dashboard is the single enforcement point for authentication and authorization.

Never mount secret tokens (service account tokens, API keys) on containers that expose unauthenticated endpoints. If a container accepts unauthenticated inbound traffic, it must not have access to credentials that grant write access to other services.

### Write endpoint authorization

All mutating API endpoints (create, update, delete) must be served exclusively on the protected route. The public route must never expose write operations, even behind application-level checks. Defense in depth: the oauth-proxy layer authenticates, the HMAC layer verifies request integrity, and the dashboard authorizes against the component owner configuration.
