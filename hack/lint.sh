#!/bin/bash

set -ex

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_ROOT"

echo "Running golangci-lint..."
if [ "$CI" = "true" ]; then
    go version
    golangci-lint version -v
    golangci-lint --timeout 10m run
else
    DOCKER=${DOCKER:-podman}

    if ! which "$DOCKER" > /dev/null 2>&1; then
        echo "$DOCKER not found, please install."
        exit 1
    fi

    # Check if running on Linux
    VOLUME_OPTION=""
    if [[ "$(uname -s)" == "Linux" ]]; then
        VOLUME_OPTION=":z"
    fi

    $DOCKER run --rm \
        --volume "${PROJECT_ROOT}:/workspace${VOLUME_OPTION}" \
        --workdir /workspace \
        quay-proxy.ci.openshift.org/openshift/ci-public:ci_golangci-lint_latest \
        golangci-lint --timeout 10m run
fi

echo ""
echo "Running eslint for frontend..."
cd "$PROJECT_ROOT/frontend"

if [ ! -f "package.json" ]; then
    echo "Error: frontend/package.json not found"
    exit 1
fi

if [ ! -d "node_modules" ]; then
    echo "Installing frontend dependencies..."
    npm install
fi

npm run lint

echo ""
echo "Running npm audit for production dependencies..."
npm audit --omit=dev

echo ""
echo "✓ All linting checks passed"
