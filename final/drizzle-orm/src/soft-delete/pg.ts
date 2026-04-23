/**
 * PostgreSQL soft-delete utilities.
 *
 * Provides a `pgSoftDelete` wrapper that automatically adds a `deletedAt` timestamp
 * column to a pgTable, plus query helpers for filtering, soft-deleting, and restoring.
 *
 * The returned table is branded as `SoftDeleteTable`, enabling type-safe usage
 * with all soft-delete query helpers.
 *
 * @example
 * ```ts
 * import { pgTable, serial, text, timestamp } from 'drizzle-orm/pg-core';
 * import { pgSoftDelete } from 'drizzle-orm/soft-delete/pg';
 *
 * // Define table with soft delete - automatically adds deletedAt column
 * const users = pgSoftDelete(pgTable('users', {
 *   id: serial().primaryKey(),
 *   name: text().notNull(),
 * }));
 * // users is typed as PgTableWithColumns<...> & SoftDeleteTable<'deletedAt', 'timestamp'>
 *
 * // Query only non-deleted rows
 * db.select().from(users).where(softDeleteWhere(users));
 *
 * // Query only deleted rows
 * db.select().from(users).where(onlyDeletedWhere(users));
 *
 * // Soft delete a row
 * db.update(users).set(softDeleteSet(users)).where(eq(users.id, 1));
 *
 * // Restore a soft-deleted row
 * db.update(users).set(restoreSet(users)).where(eq(users.id, 1));
 * ```
 */
import { Table } from '~/table.ts';
import { sql } from '~/sql/sql.ts';
import { getPgColumnBuilders } from '~/pg-core/columns/all.ts';
import type { PgTimestamp } from '~/pg-core/columns/timestamp.ts';
import type { PgTableWithColumns } from '~/pg-core/table.ts';
import {
	SoftDeleteMeta,
	type SoftDeleteConfig,
	type SoftDeleteMetaInfo,
	type SoftDeleteTable,
} from './soft-delete.ts';

export interface PgSoftDeleteConfig<
	TFieldName extends string = string,
	TType extends 'timestamp' | 'integer' = 'timestamp' | 'integer',
> extends SoftDeleteConfig<TFieldName, TType> {
	/** Whether to use `with time zone` on the timestamp column. Defaults to true */
	withTimezone?: boolean;
	/** Precision for the timestamp column */
	precision?: 0 | 1 | 2 | 3 | 4 | 5 | 6;
}

/**
 * Creates a pgTable with soft-delete support by automatically appending
 * a `deletedAt` timestamp column.
 *
 * The returned table is branded as `SoftDeleteTable`, which provides:
 * 1. **Column access** — `users.deletedAt` is properly typed as `PgTimestamp`
 * 2. **Type-safe helpers** — `softDeleteWhere(users)` works, but passing a non-soft-delete table errors at compile time
 * 3. **Configured field name** — `softDeleteSet(users)` returns `{ deletedAt: SQL }` (or the custom field name)
 *
 * @example
 * ```ts
 * // Default: adds 'deletedAt' timestamp column
 * const users = pgSoftDelete(pgTable('users', {
 *   id: serial().primaryKey(),
 *   name: text().notNull(),
 * }));
 *
 * // Custom field name
 * const posts = pgSoftDelete(pgTable('posts', { ... }), {
 *   fieldName: 'removed_at',
 *   withTimezone: true,
 * });
 * ```
 */
export function pgSoftDelete<
	TTable extends PgTableWithColumns<any>,
	TFieldName extends string = 'deletedAt',
	TType extends 'timestamp' | 'integer' = 'timestamp',
>(
	table: TTable,
	config?: PgSoftDeleteConfig<TFieldName, TType>,
): TTable & { [K in TFieldName]: PgTimestamp<any> } & SoftDeleteTable<TFieldName, TType> {
	const fieldName = (config?.fieldName ?? 'deletedAt') as TFieldName;
	const type = (config?.type ?? 'timestamp') as TType;
	const withTimezone = config?.withTimezone ?? true;

	// Store runtime metadata
	(table as any)[SoftDeleteMeta] = {
		fieldName,
		type,
	} satisfies SoftDeleteMetaInfo;

	// Create the timestamp column builder
	const { timestamp } = getPgColumnBuilders();
	const deletedAtBuilder = timestamp(fieldName, {
		withTimezone,
		precision: config?.precision,
	}).$onUpdateFn(() => sql`now()`);

	// Build the column against the existing table
	deletedAtBuilder.setName(fieldName);
	const deletedAtColumn = deletedAtBuilder.build(table as any);

	// Mix the column into the table object
	Object.assign(table as any, { [fieldName]: deletedAtColumn });

	// Also add to the internal columns registry so drizzle-kit migrations see it
	const columns = (table as any)[Table.Symbol.Columns];
	if (columns && typeof columns === 'object') {
		columns[fieldName] = deletedAtColumn;
	}

	return table as TTable & { [K in TFieldName]: PgTimestamp<any> } & SoftDeleteTable<TFieldName, TType>;
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
	pgAddSoftDeleteColumn,
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
