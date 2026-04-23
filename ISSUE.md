# Issue: 为 Drizzle ORM 实现 Soft Delete 软删除功能

## 1. 模拟的用户真实请求

> **[用户]** 我在使用 drizzle-orm 做项目，目前删除数据用的是硬删除（DELETE），但业务上很多场景只是想"标记为已删除"而不是真的删掉。比如用户注销后还想保留记录、订单取消后还要能查到。我现在只能每次手动在表里加 deletedAt 字段，然后查询时手动加 where 条件过滤，特别麻烦。
>
> 我想要一个统一的 soft-delete 方案，能自动帮我做这些事：
> 1. 自动给表加 deletedAt 列（PG 用 timestamp、MySQL 用 datetime、SQLite 用 integer）
> 2. 提供查询辅助函数：过滤未删除的行、只查已删除的行
> 3. 提供软删除和恢复的操作函数：设置 deletedAt = now()、恢复 deletedAt = null
> 4. 支持级联软删除：父表软删除时，自动找到关联的子表并一起软删除
> 5. 提供迁移辅助函数：给已有的表加 deletedAt 列的 ALTER TABLE 语句
> 6. 类型安全：只有标记了 soft-delete 的表才能使用这些辅助函数，普通表传进去应该编译报错

> **[用户]** *（上下文：看到初步实现后）* 还需要支持自定义字段名，比如有些项目用 snake_case 命名 `deleted_at` 而不是 `deletedAt`。另外批量软删除也很常用，比如一次软删除多个 id，希望能提供 batch 版本的 where 条件。

## 2. 详细问题描述

- **影响目录**：`drizzle-orm/src/soft-delete/`（新增），`drizzle-orm/tests/soft-delete.test.ts`（新增）
- **现象**：当前 drizzle-orm 没有任何内置的软删除支持。开发者必须：
  - 手动在每张表的定义中添加 `deletedAt` 列
  - 每次查询时手动加 `.where(isNull(users.deletedAt))` 过滤条件
  - 软删除操作需要手动写 `.set({ deletedAt: sql\`now()\` })` 
  - 恢复操作需要手动写 `.set({ deletedAt: null })`
  - 级联软删除需要开发者自己遍历外键关系并逐表操作
  - 迁移时需要手写 ALTER TABLE 语句添加 deletedAt 列
- **对用户的实际影响**：代码重复、容易遗漏过滤条件导致"已删除"数据泄漏、无类型安全保障

## 3. 根本原因分析

drizzle-orm 核心库只提供底层的查询构建能力（select/insert/update/delete），不提供任何软删除的业务抽象。需要新增一个 `soft-delete` 模块来封装这些通用模式：

1. **自动追加列**：通过方言特定的 wrapper 函数（`pgSoftDelete`/`mysqlSoftDelete`/`sqliteSoftDelete`），在表创建时自动追加 `deletedAt` 列
2. **类型品牌**：使用 Symbol 在表对象上标记 `SoftDeleteTable` 品牌，编译期约束只有软删除表才能使用辅助函数
3. **查询辅助**：`softDeleteWhere(table)` → `deletedAt IS NULL`，`onlyDeletedWhere(table)` → `deletedAt IS NOT NULL`
4. **操作辅助**：`softDeleteSet(table)` → `{ deletedAt: sql\`now()\` }`，`restoreSet(table)` → `{ deletedAt: null }`
5. **级联软删除**：通过 `buildCascadeRelationMap` 从 schema 的外键关系中自动构建级联映射
6. **迁移辅助**：`pgAddSoftDeleteColumn`/`mysqlAddSoftDeleteColumn`/`sqliteAddSoftDeleteColumn` 生成 ALTER TABLE DDL

## 4. 期望结果

| 验收编号 | 辅助 verifier | 评测动作（运行什么） | 检查位置（检查哪里） | 通过标准（应得到什么） | 常见失败表现 |
|----------|---------------|----------------------|----------------------|------------------------|--------------|
| A1 | `P1` | 检查 init 中 `drizzle-orm/src/soft-delete/` 目录 | 文件系统 | 目录不存在 | 目录意外存在 |
| A2 | `F1` | 将 final 的 soft-delete 文件复制到 workspace 后运行 `pnpm --filter drizzle-orm test -- tests/soft-delete.test.ts` | 终端输出 | 全部 46 个测试通过 | 测试失败或模块导入错误 |
| A3 | | 在 workspace 中编写代码：`import { pgSoftDelete, softDeleteWhere } from '~/soft-delete/pg.ts'; const users = pgSoftDelete(pgTable('users', { id: serial().primaryKey(), name: text().notNull() }));` | TypeScript 编译 + 运行时 | `users.deletedAt` 是 PgTimestamp 类型列；`softDeleteWhere(users)` 返回 SQL 对象；`isSoftDeleteTable(users)` 返回 true | 导入失败或类型不匹配 |
| A4 | | 运行 `import { mysqlSoftDelete } from '~/soft-delete/mysql.ts';` 创建 MySQL 软删除表 | 运行时 | `deletedAt` 列类型为 MySqlDateTime | 列类型不是 MySqlDateTime |
| A5 | | 运行 `import { sqliteSoftDelete } from '~/soft-delete/sqlite.ts';` 创建 SQLite 软删除表 | 运行时 | `deletedAt` 列类型为 SQLiteInteger | 列类型不是 SQLiteInteger |
| A6 | | 调用 `pgAddSoftDeleteColumn('users')` | 返回值 | 返回 `'ALTER TABLE "users" ADD COLUMN "deletedAt" timestamp with time zone DEFAULT NULL;'` | SQL 格式不正确 |
| A7 | | 使用 `buildCascadeRelationMap` 构建含 FK 的 schema 的级联映射 | 返回值 | 父表到子表的映射关系正确，包含 childTable/childColumns/parentColumns | 映射为空或结构不正确 |
| A8 | | 使用自定义字段名 `pgSoftDelete(pgTable(...), { fieldName: 'deleted_at' })` | 运行时 | `softDeleteSet(users)` 的 key 为 `'deleted_at'` 而非 `'deletedAt'` | key 仍是默认值 |

## 5. 改动方案

### 新增文件

1. **`drizzle-orm/src/soft-delete/soft-delete.ts`** — 核心模块（方言无关）
   - `SoftDeleteConfig` / `SoftDeleteMetaInfo` 配置类型
   - `SoftDeleteBrand` / `SoftDeleteMeta` Symbol 标记
   - `SoftDeleteTable` / `SoftDeleteTableConstraint` 品牌类型
   - `softDelete()` 核心包装函数
   - `softDeleteWhere()` / `onlyDeletedWhere()` 查询辅助
   - `softDeleteSet()` / `restoreSet()` 操作辅助
   - `batchSoftDeleteWhere()` / `batchOnlyDeletedWhere()` / `batchRestoreWhere()` 批量辅助
   - `pgAddSoftDeleteColumn()` / `mysqlAddSoftDeleteColumn()` / `sqliteAddSoftDeleteColumn()` 迁移辅助
   - `buildCascadeRelationMap()` / `getCascadeSoftDeleteConditions()` / `getCascadeRestoreConditions()` 级联辅助
   - `isSoftDeleteTable()` / `getSoftDeleteMeta()` / `getSoftDeletedAtColumn()` 类型守卫

2. **`drizzle-orm/src/soft-delete/pg.ts`** — PostgreSQL 方言
   - `pgSoftDelete()` 使用 `timestamp()` builder 自动追加 `deletedAt` 列
   - 支持 `withTimezone` / `precision` 配置

3. **`drizzle-orm/src/soft-delete/mysql.ts`** — MySQL 方言
   - `mysqlSoftDelete()` 使用 `datetime()` builder 自动追加 `deletedAt` 列
   - 支持 `fsp` 配置

4. **`drizzle-orm/src/soft-delete/sqlite.ts`** — SQLite 方言
   - `sqliteSoftDelete()` 使用 `integer()` builder 自动追加 `deletedAt` 列
   - 存储为 Unix 纪元毫秒

5. **`drizzle-orm/src/soft-delete/index.ts`** — 统一导出入口

6. **`drizzle-orm/tests/soft-delete.test.ts`** — 完整测试（46 个用例）

## 6. 复现步骤

### 第一步：安装初始环境
```bash
bash $ISSUE_ROOT/reproduce.sh
```

### 第二步：激活环境，运行，观察输出
```bash
conda activate drizzle-soft-delete
cd $ISSUE_ROOT/workspace
pnpm --filter drizzle-orm test -- tests/soft-delete.test.ts 2>&1 | tee /tmp/init_output.txt
```

### 第三步：验证初始现象
- 测试文件 `drizzle-orm/tests/soft-delete.test.ts` 不存在，运行会报错
- `drizzle-orm/src/soft-delete/` 目录不存在，无法导入任何软删除模块

### 第四步：验证改动后效果
```bash
# 应用 final：复制 soft-delete 文件到 workspace
cp -r $ISSUE_ROOT/final/drizzle-orm/src/soft-delete $ISSUE_ROOT/workspace/drizzle-orm/src/soft-delete
cp $ISSUE_ROOT/final/drizzle-orm/tests/soft-delete.test.ts $ISSUE_ROOT/workspace/drizzle-orm/tests/soft-delete.test.ts

# 运行测试
cd $ISSUE_ROOT/workspace
pnpm --filter drizzle-orm test -- tests/soft-delete.test.ts
```
- A1: init 中无 soft-delete 目录 ✓
- A2: 全部 46 个测试通过 ✓
- A3-A8: 按 ISSUE.md 中的验收表逐条检查

## 7. 元信息

- 仓库：https://github.com/drizzle-team/drizzle-orm
- Base commit：`273c7807`（2025-04-23）
- 题型：新功能实现
- 难度：中

## 8. 文件清单

| 文件/目录 | 用途说明 |
|-----------|---------|
| ISSUE.md | Issue 描述、复现步骤、验证方法 |
| reproduce.sh | 一键安装脚本 |
| environment.yml | conda 环境定义 |
| run-tests.sh | 自动化验证脚本 |
| changes.diff | unified diff |
| init/ | base commit 的代码库快照 |
| final/ | 实现功能后的代码库快照 |
| workspace/ | 由 reproduce.sh 自动生成，用于复现 |
