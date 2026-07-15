"""Tests for MCP server mode splitting and tool registration."""

import asyncio
from unittest.mock import patch

from api_server import build_server


_READ_TOOLS = {
    "get_infrastructure_status",
    "get_component_details",
    "get_component_outages",
    "get_outage",
    "get_outages_during",
    "list_components",
    "list_tags",
    "list_sub_components",
}

_WRITE_TOOLS = {
    "check_maintainers",
    "create_outage",
    "update_outage",
    "delete_outage",
    "add_triage_note",
    "update_triage_note",
    "delete_triage_note",
    "add_outage_link",
    "update_outage_link",
    "delete_outage_link",
}


def _tool_names(server) -> set[str]:
    tools = asyncio.run(server.list_tools())
    return {t.name for t in tools}


def _tools_by_name(server) -> dict[str, object]:
    tools = asyncio.run(server.list_tools())
    return {t.name: t for t in tools}


def test_public_mode_only_registers_read_tools():
    with patch.dict("os.environ", {"MCP_MODE": "public"}):
        server = build_server()
    assert _tool_names(server) == _READ_TOOLS


def test_authenticated_mode_only_registers_write_tools():
    with patch.dict("os.environ", {"MCP_MODE": "authenticated"}):
        server = build_server()
    assert _tool_names(server) == _WRITE_TOOLS


def test_no_mode_registers_all_tools():
    with patch.dict("os.environ", {"MCP_MODE": ""}):
        server = build_server()
    assert _tool_names(server) == _READ_TOOLS | _WRITE_TOOLS


def test_write_tools_have_acting_for_parameter():
    """All write tools (except check_maintainers which is a GET) accept acting_for."""
    with patch.dict("os.environ", {"MCP_MODE": "authenticated"}):
        server = build_server()
    tools = _tools_by_name(server)
    for name in _WRITE_TOOLS - {"check_maintainers"}:
        param_names = set(tools[name].parameters.get("properties", {}).keys())
        assert "acting_for" in param_names, f"{name} missing acting_for parameter"
