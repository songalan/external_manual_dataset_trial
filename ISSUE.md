# Issue: 为 Fiber 框架新增 IP 白名单中间件（ipguard）

## 1. 模拟的用户真实请求

> **[用户]** 我在用 Fiber 框架做一个内部管理系统，需要根据 IP 做访问控制，只允许白名单里的 IP 访问。我翻了 Fiber 的中间件列表，没有找到现成的 IP 白名单中间件。能不能帮忙写一个？基本需求：
> - 支持单个 IP 和 CIDR 范围（IPv4 和 IPv6 都要支持）
> - 能配置白名单和自定义的拒绝处理
> - 某些路径（比如 /health）需要排除检查

> **[用户]** 用了一段时间，发现我们的服务部署在 Nginx 反向代理后面，客户端的真实 IP 是通过 X-Real-IP 头传过来的。现在中间件拿到的是代理的 IP，白名单完全没效果。能不能加一个支持解析 X-Real-IP 头的功能？

> **[用户]** 又发现一个问题，服务器监听的是 IPv6 socket，IPv4 客户端连进来后，c.IP() 返回的是 `::ffff:192.168.1.1` 这种 IPv4-mapped IPv6 格式。我在白名单里写的是 `192.168.1.1`，结果匹配不上。能不能自动把 `::ffff:` 前缀的地址转成普通 IPv4？

> **[用户]** 还有个需求，下游的 handler 里也想拿到中间件解析出来的客户端 IP（比如记日志、做限流），不想再重新解析一遍 X-Real-IP 或 X-Forwarded-For。中间件能不能把解析好的 IP 存到 context 里，让我在其他地方能直接取？另外最好也支持自定义存储的 key，以及提供个 `ClientIPFromContext` 的辅助函数方便取值。

## 2. 详细问题描述

- **影响文件**：`middleware/ipguard/`（当前不存在，需新建）
- **现象一**：Fiber 框架缺少 IP 白名单中间件，无法基于客户端 IP 做访问控制
- **现象二**：部署在反向代理后时，`c.IP()` 返回的是代理 IP 而非客户端真实 IP，白名单无法正确匹配
- **现象三**：IPv6 socket 上的 IPv4 客户端地址以 `::ffff:x.x.x.x` 格式出现，无法与白名单中的纯 IPv4 地址匹配
- **现象四**：下游 handler 无法获取中间件已解析的客户端 IP，需要重复解析头部信息
- **对用户的实际影响**：内部管理系统无法按 IP 做访问控制；部署在代理后白名单失效；IPv4-mapped IPv6 地址匹配失败；日志和限流等场景需要重复解析 IP

## 3. 根本原因分析

Fiber 框架当前没有提供 IP 白名单中间件。需要新建 `middleware/ipguard/` 包，实现以下功能：

1. **核心白名单**：解析配置的 IP/CIDR 列表，与客户端 IP 做匹配
2. **X-Real-IP 解析**：从 `X-Real-IP` 头提取客户端真实 IP（优先级低于 X-Forwarded-For，高于 `c.IP()`）
3. **IPv4-mapped IPv6 归一化**：将 `::ffff:1.2.3.4` 格式的地址转为 `1.2.3.4`，确保与纯 IPv4 白名单条目匹配
4. **Context 存储**：将解析后的客户端 IP 存入 `c.Locals()`，提供 `ClientIPFromContext()` 辅助函数供下游 handler 使用

## 4. 期望结果

### 关键里程碑

| 验收编号 | 辅助 verifier | 评测动作（运行什么） | 检查位置（检查哪里） | 通过标准（应得到什么） | 常见失败表现 |
|----------|---------------|----------------------|----------------------|------------------------|--------------|
| M1 | `G1` | 在 workspace 目录运行 `go build ./...` | 终端输出 | 编译成功，无错误 | 编译报错 `cannot find package` 或语法错误 |

### 自动化验证

| 验收编号 | 辅助 verifier | 评测动作（运行什么） | 检查位置（检查哪里） | 通过标准（应得到什么） | 常见失败表现 |
|----------|---------------|----------------------|----------------------|------------------------|--------------|
| A1 | `P1` | 在 workspace 目录运行 `go test -v -count=1 -run TestIPGuard_AllowedIP ./middleware/ipguard/` | 终端输出 | 测试全部 PASS，覆盖 IPv4/IPv6 单 IP 和 CIDR 范围的允许/拒绝场景 | 包不存在导致编译失败，或测试 FAIL |
| A2 | `F2` | 在 workspace 目录运行 `go test -v -count=1 -run TestIPGuard_XRealIP ./middleware/ipguard/` | 终端输出 | 测试全部 PASS，X-Real-IP 启用时能从头部提取有效 IP，禁用时忽略头部 | 测试 FAIL 或 X-Real-IP 功能不存在 |
| A3 | `F3` | 在 workspace 目录运行 `go test -v -count=1 -run TestIPGuard_NormalizeIPv4MappedIPv6 ./middleware/ipguard/` | 终端输出 | 测试全部 PASS，`::ffff:192.168.1.1` 能匹配白名单中的 `192.168.1.1` | 归一化未实现，IPv4-mapped IPv6 无法匹配纯 IPv4 |
| A4 | `F4` | 在 workspace 目录运行 `go test -v -count=1 -run TestIPGuard_ClientIPFromContext ./middleware/ipguard/` | 终端输出 | 测试全部 PASS，`ClientIPFromContext` 能正确返回中间件解析的 IP，自定义 ContextKey 时也能通过 `ClientIPFromContext` 获取 | 函数不存在、返回空字符串、自定义 key 时查找失败 |

### 人工操作验证

| 验收编号 | 辅助 verifier | 评测动作（运行什么） | 检查位置（检查哪里） | 通过标准（应得到什么） | 常见失败表现 |
|----------|---------------|----------------------|----------------------|------------------------|--------------|
| H1 | - | 阅读 `middleware/ipguard/config.go` 中的 Config 结构体 | Config 字段定义 | 包含 `AllowedIPs`、`UseXRealIP`、`NormalizeIPv4MappedIPv6`、`ContextKey`、`DisableContextStorage` 等配置字段 | 缺少关键配置字段 |
| H2 | - | 阅读 `middleware/ipguard/ipguard.go` 中的导出函数 | 导出符号 | 存在 `New()` 构造函数和 `ClientIPFromContext()` 辅助函数 | 缺少 `ClientIPFromContext` 或其实现硬编码了 key 不兼容自定义 ContextKey |

## 5. 改动方案

新建 `middleware/ipguard/` 包，包含 3 个文件：

### config.go

定义 `Config` 结构体和默认配置：

- `AllowedIPs []string` — IP/CIDR 白名单
- `ForbiddenHandler fiber.Handler` — 自定义拒绝处理
- `ExcludedPaths []string` — 排除路径前缀
- `ExcludedExactPaths []string` — 排除精确路径
- `IPExtractor func(c fiber.Ctx) string` — 自定义 IP 提取器
- `UseXForwardedFor bool` — 启用 X-Forwarded-For 解析
- `UseXRealIP bool` — 启用 X-Real-IP 解析
- `NormalizeIPv4MappedIPv6 *bool` — 启用 IPv4-mapped IPv6 归一化（默认 true，用 `*bool` 区分"未设置"和"显式关闭"）
- `ContextKey string` — context 存储的 key（默认 `"ipguard:clientIP"`）
- `DisableContextStorage bool` — 禁用 context 存储

### ipguard.go

核心中间件逻辑：

- `New(config ...Config) fiber.Handler` — 创建中间件实例
- `getClientIP()` — IP 提取优先级：IPExtractor > X-Forwarded-For > X-Real-IP > c.IP()
- `isIPAllowed()` — IP/CIDR 匹配检查
- `ClientIPFromContext(c fiber.Ctx) string` — 从 context 获取解析后的客户端 IP
- 归一化逻辑：`net.ParseIP` 后对 IPv4-mapped IPv6 调用 `ip.To4()` 转换
- Context 存储逻辑：始终在默认 key 下存储（供 `ClientIPFromContext` 使用），自定义 key 时额外存储

### ipguard_test.go

全面的测试覆盖：

- `TestIPGuard_AllowedIP` — 基础 IPv4/IPv6 白名单匹配
- `TestIPGuard_ExcludedPaths` / `TestIPGuard_ExcludedExactPaths` — 路径排除
- `TestIPGuard_XForwardedFor` — X-Forwarded-For 解析
- `TestIPGuard_XRealIP` — X-Real-IP 解析
- `TestIPGuard_NormalizeIPv4MappedIPv6` — IPv4-mapped IPv6 归一化
- `TestIPGuard_ClientIPFromContext` — Context IP 存储和获取
- `TestIPGuard_ForbiddenHandler` / `TestIPGuard_Next` / `TestIPGuard_EmptyWhitelist` — 边界条件

## 6. 复现步骤

### 第一步：安装初始环境
```bash
bash $ISSUE_ROOT/reproduce.sh
```

### 第二步：激活环境，验证 init 状态
```bash
source $HOME/.issue_env
conda activate ipguard-env
cd $ISSUE_ROOT/workspace
go build ./...
```

### 第三步：验证初始现象
在 init 状态下，`middleware/ipguard/` 目录不存在：
```bash
ls middleware/ipguard/ 2>&1
# 预期：目录不存在，ls 报错 "No such file or directory"

go test ./middleware/ipguard/ 2>&1
# 预期：编译失败，报错 "cannot find package"
```

### 第四步：验证改动后效果
将 final 中的 ipguard 代码复制到 workspace：
```bash
cp -r $ISSUE_ROOT/final/middleware/ipguard $ISSUE_ROOT/workspace/middleware/ipguard
```

然后按期望结果中的 A1/A2/A3/A4 逐条验收：

```bash
# A1: 基础 IP 白名单
go test -v -count=1 -run TestIPGuard_AllowedIP ./middleware/ipguard/

# A2: X-Real-IP 支持
go test -v -count=1 -run TestIPGuard_XRealIP ./middleware/ipguard/

# A3: IPv4-mapped IPv6 归一化
go test -v -count=1 -run TestIPGuard_NormalizeIPv4MappedIPv6 ./middleware/ipguard/

# A4: ClientIPFromContext
go test -v -count=1 -run TestIPGuard_ClientIPFromContext ./middleware/ipguard/

# M1: 项目构建
go build ./...

# 全量测试
go test -v -count=1 ./middleware/ipguard/
```

## 7. 元信息

- 仓库：https://github.com/gofiber/fiber.git
- Base commit：`4acf46569190c58beaa8f34782bdae147be71ec5`（2026-04-12）
- 题型：新功能实现
- 难度：中

## 8. 文件清单

| 文件/目录           | 用途说明                     |
| --------------- | ------------------------ |
| ISSUE.md        | Issue 描述、复现步骤、验证方法       |
| reproduce.sh    | 一键安装脚本（Go 环境 + workspace） |
| environment.yml | conda 基础环境定义              |
| run-tests.sh    | 自动化测试入口                   |
| changes.diff    | unified diff（新增文件）        |
| tests/test_outputs.py | Python verifier 测试  |
| init/           | base commit 的代码库快照（不含 ipguard） |
| final/          | 实现功能后的代码库快照（含 ipguard）    |
| workspace/      | 由 reproduce.sh 自动生成，用于复现 |
