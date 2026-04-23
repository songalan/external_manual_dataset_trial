# Drizzle ORM Soft Delete

软删除（Soft Delete）功能模块，不真正删除数据，而是给数据添加删除标记（`deletedAt` 时间戳），支持数据恢复和历史查询。

## 特性

- 自动给表 schema 添加 `deletedAt` 时间戳字段
- 支持自定义字段名、类型和默认值
- 查询过滤：`.softDeleteWhere()`（过滤已删除）、`.onlyDeletedWhere()`（仅查已删除）
- 软删除操作：`.softDeleteSet()`（设置时间戳）、`.restoreSet()`（清空时间戳恢复数据）
- 支持 PostgreSQL、MySQL、SQLite 多数据库
- 不修改 ORM 核心引擎，纯工具层实现

## 安装使用

### PostgreSQL

```ts
import { pgTable, serial, text } from 'drizzle-orm/pg-core';
import { eq, and } from 'drizzle-orm';
import {
  pgSoftDelete,
  softDeleteWhere,
  onlyDeletedWhere,
  softDeleteSet,
  restoreSet,
} from 'drizzle-orm/soft-delete/pg';

// 1. 定义带软删除的表 - 自动添加 deletedAt 列
const users = pgSoftDelete(pgTable('users', {
  id: serial().primaryKey(),
  name: text().notNull(),
  email: text().notNull(),
}));
// users 现在包含: id, name, email, deletedAt

// 2. 查询未删除的数据（排除已软删除的行）
const activeUsers = await db
  .select()
  .from(users)
  .where(softDeleteWhere(users));

// 3. 查询所有数据（包括已软删除的行）
const allUsers = await db.select().from(users);

// 4. 仅查询已删除的数据
const deletedUsers = await db
  .select()
  .from(users)
  .where(onlyDeletedWhere(users));

// 5. 软删除（设置 deletedAt 为当前时间）
await db
  .update(users)
  .set(softDeleteSet(users))
  .where(eq(users.id, 1));

// 6. 恢复已软删除的数据（清空 deletedAt）
await db
  .update(users)
  .set(restoreSet(users))
  .where(eq(users.id, 1));

// 7. 组合条件查询
const activeUsersNamedAlice = await db
  .select()
  .from(users)
  .where(and(softDeleteWhere(users), eq(users.name, 'Alice')));
```

### MySQL

```ts
import { mysqlTable, int, varchar } from 'drizzle-orm/mysql-core';
import {
  mysqlSoftDelete,
  softDeleteWhere,
  softDeleteSet,
  restoreSet,
  onlyDeletedWhere,
} from 'drizzle-orm/soft-delete/mysql';

const users = mysqlSoftDelete(mysqlTable('users', {
  id: int().primaryKey().autoincrement(),
  name: varchar({ length: 255 }).notNull(),
}));

// 用法与 PostgreSQL 相同
const activeUsers = await db
  .select()
  .from(users)
  .where(softDeleteWhere(users));
```

### SQLite

```ts
import { sqliteTable, integer, text } from 'drizzle-orm/sqlite-core';
import {
  sqliteSoftDelete,
  softDeleteWhere,
  softDeleteSet,
  restoreSet,
  onlyDeletedWhere,
} from 'drizzle-orm/soft-delete/sqlite';

const users = sqliteSoftDelete(sqliteTable('users', {
  id: integer().primaryKey({ autoIncrement: true }),
  name: text().notNull(),
}));

// 用法与 PostgreSQL 相同
const activeUsers = await db
  .select()
  .from(users)
  .where(softDeleteWhere(users));
```

## 自定义配置

```ts
// 自定义字段名
const users = pgSoftDelete(
  pgTable('users', { id: serial().primaryKey(), name: text() }),
  { fieldName: 'deleted_at' }, // 使用 deleted_at 代替默认的 deletedAt
);

// PostgreSQL 特有：配置时区
const users = pgSoftDelete(
  pgTable('users', { id: serial().primaryKey(), name: text() }),
  {
    fieldName: 'deletedAt',
    withTimezone: true,   // 默认 true，使用 timestamp with time zone
    precision: 3,          // 时间精度
  },
);

// MySQL 特有：配置精度
const users = mysqlSoftDelete(
  mysqlTable('users', { id: int().primaryKey().autoincrement(), name: varchar({ length: 255 }) }),
  { fsp: 3 }, // 小数秒精度
);

// 使用整数类型存储（Unix 时间戳毫秒）
const users = pgSoftDelete(
  pgTable('users', { id: serial().primaryKey(), name: text() }),
  { type: 'integer' },
);
```

## 核心函数一览

| 函数 | 说明 |
|------|------|
| `pgSoftDelete(table, config?)` | PG: 包装表，自动添加 `deletedAt` 列 |
| `mysqlSoftDelete(table, config?)` | MySQL: 包装表，自动添加 `deletedAt` 列 |
| `sqliteSoftDelete(table, config?)` | SQLite: 包装表，自动添加 `deletedAt` 列 |
| `softDelete(table, config?)` | 通用: 给已有 `deletedAt` 列的表添加元数据 |
| `softDeleteWhere(table)` | 返回 `WHERE deletedAt IS NULL` 条件 |
| `onlyDeletedWhere(table)` | 返回 `WHERE deletedAt IS NOT NULL` 条件 |
| `softDeleteSet(table)` | 返回 `{ deletedAt: now() }` 用于 UPDATE SET |
| `restoreSet(table)` | 返回 `{ deletedAt: null }` 用于 UPDATE SET |
| `isSoftDeleteTable(table)` | 检查表是否启用了软删除 |

## 设计原则

- **不修改 ORM 核心**：纯工具层实现，不触碰查询构建器、dialect 等核心模块
- **不碰迁移系统**：`deletedAt` 字段的迁移需要用户自行处理
- **不用数据库触发器**：所有逻辑在应用层完成
- **不做批量操作特殊处理**：批量软删除通过标准 UPDATE 语句实现
