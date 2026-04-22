#!/bin/bash
set -e
ISSUE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# 激活环境
source "$HOME/.issue_env"

cd "$ISSUE_DIR/workspace"

echo "=== 运行 ipguard 中间件测试 ==="
go test -v -count=1 ./middleware/ipguard/

echo ""
echo "=== 所有测试通过 ==="
