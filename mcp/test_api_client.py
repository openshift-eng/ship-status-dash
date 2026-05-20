"""Tests for SHIP Status API MCP client."""

from unittest.mock import MagicMock, patch

import pytest

from api_client import DashboardClient, ShipStatusAPI, _outage_is_active


def _list_outage_no_slack(*, active: bool) -> dict:
    end = None if active else "2026-05-02T12:00:00Z"
    return {
        "ID": 6041,
        "CreatedAt": "2026-05-01T10:00:00Z",
        "component_name": "build-farm",
        "sub_component_name": "build06",
        "severity": "Degraded",
        "start_time": "2026-05-01T10:05:00Z",
        "end_time": end,
        "description": "Component monitor detected outage",
        "reasons": [{"type": "junit"}],
    }


def _detail_with_slack() -> dict:
    raw = _list_outage_no_slack(active=True)
    raw["slack_threads"] = [{"channel_id": "C01234567"}]
    return raw


@pytest.fixture
def client() -> DashboardClient:
    return DashboardClient(
        public_base_url="http://test/api",
        protected_base_url="http://test-protected/api",
        dashboard_url="http://test/",
    )


@pytest.fixture
def api(client: DashboardClient) -> ShipStatusAPI:
    return ShipStatusAPI(client)


def test_public_get_builds_url(client: DashboardClient):
    with patch("api_client.urlopen") as mock_open:
        mock_resp = MagicMock()
        mock_resp.read.return_value = b'[{"ok": true}]'
        mock_resp.__enter__ = lambda s: s
        mock_resp.__exit__ = MagicMock(return_value=False)
        mock_open.return_value = mock_resp

        result = client.public_get("/status")
        assert result == [{"ok": True}]
        call_req = mock_open.call_args[0][0]
        assert call_req.full_url == "http://test/api/status"


def test_protected_request_without_token(client: DashboardClient):
    with patch.dict("os.environ", {}, clear=True):
        result = client.protected_request("POST", "/components/x/y/outages", body={})
    assert "error" in result
    assert "SHIP_STATUS_AUTH_TOKEN_FILE" in result["error"]


def test_protected_request_with_bearer(client: DashboardClient, tmp_path):
    token_file = tmp_path / "token"
    token_file.write_text("sa-token-xyz", encoding="utf-8")
    with patch.dict("os.environ", {"SHIP_STATUS_AUTH_TOKEN_FILE": str(token_file)}):
        with patch("api_client.urlopen") as mock_open:
            mock_resp = MagicMock()
            mock_resp.read.return_value = b'{"id": 1}'
            mock_resp.__enter__ = lambda s: s
            mock_resp.__exit__ = MagicMock(return_value=False)
            mock_open.return_value = mock_resp

            result = client.protected_request("POST", "/components/a/b/outages", body={"x": 1})
            assert result == {"id": 1}
            call_req = mock_open.call_args[0][0]
            assert call_req.get_header("Authorization") == "Bearer sa-token-xyz"


def test_get_infrastructure_status(api: ShipStatusAPI):
    with patch.object(api.client, "public_get", return_value=[{"component_name": "Prow", "status": "Healthy", "active_outages": []}]):
        result = api.get_infrastructure_status()
    assert result["overall_healthy"] is True
    assert result["components"][0]["component_name"] == "Prow"


def test_get_component_outages_passthrough(api: ShipStatusAPI):
    with patch.object(api.client, "public_get", return_value=[_list_outage_no_slack(active=True)]):
        result = api.get_component_outages("build-farm", "build06")
    assert result["active_count"] == 1
    assert result["outages"][0]["ID"] == 6041


def test_get_component_outages_merges_slack(api: ShipStatusAPI):
    list_row = _list_outage_no_slack(active=True)

    def fake_get(path: str):
        if path.endswith("/outages"):
            return [list_row]
        if path.endswith("/outages/6041"):
            return _detail_with_slack()
        raise AssertionError(path)

    with patch.object(api.client, "public_get", side_effect=fake_get):
        result = api.get_component_outages("build-farm", "build06")
    assert result["outages"][0]["slack_threads"][0]["channel_id"] == "C01234567"


def test_get_outages_during_requires_start_or_end(api: ShipStatusAPI):
    result = api.get_outages_during()
    assert "error" in result


def test_get_outages_during_rejects_sub_without_component(api: ShipStatusAPI):
    result = api.get_outages_during(start="2026-05-13T00:00:00Z", sub_component_name="build04")
    assert "error" in result


def test_get_outages_during_builds_query(api: ShipStatusAPI):
    api_row = _list_outage_no_slack(active=False)

    def check_path(path: str):
        assert path.startswith("/outages/during?")
        assert "start=" in path
        assert "componentName=build-farm" in path
        return [api_row]

    with patch.object(api.client, "public_get", side_effect=check_path):
        result = api.get_outages_during(
            start="2026-05-13T00:00:00Z",
            end="2026-05-13T23:59:59Z",
            component_name="build-farm",
        )
    assert result["count"] == 1


def test_outage_is_active_sql_null():
    assert _outage_is_active({"end_time": {"Time": "2026-05-02T00:00:00Z", "Valid": False}})
