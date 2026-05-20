"""MCP server exposing the SHIP Status Dashboard REST API to AI agents."""

from __future__ import annotations

import os

from fastmcp import FastMCP

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
def list_sub_components() -> dict:
    """All sub-components across components."""
    return _api.list_sub_components()


def main() -> None:
    transport = os.environ.get("MCP_TRANSPORT", "stdio").strip() or "stdio"
    if transport == "stdio":
        mcp.run()
    else:
        port = int(os.environ.get("MCP_HTTP_PORT", "8090"))
        mcp.run(transport=transport, host="0.0.0.0", port=port)


if __name__ == "__main__":
    main()
