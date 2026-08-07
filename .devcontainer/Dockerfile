FROM registry.access.redhat.com/ubi9/ubi:latest

ARG GOLANGCI_LINT_VERSION=2.10.1

RUN dnf module install nodejs:22/common -y && \
    dnf install -y --allowerasing \
    curl \
    git \
    make \
    jq \
    tar \
    gzip \
    wget \
    vim-minimal \
    findutils \
    procps-ng \
    sudo \
    go \
    python3.12 \
    libstdc++ \
    postgresql \
    gettext \
    lsof \
    && dnf clean all

RUN ARCH=$(uname -m) && \
    if [ "$ARCH" = "aarch64" ]; then LINT_ARCH="arm64"; else LINT_ARCH="amd64"; fi && \
    rpm -i "https://github.com/golangci/golangci-lint/releases/download/v${GOLANGCI_LINT_VERSION}/golangci-lint-${GOLANGCI_LINT_VERSION}-linux-${LINT_ARCH}.rpm"

# Playwright system library dependencies (Chromium requires these at runtime)
RUN dnf install -y \
    alsa-lib at-spi2-atk at-spi2-core atk cairo cups-libs dbus-libs \
    expat glib2 libdrm libX11 libXcomposite libXdamage libXext libXfixes \
    libXrandr libxcb libxkbcommon mesa-libgbm mesa-libGL nspr nss nss-util pango \
    && dnf clean all && rm -rf /var/cache/dnf

RUN groupadd -g 1000 vscode && \
    useradd -u 1000 -g 1000 -m -s /bin/bash vscode && \
    echo "vscode ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/vscode

ENV GOPATH=/home/vscode/go
ENV PATH="/home/vscode/.local/bin:${GOPATH}/bin:/usr/local/go/bin:${PATH}"

# uv/uvx for `make apm` — pinned copy from Astral image (no curl|sh installer)
ARG UV_VERSION=0.7.13
COPY --from=ghcr.io/astral-sh/uv:${UV_VERSION} /uv /usr/local/bin/uv
COPY --from=ghcr.io/astral-sh/uv:${UV_VERSION} /uvx /usr/local/bin/uvx
RUN uv --version && uvx --version

USER vscode

RUN npx playwright@1.62.1 install chromium

RUN go install gotest.tools/gotestsum@latest

ARG PROMETHEUS_VERSION=3.4.1
RUN ARCH=$(uname -m) && \
    if [ "$ARCH" = "aarch64" ]; then PROM_ARCH="arm64"; else PROM_ARCH="amd64"; fi && \
    wget -q "https://github.com/prometheus/prometheus/releases/download/v${PROMETHEUS_VERSION}/prometheus-${PROMETHEUS_VERSION}.linux-${PROM_ARCH}.tar.gz" -O /tmp/prometheus.tar.gz && \
    tar -xzf /tmp/prometheus.tar.gz -C /tmp && \
    sudo mv /tmp/prometheus-${PROMETHEUS_VERSION}.linux-${PROM_ARCH}/prometheus /usr/local/bin/prometheus && \
    sudo mv /tmp/prometheus-${PROMETHEUS_VERSION}.linux-${PROM_ARCH}/promtool /usr/local/bin/promtool && \
    rm -rf /tmp/prometheus*

# Cursor's install script calls base64 -D (macOS flag) but GNU coreutils uses -d
RUN sudo cp /usr/bin/base64 /usr/bin/base64.gnu && \
    printf '#!/bin/sh\nfor a in "$@"; do shift; case "$a" in -D) set -- "$@" -d;; *) set -- "$@" "$a";; esac; done\nexec /usr/bin/base64.gnu "$@"\n' | sudo tee /usr/bin/base64 > /dev/null && \
    sudo chmod +x /usr/bin/base64

ARG CLAUDE_INSTALLER_VERSION=latest
RUN curl -fsSL https://claude.ai/install.sh | sh

WORKDIR /workspace
