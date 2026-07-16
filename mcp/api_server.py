"""MCP server exposing the SHIP Status Dashboard REST API to AI agents."""

from __future__ import annotations

import logging
import os

from fastmcp import FastMCP

logger = logging.getLogger(__name__)

_DEFAULT_MCP_HTTP_PORT = 8090

from api_client import ShipStatusAPI

mcp = FastMCP("ship-status")
_api = ShipStatusAPI()


@mcp.tool()
def get_infrastructure_status() -> dict:
    """Current health of all SHIP infrastructure components and active outages."""
    return _api.get_infrastructure_status()


@mcp.tool()
def get_component_details(component_slug: str) -> dict:
    """Component config: description, team, and sub-components. Slugs are lowercase with hyphens (e.g. prow, build-farm)."""
    return _api.get_component_details(component_slug)


@mcp.tool()
def get_component_outages(component_slug: str, sub_component_slug: str = "") -> dict:
    """Outage history for a component or sub-component (active and resolved). Omit sub_component_slug for all subs."""
    return _api.get_component_outages(component_slug, sub_component_slug)


@mcp.tool()
def get_outage(component_slug: str, sub_component_slug: str, outage_id: int) -> dict:
    """Single outage by ID (includes slack_threads when present)."""
    return _api.get_outage(component_slug, sub_component_slug, outage_id)


@mcp.tool()
def get_outages_during(
    start: str = "",
    end: str = "",
    component_name: str = "",
    sub_component_name: str = "",
    tag: str = "",
    team: str = "",
) -> dict:
    """Outages overlapping a time instant or interval (RFC3339 UTC). At least one of start or end is required."""
    return _api.get_outages_during(
        start=start,
        end=end,
        component_name=component_name,
        sub_component_name=sub_component_name,
        tag=tag,
        team=team,
    )


@mcp.tool()
def list_components() -> dict:
    """All configured components (use slugs in other tools)."""
    return _api.list_components()


@mcp.tool()
def list_tags() -> dict:
    """All dashboard tags (for filtering get_outages_during)."""
    return _api.list_tags()


@mcp.tool()
def list_sub_components(status: str = "") -> dict:
    """All sub-components across components. Optional status filter: comma-separated values (Healthy, Degraded, Down, CapacityExhausted, Suspected). Partial is not valid for this filter. When set, only matching items are returned. Each item includes a status field."""
    return _api.list_sub_components(status=status)


# Write tools (protected API)


@mcp.tool()
def check_maintainers(component_slug: str) -> dict:
    """Users authorized to manage a component (expands rover_group owners). Use to verify a user before creating outages on their behalf."""
    return _api.check_maintainers(component_slug)


@mcp.tool()
def create_outage(
    component_slug: str,
    sub_component_slug: str,
    severity: str,
    description: str,
    start_time: str = "",
    initial_triage_note: str = "",
    bot_initiated: bool = False,
) -> dict:
    """Create an outage on a sub-component. Severity: Down, Degraded, Suspected, or CapacityExhausted. start_time is RFC3339 UTC (defaults to now). Set bot_initiated=true when creating autonomously (forces Suspected severity, unconfirmed, and checks for duplicates)."""
    return _api.create_outage(
        component_slug,
        sub_component_slug,
        severity=severity,
        description=description,
        start_time=start_time,
        initial_triage_note=initial_triage_note,
        bot_initiated=bot_initiated,
    )


@mcp.tool()
def update_outage(
    component_slug: str,
    sub_component_slug: str,
    outage_id: int,
    severity: str = "",
    description: str = "",
    start_time: str = "",
    end_time: str = "",
    confirmed: bool | None = None,
) -> dict:
    """Update an existing outage. Set end_time (RFC3339 UTC) to resolve it. All fields are optional — only provided fields are changed."""
    return _api.update_outage(
        component_slug,
        sub_component_slug,
        outage_id,
        severity=severity,
        description=description,
        start_time=start_time,
        end_time=end_time,
        confirmed=confirmed,
    )


@mcp.tool()
def delete_outage(component_slug: str, sub_component_slug: str, outage_id: int) -> dict:
    """Permanently delete an outage. Prefer resolving (update_outage with end_time) over deleting."""
    return _api.delete_outage(component_slug, sub_component_slug, outage_id)


@mcp.tool()
def add_triage_note(component_slug: str, sub_component_slug: str, outage_id: int, body: str) -> dict:
    """Add a triage note to an outage for incident documentation."""
    return _api.add_triage_note(component_slug, sub_component_slug, outage_id, body)


@mcp.tool()
def update_triage_note(
    component_slug: str, sub_component_slug: str, outage_id: int, note_id: int, body: str
) -> dict:
    """Update an existing triage note."""
    return _api.update_triage_note(component_slug, sub_component_slug, outage_id, note_id, body)


@mcp.tool()
def delete_triage_note(
    component_slug: str, sub_component_slug: str, outage_id: int, note_id: int
) -> dict:
    """Delete a triage note from an outage."""
    return _api.delete_triage_note(component_slug, sub_component_slug, outage_id, note_id)


@mcp.tool()
def add_outage_link(
    component_slug: str,
    sub_component_slug: str,
    outage_id: int,
    url: str,
    link_type: str = "other",
    description: str = "",
) -> dict:
    """Attach a link to an outage. link_type: incident_channel_thread, rca, or other. description is only used for 'other'."""
    return _api.add_outage_link(
        component_slug, sub_component_slug, outage_id, url, link_type=link_type, description=description
    )


@mcp.tool()
def update_outage_link(
    component_slug: str,
    sub_component_slug: str,
    outage_id: int,
    link_id: int,
    url: str,
    link_type: str = "other",
    description: str = "",
) -> dict:
    """Update an existing outage link."""
    return _api.update_outage_link(
        component_slug, sub_component_slug, outage_id, link_id, url, link_type=link_type, description=description
    )


@mcp.tool()
def delete_outage_link(
    component_slug: str, sub_component_slug: str, outage_id: int, link_id: int
) -> dict:
    """Delete a link from an outage."""
    return _api.delete_outage_link(component_slug, sub_component_slug, outage_id, link_id)


def main() -> None:
    transport = os.environ.get("MCP_TRANSPORT", "stdio").strip() or "stdio"
    if transport == "stdio":
        mcp.run()
    else:
        port = _DEFAULT_MCP_HTTP_PORT
        raw = os.environ.get("MCP_HTTP_PORT", "").strip()
        if raw:
            try:
                parsed = int(raw)
                if 1 <= parsed <= 65535:
                    port = parsed
                else:
                    logger.warning(
                        "MCP_HTTP_PORT %r out of range (1-65535); using default %d",
                        raw,
                        port,
                    )
            except ValueError:
                logger.warning("Invalid MCP_HTTP_PORT %r; using default %d", raw, port)
        mcp.run(transport=transport, host="0.0.0.0", port=port)


if __name__ == "__main__":
    main()
