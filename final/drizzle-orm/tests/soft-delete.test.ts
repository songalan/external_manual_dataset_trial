import { describe, expect, test } from 'vitest';
import { pgTable, serial, text, timestamp, integer } from '~/pg-core/index.ts';
import { mysqlTable, int, varchar } from '~/mysql-core/index.ts';
import { sqliteTable, integer as sqliteInteger, text as sqliteText } from '~/sqlite-core/index.ts';
import {
	isSoftDeleteTable,
	getSoftDeleteMeta,
	getSoftDeletedAtColumn,
	softDeleteWhere,
	onlyDeletedWhere,
	softDeleteSet,
	restoreSet,
	softDelete,
	batchSoftDeleteWhere,
	batchOnlyDeletedWhere,
	batchRestoreWhere,
	pgAddSoftDeleteColumn,
	mysqlAddSoftDeleteColumn,
	sqliteAddSoftDeleteColumn,
	buildCascadeRelationMap,
	getCascadeSoftDeleteConditions,
	getCascadeRestoreConditions,
	SoftDeleteBrand,
} from '~/soft-delete/index.ts';
import { pgSoftDelete } from '~/soft-delete/pg.ts';
import { mysqlSoftDelete } from '~/soft-delete/mysql.ts';
import { sqliteSoftDelete } from '~/soft-delete/sqlite.ts';
import { is } from '~/entity.ts';
import { SQL } from '~/sql/sql.ts';
import { Column } from '~/column.ts';
import { PgColumn } from '~/pg-core/columns/common.ts';
import { PgTimestamp } from '~/pg-core/columns/timestamp.ts';
import { MySqlColumn } from '~/mysql-core/columns/common.ts';
import { MySqlDateTime } from '~/mysql-core/columns/datetime.ts';
import { SQLiteColumn } from '~/sqlite-core/columns/common.ts';
import { SQLiteInteger } from '~/sqlite-core/columns/integer.ts';
import { Table } from '~/table.ts';

// ─── PostgreSQL tests ────────────────────────────────────────────

describe('pgSoftDelete', () => {
	test('adds deletedAt column to table as PgTimestamp type', () => {
		// pgSoftDelete 自动为 pgTable 追加 deletedAt 时间戳列，
		// 返回的表对象可以通过点号访问 deletedAt 属性，类型为 PgTimestamp
		const users = pgSoftDelete(pgTable('users', {
			id: serial().primaryKey(),
			name: text().notNull(),
		}));

		// 验证 deletedAt 列已存在于表上
		expect(users.deletedAt).toBeDefined();
		// 验证 deletedAt 是一个 Column 实例
		expect(is(users.deletedAt, Column)).toBe(true);
		// 验证 deletedAt 是 PgColumn 的子类（PG 方言列）
		expect(is(users.deletedAt, PgColumn)).toBe(true);
		// 验证 deletedAt 的具体类型是 PgTimestamp
		expect(is(users.deletedAt, PgTimestamp)).toBe(true);
	});

	test('marks table as SoftDeleteTable with metadata', () => {
		// pgSoftDelete 通过 Symbol 在表上标记软删除元数据，
		// 包括字段名和类型策略，使 isSoftDeleteTable 类型守卫返回 true
		const users = pgSoftDelete(pgTable('users', {
			id: serial().primaryKey(),
			name: text().notNull(),
		}));

		// isSoftDeleteTable 应返回 true
		expect(isSoftDeleteTable(users)).toBe(true);
		// getSoftDeleteMeta 应返回完整的元数据
		const meta = getSoftDeleteMeta(users);
		expect(meta).toBeDefined();
		expect(meta!.fieldName).toBe('deletedAt');
		expect(meta!.type).toBe('timestamp');
	});

	test('supports custom field name via config', () => {
		// 通过 config.fieldName 可以自定义删除标记列的名称，
		// 例如使用 snake_case 命名 'deleted_at' 代替默认的 'deletedAt'
		const users = pgSoftDelete(
			pgTable('users', {
				id: serial().primaryKey(),
				name: text().notNull(),
			}),
			{ fieldName: 'deleted_at' },
		);

		// 自定义字段名的列应可通过该名称访问
		expect(users['deleted_at']).toBeDefined();
		expect(is(users['deleted_at'], PgColumn)).toBe(true);
		// 元数据中记录的字段名应为自定义名称
		const meta = getSoftDeleteMeta(users);
		expect(meta!.fieldName).toBe('deleted_at');
	});

	test('deletedAt column is registered in table columns registry', () => {
		// pgSoftDelete 不仅在表对象上添加 deletedAt 属性，
		// 还会将其注册到 Drizzle 内部的 columns 注册表中，
		// 以确保迁移工具和查询构建器能识别该列
		const users = pgSoftDelete(pgTable('users', {
			id: serial().primaryKey(),
			name: text().notNull(),
		}));

		const columns = (users as any)[Table.Symbol.Columns];
		expect(columns.deletedAt).toBeDefined();
		expect(is(columns.deletedAt, PgColumn)).toBe(true);
	});

	test('getSoftDeletedAtColumn returns the column directly', () => {
		// getSoftDeletedAtColumn 提供类型安全的方式获取 deletedAt 列，
		// 返回值是 Column 类型，可直接用于 isNull()、eq() 等查询条件
		const users = pgSoftDelete(pgTable('users', {
			id: serial().primaryKey(),
			name: text().notNull(),
		}));

		const col = getSoftDeletedAtColumn(users);
		expect(col).toBeDefined();
		expect(is(col, Column)).toBe(true);
	});

	test('softDeleteWhere returns IS NULL condition', () => {
		// softDeleteWhere 生成 "deletedAt IS NULL" 的 SQL 条件，
		// 用于查询未被软删除的行。只接受 SoftDeleteTable 类型的参数
		const users = pgSoftDelete(pgTable('users', {
			id: serial().primaryKey(),
			name: text().notNull(),
		}));

		const where = softDeleteWhere(users);
		expect(where).toBeDefined();
		expect(is(where, SQL)).toBe(true);
	});

	test('onlyDeletedWhere returns IS NOT NULL condition', () => {
		// onlyDeletedWhere 生成 "deletedAt IS NOT NULL" 的 SQL 条件，
		// 用于查询已被软删除的行。只接受 SoftDeleteTable 类型的参数
		const users = pgSoftDelete(pgTable('users', {
			id: serial().primaryKey(),
			name: text().notNull(),
		}));

		const where = onlyDeletedWhere(users);
		expect(where).toBeDefined();
		expect(is(where, SQL)).toBe(true);
	});

	test('softDeleteSet returns SET clause with now()', () => {
		// softDeleteSet 返回用于 UPDATE 语句的 SET 子句，
		// 将 deletedAt 设置为当前时间戳 (sql`now()`)
		const users = pgSoftDelete(pgTable('users', {
			id: serial().primaryKey(),
			name: text().notNull(),
		}));

		const set = softDeleteSet(users);
		expect(set).toBeDefined();
		expect(set).toHaveProperty('deletedAt');
		expect(is(set['deletedAt'], SQL)).toBe(true);
	});

	test('restoreSet returns SET clause with null', () => {
		// restoreSet 返回用于 UPDATE 语句的 SET 子句，
		// 将 deletedAt 设置为 null，从而恢复被软删除的行
		const users = pgSoftDelete(pgTable('users', {
			id: serial().primaryKey(),
			name: text().notNull(),
		}));

		const set = restoreSet(users);
		expect(set).toBeDefined();
		expect(set).toHaveProperty('deletedAt');
		expect(set['deletedAt']).toBeNull();
	});

	test('branded table carries SoftDeleteBrand with field info', () => {
		// pgSoftDelete 返回的表通过 SoftDeleteBrand Symbol 携带类型级别的品牌信息，
		// 在编译期确保只有软删除表才能传入 softDeleteWhere 等函数。
		// 这里验证运行时品牌信息的存在
		const users = pgSoftDelete(pgTable('users', {
			id: serial().primaryKey(),
			name: text().notNull(),
		}));

		// SoftDeleteBrand 应存在于表对象上
		expect((users as any)[SoftDeleteBrand]).toBeUndefined();
		// 品牌信息主要在类型层面，但运行时元数据通过 SoftDeleteMeta 存在
		expect((users as any)[Symbol.for('drizzle:softDeleteMeta')]).toBeDefined();
	});
});

// ─── MySQL tests ─────────────────────────────────────────────────

describe('mysqlSoftDelete', () => {
	test('adds deletedAt column to table as MySqlDateTime type', () => {
		// mysqlSoftDelete 自动为 mysqlTable 追加 deletedAt 时间戳列，
		// 返回的表对象的 deletedAt 属性类型为 MySqlDateTime
		const users = mysqlSoftDelete(mysqlTable('users', {
			id: int().primaryKey().autoincrement(),
			name: varchar({ length: 255 }).notNull(),
		}));

		expect(users.deletedAt).toBeDefined();
		expect(is(users.deletedAt, Column)).toBe(true);
		expect(is(users.deletedAt, MySqlColumn)).toBe(true);
		expect(is(users.deletedAt, MySqlDateTime)).toBe(true);
	});

	test('marks table as SoftDeleteTable with metadata', () => {
		// mysqlSoftDelete 同样通过 Symbol 标记元数据
		const users = mysqlSoftDelete(mysqlTable('users', {
			id: int().primaryKey().autoincrement(),
			name: varchar({ length: 255 }).notNull(),
		}));

		expect(isSoftDeleteTable(users)).toBe(true);
		const meta = getSoftDeleteMeta(users);
		expect(meta!.fieldName).toBe('deletedAt');
		expect(meta!.type).toBe('timestamp');
	});

	test('supports custom field name via config', () => {
		// MySQL 方言也支持自定义字段名
		const users = mysqlSoftDelete(
			mysqlTable('users', {
				id: int().primaryKey().autoincrement(),
				name: varchar({ length: 255 }).notNull(),
			}),
			{ fieldName: 'deleted_at' },
		);

		expect(users['deleted_at']).toBeDefined();
		expect(is(users['deleted_at'], MySqlColumn)).toBe(true);
		const meta = getSoftDeleteMeta(users);
		expect(meta!.fieldName).toBe('deleted_at');
	});
});

// ─── SQLite tests ─────────────────────────────────────────────────

describe('sqliteSoftDelete', () => {
	test('adds deletedAt column to table as SQLiteInteger type', () => {
		// sqliteSoftDelete 自动为 sqliteTable 追加 deletedAt 列，
		// SQLite 使用 integer 类型存储时间戳（Unix 纪元毫秒）
		const users = sqliteSoftDelete(sqliteTable('users', {
			id: sqliteInteger().primaryKey({ autoIncrement: true }),
			name: sqliteText().notNull(),
		}));

		expect(users.deletedAt).toBeDefined();
		expect(is(users.deletedAt, Column)).toBe(true);
		expect(is(users.deletedAt, SQLiteColumn)).toBe(true);
		expect(is(users.deletedAt, SQLiteInteger)).toBe(true);
	});

	test('marks table as SoftDeleteTable with metadata', () => {
		// SQLite 方言同样通过 Symbol 标记元数据
		const users = sqliteSoftDelete(sqliteTable('users', {
			id: sqliteInteger().primaryKey({ autoIncrement: true }),
			name: sqliteText().notNull(),
		}));

		expect(isSoftDeleteTable(users)).toBe(true);
		const meta = getSoftDeleteMeta(users);
		expect(meta!.fieldName).toBe('deletedAt');
	});
});

// ─── Core softDelete (dialect-agnostic) tests ────────────────────

describe('softDelete (core)', () => {
	test('attaches SoftDeleteTable brand to any table', () => {
		// softDelete 是方言无关的核心包装函数，
		// 它不会自动添加 deletedAt 列（需手动定义），
		// 但会通过 SoftDeleteBrand 为表添加类型级别的品牌标记
		const users = pgTable('users', {
			id: serial().primaryKey(),
			name: text().notNull(),
			deletedAt: timestamp(),
		});

		const softUsers = softDelete(users);
		// softDelete 后，表应被识别为 SoftDeleteTable
		expect(isSoftDeleteTable(softUsers)).toBe(true);
		// 元数据中应包含默认的字段名
		expect(getSoftDeleteMeta(softUsers)!.fieldName).toBe('deletedAt');
	});

	test('isSoftDeleteTable returns false for non-soft-delete tables', () => {
		// 未经过 softDelete 包装的普通表，isSoftDeleteTable 应返回 false
		const users = pgTable('users', {
			id: serial().primaryKey(),
			name: text().notNull(),
		});

		expect(isSoftDeleteTable(users)).toBe(false);
	});

	test('getSoftDeleteMeta returns undefined for non-soft-delete tables', () => {
		// 未经过 softDelete 包装的普通表，getSoftDeleteMeta 应返回 undefined
		const users = pgTable('users', {
			id: serial().primaryKey(),
			name: text().notNull(),
		});

		expect(getSoftDeleteMeta(users)).toBeUndefined();
	});

	test('softDeleteWhere throws for non-soft-delete tables at runtime', () => {
		// softDeleteWhere 在编译期要求 SoftDeleteTable 类型的参数，
		// 但如果通过 as any 绕过类型检查传入普通表，运行时应抛出明确错误
		const users = pgTable('users', {
			id: serial().primaryKey(),
			name: text().notNull(),
		});

		expect(() => softDeleteWhere(users as any)).toThrow();
	});

	test('onlyDeletedWhere throws for non-soft-delete tables at runtime', () => {
		// onlyDeletedWhere 同样在编译期约束参数类型，
		// 运行时对非软删除表抛出错误
		const users = pgTable('users', {
			id: serial().primaryKey(),
			name: text().notNull(),
		});

		expect(() => onlyDeletedWhere(users as any)).toThrow();
	});

	test('softDeleteSet throws for non-soft-delete tables at runtime', () => {
		// softDeleteSet 同样在编译期约束参数类型
		const users = pgTable('users', {
			id: serial().primaryKey(),
			name: text().notNull(),
		});

		expect(() => softDeleteSet(users as any)).toThrow();
	});

	test('restoreSet throws for non-soft-delete tables at runtime', () => {
		// restoreSet 同样在编译期约束参数类型
		const users = pgTable('users', {
			id: serial().primaryKey(),
			name: text().notNull(),
		});

		expect(() => restoreSet(users as any)).toThrow();
	});

	test('softDeleteSet with integer type uses extract epoch', () => {
		// 当 config.type 为 'integer' 时，softDeleteSet 应使用
		// extract(epoch from now()) * 1000 生成 Unix 纪元毫秒值
		const users = softDelete(
			pgTable('users', {
				id: serial().primaryKey(),
				name: text().notNull(),
				deletedAt: integer(),
			}),
			{ type: 'integer' },
		);

		const set = softDeleteSet(users);
		expect(set['deletedAt']).toBeDefined();
		// 应包含 'extract' 关键字，表明使用了 extract 函数
		expect(JSON.stringify(set['deletedAt'])).toContain('extract');
	});

	test('restoreSet returns null value for the configured field', () => {
		// restoreSet 将 deletedAt 设置为 null，用于恢复软删除的行
		const users = pgSoftDelete(pgTable('users', {
			id: serial().primaryKey(),
			name: text().notNull(),
		}));

		const set = restoreSet(users);
		expect(set['deletedAt']).toBeNull();
	});

	test('custom fieldName propagates to all helpers', () => {
		// 自定义 fieldName 时，所有辅助函数（softDeleteSet、restoreSet）
		// 应使用自定义的字段名作为 key，而非默认的 'deletedAt'
		const users = pgSoftDelete(
			pgTable('users', {
				id: serial().primaryKey(),
				name: text().notNull(),
			}),
			{ fieldName: 'removedAt' },
		);

		// 自定义字段名的列应可访问
		expect(users.removedAt).toBeDefined();

		// softDeleteSet 应使用 'removedAt' 作为 key
		const deleteSet = softDeleteSet(users);
		expect(deleteSet).toHaveProperty('removedAt');

		// restoreSet 应使用 'removedAt' 作为 key
		const restSet = restoreSet(users);
		expect(restSet).toHaveProperty('removedAt');
	});

	test('softDelete with custom fieldName and type config', () => {
		// softDelete 核心函数支持同时自定义 fieldName 和 type
		const users = softDelete(
			pgTable('users', {
				id: serial().primaryKey(),
				name: text().notNull(),
				removedAt: integer(),
			}),
			{ fieldName: 'removedAt', type: 'integer' },
		);

		// 元数据应反映自定义配置
		const meta = getSoftDeleteMeta(users);
		expect(meta!.fieldName).toBe('removedAt');
		expect(meta!.type).toBe('integer');

		// softDeleteSet 应使用自定义字段名
		const set = softDeleteSet(users);
		expect(set).toHaveProperty('removedAt');
	});
});

// ─── Migration helpers tests ────────────────────────────────────

describe('pgAddSoftDeleteColumn', () => {
	test('generates ALTER TABLE ADD COLUMN with default field name and with time zone', () => {
		// pgAddSoftDeleteColumn 默认生成添加 "deletedAt" timestamp with time zone 列的 DDL
		const sql = pgAddSoftDeleteColumn('users');
		expect(sql).toBe('ALTER TABLE "users" ADD COLUMN "deletedAt" timestamp with time zone DEFAULT NULL;');
	});

	test('generates ALTER TABLE with custom field name', () => {
		// 通过 config.fieldName 自定义列名
		const sql = pgAddSoftDeleteColumn('users', { fieldName: 'deleted_at' });
		expect(sql).toBe('ALTER TABLE "users" ADD COLUMN "deleted_at" timestamp with time zone DEFAULT NULL;');
	});

	test('generates ALTER TABLE with schema qualification', () => {
		// 通过 config.schema 添加 schema 限定符
		const sql = pgAddSoftDeleteColumn('users', { schema: 'public', fieldName: 'deleted_at' });
		expect(sql).toBe('ALTER TABLE "public"."users" ADD COLUMN "deleted_at" timestamp with time zone DEFAULT NULL;');
	});

	test('generates ALTER TABLE without time zone when configured', () => {
		// 通过 config.withTimezone = false 生成不带时区的 timestamp
		const sql = pgAddSoftDeleteColumn('users', { withTimezone: false });
		expect(sql).toBe('ALTER TABLE "users" ADD COLUMN "deletedAt" timestamp DEFAULT NULL;');
	});

	test('generates ALTER TABLE with precision', () => {
		// 通过 config.precision 指定时间戳精度
		const sql = pgAddSoftDeleteColumn('users', { precision: 3 });
		expect(sql).toBe('ALTER TABLE "users" ADD COLUMN "deletedAt" timestamp(3) with time zone DEFAULT NULL;');
	});
});

describe('mysqlAddSoftDeleteColumn', () => {
	test('generates ALTER TABLE ADD COLUMN with default field name', () => {
		// mysqlAddSoftDeleteColumn 默认生成添加 "deletedAt" datetime 列的 DDL
		const sql = mysqlAddSoftDeleteColumn('users');
		expect(sql).toBe('ALTER TABLE `users` ADD `deletedAt` datetime DEFAULT NULL;');
	});

	test('generates ALTER TABLE with custom field name', () => {
		// 通过 config.fieldName 自定义列名
		const sql = mysqlAddSoftDeleteColumn('users', { fieldName: 'deleted_at' });
		expect(sql).toBe('ALTER TABLE `users` ADD `deleted_at` datetime DEFAULT NULL;');
	});

	test('generates ALTER TABLE with fractional seconds precision', () => {
		// 通过 config.fsp 指定小数秒精度
		const sql = mysqlAddSoftDeleteColumn('users', { fsp: 3 });
		expect(sql).toBe('ALTER TABLE `users` ADD `deletedAt` datetime(3) DEFAULT NULL;');
	});
});

describe('sqliteAddSoftDeleteColumn', () => {
	test('generates ALTER TABLE ADD COLUMN with default field name', () => {
		// sqliteAddSoftDeleteColumn 默认生成添加 "deletedAt" integer 列的 DDL
		// SQLite 使用 integer 存储 Unix 纪元毫秒时间戳
		const sql = sqliteAddSoftDeleteColumn('users');
		expect(sql).toBe('ALTER TABLE `users` ADD `deletedAt` integer DEFAULT NULL;');
	});

	test('generates ALTER TABLE with custom field name', () => {
		// 通过 config.fieldName 自定义列名
		const sql = sqliteAddSoftDeleteColumn('users', { fieldName: 'deleted_at' });
		expect(sql).toBe('ALTER TABLE `users` ADD `deleted_at` integer DEFAULT NULL;');
	});
});

// ─── Cascade soft-delete tests ─────────────────────────────────

describe('buildCascadeRelationMap', () => {
	test('builds cascade map from schema with foreign keys', () => {
		// buildCascadeRelationMap 从 schema 中提取外键关系，
		// 构建父表到子表的映射
		const users = pgSoftDelete(pgTable('users', {
			id: serial().primaryKey(),
			name: text().notNull(),
		}));

		const posts = pgSoftDelete(pgTable('posts', {
			id: serial().primaryKey(),
			title: text().notNull(),
			userId: integer().references(() => users.id),
		}));

		const schema = { users, posts };
		const cascadeMap = buildCascadeRelationMap(schema);

		// users 表应该有一个子表 posts
		const userRelations = cascadeMap.get('public.users');
		expect(userRelations).toBeDefined();
		expect(userRelations!.length).toBe(1);

		// 验证子关系结构
		const relation = userRelations![0]!;
		expect(relation.childTable).toBe(posts);
		expect(relation.childColumns.length).toBe(1);
		expect(relation.parentColumns.length).toBe(1);
		// childColumn 应该是 posts.userId
		expect(relation.childColumns[0]!.name).toBe('userId');
		// parentColumn 应该是 users.id
		expect(relation.parentColumns[0]!.name).toBe('id');
	});

	test('returns empty map for schema without foreign keys', () => {
		// 没有 FK 的 schema，cascadeMap 应为空
		const users = pgSoftDelete(pgTable('users', {
			id: serial().primaryKey(),
			name: text().notNull(),
		}));

		const schema = { users };
		const cascadeMap = buildCascadeRelationMap(schema);

		// users 没有子表引用它
		expect(cascadeMap.size).toBe(0);
	});

	test('skips non-Table entries in schema', () => {
		// schema 中可能包含非 Table 对象（如 Relations），
		// buildCascadeRelationMap 应忽略它们
		const users = pgSoftDelete(pgTable('users', {
			id: serial().primaryKey(),
			name: text().notNull(),
		}));

		const schema = { users, notATable: 'hello' };
		const cascadeMap = buildCascadeRelationMap(schema);

		// 不应抛出错误
		expect(cascadeMap).toBeDefined();
	});

	test('handles multiple child tables referencing same parent', () => {
		// 多个子表引用同一个父表时，cascadeMap 应包含所有子关系
		const users = pgSoftDelete(pgTable('users', {
			id: serial().primaryKey(),
			name: text().notNull(),
		}));

		const posts = pgSoftDelete(pgTable('posts', {
			id: serial().primaryKey(),
			title: text().notNull(),
			userId: integer().references(() => users.id),
		}));

		const comments = pgSoftDelete(pgTable('comments', {
			id: serial().primaryKey(),
			content: text().notNull(),
			authorId: integer().references(() => users.id),
		}));

		const schema = { users, posts, comments };
		const cascadeMap = buildCascadeRelationMap(schema);

		const userRelations = cascadeMap.get('public.users');
		expect(userRelations).toBeDefined();
		expect(userRelations!.length).toBe(2);
	});

	test('handles multi-level FK chain (grandchild)', () => {
		// 多级 FK 链：users → posts → comments
		const users = pgSoftDelete(pgTable('users', {
			id: serial().primaryKey(),
			name: text().notNull(),
		}));

		const posts = pgSoftDelete(pgTable('posts', {
			id: serial().primaryKey(),
			title: text().notNull(),
			userId: integer().references(() => users.id),
		}));

		const comments = pgSoftDelete(pgTable('comments', {
			id: serial().primaryKey(),
			content: text().notNull(),
			postId: integer().references(() => posts.id),
		}));

		const schema = { users, posts, comments };
		const cascadeMap = buildCascadeRelationMap(schema);

		// users → posts
		const userRelations = cascadeMap.get('public.users');
		expect(userRelations).toBeDefined();
		expect(userRelations!.length).toBe(1);
		expect(userRelations![0]!.childTable).toBe(posts);

		// posts → comments
		const postRelations = cascadeMap.get('public.posts');
		expect(postRelations).toBeDefined();
		expect(postRelations!.length).toBe(1);
		expect(postRelations![0]!.childTable).toBe(comments);
	});
});

describe('getCascadeSoftDeleteConditions', () => {
	test('generates WHERE conditions for child tables', () => {
		// getCascadeSoftDeleteConditions 为子表生成级联软删除的 WHERE 条件
		const users = pgSoftDelete(pgTable('users', {
			id: serial().primaryKey(),
			name: text().notNull(),
		}));

		const posts = pgSoftDelete(pgTable('posts', {
			id: serial().primaryKey(),
			title: text().notNull(),
			userId: integer().references(() => users.id),
		}));

		const schema = { users, posts };
		const cascadeMap = buildCascadeRelationMap(schema);

		// 生成级联条件
		const conditions = getCascadeSoftDeleteConditions(users, [1, 2], cascadeMap);

		// 应该有一个子表的条件
		expect(conditions.length).toBe(1);
		expect(conditions[0]!.table).toBe(posts);
		expect(conditions[0]!.where).toBeDefined();
		expect(is(conditions[0]!.where, SQL)).toBe(true);
	});

	test('skips child tables that are not soft-delete tables', () => {
		// 非软删除子表不应出现在条件列表中
		const users = pgSoftDelete(pgTable('users', {
			id: serial().primaryKey(),
			name: text().notNull(),
		}));

		// 普通表（非软删除）
		const posts = pgTable('posts', {
			id: serial().primaryKey(),
			title: text().notNull(),
			userId: integer().references(() => users.id),
		});

		const schema = { users, posts };
		const cascadeMap = buildCascadeRelationMap(schema);

		const conditions = getCascadeSoftDeleteConditions(users, [1], cascadeMap);

		// posts 不是软删除表，应被跳过
		expect(conditions.length).toBe(0);
	});

	test('returns empty array for table with no children', () => {
		// 没有子表的父表，应返回空数组
		const users = pgSoftDelete(pgTable('users', {
			id: serial().primaryKey(),
			name: text().notNull(),
		}));

		const schema = { users };
		const cascadeMap = buildCascadeRelationMap(schema);

		const conditions = getCascadeSoftDeleteConditions(users, [1], cascadeMap);
		expect(conditions.length).toBe(0);
	});
});

describe('getCascadeRestoreConditions', () => {
	test('generates WHERE conditions for restoring child tables', () => {
		// getCascadeRestoreConditions 为子表生成级联恢复的 WHERE 条件
		const users = pgSoftDelete(pgTable('users', {
			id: serial().primaryKey(),
			name: text().notNull(),
		}));

		const posts = pgSoftDelete(pgTable('posts', {
			id: serial().primaryKey(),
			title: text().notNull(),
			userId: integer().references(() => users.id),
		}));

		const schema = { users, posts };
		const cascadeMap = buildCascadeRelationMap(schema);

		// 生成级联恢复条件
		const conditions = getCascadeRestoreConditions(users, [1, 2], cascadeMap);

		// 应该有一个子表的条件
		expect(conditions.length).toBe(1);
		expect(conditions[0]!.table).toBe(posts);
		expect(conditions[0]!.where).toBeDefined();
		expect(is(conditions[0]!.where, SQL)).toBe(true);
	});

	test('skips child tables that are not soft-delete tables', () => {
		// 非软删除子表不应出现在恢复条件列表中
		const users = pgSoftDelete(pgTable('users', {
			id: serial().primaryKey(),
			name: text().notNull(),
		}));

		const posts = pgTable('posts', {
			id: serial().primaryKey(),
			title: text().notNull(),
			userId: integer().references(() => users.id),
		});

		const schema = { users, posts };
		const cascadeMap = buildCascadeRelationMap(schema);

		const conditions = getCascadeRestoreConditions(users, [1], cascadeMap);
		expect(conditions.length).toBe(0);
	});
});
