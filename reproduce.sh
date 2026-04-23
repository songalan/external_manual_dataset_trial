#!/bin/bash
set -e
ISSUE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TOOLS_DIR="$HOME/.tools"
ENV_FILE="$HOME/.issue_env"

# ──────────────────────────────────────────
# 1. 安装运行环境
# ──────────────────────────────────────────

# [conda 环境]
if ! conda env list | grep -q "^dbmate-status "; then
    conda env create -f "$ISSUE_DIR/environment.yml"
fi

# [Go 依赖下载]
# Go modules will be downloaded during build/test, which requires network.
# For offline Docker, vendor the dependencies beforehand.
# conda provides go 1.22; the project uses go 1.25+ so we may need a newer version.
# Check if go version meets requirements, otherwise download manually.
GO_VERSION=$(go version 2>/dev/null | grep -oP 'go\K[0-9.]+' || echo "0.0")
GO_REQUIRED="1.25.0"

# Simple version comparison: if installed go < 1.25, download newer go
if [ "$(printf '%s\n' "$GO_REQUIRED" "$GO_VERSION" | sort -V | head -n1)" != "$GO_REQUIRED" ]; then
    echo "Go $GO_VERSION is sufficient (>= $GO_REQUIRED)"
else
    echo "Installed Go $GO_VERSION is older than $GO_REQUIRED, downloading Go 1.25.0..."
    mkdir -p "$TOOLS_DIR"
    if [ ! -d "$TOOLS_DIR/go125" ]; then
        curl -fsSL "https://go.dev/dl/go1.25.0.linux-amd64.tar.gz" | tar -xz -C "$TOOLS_DIR"
        mv "$TOOLS_DIR/go" "$TOOLS_DIR/go125"
    fi
    export GOROOT="$TOOLS_DIR/go125"
    export PATH="$GOROOT/bin:$PATH"
    echo "export GOROOT=$GOROOT" >> "$ENV_FILE"
    echo "export PATH=$GOROOT/bin:\$PATH" >> "$ENV_FILE"
fi

# 持久化环境变量
[ -f "$ENV_FILE" ] && grep -qF "source $ENV_FILE" "$HOME/.bashrc" || echo "source $ENV_FILE" >> "$HOME/.bashrc"

# ──────────────────────────────────────────
# 2. 将 init/ 复制为 workspace/（此步必须成功）
# ──────────────────────────────────────────
[ -d "$ISSUE_DIR/workspace" ] && rm -rf "$ISSUE_DIR/workspace"
cp -r "$ISSUE_DIR/init" "$ISSUE_DIR/workspace"
