#!/bin/bash
set -e
ISSUE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TOOLS_DIR="$HOME/.tools"
ENV_FILE="$HOME/.issue_env"

# ──────────────────────────────────────────
# 1. 安装运行环境
# ──────────────────────────────────────────

# [conda 环境]
if ! conda env list | grep -q "^drizzle-soft-delete "; then
    conda env create -f "$ISSUE_DIR/environment.yml"
fi

# [安装 pnpm]
eval "$(conda shell.bash hook)"
conda activate drizzle-soft-delete
if ! command -v pnpm &>/dev/null; then
    npm install -g pnpm@10
fi

# ──────────────────────────────────────────
# 2. 将 init/ 复制为 workspace/（此步必须成功）
# ──────────────────────────────────────────
[ -d "$ISSUE_DIR/workspace" ] && rm -rf "$ISSUE_DIR/workspace"
cp -r "$ISSUE_DIR/init" "$ISSUE_DIR/workspace"

# ──────────────────────────────────────────
# 3. 安装项目依赖
# ──────────────────────────────────────────
cd "$ISSUE_DIR/workspace"
pnpm install --frozen-lockfile 2>/dev/null || pnpm install

echo "✅ Environment ready. Run: conda activate drizzle-soft-delete"
