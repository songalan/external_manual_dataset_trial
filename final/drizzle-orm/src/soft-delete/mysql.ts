/**
 * MySQL soft-delete utilities.
 *
 * Provides a `mysqlSoftDelete` wrapper that automatically adds a `deletedAt` timestamp
 * column to a mysqlTable, plus query helpers for filtering, soft-deleting, and restoring.
 *
 * The returned table is branded as `SoftDeleteTable`, enabling type-safe usage
 * with all soft-delete query helpers.
 *
 * @example
 * ```ts
 * import { mysqlTable, int, varchar } from 'drizzle-orm/mysql-core';
 * import { mysqlSoftDelete } from 'drizzle-orm/soft-delete/mysql';
 *
 * const users = mysqlSoftDelete(mysqlTable('users', {
 *   id: int().primaryKey().autoincrement(),
 *   name: varchar({ length: 255 }).notNull(),
 * }));
 * // users is typed as MySqlTableWithColumns<...> & SoftDeleteTable<'deletedAt', 'timestamp'>
 * ```
 */
import { Table } from '~/table.ts';
import { sql } from '~/sql/sql.ts';
import { getMySqlColumnBuilders } from '~/mysql-core/columns/all.ts';
import type { MySqlDateTime } from '~/mysql-core/columns/datetime.ts';
import type { MySqlTableWithColumns } from '~/mysql-core/table.ts';
import {
	SoftDeleteMeta,
	type SoftDeleteConfig,
	type SoftDeleteMetaInfo,
	type SoftDeleteTable,
} from './soft-delete.ts';

export interface MysqlSoftDeleteConfig<
	TFieldName extends string = string,
	TType extends 'timestamp' | 'integer' = 'timestamp' | 'integer',
> extends SoftDeleteConfig<TFieldName, TType> {
	/** Fractional seconds precision for the timestamp column */
	fsp?: 0 | 1 | 2 | 3 | 4 | 5 | 6;
}

/**
 * Creates a mysqlTable with soft-delete support by automatically appending
 * a `deletedAt` timestamp column.
 *
 * The returned table is branded as `SoftDeleteTable`, which provides:
 * 1. **Column access** — `users.deletedAt` is properly typed as `MySqlDateTime`
 * 2. **Type-safe helpers** — `softDeleteWhere(users)` works, but passing a non-soft-delete table errors at compile time
 * 3. **Configured field name** — `softDeleteSet(users)` returns `{ deletedAt: SQL }` (or the custom field name)
 *
 * @example
 * ```ts
 * const users = mysqlSoftDelete(mysqlTable('users', {
 *   id: int().primaryKey().autoincrement(),
 *   name: varchar({ length: 255 }).notNull(),
 * }));
 *
 * // Custom field name
 * const posts = mysqlSoftDelete(mysqlTable('posts', { ... }), {
 *   fieldName: 'removed_at',
 *   fsp: 3,
 * });
 * ```
 */
export function mysqlSoftDelete<
	TTable extends MySqlTableWithColumns<any>,
	TFieldName extends string = 'deletedAt',
	TType extends 'timestamp' | 'integer' = 'timestamp',
>(
	table: TTable,
	config?: MysqlSoftDeleteConfig<TFieldName, TType>,
): TTable & { [K in TFieldName]: MySqlDateTime<any> } & SoftDeleteTable<TFieldName, TType> {
	const fieldName = (config?.fieldName ?? 'deletedAt') as TFieldName;
	const type = (config?.type ?? 'timestamp') as TType;

	(table as any)[SoftDeleteMeta] = { fieldName, type } satisfies SoftDeleteMetaInfo;

	const { datetime } = getMySqlColumnBuilders();

	const deletedAtBuilder = datetime(fieldName, {
		fsp: config?.fsp,
	}).$onUpdateFn(() => sql`now()`);

	deletedAtBuilder.setName(fieldName);
	const deletedAtColumn = deletedAtBuilder.build(table as any);

	Object.assign(table as any, { [fieldName]: deletedAtColumn });

	const columns = (table as any)[Table.Symbol.Columns];
	if (columns && typeof columns === 'object') {
		columns[fieldName] = deletedAtColumn;
	}

	return table as TTable & { [K in TFieldName]: MySqlDateTime<any> } & SoftDeleteTable<TFieldName, TType>;
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
	mysqlAddSoftDeleteColumn,
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
