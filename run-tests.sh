#!/bin/bash
set -e
ISSUE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

eval "$(conda shell.bash hook)"
conda activate drizzle-soft-delete

cd "$ISSUE_DIR/workspace"

# P1: init 状态下 soft-delete 模块不存在
echo "=== P1: Verify soft-delete module does NOT exist in init ==="
if [ -d "drizzle-orm/src/soft-delete" ]; then
    echo "FAIL: drizzle-orm/src/soft-delete should NOT exist in init"
    exit 1
fi
if [ -f "drizzle-orm/tests/soft-delete.test.ts" ]; then
    echo "FAIL: drizzle-orm/tests/soft-delete.test.ts should NOT exist in init"
    exit 1
fi
echo "PASS: soft-delete module correctly absent in init"

# F1: final 状态下 soft-delete 测试通过
echo "=== F1: Verify soft-delete tests pass in final ==="
# Apply final: copy soft-delete files into workspace
cp -r "$ISSUE_DIR/final/drizzle-orm/src/soft-delete" "drizzle-orm/src/soft-delete"
cp "$ISSUE_DIR/final/drizzle-orm/tests/soft-delete.test.ts" "drizzle-orm/tests/soft-delete.test.ts"

# Run the soft-delete tests
cd "$ISSUE_DIR/workspace"
pnpm --filter drizzle-orm test -- tests/soft-delete.test.ts 2>&1 | tee /tmp/test_output.txt

if grep -q "46 passed" /tmp/test_output.txt; then
    echo "PASS: All 46 soft-delete tests passed"
else
    echo "FAIL: Soft-delete tests did not pass"
    exit 1
fi

echo ""
echo "=== All checks completed ==="
