# Issue: 为 dbmate status 命令增加分页、限制、格式化、过滤和颜色功能

## 1. 模拟的用户真实请求

> **[用户]** 针对大量迁移的情况，我希望增加一个分页功能，比如 `--page` 和 `--page-size` 参数，这样 migration 很多的时候可以分页查看。

> **[用户]** 再添加一个 `--limit` 参数用于限制显示数量，和分页互斥。

> **[用户]** 针对 JSON 输出增加一个 `--pretty` 参数，用于控制缩进。不指定默认2个空格缩进，指定的话就根据指定的数字来。

> **[用户]** 再增加一个 `--filter` 参数，过滤只看 applied 或者 pending 的迁移。

> **[用户]** 再增加一个 `--color` 和 `--no-color` 参数用于控制台颜色的控制。默认自动检测是否终端，`--color` 强制开启，`--no-color` 强制关闭。

## 2. 详细问题描述

- **影响文件**：`main.go`（CLI 入口）、`pkg/dbmate/db.go`（核心逻辑）
- **现象一**：当项目有大量迁移文件时，`dbmate status` 输出过长，无法快速定位感兴趣的记录
- **现象二**：JSON 输出始终是紧凑格式，不便于人工阅读
- **现象三**：无法只查看已应用或待应用的迁移，需要在大量输出中手动查找
- **现象四**：终端输出没有颜色区分，已应用和待应用的迁移难以快速区分
- **对用户的实际影响**：在日常数据库迁移管理中，缺少这些功能导致操作效率低下

## 3. 根本原因分析

- `pkg/dbmate/db.go` 中的 `DashboardResult` 结构体和打印函数不具备分页、限制、过滤能力
- `PrintDashboardTable` 和 `PrintDashboardJSON` 函数缺少颜色和缩进控制参数
- `main.go` 中 `status` 命令的 CLI 定义缺少对应参数

## 4. 期望结果

| 验收编号 | 辅助 verifier | 评测动作（运行什么） | 检查位置（检查哪里） | 通过标准（应得到什么） | 常见失败表现 |
|----------|---------------|----------------------|----------------------|------------------------|--------------|
| A1 | P1 | 检查 `init/pkg/dbmate/db.go` 源码 | 源码文件 | 不存在 `PaginateDashboardResult`、`LimitDashboardResult`、`FilterDashboardResult` 函数 | init 中已包含这些函数 |
| A2 | F1 | 在 `final/` 目录运行 `go build ./...` | 终端输出 | 编译成功，无错误 | 编译失败 |
| A3 | F2 | 检查 `final/pkg/dbmate/db.go` 源码 | 源码文件 | 存在 `PaginateDashboardResult`、`LimitDashboardResult`、`FilterDashboardResult` 函数；`PrintDashboardTable` 支持 color 参数；`PrintDashboardJSON` 支持 indent 参数 | 缺少关键函数或参数 |
| A4 | F2 | 检查 `final/main.go` 源码 | 源码文件 | 存在 `--page`、`--page-size`、`--limit`、`--filter`、`--pretty`、`--color`、`--no-color` CLI flag 定义 | 缺少某个 flag |
| A5 | F3 | 在 `final/` 目录运行 `go test -v -run "TestPaginate|TestLimit|TestFilter|TestPrintDashboard" ./pkg/dbmate/...` | 终端输出 | 所有测试通过（PASS） | 测试失败 |
| A6 | G1 | 在 `final/` 目录运行 `go vet ./...` | 终端输出 | 无输出，退出码为 0 | 报告代码问题 |

## 5. 改动方案

### main.go 改动

新增以下 CLI flag：
- `--page` (IntFlag)：分页页码，与 `--page-size` 必须同时使用
- `--page-size` (IntFlag)：每页记录数，默认 20
- `--limit` (IntFlag)：限制显示数量，与分页互斥
- `--pretty` (IntFlag)：JSON 缩进空格数，默认 2，0 表示紧凑输出
- `--filter` (StringFlag)：过滤类型，值为 "applied" 或 "pending"
- `--color` (BoolFlag)：强制开启颜色输出
- `--no-color` (BoolFlag)：强制关闭颜色输出

在 status 命令的 action 中，按 Sort → Filter → Limit/Paginate → Print 的流水线处理。

### pkg/dbmate/db.go 改动

- `DashboardResult` 结构体新增 `Page`、`PageSize`、`TotalPages` 字段
- 新增 `PaginateDashboardResult(result, page, pageSize)` 函数：1-based 分页，自动钳制页码
- 新增 `LimitDashboardResult(result, limit)` 函数：截断到前 N 条
- 新增 `FilterDashboardResult(result, filter)` 函数：按 "applied"/"pending" 过滤，重算统计
- `PrintDashboardTable` 新增 `useColor bool` 参数：绿色标记已应用，黄色标记待应用
- `PrintDashboardJSON` 新增 `indent string` 参数：非空时用指定缩进格式化输出

## 6. 复现步骤

### 第一步：安装初始环境

```bash
bash $ISSUE_ROOT/reproduce.sh
```

### 第二步：激活环境，运行，观察输出

```bash
conda activate dbmate-status
cd $ISSUE_ROOT/workspace
go build ./...
```

### 第三步：验证初始现象

- 检查 `pkg/dbmate/db.go` 中不存在 `PaginateDashboardResult` 等新函数
- 检查 `main.go` 中不存在 `--page`、`--page-size` 等新 flag
- 运行 `go test ./pkg/dbmate/...` 确认基础测试通过

### 第四步：验证改动后效果

将 final 目录的改动应用到 workspace：

```bash
cp -r $ISSUE_ROOT/final/main.go $ISSUE_ROOT/workspace/main.go
cp -r $ISSUE_ROOT/final/pkg/dbmate/db.go $ISSUE_ROOT/workspace/pkg/dbmate/db.go
cp -r $ISSUE_ROOT/final/pkg/dbmate/dashboard_test.go $ISSUE_ROOT/workspace/pkg/dbmate/dashboard_test.go
cp -r $ISSUE_ROOT/final/testdata/ $ISSUE_ROOT/workspace/testdata/
```

然后按 A1-A6 逐条验收：

- **A1**: 在 init 源码中确认缺少新函数 → `grep "PaginateDashboardResult" $ISSUE_ROOT/init/pkg/dbmate/db.go` 应无输出
- **A2**: `cd $ISSUE_ROOT/workspace && go build ./...` → 编译成功
- **A3**: `grep "func PaginateDashboardResult" $ISSUE_ROOT/final/pkg/dbmate/db.go` → 有输出
- **A4**: `grep "\-\-page" $ISSUE_ROOT/final/main.go` → 有输出
- **A5**: `cd $ISSUE_ROOT/workspace && go test -v -run "TestPaginate|TestLimit|TestFilter|TestPrintDashboard" ./pkg/dbmate/...` → PASS
- **A6**: `cd $ISSUE_ROOT/workspace && go vet ./...` → 无错误

## 7. 元信息

- 仓库：https://github.com/amacneil/dbmate
- Base commit：`2036c594de2cf49e357d14561665f03b7df108c7`（2026-04-04）
- 题型：新功能实现
- 难度：中

## 8. 文件清单

| 文件/目录           | 用途说明                     |
| --------------- | ------------------------ |
| ISSUE.md        | Issue 描述、复现步骤、验证方法       |
| reproduce.sh    | 一键安装脚本                   |
| environment.yml | conda 环境定义               |
| run-tests.sh    | 自动化测试入口                  |
| changes.diff    | unified diff             |
| tests/test_outputs.py | Python 自动化 verifier |
| init/           | base commit 的代码库快照       |
| final/          | 实现功能后的代码库快照              |
| workspace/      | 由 reproduce.sh 自动生成，用于复现 |
