package dbmate

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/amacneil/dbmate/v2/pkg/dbutil"
)

// Error codes
var (
	ErrNoMigrationFiles      = errors.New("no migration files found")
	ErrInvalidURL            = errors.New("invalid url, have you set your --url flag or DATABASE_URL environment variable?")
	ErrNoRollback            = errors.New("can't rollback: no migrations have been applied")
	ErrCantConnect           = errors.New("unable to connect to database")
	ErrUnsupportedDriver     = errors.New("unsupported driver")
	ErrNoMigrationName       = errors.New("please specify a name for the new migration")
	ErrMigrationAlreadyExist = errors.New("file already exists")
	ErrMigrationDirNotFound  = errors.New("could not find migrations directory")
	ErrMigrationNotFound     = errors.New("can't find migration file")
	ErrCreateDirectory       = errors.New("unable to create directory")
)

// migrationFileRegexp pattern for valid migration files
var migrationFileRegexp = regexp.MustCompile(`^(\d+).*\.sql$`)

// DB allows dbmate actions to be performed on a specified database
type DB struct {
	// AutoDumpSchema generates schema.sql after each action
	AutoDumpSchema bool
	// DatabaseURL is the database connection string
	DatabaseURL *url.URL
	// DriverName used to force specific driver (overrides deriving from url scheme)
	DriverName string
	// FS specifies the filesystem, or nil for OS filesystem
	FS fs.FS
	// Log is the interface to write stdout
	Log io.Writer
	// MigrationsDir specifies the directory or directories to find migration files
	MigrationsDir []string
	// MigrationsTableName specifies the database table to record migrations in
	MigrationsTableName string
	// SchemaFile specifies the location for schema.sql file
	SchemaFile string
	// Fail if migrations would be applied out of order
	Strict bool
	// Verbose prints the result of each statement execution
	Verbose bool
	// WaitBefore will wait for database to become available before running any actions
	WaitBefore bool
	// WaitInterval specifies length of time between connection attempts
	WaitInterval time.Duration
	// WaitTimeout specifies maximum time for connection attempts
	WaitTimeout time.Duration
	// Additional arguments for the subcommand being invoked e.g. pg_dump/mysqldump
	Args []string
}

// StatusResult represents an available migration status
type StatusResult struct {
	Filename string
	Applied  bool
}

// MigrationStatus represents a single migration's status for dashboard output
type MigrationStatus struct {
	Version  string `json:"version"`
	FileName string `json:"fileName"`
	Applied  bool   `json:"applied"`
	Status   string `json:"status"`
	Modified string `json:"modified,omitempty"`
}

// DashboardResult represents the full status dashboard output
type DashboardResult struct {
	TotalApplied int              `json:"totalApplied"`
	TotalPending int              `json:"totalPending"`
	Total        int              `json:"total"`
	Migrations   []MigrationStatus `json:"migrations"`
	// Pagination fields
	Page       int `json:"page"`
	PageSize   int `json:"pageSize"`
	TotalPages int `json:"totalPages"`
}

// New initializes a new dbmate database
func New(databaseURL *url.URL) *DB {
	return &DB{
		AutoDumpSchema:      true,
		DatabaseURL:         databaseURL,
		FS:                  nil,
		Log:                 os.Stdout,
		MigrationsDir:       []string{"./db/migrations"},
		MigrationsTableName: "schema_migrations",
		SchemaFile:          "./db/schema.sql",
		Strict:              false,
		Verbose:             false,
		WaitBefore:          false,
		WaitInterval:        time.Second,
		WaitTimeout:         60 * time.Second,
		Args:                []string{},
	}
}

// Driver initializes the appropriate database driver
func (db *DB) Driver() (Driver, error) {
	if db.DatabaseURL == nil || db.DatabaseURL.Scheme == "" {
		return nil, ErrInvalidURL
	}

	driverName := db.DatabaseURL.Scheme
	if db.DriverName != "" {
		driverName = db.DriverName
	}
	driverFunc := drivers[driverName]
	if driverFunc == nil {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedDriver, driverName)
	}

	config := DriverConfig{
		DatabaseURL:         db.DatabaseURL,
		Log:                 db.Log,
		MigrationsTableName: db.MigrationsTableName,
	}
	drv := driverFunc(config)

	if db.WaitBefore {
		if err := db.wait(drv); err != nil {
			return nil, err
		}
	}

	return drv, nil
}

func (db *DB) wait(drv Driver) error {
	// attempt connection to database server
	err := drv.Ping()
	if err == nil {
		// connection successful
		return nil
	}

	fmt.Fprint(db.Log, "Waiting for database")
	for i := 0 * time.Second; i < db.WaitTimeout; i += db.WaitInterval {
		fmt.Fprint(db.Log, ".")
		time.Sleep(db.WaitInterval)

		// attempt connection to database server
		err = drv.Ping()
		if err == nil {
			// connection successful
			fmt.Fprint(db.Log, "\n")
			return nil
		}
	}

	// if we find outselves here, we could not connect within the timeout
	fmt.Fprint(db.Log, "\n")
	return fmt.Errorf("%w: %s", ErrCantConnect, err)
}

// Wait blocks until the database server is available. It does not verify that
// the specified database exists, only that the host is ready to accept connections.
func (db *DB) Wait() error {
	drv, err := db.Driver()
	if err != nil {
		return err
	}

	// if db.WaitBefore is true, wait() will get called twice, no harm
	return db.wait(drv)
}

// CreateAndMigrate creates the database (if necessary) and runs migrations
func (db *DB) CreateAndMigrate() error {
	drv, err := db.Driver()
	if err != nil {
		return err
	}

	// create database if it does not already exist
	// skip this step if we cannot determine status
	// (e.g. user does not have list database permission)
	exists, err := drv.DatabaseExists()
	if err == nil && !exists {
		if err := drv.CreateDatabase(); err != nil {
			return err
		}
	}

	// migrate
	return db.Migrate()
}

// Create creates the current database
func (db *DB) Create() error {
	drv, err := db.Driver()
	if err != nil {
		return err
	}

	return drv.CreateDatabase()
}

// Drop drops the current database (if it exists)
func (db *DB) Drop() error {
	drv, err := db.Driver()
	if err != nil {
		return err
	}

	return drv.DropDatabase()
}

// DumpSchema writes the current database schema to a file
func (db *DB) DumpSchema() error {
	drv, err := db.Driver()
	if err != nil {
		return err
	}

	sqlDB, err := db.openDatabaseForMigration(drv)
	if err != nil {
		return err
	}
	defer dbutil.MustClose(sqlDB)

	schema, err := drv.DumpSchema(sqlDB, db.Args...)
	if err != nil {
		return err
	}

	fmt.Fprintf(db.Log, "Writing: %s\n", db.SchemaFile)

	// ensure schema directory exists
	if err = ensureDir(filepath.Dir(db.SchemaFile)); err != nil {
		return err
	}

	// write schema to file
	return os.WriteFile(db.SchemaFile, schema, 0o644)
}

// LoadSchema loads schema file to the current database
func (db *DB) LoadSchema() error {
	drv, err := db.Driver()
	if err != nil {
		return err
	}

	sqlDB, err := drv.Open()
	if err != nil {
		return err
	}
	defer dbutil.MustClose(sqlDB)

	_, err = os.Stat(db.SchemaFile)
	if err != nil {
		return err
	}

	fmt.Fprintf(db.Log, "Reading: %s\n", db.SchemaFile)

	bytes, err := os.ReadFile(db.SchemaFile)
	if err != nil {
		return err
	}

	// Strip psql meta-commands (e.g., \restrict, \unrestrict) that cannot be
	// executed directly against the database server.
	bytes, err = dbutil.StripPsqlMetaCommands(bytes)
	if err != nil {
		return err
	}

	result, err := sqlDB.Exec(string(bytes))
	if err != nil {
		return err
	} else if db.Verbose {
		db.printVerbose(result)
	}

	return nil
}

// ensureDir creates a directory if it does not already exist
func ensureDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("%w `%s`", ErrCreateDirectory, dir)
	}

	return nil
}

const migrationTemplate = "-- migrate:up\n\n\n-- migrate:down\n\n"

// NewMigration creates a new migration file
func (db *DB) NewMigration(name string) error {
	// new migration name
	timestamp := time.Now().UTC().Format("20060102150405")
	if name == "" {
		return ErrNoMigrationName
	}
	name = fmt.Sprintf("%s_%s.sql", timestamp, name)

	// create migrations dir if missing
	if err := ensureDir(db.MigrationsDir[0]); err != nil {
		return err
	}

	// check file does not already exist
	path := filepath.Join(db.MigrationsDir[0], name)
	fmt.Fprintf(db.Log, "Creating migration: %s\n", path)

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return ErrMigrationAlreadyExist
	}

	// write new migration
	file, err := os.Create(path)
	if err != nil {
		return err
	}

	defer dbutil.MustClose(file)
	_, err = file.WriteString(migrationTemplate)
	return err
}

func doTransaction(sqlDB *sql.DB, txFunc func(dbutil.Transaction) error) error {
	tx, err := sqlDB.Begin()
	if err != nil {
		return err
	}

	if err := txFunc(tx); err != nil {
		if err1 := tx.Rollback(); err1 != nil {
			return err1
		}

		return err
	}

	return tx.Commit()
}

func (db *DB) openDatabaseForMigration(drv Driver) (*sql.DB, error) {
	sqlDB, err := drv.Open()
	if err != nil {
		return nil, err
	}

	if err := drv.CreateMigrationsTable(sqlDB); err != nil {
		dbutil.MustClose(sqlDB)
		return nil, err
	}

	return sqlDB, nil
}

// Migrate migrates database to the latest version
func (db *DB) Migrate() error {
	drv, err := db.Driver()
	if err != nil {
		return err
	}

	migrations, err := db.FindMigrations()
	if err != nil {
		return err
	}

	if len(migrations) == 0 {
		return ErrNoMigrationFiles
	}

	highestAppliedMigrationVersion := ""
	pendingMigrations := []Migration{}
	for _, migration := range migrations {
		if migration.Applied {
			if db.Strict && highestAppliedMigrationVersion <= migration.Version {
				highestAppliedMigrationVersion = migration.Version
			}
		} else {
			pendingMigrations = append(pendingMigrations, migration)
		}
	}

	if len(pendingMigrations) > 0 && db.Strict && pendingMigrations[0].Version <= highestAppliedMigrationVersion {
		return fmt.Errorf(
			"migration `%s` is out of order with already applied migrations, the version number has to be higher than the applied migration `%s` in --strict mode",
			pendingMigrations[0].Version,
			highestAppliedMigrationVersion,
		)
	}

	sqlDB, err := db.openDatabaseForMigration(drv)
	if err != nil {
		return err
	}
	defer dbutil.MustClose(sqlDB)

	for _, migration := range pendingMigrations {
		fmt.Fprintf(db.Log, "Applying: %s\n", migration.FileName)

		start := time.Now()

		parsed, err := migration.Parse()
		if err != nil {
			return err
		}

		for _, migrationSection := range parsed {
			execMigration := func(tx dbutil.Transaction) error {
				// run actual migration
				result, err := tx.Exec(migrationSection.Up)
				if err != nil {
					return drv.QueryError(migrationSection.Up, err)
				} else if db.Verbose {
					db.printVerbose(result)
				}

				// record migration
				return drv.InsertMigration(tx, migration.Version)
			}

			if migrationSection.UpOptions.Transaction() {
				// begin transaction
				err = doTransaction(sqlDB, execMigration)
			} else {
				// run outside of transaction
				err = execMigration(sqlDB)
			}

			elapsed := time.Since(start)
			fmt.Fprintf(db.Log, "Applied: %s in %s\n", migration.FileName, elapsed)

			if err != nil {
				return err
			}
		}
	}

	// automatically update schema file, silence errors
	if db.AutoDumpSchema {
		_ = db.DumpSchema()
	}

	return nil
}

func (db *DB) printVerbose(result sql.Result) {
	lastInsertID, err := result.LastInsertId()
	if err == nil {
		fmt.Fprintf(db.Log, "Last insert ID: %d\n", lastInsertID)
	}
	rowsAffected, err := result.RowsAffected()
	if err == nil {
		fmt.Fprintf(db.Log, "Rows affected: %d\n", rowsAffected)
	}
}

func (db *DB) readMigrationsDir(dir string) ([]fs.DirEntry, error) {
	path := path.Clean(dir)

	// We use nil instead of os.DirFS() because DirFS cannot support both relative and absolute
	// directory paths - it must be anchored at either "." or "/", which we do not know in advance.
	// See: https://github.com/amacneil/dbmate/issues/403
	if db.FS == nil {
		return os.ReadDir(path)
	}

	return fs.ReadDir(db.FS, path)
}

// FindMigrations lists all available migrations
func (db *DB) FindMigrations() ([]Migration, error) {
	drv, err := db.Driver()
	if err != nil {
		return nil, err
	}

	sqlDB, err := drv.Open()
	if err != nil {
		return nil, err
	}
	defer dbutil.MustClose(sqlDB)

	// find applied migrations
	appliedMigrations := map[string]bool{}
	migrationsTableExists, err := drv.MigrationsTableExists(sqlDB)
	if err != nil {
		return nil, err
	}

	if migrationsTableExists {
		appliedMigrations, err = drv.SelectMigrations(sqlDB, -1)
		if err != nil {
			return nil, err
		}
	}

	migrations := []Migration{}
	for _, dir := range db.MigrationsDir {
		// find filesystem migrations
		files, err := db.readMigrationsDir(dir)
		if err != nil {
			return nil, fmt.Errorf("%w `%s`", ErrMigrationDirNotFound, dir)
		}

		for _, file := range files {
			if file.IsDir() {
				continue
			}

			matches := migrationFileRegexp.FindStringSubmatch(file.Name())
			if len(matches) < 2 {
				continue
			}

			migration := Migration{
				Applied:  false,
				FileName: matches[0],
				FilePath: path.Join(dir, matches[0]),
				FS:       db.FS,
				Version:  matches[1],
			}
			if ok := appliedMigrations[migration.Version]; ok {
				migration.Applied = true
			}

			migrations = append(migrations, migration)
		}
	}

	sort.Slice(
		migrations, func(i, j int) bool {
			return migrations[i].FileName < migrations[j].FileName
		},
	)

	return migrations, nil
}

// Rollback rolls back the most recent migration
func (db *DB) Rollback() error {
	drv, err := db.Driver()
	if err != nil {
		return err
	}

	sqlDB, err := db.openDatabaseForMigration(drv)
	if err != nil {
		return err
	}
	defer dbutil.MustClose(sqlDB)

	// find last applied migration
	var latest *Migration
	migrations, err := db.FindMigrations()
	if err != nil {
		return err
	}

	for i, migration := range migrations {
		if migration.Applied {
			latest = &migrations[i]
		}
	}

	if latest == nil {
		return ErrNoRollback
	}

	fmt.Fprintf(db.Log, "Rolling back: %s\n", latest.FileName)

	start := time.Now()

	parsedSections, err := latest.Parse()
	if err != nil {
		return err
	}

	for _, migrationSection := range parsedSections {
		execMigration := func(tx dbutil.Transaction) error {
			// rollback migration
			result, err := tx.Exec(migrationSection.Down)
			if err != nil {
				return drv.QueryError(migrationSection.Down, err)
			} else if db.Verbose {
				db.printVerbose(result)
			}

			// remove migration record
			return drv.DeleteMigration(tx, latest.Version)
		}

		if migrationSection.DownOptions.Transaction() {
			// begin transaction
			err = doTransaction(sqlDB, execMigration)
		} else {
			// run outside of transaction
			err = execMigration(sqlDB)
		}

		elapsed := time.Since(start)
		fmt.Fprintf(db.Log, "Rolled back: %s in %s\n", latest.FileName, elapsed)

		if err != nil {
			return err
		}

		// automatically update schema file, silence errors
		if db.AutoDumpSchema {
			_ = db.DumpSchema()
		}
	}

	return nil
}

// Status shows the status of all migrations
func (db *DB) Status(quiet bool) (int, error) {
	results, err := db.FindMigrations()
	if err != nil {
		return -1, err
	}

	var totalApplied int
	var line string

	for _, res := range results {
		if res.Applied {
			line = fmt.Sprintf("[X] %s", res.FileName)
			totalApplied++
		} else {
			line = fmt.Sprintf("[ ] %s", res.FileName)
		}
		if !quiet {
			fmt.Fprintln(db.Log, line)
		}
	}

	totalPending := len(results) - totalApplied
	if !quiet {
		fmt.Fprintln(db.Log)
		fmt.Fprintf(db.Log, "Applied: %d\n", totalApplied)
		fmt.Fprintf(db.Log, "Pending: %d\n", totalPending)
	}

	return totalPending, nil
}

// StatusDashboard returns a DashboardResult with migration status details
func (db *DB) StatusDashboard() (*DashboardResult, error) {
	results, err := db.FindMigrations()
	if err != nil {
		return nil, err
	}

	dashboard := &DashboardResult{
		Total: len(results),
	}

	for _, m := range results {
		status := "pending"
		if m.Applied {
			status = "applied"
			dashboard.TotalApplied++
		}

		var modified string
		if m.FS == nil {
			if info, err := os.Stat(m.FilePath); err == nil {
				modified = info.ModTime().UTC().Format(time.RFC3339)
			}
		}

		dashboard.Migrations = append(dashboard.Migrations, MigrationStatus{
			Version:  m.Version,
			FileName: m.FileName,
			Applied:  m.Applied,
			Status:   status,
			Modified: modified,
		})
	}

	dashboard.TotalPending = dashboard.Total - dashboard.TotalApplied

	return dashboard, nil
}

// FilterDashboardResult filters migrations by status ("applied" or "pending").
// If filter is empty, no filtering is applied.
// TotalApplied and TotalPending are recalculated after filtering.
func FilterDashboardResult(result *DashboardResult, filter string) {
	if filter == "" {
		return
	}

	filtered := make([]MigrationStatus, 0, len(result.Migrations))
	for _, m := range result.Migrations {
		if filter == "applied" && m.Applied {
			filtered = append(filtered, m)
		} else if filter == "pending" && !m.Applied {
			filtered = append(filtered, m)
		}
	}

	result.Migrations = filtered
	result.TotalApplied = 0
	result.TotalPending = 0
	for _, m := range result.Migrations {
		if m.Applied {
			result.TotalApplied++
		} else {
			result.TotalPending++
		}
	}
	result.Total = len(result.Migrations)
}

// LimitDashboardResult limits the number of migrations shown.
// If limit is <= 0, no limit is applied.
func LimitDashboardResult(result *DashboardResult, limit int) {
	if limit <= 0 {
		return
	}
	if limit >= len(result.Migrations) {
		return
	}
	result.Migrations = result.Migrations[:limit]
}

// PaginateDashboardResult applies pagination to the dashboard result.
// page is 1-based. pageSize must be > 0.
// If page or pageSize are zero, no pagination is applied (all migrations are shown).
func PaginateDashboardResult(result *DashboardResult, page, pageSize int) {
	if page <= 0 || pageSize <= 0 {
		result.Page = 0
		result.PageSize = 0
		result.TotalPages = 0
		return
	}

	total := len(result.Migrations)
	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}

	// Clamp page to valid range
	if page > totalPages {
		page = totalPages
	}

	start := (page - 1) * pageSize
	end := start + pageSize
	if end > total {
		end = total
	}

	result.Migrations = result.Migrations[start:end]
	result.Page = page
	result.PageSize = pageSize
	result.TotalPages = totalPages
}

// SortDashboardResult sorts migrations in the DashboardResult by the specified field.
// Supported sort fields: "version" (default), "status", "name", "modified".
func SortDashboardResult(result *DashboardResult, sortBy string) {
	if sortBy == "" {
		sortBy = "version"
	}

	sort.SliceStable(result.Migrations, func(i, j int) bool {
		a, b := result.Migrations[i], result.Migrations[j]
		switch sortBy {
		case "status":
			// applied first, then pending; within same status, sort by version
			if a.Applied != b.Applied {
				return a.Applied
			}
			return a.Version < b.Version
		case "name":
			return a.FileName < b.FileName
		case "modified":
			// most recent first, then by version for ties
			if a.Modified != b.Modified {
				return a.Modified > b.Modified
			}
			return a.Version < b.Version
		default: // "version"
			return a.Version < b.Version
		}
	})
}

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
)

// PrintDashboardTable prints the dashboard result in table format
func PrintDashboardTable(w io.Writer, result *DashboardResult, noHeader bool, useColor bool) {
	if len(result.Migrations) == 0 {
		fmt.Fprintln(w, "No migrations found.")
		return
	}

	// Color helpers
	green := func(s string) string {
		if useColor {
			return colorGreen + s + colorReset
		}
		return s
	}
	yellow := func(s string) string {
		if useColor {
			return colorYellow + s + colorReset
		}
		return s
	}
	bold := func(s string) string {
		if useColor {
			return colorBold + s + colorReset
		}
		return s
	}
	cyan := func(s string) string {
		if useColor {
			return colorCyan + s + colorReset
		}
		return s
	}

	// Column headers
	headers := []string{"Status", "Version", "Migration Name", "Modified"}
	colWidths := make([]int, len(headers))
	for i, h := range headers {
		colWidths[i] = len(h)
	}

	// Calculate column widths (use plain text widths, without ANSI codes)
	plainStatus := make([]string, len(result.Migrations))
	for i, m := range result.Migrations {
		if m.Applied {
			plainStatus[i] = "[X]"
		} else {
			plainStatus[i] = "[ ]"
		}
	}

	rows := make([][]string, len(result.Migrations))
	for i, m := range result.Migrations {
		rows[i] = []string{plainStatus[i], m.Version, m.FileName, m.Modified}
		for j, cell := range rows[i] {
			if len(cell) > colWidths[j] {
				colWidths[j] = len(cell)
			}
		}
	}

	// Print separator line
	printSeparator := func() {
		fmt.Fprint(w, "+")
		for _, cw := range colWidths {
			fmt.Fprintf(w, "%s+", strings.Repeat("-", cw+2))
		}
		fmt.Fprintln(w)
	}

	// Print row with padding
	printRow := func(cells []string) {
		fmt.Fprint(w, "|")
		for j, cell := range cells {
			fmt.Fprintf(w, " %-*s |", colWidths[j], cell)
		}
		fmt.Fprintln(w)
	}

	// Print colored data row
	printDataRow := func(idx int) {
		m := result.Migrations[idx]
		statusIcon := plainStatus[idx]
		if useColor {
			if m.Applied {
				statusIcon = green(statusIcon)
			} else {
				statusIcon = yellow(statusIcon)
			}
		}
		fmt.Fprint(w, "|")
		fmt.Fprintf(w, " %-*s |", colWidths[0], statusIcon)
		fmt.Fprintf(w, " %-*s |", colWidths[1], m.Version)
		fmt.Fprintf(w, " %-*s |", colWidths[2], m.FileName)
		fmt.Fprintf(w, " %-*s |", colWidths[3], m.Modified)
		fmt.Fprintln(w)
	}

	// Print table
	if !noHeader {
		printSeparator()
		if useColor {
			printRow([]string{bold(headers[0]), bold(headers[1]), bold(headers[2]), bold(headers[3])})
		} else {
			printRow(headers)
		}
	}
	printSeparator()

	// Print data rows
	for i := range rows {
		printDataRow(i)
	}

	printSeparator()

	// Print summary
	fmt.Fprintln(w)
	appliedLabel := fmt.Sprintf("Applied: %d", result.TotalApplied)
	pendingLabel := fmt.Sprintf("Pending: %d", result.TotalPending)
	totalLabel := fmt.Sprintf("Total: %d", result.Total)
	if useColor {
		appliedLabel = green(appliedLabel)
		pendingLabel = yellow(pendingLabel)
		totalLabel = cyan(totalLabel)
	}
	fmt.Fprintf(w, "%s  |  %s  |  %s\n", appliedLabel, pendingLabel, totalLabel)

	// Print pagination info if pagination is active
	if result.Page > 0 {
		fmt.Fprintf(w, "Page: %d/%d  |  Page Size: %d\n",
			result.Page, result.TotalPages, result.PageSize)
	}
}

// PrintDashboardJSON prints the dashboard result in JSON format
func PrintDashboardJSON(w io.Writer, result *DashboardResult, indent string) error {
	encoder := json.NewEncoder(w)
	if indent != "" {
		encoder.SetIndent("", indent)
	}
	return encoder.Encode(result)
}
