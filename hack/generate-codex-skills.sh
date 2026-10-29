#!/usr/bin/env bash

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROMPT_DIR="$PROJECT_ROOT/.apm/prompts"
SKILL_DIR="$PROJECT_ROOT/.agents/skills"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

shopt -s nullglob
prompts=("$PROMPT_DIR"/*.prompt.md)
if ((${#prompts[@]} == 0)); then
    echo "No prompt files found in $PROMPT_DIR" >&2
    exit 1
fi

for prompt in "${prompts[@]}"; do
    name="$(basename "$prompt" .prompt.md)"
    description="$(sed -n 's/^description: "\(.*\)"$/\1/p' "$prompt" | head -1)"
    if [[ -z "$description" ]]; then
        echo "Missing description in $prompt" >&2
        exit 1
    fi

    target="$TMP_DIR/$name"
    mkdir -p "$target"
    {
        printf -- '---\n'
        printf 'name: %s\n' "$name"
        printf 'description: "%s"\n' "$description"
        printf -- '---\n\n'
        awk 'BEGIN { in_body = 0 } /^---$/ { if (in_body) { in_body = 0; body = 1; next } in_body = 1; next } body { print }' "$prompt" | sed 's/—/-/g'
    } > "$target/SKILL.md"
done

mkdir -p "$SKILL_DIR"
find "$SKILL_DIR" -mindepth 1 -maxdepth 1 -type d -name 'ship-status-dev-*' -exec rm -rf {} +
for directory in "$TMP_DIR"/*; do
    mv "$directory" "$SKILL_DIR/"
done
