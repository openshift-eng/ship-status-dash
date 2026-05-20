"""HTTP client for the SHIP Status Dashboard public and protected APIs."""

from __future__ import annotations

import json
import logging
import os
from pathlib import Path
from typing import Any
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode
from urllib.request import Request, urlopen

logger = logging.getLogger(__name__)

DEFAULT_PUBLIC_API_URL = "https://ship-status.ci.openshift.org/api"
DEFAULT_PROTECTED_API_URL = "https://protected.ship-status.ci.openshift.org/api"
DEFAULT_DASHBOARD_URL = "https://ship-status.ci.openshift.org/"
DEFAULT_TIMEOUT = 10

_MAX_OUTAGES_SLACK_ENRICH = 60
_MAX_RESPONSE_CHARS = 28000


def _env(name: str, default: str) -> str:
    val = os.environ.get(name, "").strip()
    return val if val else default


def _read_bearer_token_file(path: str) -> str | None:
    try:
        return Path(path).read_text(encoding="utf-8").strip()
    except OSError as e:
        logger.error("Failed to read auth token file %s: %s", path, e)
        return None


class DashboardClient:
    """Wraps dashboard REST calls. Reads use the public API; writes use protected + Bearer SA."""

    def __init__(
        self,
        *,
        public_base_url: str | None = None,
        protected_base_url: str | None = None,
        dashboard_url: str | None = None,
        timeout: float | None = None,
        auth_token_file: str | None = None,
    ) -> None:
        legacy = os.environ.get("SHIP_STATUS_API_BASE_URL", "").strip()
        public_default = legacy or DEFAULT_PUBLIC_API_URL
        self.public_base_url = (public_base_url or _env("SHIP_STATUS_PUBLIC_API_URL", public_default)).rstrip(
            "/"
        )
        self.protected_base_url = (
            protected_base_url or _env("SHIP_STATUS_PROTECTED_API_URL", DEFAULT_PROTECTED_API_URL)
        ).rstrip("/")
        self.dashboard_url = (dashboard_url or _env("SHIP_STATUS_DASHBOARD_URL", DEFAULT_DASHBOARD_URL)).rstrip(
            "/"
        ) + "/"
        self.timeout = timeout if timeout is not None else float(
            os.environ.get("SHIP_STATUS_REQUEST_TIMEOUT", DEFAULT_TIMEOUT)
        )
        self.auth_token_file = auth_token_file.strip() if auth_token_file else None

    def _auth_token_path(self) -> str | None:
        if self.auth_token_file:
            return self.auth_token_file
        env_path = os.environ.get("SHIP_STATUS_AUTH_TOKEN_FILE", "").strip()
        return env_path or None

    def _load_bearer_token(self) -> str | None:
        path = self._auth_token_path()
        if not path:
            return None
        return _read_bearer_token_file(path)

    @property
    def writes_enabled(self) -> bool:
        return bool(self._load_bearer_token())

    def public_get(self, path: str) -> dict | list | None:
        return self._request("GET", self.public_base_url, path)

    def protected_request(
        self,
        method: str,
        path: str,
        body: dict | None = None,
    ) -> dict | list | None:
        token = self._load_bearer_token()
        if not token:
            return {
                "error": (
                    "Protected API calls require SHIP_STATUS_AUTH_TOKEN_FILE "
                    "(OpenShift service account Bearer token)."
                )
            }
        headers = {"Authorization": f"Bearer {token}"}
        data = None
        if body is not None:
            headers["Content-Type"] = "application/json"
            data = json.dumps(body).encode("utf-8")
        return self._request(method, self.protected_base_url, path, headers=headers, data=data)

    def _request(
        self,
        method: str,
        base_url: str,
        path: str,
        *,
        headers: dict[str, str] | None = None,
        data: bytes | None = None,
    ) -> dict | list | None:
        if not path.startswith("/"):
            path = "/" + path
        url = f"{base_url}{path}"
        req_headers = dict(headers or {})
        try:
            req = Request(url, data=data, headers=req_headers, method=method)
            with urlopen(req, timeout=self.timeout) as response:
                return json.loads(response.read().decode())
        except HTTPError as e:
            try:
                body = e.read().decode()
                parsed = json.loads(body)
                if isinstance(parsed, dict):
                    return parsed
            except (json.JSONDecodeError, OSError):
                pass
            logger.error("SHIP Status API HTTP %s (%s): %s", e.code, url, e.reason)
            return {"error": f"HTTP {e.code}: {e.reason}"}
        except (URLError, TimeoutError, json.JSONDecodeError) as e:
            logger.error("SHIP Status API request failed (%s): %s", url, e)
            return None


def _outage_is_active(raw: dict[str, Any]) -> bool:
    et = raw.get("end_time")
    return et is None or (isinstance(et, dict) and not et.get("Valid"))


def _truncate_json(obj: Any) -> Any:
    text = json.dumps(obj, default=str)
    if len(text) <= _MAX_RESPONSE_CHARS:
        return obj
    return {
        "error": f"Response truncated ({len(text)} chars exceeds {_MAX_RESPONSE_CHARS} limit).",
        "preview": text[:_MAX_RESPONSE_CHARS] + "...",
    }


class ShipStatusAPI:
    """Domain operations for MCP tools."""

    def __init__(self, client: DashboardClient | None = None) -> None:
        self.client = client or DashboardClient()

    def get_infrastructure_status(self) -> dict[str, Any]:
        data = self.client.public_get("/status")
        if data is None:
            return {"error": "Failed to retrieve status from SHIP Status Dashboard."}
        if not isinstance(data, list):
            return {"error": "Unexpected response shape from /api/status (expected a list)."}

        components = []
        all_healthy = True
        for entry in data:
            if not isinstance(entry, dict):
                continue
            status = entry.get("status", "Unknown")
            outages = entry.get("active_outages") or []
            if status != "Healthy":
                all_healthy = False
            components.append(
                {
                    "component_name": entry.get("component_name", "Unknown"),
                    "status": status,
                    "active_outage_count": len(outages),
                    "active_outages": [dict(o) for o in outages if isinstance(o, dict)],
                }
            )

        return _truncate_json(
            {
                "dashboard_url": self.client.dashboard_url,
                "overall_healthy": all_healthy,
                "components": components,
            }
        )

    def get_component_details(self, component_slug: str) -> dict[str, Any]:
        data = self.client.public_get(f"/components/{component_slug}")
        if data is None:
            return {
                "error": f"Failed to retrieve component '{component_slug}'. Check the slug is correct."
            }
        if isinstance(data, dict) and "error" in data:
            return data
        if not isinstance(data, dict):
            return {"error": "Unexpected response shape from component endpoint."}

        sub_components = []
        for sc in data.get("sub_components") or []:
            if isinstance(sc, dict):
                sub_components.append(
                    {
                        "name": sc.get("name"),
                        "slug": sc.get("slug"),
                        "description": sc.get("description", ""),
                    }
                )

        return _truncate_json(
            {
                "name": data.get("name"),
                "slug": data.get("slug"),
                "description": data.get("description", ""),
                "ship_team": data.get("ship_team", ""),
                "sub_components": sub_components,
            }
        )

    def get_outage(self, component_slug: str, sub_component_slug: str, outage_id: int) -> dict[str, Any]:
        path = f"/components/{component_slug}/{sub_component_slug}/outages/{outage_id}"
        data = self.client.public_get(path)
        if data is None:
            return {
                "error": (
                    f"Failed to retrieve outage {outage_id} for "
                    f"'{component_slug}/{sub_component_slug}'."
                )
            }
        if isinstance(data, dict) and "error" in data:
            return data
        if not isinstance(data, dict):
            return {"error": "Unexpected response shape from SHIP Status outage endpoint."}
        return _truncate_json(data)

    def _enrich_outages_slack_threads(
        self,
        component_slug: str,
        sub_component_slug: str,
        raw_list: list[dict[str, Any]],
    ) -> tuple[list[dict[str, Any]], str | None]:
        copies: list[dict[str, Any]] = [dict(x) for x in raw_list]
        if not copies:
            return [], None

        total = len(copies)
        note: str | None = None
        if total > _MAX_OUTAGES_SLACK_ENRICH:
            note = (
                f"The outages list omits slack_threads. Per-outage enrichment ran only for the first "
                f"{_MAX_OUTAGES_SLACK_ENRICH} of {total} outages (cap). Remaining rows were not merged. "
                "Use get_outage(component_slug, sub_component_slug, outage_id) for full records "
                "including slack_threads."
            )

        enrich_limit = min(total, _MAX_OUTAGES_SLACK_ENRICH)
        for i in range(enrich_limit):
            raw = copies[i]
            if raw.get("slack_threads"):
                continue
            oid = raw.get("ID", raw.get("id"))
            sub = (sub_component_slug or raw.get("sub_component_name") or "").strip()
            if oid is None or not sub:
                continue
            detail = self.client.public_get(f"/components/{component_slug}/{sub}/outages/{oid}")
            if isinstance(detail, dict) and not detail.get("error") and detail.get("slack_threads"):
                merged = dict(raw)
                merged["slack_threads"] = detail["slack_threads"]
                copies[i] = merged
        return copies, note

    def get_component_outages(
        self, component_slug: str, sub_component_slug: str = ""
    ) -> dict[str, Any]:
        if sub_component_slug:
            path = f"/components/{component_slug}/{sub_component_slug}/outages"
        else:
            path = f"/components/{component_slug}/outages"

        data = self.client.public_get(path)
        if data is None:
            return {
                "error": f"Failed to retrieve outages for '{component_slug}'. Check the slug is correct."
            }
        if isinstance(data, dict) and "error" in data:
            return data
        if not isinstance(data, list):
            sub_disp = sub_component_slug or "(all)"
            preview = repr(data)
            if len(preview) > 400:
                preview = preview[:400] + "..."
            return {
                "error": (
                    f"SHIP Status outages list for component={component_slug!r} "
                    f"sub_component={sub_disp!r} expected a JSON array, "
                    f"got {type(data).__name__}: {preview}"
                ),
            }

        raw_list = [x for x in data if isinstance(x, dict)]
        outages, enrich_note = self._enrich_outages_slack_threads(
            component_slug, sub_component_slug, raw_list
        )

        active = [o for o in outages if _outage_is_active(o)]
        resolved = [o for o in outages if not _outage_is_active(o)]

        out: dict[str, Any] = {
            "component": component_slug,
            "sub_component": sub_component_slug or "(all)",
            "total_outages": len(outages),
            "active_count": len(active),
            "resolved_count": len(resolved),
            "outages": outages,
        }
        if enrich_note:
            out["enrichment_note"] = enrich_note
        return _truncate_json(out)

    def get_outages_during(
        self,
        start: str = "",
        end: str = "",
        component_name: str = "",
        sub_component_name: str = "",
        tag: str = "",
        team: str = "",
    ) -> dict[str, Any]:
        start_s = (start or "").strip()
        end_s = (end or "").strip()
        if not start_s and not end_s:
            return {
                "error": (
                    "Provide at least one of start or end (RFC3339 or RFC3339Nano UTC), "
                    "e.g. 2026-05-13T14:22:06Z"
                )
            }

        comp = (component_name or "").strip()
        sub = (sub_component_name or "").strip()
        if sub and not comp:
            return {
                "error": "sub_component_name requires component_name (API rejects subComponentName alone)."
            }

        params: list[tuple[str, str]] = []
        if start_s:
            params.append(("start", start_s))
        if end_s:
            params.append(("end", end_s))
        if comp:
            params.append(("componentName", comp))
        if sub:
            params.append(("subComponentName", sub))
        tag_s = (tag or "").strip()
        if tag_s:
            params.append(("tag", tag_s))
        team_s = (team or "").strip()
        if team_s:
            params.append(("team", team_s))

        path = f"/outages/during?{urlencode(params)}"
        data = self.client.public_get(path)
        if data is None:
            return {
                "error": (
                    "Failed to retrieve outages from SHIP Status /api/outages/during "
                    "(network or server error)."
                )
            }
        if isinstance(data, dict) and "error" in data:
            return data
        if isinstance(data, list):
            return _truncate_json({"outages": data, "count": len(data)})
        return {"error": "Unexpected JSON shape from /api/outages/during (expected a list)."}

    def list_components(self) -> dict[str, Any]:
        data = self.client.public_get("/components")
        if data is None:
            return {"error": "Failed to retrieve components list."}
        if isinstance(data, dict) and "error" in data:
            return data
        return _truncate_json({"components": data})

    def list_tags(self) -> dict[str, Any]:
        data = self.client.public_get("/tags")
        if data is None:
            return {"error": "Failed to retrieve tags list."}
        if isinstance(data, dict) and "error" in data:
            return data
        return _truncate_json({"tags": data})

    def list_sub_components(self) -> dict[str, Any]:
        data = self.client.public_get("/sub-components")
        if data is None:
            return {"error": "Failed to retrieve sub-components list."}
        if isinstance(data, dict) and "error" in data:
            return data
        return _truncate_json({"sub_components": data})
