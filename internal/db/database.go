package db

import (
	"database/sql"
	"embed"
	"fmt"
	"os"

	_ "modernc.org/sqlite"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations
var migrationsFS embed.FS

// busyTimeoutDSN makes every connection of the pool wait up to 5 s for a lock
// held by another connection instead of failing at once with "database is
// locked" (Bug 34) — e.g. a status change while a rescan commits. modernc runs
// each _pragma on every new connection; the query part is cut off before the
// path is opened, so plain Windows paths keep working.
const busyTimeoutDSN = "?_pragma=busy_timeout(5000)"

func Open(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath+busyTimeoutDSN)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	return db, nil
}

// newMigrate builds a migrator over the embedded migrations for db. The caller
// must not Close the returned instance: that would also close db, which stays
// owned by the caller. A test uses it to stop at an older schema version
// before running Migrate (Bug 27).
func newMigrate(db *sql.DB) (*migrate.Migrate, error) {
	driver, err := sqlite3.WithInstance(db, &sqlite3.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to create migrate driver: %w", err)
	}

	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("failed to create migrate source: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "sqlite3", driver)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrate instance: %w", err)
	}
	return m, nil
}

func Migrate(db *sql.DB) error {
	m, err := newMigrate(db)
	if err != nil {
		return err
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	return nil
}

// LeftoverJournal reports whether a rollback journal (<dbPath>-journal) lies
// next to the database before it is opened, and its size. SQLite deletes the
// journal when a write transaction ends, so a leftover one means the previous
// run was interrupted in the middle of a write (Bug 34). SQLite rolls it back
// on the first read after opening.
func LeftoverJournal(dbPath string) (size int64, found bool) {
	info, err := os.Stat(dbPath + "-journal")
	if err != nil || info.IsDir() {
		return 0, false
	}
	return info.Size(), true
}
