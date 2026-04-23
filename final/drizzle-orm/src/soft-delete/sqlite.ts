/**
 * SQLite soft-delete utilities.
 *
 * Provides a `sqliteSoftDelete` wrapper that automatically adds a `deletedAt` column
 * to an sqliteTable, plus query helpers for filtering, soft-deleting, and restoring.
 *
 * SQLite uses integer columns for timestamps (unix epoch in milliseconds).
 *
 * The returned table is branded as `SoftDeleteTable`, enabling type-safe usage
 * with all soft-delete query helpers.
 *
 * @example
 * ```ts
 * import { sqliteTable, integer, text } from 'drizzle-orm/sqlite-core';
 * import { sqliteSoftDelete } from 'drizzle-orm/soft-delete/sqlite';
 *
 * const users = sqliteSoftDelete(sqliteTable('users', {
 *   id: integer().primaryKey({ autoIncrement: true }),
 *   name: text().notNull(),
 * }));
 * // users is typed as SQLiteTableWithColumns<...> & SoftDeleteTable<'deletedAt', 'timestamp'>
 * ```
 */
import { Table } from '~/table.ts';
import { sql } from '~/sql/sql.ts';
import { getSQLiteColumnBuilders } from '~/sqlite-core/columns/all.ts';
import type { SQLiteInteger } from '~/sqlite-core/columns/integer.ts';
import type { SQLiteTableWithColumns } from '~/sqlite-core/table.ts';
import {
	SoftDeleteMeta,
	type SoftDeleteConfig,
	type SoftDeleteMetaInfo,
	type SoftDeleteTable,
} from './soft-delete.ts';

export interface SqliteSoftDeleteConfig<
	TFieldName extends string = string,
	TType extends 'timestamp' | 'integer' = 'timestamp' | 'integer',
> extends SoftDeleteConfig<TFieldName, TType> {
	/**
	 * For SQLite, 'integer' is the default and recommended type since SQLite
	 * doesn't have a native timestamp type. 'timestamp' mode uses integer
	 * with mode='timestamp_ms' which stores unix epoch in ms.
	 */
	type?: TType;
}

/**
 * Creates an sqliteTable with soft-delete support by automatically appending
 * a `deletedAt` column.
 *
 * In SQLite, timestamps are stored as integers (unix epoch milliseconds).
 *
 * The returned table is branded as `SoftDeleteTable`, which provides:
 * 1. **Column access** — `users.deletedAt` is properly typed as `SQLiteInteger`
 * 2. **Type-safe helpers** — `softDeleteWhere(users)` works, but passing a non-soft-delete table errors at compile time
 * 3. **Configured field name** — `softDeleteSet(users)` returns `{ deletedAt: SQL }` (or the custom field name)
 *
 * @example
 * ```ts
 * const users = sqliteSoftDelete(sqliteTable('users', {
 *   id: integer().primaryKey({ autoIncrement: true }),
 *   name: text().notNull(),
 * }));
 *
 * // Custom field name
 * const posts = sqliteSoftDelete(sqliteTable('posts', { ... }), {
 *   fieldName: 'removed_at',
 * });
 * ```
 */
export function sqliteSoftDelete<
	TTable extends SQLiteTableWithColumns<any>,
	TFieldName extends string = 'deletedAt',
	TType extends 'timestamp' | 'integer' = 'timestamp',
>(
	table: TTable,
	config?: SqliteSoftDeleteConfig<TFieldName, TType>,
): TTable & { [K in TFieldName]: SQLiteInteger<any> } & SoftDeleteTable<TFieldName, TType> {
	const fieldName = (config?.fieldName ?? 'deletedAt') as TFieldName;
	const type = (config?.type ?? 'timestamp') as TType;

	(table as any)[SoftDeleteMeta] = { fieldName, type } satisfies SoftDeleteMetaInfo;

	const { integer } = getSQLiteColumnBuilders();

	// SQLite uses integer for timestamps (unix epoch ms)
	const deletedAtBuilder = integer(fieldName).$onUpdateFn(
		() => sql`(cast((julianday('now') - 2440587.5)*86400000 as integer))`,
	);

	deletedAtBuilder.setName(fieldName);
	const deletedAtColumn = deletedAtBuilder.build(table as any);

	Object.assign(table as any, { [fieldName]: deletedAtColumn });

	const columns = (table as any)[Table.Symbol.Columns];
	if (columns && typeof columns === 'object') {
		columns[fieldName] = deletedAtColumn;
	}

	return table as TTable & { [K in TFieldName]: SQLiteInteger<any> } & SoftDeleteTable<TFieldName, TType>;
}

// Re-export common helpers
export {
	softDeleteWhere,
	onlyDeletedWhere,
	softDeleteSet,
	restoreSet,
	batchSoftDeleteWhere,
	batchOnlyDeletedWhere,
	batchRestoreWhere,
	sqliteAddSoftDeleteColumn,
	buildCascadeRelationMap,
	getCascadeSoftDeleteConditions,
	getCascadeRestoreConditions,
	isSoftDeleteTable,
	getSoftDeleteMeta,
	getSoftDeletedAtColumn,
	SoftDeleteBrand,
	type SoftDeleteTable,
	type SoftDeleteTableConstraint,
	type CascadeRelation,
	type CascadeRelationMap,
} from './soft-delete.ts';
