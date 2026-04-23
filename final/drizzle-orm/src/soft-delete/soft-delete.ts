import { type Column } from '~/column.ts';
import { isTable, Table, getTableName, getTableUniqueName } from '~/table.ts';
import { and, inArray, isNotNull, isNull } from '~/sql/expressions/index.ts';
import { sql } from '~/sql/sql.ts';
import type { SQL } from '~/sql/sql.ts';

// ─── Configuration ───────────────────────────────────────────────

export interface SoftDeleteConfig<
	TFieldName extends string = string,
	TType extends 'timestamp' | 'integer' = 'timestamp' | 'integer',
> {
	/** Field name for the deleted-at column. Defaults to 'deletedAt' */
	fieldName?: TFieldName;
	/**
	 * Column type strategy:
	 * - 'timestamp' (default): uses timestamp/timestamp-like column, stores Date | null
	 * - 'integer': uses integer column (unix epoch ms), stores number | null
	 */
	type?: TType;
}

// ─── Internal: Soft-deleted table marker symbol ──────────────────

/** @internal */
export const SoftDeleteMeta = Symbol.for('drizzle:softDeleteMeta');

export interface SoftDeleteMetaInfo<
	TFieldName extends string = string,
	TType extends 'timestamp' | 'integer' = 'timestamp' | 'integer',
> {
	fieldName: TFieldName;
	type: TType;
}

// ─── Branded type for soft-delete tables ─────────────────────────

/**
 * The Symbol used to brand a table as a soft-delete table at the type level.
 * This enables type-safe constraints: only soft-delete tables can be
 * passed to softDeleteWhere, onlyDeletedWhere, softDeleteSet, and restoreSet.
 */
export const SoftDeleteBrand = Symbol.for('drizzle:softDeleteBrand');

/**
 * A branded table type that indicates this table supports soft-delete.
 *
 * The brand carries two type parameters:
 * - `TFieldName`: the name of the deletedAt column (defaults to 'deletedAt')
 * - `TType`: the column type strategy ('timestamp' | 'integer')
 *
 * When you use `pgSoftDelete()` / `mysqlSoftDelete()` / `sqliteSoftDelete()`,
 * the returned table is branded as `SoftDeleteTable`, which enables the
 * type-safe query helpers.
 *
 * @example
 * ```ts
 * const users = pgSoftDelete(pgTable('users', { ... }));
 * // users is typed as PgTableWithColumns<...> & SoftDeleteTable<'deletedAt', 'timestamp'>
 * // Now you can safely use:
 * softDeleteWhere(users);       // ✓ type-safe
 * softDeleteWhere(regularTable); // ✗ type error
 * ```
 */
export interface SoftDeleteTable<
	TFieldName extends string = 'deletedAt',
	TType extends 'timestamp' | 'integer' = 'timestamp',
> {
	/** @internal Branded property for type-level soft-delete detection */
	readonly [SoftDeleteBrand]: {
		readonly fieldName: TFieldName;
		readonly type: TType;
	};
	/** @internal Runtime metadata stored on the table instance */
	readonly [SoftDeleteMeta]: SoftDeleteMetaInfo<TFieldName, TType>;
}

/**
 * Structural type constraint for soft-delete query helpers.
 *
 * Instead of requiring the full `SoftDeleteTable` interface (which can be
 * hard for TypeScript to match in intersection types like
 * `PgTableWithColumns & SoftDeleteTable`), we only require the runtime
 * metadata Symbol. This provides:
 *
 * 1. **Compile-time safety**: plain `Table` objects don't have `[SoftDeleteMeta]`,
 *    so they'll be rejected by the type checker.
 * 2. **Intersection-friendly**: `PgTableWithColumns<any> & SoftDeleteTable` easily
 *    satisfies this constraint since it has the `[SoftDeleteMeta]` property.
 * 3. **Exact field name inference**: the `TFieldName` parameter allows
 *    `softDeleteSet` and `restoreSet` to return properly keyed records.
 *
 * @internal
 */
export interface SoftDeleteTableConstraint<
	TFieldName extends string = string,
	TType extends 'timestamp' | 'integer' = 'timestamp' | 'integer',
> {
	readonly [SoftDeleteMeta]: SoftDeleteMetaInfo<TFieldName, TType>;
}

// ─── Type guard & metadata access ────────────────────────────────

/**
 * Type guard that checks if a table has been wrapped with a soft-delete function.
 * Narrows the type to include the `SoftDeleteTable` brand.
 *
 * @example
 * ```ts
 * if (isSoftDeleteTable(users)) {
 *   // users is now typed as Table & SoftDeleteTable
 *   const meta = getSoftDeleteMeta(users); // SoftDeleteMetaInfo
 * }
 * ```
 */
export function isSoftDeleteTable<
	TFieldName extends string = string,
	TType extends 'timestamp' | 'integer' = 'timestamp' | 'integer',
>(
	table: Table,
): table is Table & SoftDeleteTable<TFieldName, TType> {
	return typeof table === 'object' && table !== null && SoftDeleteMeta in table;
}

/**
 * Retrieves the soft-delete metadata from a table.
 * Returns `undefined` if the table is not a soft-delete table.
 *
 * The metadata includes:
 * - `fieldName`: the name of the deletedAt column
 * - `type`: the column type strategy ('timestamp' or 'integer')
 */
export function getSoftDeleteMeta<
	TFieldName extends string = string,
	TType extends 'timestamp' | 'integer' = 'timestamp' | 'integer',
>(table: Table & SoftDeleteTableConstraint<TFieldName, TType>): SoftDeleteMetaInfo<TFieldName, TType>;
export function getSoftDeleteMeta(table: Table): SoftDeleteMetaInfo | undefined;
export function getSoftDeleteMeta(table: Table): SoftDeleteMetaInfo | undefined {
	if (isSoftDeleteTable(table)) {
		return (table as any)[SoftDeleteMeta] as SoftDeleteMetaInfo;
	}
	return undefined;
}

// ─── Column access helper ────────────────────────────────────────

/**
 * Extracts the deletedAt column from a soft-delete table in a type-safe way.
 *
 * @example
 * ```ts
 * const users = pgSoftDelete(pgTable('users', { ... }));
 * const deletedAtCol = getSoftDeletedAtColumn(users);
 * // deletedAtCol is typed as Column - can be used in eq(), isNull(), etc.
 * ```
 */
export function getSoftDeletedAtColumn<
	TFieldName extends string = string,
	TType extends 'timestamp' | 'integer' = 'timestamp' | 'integer',
>(table: SoftDeleteTableConstraint<TFieldName, TType>): Column {
	const meta = (table as any)[SoftDeleteMeta] as SoftDeleteMetaInfo<TFieldName, TType>;
	const column = (table as any)[meta.fieldName] as Column;
	if (!column) {
		throw new Error(
			`Soft delete column "${meta.fieldName}" not found on table "${getTableName(table as unknown as Table)}". `
				+ 'Make sure you used softDelete() to create the table.',
		);
	}
	return column;
}

// ─── Query filter helpers (internal) ─────────────────────────────

/**
 * Build a where clause that filters out soft-deleted rows (deletedAt IS NULL).
 * @internal
 */
export function filterNotDeleted<
	TFieldName extends string = string,
	TType extends 'timestamp' | 'integer' = 'timestamp' | 'integer',
>(table: SoftDeleteTableConstraint<TFieldName, TType>): SQL {
	const meta = (table as any)[SoftDeleteMeta] as SoftDeleteMetaInfo<TFieldName, TType>;
	const column = (table as any)[meta.fieldName] as Column;
	if (!column) {
		throw new Error(
			`Soft delete column "${meta.fieldName}" not found on table "${getTableName(table as unknown as Table)}". `
				+ 'Make sure you used softDelete() to create the table.',
		);
	}
	return isNull(column);
}

/**
 * Build a where clause that selects only soft-deleted rows (deletedAt IS NOT NULL).
 * @internal
 */
export function filterOnlyDeleted<
	TFieldName extends string = string,
	TType extends 'timestamp' | 'integer' = 'timestamp' | 'integer',
>(table: SoftDeleteTableConstraint<TFieldName, TType>): SQL {
	const meta = (table as any)[SoftDeleteMeta] as SoftDeleteMetaInfo<TFieldName, TType>;
	const column = (table as any)[meta.fieldName] as Column;
	if (!column) {
		throw new Error(
			`Soft delete column "${meta.fieldName}" not found on table "${getTableName(table as unknown as Table)}". `
				+ 'Make sure you used softDelete() to create the table.',
		);
	}
	return isNotNull(column);
}

// ─── Core softDelete wrapper ─────────────────────────────────────

/**
 * Wraps a Drizzle table with soft-delete capabilities.
 *
 * This returns the original table with soft-delete metadata attached via a Symbol.
 * Unlike dialect-specific wrappers (`pgSoftDelete`, `mysqlSoftDelete`, `sqliteSoftDelete`),
 * this function does **not** automatically add a `deletedAt` column — you must
 * define it yourself in the table schema.
 *
 * The returned table is branded as `SoftDeleteTable`, enabling type-safe usage
 * with `softDeleteWhere`, `onlyDeletedWhere`, `softDeleteSet`, and `restoreSet`.
 *
 * @example
 * ```ts
 * import { pgTable, serial, text, timestamp } from 'drizzle-orm/pg-core';
 * import { softDelete } from 'drizzle-orm/soft-delete';
 *
 * // You must define the deletedAt column yourself
 * const users = softDelete(pgTable('users', {
 *   id: serial().primaryKey(),
 *   name: text().notNull(),
 *   deletedAt: timestamp(),
 * }));
 *
 * // Now type-safe:
 * db.select().from(users).where(softDeleteWhere(users));
 * ```
 */
export function softDelete<
	TTable extends Table,
	TFieldName extends string = 'deletedAt',
	TType extends 'timestamp' | 'integer' = 'timestamp',
>(
	table: TTable,
	config?: SoftDeleteConfig<TFieldName, TType>,
): TTable & SoftDeleteTable<TFieldName, TType> {
	const fieldName = (config?.fieldName ?? 'deletedAt') as TFieldName;
	const type = (config?.type ?? 'timestamp') as TType;

	// Store runtime metadata on the table via Symbol
	(table as any)[SoftDeleteMeta] = { fieldName, type } satisfies SoftDeleteMetaInfo;

	return table as TTable & SoftDeleteTable<TFieldName, TType>;
}

// ─── Query helper functions ──────────────────────────────────────

/**
 * Returns a WHERE condition that excludes soft-deleted rows (`deletedAt IS NULL`).
 * Use in select queries to only see active (non-deleted) records.
 *
 * Only accepts tables that have been wrapped with a soft-delete function,
 * ensuring compile-time safety: plain `Table` objects will produce a type error
 * because they lack the `[SoftDeleteMeta]` Symbol property.
 *
 * @example
 * ```ts
 * db.select().from(users).where(and(eq(users.id, 1), softDeleteWhere(users)));
 * ```
 */
export function softDeleteWhere<
	TTable extends SoftDeleteTableConstraint,
>(table: TTable): SQL {
	return filterNotDeleted(table);
}

/**
 * Returns a WHERE condition that includes only soft-deleted rows (`deletedAt IS NOT NULL`).
 *
 * Only accepts tables that have been wrapped with a soft-delete function,
 * ensuring compile-time safety.
 *
 * @example
 * ```ts
 * db.select().from(users).where(onlyDeletedWhere(users));
 * ```
 */
export function onlyDeletedWhere<
	TTable extends SoftDeleteTableConstraint,
>(table: TTable): SQL {
	return filterOnlyDeleted(table);
}

/**
 * Returns the SET clause for a soft delete operation (sets deletedAt to current timestamp).
 *
 * The returned object is keyed by the configured field name (defaults to 'deletedAt'),
 * so it works correctly with custom field names.
 *
 * For `type: 'timestamp'` → `{ deletedAt: sql\`now()\` }`
 * For `type: 'integer'`   → `{ deletedAt: sql\`(cast(extract(epoch from now()) * 1000 as integer))\` }`
 *
 * Only accepts tables that have been wrapped with a soft-delete function,
 * ensuring compile-time safety.
 *
 * @example
 * ```ts
 * db.update(users).set(softDeleteSet(users)).where(eq(users.id, 1));
 * ```
 */
export function softDeleteSet<
	TTable extends SoftDeleteTableConstraint,
>(table: TTable): Record<TTable[typeof SoftDeleteMeta]['fieldName'], SQL>;
export function softDeleteSet(table: SoftDeleteTableConstraint): Record<string, SQL>;
export function softDeleteSet(table: SoftDeleteTableConstraint): Record<string, SQL> {
	const meta = (table as any)[SoftDeleteMeta] as SoftDeleteMetaInfo;

	if (meta.type === 'integer') {
		return {
			[meta.fieldName]: sql`(cast(extract(epoch from now()) * 1000 as integer))`,
		};
	}
	// timestamp
	return {
		[meta.fieldName]: sql`now()`,
	};
}

/**
 * Returns the SET clause for a restore operation (sets deletedAt to NULL).
 *
 * The returned object is keyed by the configured field name (defaults to 'deletedAt'),
 * so it works correctly with custom field names.
 *
 * Only accepts tables that have been wrapped with a soft-delete function,
 * ensuring compile-time safety.
 *
 * @example
 * ```ts
 * db.update(users).set(restoreSet(users)).where(eq(users.id, 1));
 * ```
 */
export function restoreSet<
	TTable extends SoftDeleteTableConstraint,
>(table: TTable): Record<TTable[typeof SoftDeleteMeta]['fieldName'], null>;
export function restoreSet(table: SoftDeleteTableConstraint): Record<string, null>;
export function restoreSet(table: SoftDeleteTableConstraint): Record<string, null> {
	const meta = (table as any)[SoftDeleteMeta] as SoftDeleteMetaInfo;
	return {
		[meta.fieldName]: null,
	};
}

// ─── Batch soft-delete helpers ──────────────────────────────────

/**
 * Returns a WHERE condition for batch soft-deleting rows:
 * combines `deletedAt IS NULL` (only non-deleted rows) with an `IN` clause
 * on the provided column and value list.
 *
 * This is used in UPDATE statements to soft-delete multiple rows at once.
 * The `idColumn` parameter must be the primary key (or unique identifier) column
 * of the table, and `ids` is the array of values to match.
 *
 * Only accepts tables that have been wrapped with a soft-delete function,
 * ensuring compile-time safety.
 *
 * @example
 * ```ts
 * // Soft-delete rows with id 1, 2, 3
 * db.update(users)
 *   .set(softDeleteSet(users))
 *   .where(batchSoftDeleteWhere(users, users.id, [1, 2, 3]));
 * ```
 *
 * @param table - A soft-delete branded table
 * @param idColumn - The column to use for the IN clause (typically the primary key)
 * @param ids - Array of values to match in the IN clause
 */
export function batchSoftDeleteWhere<
	TTable extends SoftDeleteTableConstraint,
	TValue,
>(table: TTable, idColumn: Column, ids: TValue[]): SQL {
	return and(
		filterNotDeleted(table),
		inArray(idColumn, ids as any),
	)!;
}

/**
 * Returns a WHERE condition for batch querying only soft-deleted rows:
 * combines `deletedAt IS NOT NULL` with an `IN` clause on the provided
 * column and value list.
 *
 * This is used in SELECT statements to query multiple soft-deleted rows at once.
 *
 * Only accepts tables that have been wrapped with a soft-delete function,
 * ensuring compile-time safety.
 *
 * @example
 * ```ts
 * // Query only deleted rows with id 1, 2, 3
 * db.select().from(users).where(batchOnlyDeletedWhere(users, users.id, [1, 2, 3]));
 * ```
 *
 * @param table - A soft-delete branded table
 * @param idColumn - The column to use for the IN clause (typically the primary key)
 * @param ids - Array of values to match in the IN clause
 */
export function batchOnlyDeletedWhere<
	TTable extends SoftDeleteTableConstraint,
	TValue,
>(table: TTable, idColumn: Column, ids: TValue[]): SQL {
	return and(
		filterOnlyDeleted(table),
		inArray(idColumn, ids as any),
	)!;
}

/**
 * Returns a WHERE condition for batch restoring soft-deleted rows:
 * combines `deletedAt IS NOT NULL` (only deleted rows can be restored)
 * with an `IN` clause on the provided column and value list.
 *
 * This is used in UPDATE statements to restore multiple soft-deleted rows at once.
 *
 * Only accepts tables that have been wrapped with a soft-delete function,
 * ensuring compile-time safety.
 *
 * @example
 * ```ts
 * // Restore soft-deleted rows with id 1, 2, 3
 * db.update(users)
 *   .set(restoreSet(users))
 *   .where(batchRestoreWhere(users, users.id, [1, 2, 3]));
 * ```
 *
 * @param table - A soft-delete branded table
 * @param idColumn - The column to use for the IN clause (typically the primary key)
 * @param ids - Array of values to match in the IN clause
 */
export function batchRestoreWhere<
	TTable extends SoftDeleteTableConstraint,
	TValue,
>(table: TTable, idColumn: Column, ids: TValue[]): SQL {
	return and(
		filterOnlyDeleted(table),
		inArray(idColumn, ids as any),
	)!;
}

// ─── Migration helpers ───────────────────────────────────────────
// These functions generate raw SQL DDL statements for adding a
// deletedAt column to an existing table. They are useful when you
// need to write a migration to add soft-delete support to an
// existing table without recreating it.

/**
 * Generates a PostgreSQL `ALTER TABLE ... ADD COLUMN` statement
 * to add a soft-delete timestamp column to an existing table.
 *
 * This is intended for use in migration files where you need to
 * add soft-delete support to a table that already exists in the
 * database. The generated SQL includes a `DEFAULT NULL` clause
 * so that existing rows are not marked as deleted.
 *
 * @example
 * ```ts
 * // In a migration file:
 * import { pgAddSoftDeleteColumn } from 'drizzle-orm/soft-delete';
 *
 * // Default: adds "deletedAt" timestamp with time zone column
 * await db.execute(sql.raw(pgAddSoftDeleteColumn('users')));
 *
 * // Custom field name
 * await db.execute(sql.raw(pgAddSoftDeleteColumn('users', { fieldName: 'deleted_at' })));
 *
 * // Custom field name and schema
 * await db.execute(sql.raw(pgAddSoftDeleteColumn('users', { schema: 'public', fieldName: 'deleted_at' })));
 * ```
 *
 * @param tableName - The name of the table to alter
 * @param config - Configuration for the column
 * @returns A SQL string that adds the soft-delete column
 */
export function pgAddSoftDeleteColumn(
	tableName: string,
	config?: {
		/** Schema name. If provided, the table name is qualified as "schema"."table" */
		schema?: string;
		/** Column name for the deleted-at field. Defaults to 'deletedAt' */
		fieldName?: string;
		/** Whether to use `with time zone`. Defaults to true */
		withTimezone?: boolean;
		/** Precision for the timestamp column */
		precision?: 0 | 1 | 2 | 3 | 4 | 5 | 6;
	},
): string {
	const fieldName = config?.fieldName ?? 'deletedAt';
	const withTimezone = config?.withTimezone ?? true;
	const precision = config?.precision;
	const tableNameWithSchema = config?.schema
		? `"${config.schema}"."${tableName}"`
		: `"${tableName}"`;

	const precisionStr = precision !== undefined ? `(${precision})` : '';
	const tzStr = withTimezone ? ' with time zone' : '';

	return `ALTER TABLE ${tableNameWithSchema} ADD COLUMN "${fieldName}" timestamp${precisionStr}${tzStr} DEFAULT NULL;`;
}

/**
 * Generates a MySQL `ALTER TABLE ... ADD COLUMN` statement
 * to add a soft-delete timestamp column to an existing table.
 *
 * @example
 * ```ts
 * // Default: adds "deletedAt" datetime column
 * await db.execute(sql.raw(mysqlAddSoftDeleteColumn('users')));
 *
 * // Custom field name and fractional seconds precision
 * await db.execute(sql.raw(mysqlAddSoftDeleteColumn('users', { fieldName: 'deleted_at', fsp: 3 })));
 * ```
 *
 * @param tableName - The name of the table to alter
 * @param config - Configuration for the column
 * @returns A SQL string that adds the soft-delete column
 */
export function mysqlAddSoftDeleteColumn(
	tableName: string,
	config?: {
		/** Column name for the deleted-at field. Defaults to 'deletedAt' */
		fieldName?: string;
		/** Fractional seconds precision for the datetime column */
		fsp?: 0 | 1 | 2 | 3 | 4 | 5 | 6;
	},
): string {
	const fieldName = config?.fieldName ?? 'deletedAt';
	const fsp = config?.fsp;
	const fspStr = fsp !== undefined ? `(${fsp})` : '';

	return `ALTER TABLE \`${tableName}\` ADD \`${fieldName}\` datetime${fspStr} DEFAULT NULL;`;
}

/**
 * Generates a SQLite `ALTER TABLE ... ADD COLUMN` statement
 * to add a soft-delete integer column to an existing table.
 *
 * SQLite uses integer columns for timestamps (Unix epoch in milliseconds).
 *
 * @example
 * ```ts
 * // Default: adds "deletedAt" integer column
 * await db.run(sql.raw(sqliteAddSoftDeleteColumn('users')));
 *
 * // Custom field name
 * await db.run(sql.raw(sqliteAddSoftDeleteColumn('users', { fieldName: 'deleted_at' })));
 * ```
 *
 * @param tableName - The name of the table to alter
 * @param config - Configuration for the column
 * @returns A SQL string that adds the soft-delete column
 */
export function sqliteAddSoftDeleteColumn(
	tableName: string,
	config?: {
		/** Column name for the deleted-at field. Defaults to 'deletedAt' */
		fieldName?: string;
	},
): string {
	const fieldName = config?.fieldName ?? 'deletedAt';

	return `ALTER TABLE \`${tableName}\` ADD \`${fieldName}\` integer DEFAULT NULL;`;
}

// ─── Cascade soft-delete helpers ─────────────────────────────────
// These functions help you cascade soft-delete operations across
// foreign key relationships. When you soft-delete a parent row,
// these helpers can find and soft-delete all related child rows.
//
// They use the inline foreign key metadata stored on each table
// (via `references()`) to determine the parent-child relationships.

/**
 * Represents a foreign key relationship between two tables.
 * Used by cascade soft-delete to understand which child tables
 * reference a given parent table.
 */
export interface CascadeRelation {
	/** The child table that has a foreign key pointing to the parent */
	childTable: Table;
	/** The FK columns on the child table that reference the parent */
	childColumns: Column[];
	/** The referenced columns on the parent table (typically the primary key) */
	parentColumns: Column[];
}

/**
 * A map from parent table unique name to the list of cascade relations
 * (child tables that reference it via foreign keys).
 *
 * Key format: `"schema.tableName"` (e.g., `"public.users"` or `"undefined.users"`).
 */
export type CascadeRelationMap = Map<string, CascadeRelation[]>;

/**
 * Builds a map of cascade relations from a Drizzle schema object.
 *
 * This function inspects the inline foreign keys (created via `.references()`)
 * on each table in the schema, and builds a mapping from each parent table
 * to the list of child tables that reference it.
 *
 * This map can then be used with `cascadeSoftDelete` to automatically
 * soft-delete all child rows when a parent row is soft-deleted.
 *
 * @example
 * ```ts
 * import { pgTable, serial, text, integer } from 'drizzle-orm/pg-core';
 * import { pgSoftDelete, buildCascadeRelationMap } from 'drizzle-orm/soft-delete/pg';
 *
 * const users = pgSoftDelete(pgTable('users', {
 *   id: serial().primaryKey(),
 *   name: text().notNull(),
 * }));
 *
 * const posts = pgSoftDelete(pgTable('posts', {
 *   id: serial().primaryKey(),
 *   title: text().notNull(),
 *   userId: integer().references(() => users.id),
 * }));
 *
 * const schema = { users, posts };
 * const cascadeMap = buildCascadeRelationMap(schema);
 * // cascadeMap.get('public.users') → [{ childTable: posts, childColumns: [posts.userId], parentColumns: [users.id] }]
 * ```
 *
 * @param schema - A Drizzle schema object (Record<string, Table>), typically the same
 *   object passed to `drizzle()` as `schema`.
 * @returns A Map where keys are parent table unique names and values are arrays
 *   of CascadeRelation objects describing child tables that reference the parent.
 */
export function buildCascadeRelationMap(schema: Record<string, unknown>): CascadeRelationMap {
	const relationMap: CascadeRelationMap = new Map();

	// Collect all tables from the schema
	const tables: Table[] = [];
	for (const [, value] of Object.entries(schema)) {
		if (isTable(value)) {
			tables.push(value);
		}
	}

	// For each table, inspect its foreign keys and register the reverse relation
	for (const table of tables) {
		const foreignKeys = getTableForeignKeys(table);

		for (const fk of foreignKeys) {
			const { columns: childColumns, foreignTable: parentTable, foreignColumns: parentColumns } = fk.reference();
			const parentUniqueName = getTableUniqueName(parentTable as Table);

			if (!relationMap.has(parentUniqueName)) {
				relationMap.set(parentUniqueName, []);
			}

			relationMap.get(parentUniqueName)!.push({
				childTable: table,
				childColumns: childColumns as Column[],
				parentColumns: parentColumns as Column[],
			});
		}
	}

	return relationMap;
}

/**
 * Generates the SQL WHERE conditions for cascading a soft-delete from
 * a parent row to its child rows.
 *
 * Given a parent table, the IDs of the rows being soft-deleted, and the
 * cascade relation map, this function returns an array of objects, each
 * containing:
 * - `table`: the child table to soft-delete
 * - `where`: the SQL WHERE condition to match child rows that reference
 *   the deleted parent rows AND are not already soft-deleted
 *
 * Only child tables that are soft-delete tables will be included in the
 * result (non-soft-delete tables are skipped since they don't support
 * soft-delete).
 *
 * @example
 * ```ts
 * const cascadeMap = buildCascadeRelationMap(schema);
 *
 * // Soft-delete user with id=1, and cascade to posts
 * const conditions = getCascadeSoftDeleteConditions(users, [1], cascadeMap);
 * for (const { table, where } of conditions) {
 *   await db.update(table).set(softDeleteSet(table)).where(where);
 * }
 * ```
 *
 * @param parentTable - The parent table from which the cascade originates
 * @param parentIds - Array of parent row IDs (primary key values) being soft-deleted
 * @param cascadeMap - The cascade relation map built by `buildCascadeRelationMap`
 * @returns An array of objects with `table` and `where` for each child table that
 *   needs to be cascade soft-deleted
 */
export function getCascadeSoftDeleteConditions(
	parentTable: Table,
	parentIds: unknown[],
	cascadeMap: CascadeRelationMap,
): Array<{ table: SoftDeleteTableConstraint; where: SQL }> {
	const parentUniqueName = getTableUniqueName(parentTable);
	const relations = cascadeMap.get(parentUniqueName) ?? [];

	const result: Array<{ table: SoftDeleteTableConstraint; where: SQL }> = [];

	for (const relation of relations) {
		const childTable = relation.childTable;

		// Skip child tables that don't support soft-delete
		if (!isSoftDeleteTable(childTable)) {
			continue;
		}

		// Build the WHERE condition:
		// childColumn IN (parentIds) AND childDeletedAt IS NULL
		const conditions: SQL[] = [];

		for (let i = 0; i < relation.childColumns.length; i++) {
			const childColumn = relation.childColumns[i]!;
			conditions.push(
				inArray(childColumn, parentIds as any),
			);
		}

		// Only soft-delete child rows that are not already deleted
		conditions.push(filterNotDeleted(childTable));

		result.push({
			table: childTable as unknown as SoftDeleteTableConstraint,
			where: and(...conditions)!,
		});
	}

	return result;
}

/**
 * Generates the SQL WHERE conditions for cascading a restore from
 * a parent row to its child rows.
 *
 * Similar to `getCascadeSoftDeleteConditions`, but for restore operations:
 * finds child rows that reference the restored parent rows AND are currently
 * soft-deleted.
 *
 * @example
 * ```ts
 * const cascadeMap = buildCascadeRelationMap(schema);
 *
 * // Restore user with id=1, and cascade to posts
 * const conditions = getCascadeRestoreConditions(users, [1], cascadeMap);
 * for (const { table, where } of conditions) {
 *   await db.update(table).set(restoreSet(table)).where(where);
 * }
 * ```
 *
 * @param parentTable - The parent table from which the cascade originates
 * @param parentIds - Array of parent row IDs (primary key values) being restored
 * @param cascadeMap - The cascade relation map built by `buildCascadeRelationMap`
 * @returns An array of objects with `table` and `where` for each child table that
 *   needs to be cascade restored
 */
export function getCascadeRestoreConditions(
	parentTable: Table,
	parentIds: unknown[],
	cascadeMap: CascadeRelationMap,
): Array<{ table: SoftDeleteTableConstraint; where: SQL }> {
	const parentUniqueName = getTableUniqueName(parentTable);
	const relations = cascadeMap.get(parentUniqueName) ?? [];

	const result: Array<{ table: SoftDeleteTableConstraint; where: SQL }> = [];

	for (const relation of relations) {
		const childTable = relation.childTable;

		// Skip child tables that don't support soft-delete
		if (!isSoftDeleteTable(childTable)) {
			continue;
		}

		// Build the WHERE condition:
		// childColumn IN (parentIds) AND childDeletedAt IS NOT NULL
		const conditions: SQL[] = [];

		for (let i = 0; i < relation.childColumns.length; i++) {
			const childColumn = relation.childColumns[i]!;
			conditions.push(
				inArray(childColumn, parentIds as any),
			);
		}

		// Only restore child rows that are currently deleted
		conditions.push(filterOnlyDeleted(childTable));

		result.push({
			table: childTable as unknown as SoftDeleteTableConstraint,
			where: and(...conditions)!,
		});
	}

	return result;
}

// ─── Internal: FK extraction helper ───────────────────────────────

/**
 * Extracts foreign keys from a table in a dialect-agnostic way.
 * Works with PG, MySQL, SQLite, Gel, and SingleStore tables.
 *
 * This function checks each dialect's InlineForeignKeys symbol to find
 * FK relationships defined via `.references()` on column builders.
 *
 * Note: Foreign keys defined via the `foreignKey()` builder in the
 * extra config (third argument to `pgTable()` etc.) are NOT included
 * here, as they require building against the table instance. For those,
 * use the dialect-specific `getTableConfig()` utility instead.
 *
 * @internal
 */
function getTableForeignKeys(table: Table): Array<{ reference: () => { columns: Column[]; foreignTable: Table; foreignColumns: Column[] } }> {
	const tableAny = table as any;

	// Try each dialect's InlineForeignKeys symbol
	const fkSymbols = [
		Symbol.for('drizzle:PgInlineForeignKeys'),
		Symbol.for('drizzle:MySqlInlineForeignKeys'),
		Symbol.for('drizzle:SQLiteInlineForeignKeys'),
		Symbol.for('drizzle:GelInlineForeignKeys'),
		Symbol.for('drizzle:SingleStoreInlineForeignKeys'),
	];

	for (const symbol of fkSymbols) {
		const fks = tableAny[symbol];
		if (fks && typeof fks === 'object') {
			return Object.values(fks) as any;
		}
	}

	return [];
}
