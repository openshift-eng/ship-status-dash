"""Tests for the ship-status-dev MCP launcher."""

from __future__ import annotations

import os
import shutil
import subprocess
from pathlib import Path


def test_concurrent_stale_venv_repair_serializes_without_flock(tmp_path: Path):
    repo = tmp_path / "repo"
    script_dir = repo / "ship-status-dev"
    script_dir.mkdir(parents=True)
    shutil.copy2(Path(__file__).with_name("run.sh"), script_dir / "run.sh")
    (script_dir / "requirements.txt").write_text("fastmcp\n", encoding="utf-8")

    stale_marker = tmp_path / "stale"
    stale_marker.write_text("", encoding="utf-8")
    stale_venv = script_dir / ".venv" / "bin"
    stale_venv.mkdir(parents=True)

    bin_dir = tmp_path / "bin"
    bin_dir.mkdir()
    for command in ("dirname", "pwd", "shasum", "cut", "chmod", "mv", "rm", "mkdir", "cat", "sleep"):
        source = shutil.which(command)
        if source is not None:
            os.symlink(source, bin_dir / command)

    real_python = shutil.which("python3")
    assert real_python is not None

    python_stub = bin_dir / "python3.12"
    python_stub.write_text(
        """#!/bin/sh
if [ "$1" = "-m" ] && [ "$2" = "venv" ]; then
    mkdir -p "$3/bin"
    rm -f "$STALE_MARKER"
    cat > "$3/bin/python" <<'EOF'
#!/bin/sh
if [ "$1" = "-c" ]; then
    if [ -f "$STALE_MARKER" ]; then
        sleep 0.2
        exit 1
    fi
    exit 0
fi
exit 0
EOF
    chmod +x "$3/bin/python"
    cat > "$3/bin/pip" <<'EOF'
#!/bin/sh
exit 0
EOF
    chmod +x "$3/bin/pip"
    exit 0
fi
if [ "$1" = "-" ]; then
    exec "$REAL_PYTHON" "$@"
fi
exit 1
""",
        encoding="utf-8",
    )
    python_stub.chmod(0o755)

    env = {
        "PATH": str(bin_dir),
        "HOME": str(tmp_path),
        "STALE_MARKER": str(stale_marker),
        "REAL_PYTHON": real_python,
    }
    processes = [
        subprocess.Popen(
            ["/bin/sh", str(script_dir / "run.sh")],
            cwd=repo,
            env=env,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )
        for _ in range(2)
    ]
    results = [process.communicate() for process in processes]
    assert [process.returncode for process in processes] == [0, 0], results
    assert not stale_marker.exists()
