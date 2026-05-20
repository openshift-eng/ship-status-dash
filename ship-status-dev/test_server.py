"""Tests for ship-status-dev MCP helpers and tool early paths."""

from __future__ import annotations

import subprocess
from pathlib import Path
from unittest.mock import MagicMock, patch

import pytest

import server


def test_tail_file_returns_last_lines(tmp_path: Path):
    log = tmp_path / "app.log"
    log.write_text("\n".join(f"line{i}" for i in range(10)), encoding="utf-8")
    out = server._tail_file(log, 3)
    assert out == "line7\nline8\nline9"


def test_tail_file_missing_file(tmp_path: Path):
    out = server._tail_file(tmp_path / "missing.log", 5)
    assert "could not read log" in out


def test_default_dsn_from_env():
    with patch.dict("os.environ", {"SHIP_STATUS_DSN": "postgres://custom/db"}):
        assert server._default_dsn() == "postgres://custom/db"


def test_default_dsn_fallback():
    with patch.dict("os.environ", {}, clear=True):
        dsn = server._default_dsn()
    assert "postgres://" in dsn
    assert "ship_status" in dsn


def test_proc_cmdline_null_separated(tmp_path: Path):
    (tmp_path / "cmdline").write_bytes(b"go\x00run\x00./cmd/dashboard\x00--config\x00x")
    cmd = server._proc_cmdline(tmp_path)
    assert "cmd/dashboard" in cmd
    assert "--config" in cmd


def test_find_pids_via_pgrep_when_not_linux():
    expected = server.REPO_ROOT.resolve()

    def match(cmd: str) -> bool:
        return "cmd/dashboard" in cmd

    with (
        patch.object(server.sys, "platform", "darwin"),
        patch.object(server, "_pgrep_pids", return_value=[42]),
        patch.object(server, "_filter_pids_by_cwd", return_value=[42]) as mock_filter,
    ):
        pids = server._find_pids(expected, match, ["cmd/dashboard"])

    assert pids == [42]
    mock_filter.assert_called_once_with([42], expected)


def test_run_script_background_missing_script():
    with patch.object(server, "REPO_ROOT", Path("/nonexistent-repo")):
        result = server._run_script_background(
            "test",
            "hack/local/nope.sh",
            [],
            Path("/tmp/out.log"),
        )
    assert "script not found" in result


def test_run_script_background_success(tmp_path: Path):
    repo = tmp_path / "repo"
    script_dir = repo / "hack" / "local"
    script_dir.mkdir(parents=True)
    script = script_dir / "ok.sh"
    script.write_text("#!/bin/bash\necho started\n", encoding="utf-8")
    log_path = tmp_path / "run.log"

    with (
        patch.object(server, "REPO_ROOT", repo),
        patch.object(
            server.subprocess,
            "run",
            return_value=subprocess.CompletedProcess(args=[], returncode=0),
        ),
    ):
        result = server._run_script_background("ok", "hack/local/ok.sh", [], log_path)

    assert "ok started" in result
    assert str(log_path) in result


def test_dashboard_serve_reports_already_running():
    with (
        patch.object(server, "_pids_dashboard", return_value=[100]),
        patch.object(server, "_pids_oauth_proxy", return_value=[101]),
    ):
        result = server.dashboard_serve(restart=False)

    assert "already running" in result
    assert "100" in result
    assert "8180" in result


def test_run_migrate_passes_dsn():
    captured: dict[str, object] = {}

    def fake_foreground(label, args, log_filename, timeout_seconds, env_extra=None):
        captured["label"] = label
        captured["args"] = args
        return server.ForegroundRunResult(True, False, "ok")

    with patch.object(server, "_run_foreground", side_effect=fake_foreground):
        server.run_migrate(database_dsn="postgres://test/db", timeout_seconds=30)

    assert captured["label"] == "run_migrate"
    assert captured["args"] == ["go", "run", "./cmd/migrate", "--dsn", "postgres://test/db"]


def test_component_monitor_start_reports_already_running():
    with (
        patch.object(server, "_pids_component_monitor", return_value=[200]),
        patch.object(server, "_pids_mock_component", return_value=[]),
        patch.object(server, "_pids_prometheus", return_value=[]),
    ):
        result = server.component_monitor_start(restart=False)

    assert "already running" in result
    assert "200" in result


def test_run_tests_stops_when_lint_fails():
    with patch.object(
        server,
        "_run_foreground",
        return_value=server.ForegroundRunResult(False, False, "lint broke"),
    ):
        result = server.run_tests(timeout_seconds=60)

    assert "Lint failed" in result
    assert "lint broke" in result
