package testutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jeffanddom/fixity/internal/database"
	"github.com/jmoiron/sqlx"
)

// Common time constants for tests
const (
	Hour = time.Hour
)

// TimeNow returns current time (wrapper for easier mocking in tests if needed)
func TimeNow() time.Time {
	return time.Now()
}

// TestDBConfig holds test database configuration
type TestDBConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
}

// NewTestDB creates a new test database instance
// Uses environment variables or defaults to Ra test instance
func NewTestDB(t *testing.T) *database.Database {
	t.Helper()

	cfg := database.ConnectionConfig{
		Host:            "ra.homelab.justdev.ca",
		Port:            5432,
		User:            "fixity",
		Password:        "CFgqRjyTNIjODqi/O3n/HlR3erN7n0wFVhuwklSGmLk=",
		Database:        "fixity",
		SSLMode:         "disable",
		MaxOpenConns:    10,
		MaxIdleConns:    2,
		ConnMaxLifetime: 5 * time.Minute,
	}

	db, err := database.New(cfg)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.Health(ctx); err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}

	return db
}

// CleanupDB cleans up the test database by truncating all tables
func CleanupDB(t *testing.T, db *database.Database) {
	t.Helper()

	ctx := context.Background()
	tables := []string{
		"webhook_deliveries",
		"webhooks",
		"change_events",
		"scan_checkpoints",
		"scans",
		"files",
		"storage_targets",
		"sessions",
		"users",
		// Don't truncate config table (contains default values)
	}

	for _, table := range tables {
		query := fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", table)
		err := db.WithinTransaction(ctx, func(tx *sqlx.Tx) error {
			_, err := tx.ExecContext(ctx, query)
			return err
		})
		if err != nil {
			t.Fatalf("failed to truncate table %s: %v", table, err)
		}
	}
}

// MustCreateStorageTarget creates a storage target or fails the test
func MustCreateStorageTarget(t *testing.T, db *database.Database, name string) *database.StorageTarget {
	t.Helper()

	target := &database.StorageTarget{
		Name:                name,
		Type:                database.StorageTypeLocal,
		Path:                "/tmp/test",
		Enabled:             true,
		ParallelWorkers:     1,
		RandomSamplePercent: 1.0,
		ChecksumAlgorithm:   "md5",
		CheckpointInterval:  1000,
		BatchSize:           1000,
	}

	if err := db.StorageTargets.Create(context.Background(), target); err != nil {
		t.Fatalf("failed to create storage target: %v", err)
	}

	return target
}

// MustCreateFile creates a file or fails the test
func MustCreateFile(t *testing.T, db *database.Database, targetID int64, path string) *database.File {
	t.Helper()

	now := time.Now()
	checksum := "abc123"
	checksumType := "md5"

	file := &database.File{
		StorageTargetID:   targetID,
		Path:              path,
		Size:              1024,
		FirstSeen:         now,
		LastSeen:          now,
		CurrentChecksum:   &checksum,
		ChecksumType:      &checksumType,
		LastChecksummedAt: &now,
	}

	if err := db.Files.Create(context.Background(), file); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	return file
}

// MustCreateScan creates a scan or fails the test
func MustCreateScan(t *testing.T, db *database.Database, targetID int64) *database.Scan {
	t.Helper()

	scan := &database.Scan{
		StorageTargetID: targetID,
		Status:          database.ScanStatusRunning,
		StartedAt:       time.Now(),
	}

	if err := db.Scans.Create(context.Background(), scan); err != nil {
		t.Fatalf("failed to create scan: %v", err)
	}

	return scan
}

// MustCreateUser creates a user or fails the test
func MustCreateUser(t *testing.T, db *database.Database, username string, isAdmin bool) *database.User {
	t.Helper()

	user := &database.User{
		Username:     username,
		PasswordHash: "$2a$12$hash", // bcrypt hash placeholder
		IsAdmin:      isAdmin,
	}

	if err := db.Users.Create(context.Background(), user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	return user
}

// MustCreateChangeEvent creates a change event or fails the test
func MustCreateChangeEvent(t *testing.T, db *database.Database, scanID, fileID int64, eventType database.ChangeEventType) *database.ChangeEvent {
	t.Helper()

	event := &database.ChangeEvent{
		ScanID:     scanID,
		FileID:     fileID,
		EventType:  eventType,
		DetectedAt: time.Now(),
	}

	if err := db.ChangeEvents.Create(context.Background(), event); err != nil {
		t.Fatalf("failed to create change event: %v", err)
	}

	return event
}

// MustGetAllChangeEvents retrieves all change events for a scan or fails the test
func MustGetAllChangeEvents(t *testing.T, db *database.Database, scanID int64) []*database.ChangeEvent {
	t.Helper()

	events, err := db.ChangeEvents.GetByScan(context.Background(), scanID)
	if err != nil {
		t.Fatalf("failed to get change events: %v", err)
	}

	return events
}
