/**
 * Drizzle ORM Soft Delete - cross-dialect utilities.
 *
 * This module provides a non-intrusive soft-delete feature for Drizzle ORM
 * tables. Instead of truly deleting rows, it marks them with a `deletedAt`
 * timestamp, allowing data recovery and history tracking.
 *
 * ## Type Safety
 *
 * Soft-delete tables are branded with `SoftDeleteTable` via a Symbol, which
 * provides compile-time safety: only soft-delete tables can be passed to
 * `softDeleteWhere`, `onlyDeletedWhere`, `softDeleteSet`, and `restoreSet`.
 * Regular tables will produce a type error.
 *
 * ```ts
 * const users = pgSoftDelete(pgTable('users', { ... }));
 * softDeleteWhere(users);  // ✓ type-safe
 *
 * const regularTable = pgTable('regular', { ... });
 * softDeleteWhere(regularTable);  // ✗ type error: regularTable is not SoftDeleteTable
 * ```
 *
 * ## Quick Start (PostgreSQL)
 *
 * ```ts
 * import { pgTable, serial, text } from 'drizzle-orm/pg-core';
 * import { pgSoftDelete, softDeleteWhere, softDeleteSet, restoreSet, onlyDeletedWhere } from 'drizzle-orm/soft-delete/pg';
 *
 * // 1. Define table with soft delete
 * const users = pgSoftDelete(pgTable('users', {
 *   id: serial().primaryKey(),
 *   name: text().notNull(),
 * }));
 * // users now has: id, name, deletedAt (typed as PgTimestamp)
 *
 * // 2. Query only active (non-deleted) rows
 * const activeUsers = await db.select().from(users).where(softDeleteWhere(users));
 *
 * // 3. Query all rows (including soft-deleted)
 * const allUsers = await db.select().from(users);
 *
 * // 4. Query only deleted rows
 * const deletedUsers = await db.select().from(users).where(onlyDeletedWhere(users));
 *
 * // 5. Soft delete a row (sets deletedAt to now)
 * await db.update(users).set(softDeleteSet(users)).where(eq(users.id, 1));
 *
 * // 6. Restore a soft-deleted row (sets deletedAt to null)
 * await db.update(users).set(restoreSet(users)).where(eq(users.id, 1));
 * ```
 *
 * ## MySQL
 *
 * ```ts
 * import { mysqlSoftDelete, softDeleteWhere, softDeleteSet, restoreSet, onlyDeletedWhere } from 'drizzle-orm/soft-delete/mysql';
 * ```
 *
 * ## SQLite
 *
 * ```ts
 * import { sqliteSoftDelete, softDeleteWhere, softDeleteSet, restoreSet, onlyDeletedWhere } from 'drizzle-orm/soft-delete/sqlite';
 * ```
 *
 * ## Custom Configuration
 *
 * ```ts
 * const users = pgSoftDelete(pgTable('users', { ... }), {
 *   fieldName: 'deleted_at',     // custom column name
 *   withTimezone: true,          // PG only: timestamp with time zone
 * });
 * // softDeleteSet(users) → { deleted_at: sql`now()` }
 * // restoreSet(users) → { deleted_at: null }
 * ```
 */

// Core utilities (dialect-agnostic)
export {
	softDelete,
	softDeleteWhere,
	onlyDeletedWhere,
	softDeleteSet,
	restoreSet,
	batchSoftDeleteWhere,
	batchOnlyDeletedWhere,
	batchRestoreWhere,
	pgAddSoftDeleteColumn,
	mysqlAddSoftDeleteColumn,
	sqliteAddSoftDeleteColumn,
	buildCascadeRelationMap,
	getCascadeSoftDeleteConditions,
	getCascadeRestoreConditions,
	isSoftDeleteTable,
	getSoftDeleteMeta,
	getSoftDeletedAtColumn,
	SoftDeleteBrand,
	type SoftDeleteConfig,
	type SoftDeleteMetaInfo,
	type SoftDeleteTable,
	type SoftDeleteTableConstraint,
	type CascadeRelation,
	type CascadeRelationMap,
} from './soft-delete.ts';
