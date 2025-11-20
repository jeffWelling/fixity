package migrate

import (
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/postgres/*.sql
var postgresFS embed.FS

// AutoMigrate runs all pending migrations automatically
func AutoMigrate(db *sql.DB, dbName string) error {
	fmt.Println("Running database migrations...")

	// Create migration source from embedded files
	sourceDriver, err := iofs.New(postgresFS, "migrations/postgres")
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	// Create database driver
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create database driver: %w", err)
	}

	// Create migrator
	m, err := migrate.NewWithInstance("iofs", sourceDriver, dbName, driver)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}

	// Get current version
	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return fmt.Errorf("failed to get migration version: %w", err)
	}

	if dirty {
		return fmt.Errorf("database is in dirty state at version %d - manual intervention required", version)
	}

	// Run migrations
	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	if err == migrate.ErrNoChange {
		fmt.Printf("Database is up to date (version %d)\n", version)
	} else {
		newVersion, _, _ := m.Version()
		fmt.Printf("Migrations complete: v%d -> v%d\n", version, newVersion)
	}

	return nil
}

// MigrateDown rolls back the last migration (for manual use)
func MigrateDown(db *sql.DB, dbName string) error {
	fmt.Println("Rolling back last migration...")

	sourceDriver, err := iofs.New(postgresFS, "migrations/postgres")
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create database driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, dbName, driver)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}

	version, _, _ := m.Version()

	err = m.Steps(-1)
	if err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to rollback migration: %w", err)
	}

	newVersion, _, _ := m.Version()
	fmt.Printf("Rollback complete: v%d -> v%d\n", version, newVersion)

	return nil
}

// ListMigrations lists all available migrations
func ListMigrations() ([]string, error) {
	entries, err := postgresFS.ReadDir("migrations/postgres")
	if err != nil {
		return nil, fmt.Errorf("failed to read migrations: %w", err)
	}

	var migrations []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			migrations = append(migrations, entry.Name())
		}
	}

	sort.Strings(migrations)
	return migrations, nil
}
