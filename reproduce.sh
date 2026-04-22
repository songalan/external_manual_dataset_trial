#!/bin/bash
set -e
ISSUE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TOOLS_DIR="$HOME/.tools"
ENV_FILE="$HOME/.issue_env"

# ──────────────────────────────────────────
# 1. 安装运行环境
# ──────────────────────────────────────────

# [conda 环境] 基础 Python 环境
if ! conda env list | grep -q "^ipguard-env "; then
    conda env create -f "$ISSUE_DIR/environment.yml"
fi

# [Go 安装] Go 1.25 不在 conda-forge 中，手动下载安装
GO_VERSION="1.25.2"
GO_TAR="go${GO_VERSION}.linux-amd64.tar.gz"
GO_URL="https://go.dev/dl/${GO_TAR}"

mkdir -p "$TOOLS_DIR"
if [ ! -d "$TOOLS_DIR/go" ]; then
    echo "Installing Go ${GO_VERSION}..."
    curl -fsSL "$GO_URL" | tar -C "$TOOLS_DIR" -xzf -
    mv "$TOOLS_DIR/go" "$TOOLS_DIR/go-${GO_VERSION}" 2>/dev/null || true
    ln -s "$TOOLS_DIR/go-${GO_VERSION}" "$TOOLS_DIR/go" 2>/dev/null || true
fi

export GOROOT="$TOOLS_DIR/go"
export GOPATH="$HOME/go"
export PATH="$GOROOT/bin:$GOPATH/bin:$PATH"

# 持久化环境变量
echo "export GOROOT=$GOROOT" > "$ENV_FILE"
echo "export GOPATH=$GOPATH" >> "$ENV_FILE"
echo "export PATH=$GOROOT/bin:\$GOPATH/bin:\$PATH" >> "$ENV_FILE"
grep -qF "source $ENV_FILE" "$HOME/.bashrc" || echo "source $ENV_FILE" >> "$HOME/.bashrc"

# ──────────────────────────────────────────
# 2. 将 init/ 复制为 workspace/（此步必须成功）
# ──────────────────────────────────────────
[ -d "$ISSUE_DIR/workspace" ] && rm -rf "$ISSUE_DIR/workspace"
cp -r "$ISSUE_DIR/init" "$ISSUE_DIR/workspace"

# ──────────────────────────────────────────
# 3. 下载 Go 依赖
# ──────────────────────────────────────────
cd "$ISSUE_DIR/workspace"
go mod download

echo ""
echo "=== 环境安装完成 ==="
echo "  Go: $(go version)"
echo "  工作区: $ISSUE_DIR/workspace"
echo ""
echo "请运行: source $ENV_FILE && conda activate ipguard-env"
