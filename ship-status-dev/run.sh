#!/bin/sh
set -eu

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
VENV_DIR="$SCRIPT_DIR/.venv"
HASH_FILE="$VENV_DIR/.requirements.hash"
LOCK_FILE="$SCRIPT_DIR/.venv-setup.lock"

REQ_HASH=""
if command -v shasum >/dev/null 2>&1; then
    REQ_HASH=$(shasum -a 256 "$SCRIPT_DIR/requirements.txt" | cut -d' ' -f1)
elif command -v sha256sum >/dev/null 2>&1; then
    REQ_HASH=$(sha256sum "$SCRIPT_DIR/requirements.txt" | cut -d' ' -f1)
fi

_venv_ok() {
    _py="$VENV_DIR/bin/python"
    [ -x "$_py" ] && "$_py" -c "import fastmcp" >/dev/null 2>&1
}

_needs_install() {
    if [ ! -d "$VENV_DIR" ]; then
        return 0
    fi
    if [ -n "$REQ_HASH" ] && { [ ! -f "$HASH_FILE" ] || [ "$(cat "$HASH_FILE" 2>/dev/null)" != "$REQ_HASH" ]; }; then
        return 0
    fi
    if ! _venv_ok; then
        return 0
    fi
    return 1
}

_remove_venv() {
    # -e is false for dangling symlinks; clear any .venv link so venv recreate can proceed.
    if [ -L "$VENV_DIR" ]; then
        rm -f "$VENV_DIR"
        return 0
    fi
    if [ ! -e "$VENV_DIR" ]; then
        return 0
    fi
    chmod -R u+w "$VENV_DIR" 2>/dev/null || true
    # Rename first so concurrent readers cannot race against rm on virtiofs.
    stale="$SCRIPT_DIR/.venv.stale.$$"
    mv "$VENV_DIR" "$stale"
    rm -rf "$stale" 2>/dev/null || true
}

_setup_venv() {
    if _needs_install; then
        _remove_venv
        python3.12 -m venv "$VENV_DIR" 2>/dev/null || python3 -m venv "$VENV_DIR"
        "$VENV_DIR/bin/pip" install --upgrade pip -q
        "$VENV_DIR/bin/pip" install -r "$SCRIPT_DIR/requirements.txt" -q
        if [ -n "$REQ_HASH" ]; then
            echo "$REQ_HASH" > "$HASH_FILE"
        fi
    fi
}

if [ "${SHIP_STATUS_DEV_SETUP_ONLY:-}" = "1" ]; then
    _setup_venv
    exit 0
fi

# fcntl works on Unix systems, including macOS where flock is not installed.
python3 - "$LOCK_FILE" "$0" <<'PY'
import fcntl
import os
import subprocess
import sys

with open(sys.argv[1], "a+") as lock_file:
    fcntl.flock(lock_file, fcntl.LOCK_EX)
    env = dict(os.environ, SHIP_STATUS_DEV_SETUP_ONLY="1")
    subprocess.run(["/bin/sh", sys.argv[2]], env=env, check=True)
PY

exec "$VENV_DIR/bin/python" "$SCRIPT_DIR/server.py"
