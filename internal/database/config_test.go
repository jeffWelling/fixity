package database_test

import (
	"context"
	"testing"

	"github.com/jeffanddom/fixity/internal/database"
	"github.com/jeffanddom/fixity/tests/testutil"
)

func TestConfigRepository_Set(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	t.Run("sets new config value", func(t *testing.T) {
		err := db.Config.Set(context.Background(), "test.key", "test-value", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		value, err := db.Config.GetValue(context.Background(), "test.key")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if value != "test-value" {
			t.Errorf("expected value test-value, got %s", value)
		}
	})

	t.Run("sets config value with updater", func(t *testing.T) {
		user := testutil.MustCreateUser(t, db, "admin", true)
		userID := user.ID

		err := db.Config.Set(context.Background(), "admin.key", "admin-value", &userID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		config, err := db.Config.Get(context.Background(), "admin.key")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if config.UpdatedBy == nil || *config.UpdatedBy != userID {
			t.Errorf("expected updated_by %d, got %v", userID, config.UpdatedBy)
		}
	})

	t.Run("updates existing config value", func(t *testing.T) {
		db.Config.Set(context.Background(), "update.key", "original", nil)

		err := db.Config.Set(context.Background(), "update.key", "updated", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		value, err := db.Config.GetValue(context.Background(), "update.key")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if value != "updated" {
			t.Errorf("expected value updated, got %s", value)
		}
	})
}

func TestConfigRepository_Get(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	db.Config.Set(context.Background(), "test.key", "test-value", nil)

	t.Run("retrieves config by key", func(t *testing.T) {
		config, err := db.Config.Get(context.Background(), "test.key")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if config.Key != "test.key" {
			t.Errorf("expected key test.key, got %s", config.Key)
		}

		if config.Value != "test-value" {
			t.Errorf("expected value test-value, got %s", config.Value)
		}

		if config.UpdatedAt.IsZero() {
			t.Error("expected UpdatedAt to be set")
		}
	})

	t.Run("returns error for non-existent key", func(t *testing.T) {
		_, err := db.Config.Get(context.Background(), "nonexistent.key")
		if err == nil {
			t.Error("expected error for non-existent key")
		}
	})
}

func TestConfigRepository_GetValue(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	db.Config.Set(context.Background(), "test.key", "test-value", nil)

	t.Run("retrieves config value by key", func(t *testing.T) {
		value, err := db.Config.GetValue(context.Background(), "test.key")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if value != "test-value" {
			t.Errorf("expected value test-value, got %s", value)
		}
	})

	t.Run("returns error for non-existent key", func(t *testing.T) {
		_, err := db.Config.GetValue(context.Background(), "nonexistent.key")
		if err == nil {
			t.Error("expected error for non-existent key")
		}
	})
}

func TestConfigRepository_ListAll(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	// Create multiple config entries
	db.Config.Set(context.Background(), "key.a", "value-a", nil)
	db.Config.Set(context.Background(), "key.b", "value-b", nil)
	db.Config.Set(context.Background(), "key.c", "value-c", nil)

	t.Run("lists all config entries", func(t *testing.T) {
		configs, err := db.Config.ListAll(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(configs) != 3 {
			t.Errorf("expected 3 configs, got %d", len(configs))
		}
	})

	t.Run("orders configs by key", func(t *testing.T) {
		configs, err := db.Config.ListAll(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(configs) >= 2 {
			for i := 0; i < len(configs)-1; i++ {
				if configs[i].Key > configs[i+1].Key {
					t.Error("configs not ordered by key")
				}
			}
		}
	})

	t.Run("returns empty list when no configs", func(t *testing.T) {
		testutil.CleanupDB(t, db)

		configs, err := db.Config.ListAll(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(configs) != 0 {
			t.Errorf("expected 0 configs, got %d", len(configs))
		}
	})
}

func TestConfigRepository_Delete(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	db.Config.Set(context.Background(), "delete.key", "delete-value", nil)

	t.Run("deletes config successfully", func(t *testing.T) {
		err := db.Config.Delete(context.Background(), "delete.key")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify deletion
		_, err = db.Config.Get(context.Background(), "delete.key")
		if err == nil {
			t.Error("expected error when getting deleted config")
		}
	})

	t.Run("returns error for non-existent key", func(t *testing.T) {
		err := db.Config.Delete(context.Background(), "nonexistent.key")
		if err == nil {
			t.Error("expected error for non-existent key")
		}
	})
}

func TestCheckpointRepository_Create(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	target := testutil.MustCreateStorageTarget(t, db, "test-target")
	scan := testutil.MustCreateScan(t, db, target.ID)

	t.Run("creates checkpoint successfully", func(t *testing.T) {
		checkpoint := &database.ScanCheckpoint{
			ScanID:            scan.ID,
			LastProcessedPath: "/test/file.txt",
			FilesProcessed:    100,
		}

		err := db.Checkpoints.Create(context.Background(), checkpoint)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if checkpoint.CheckpointAt.IsZero() {
			t.Error("expected CheckpointAt to be set")
		}
	})

	t.Run("updates existing checkpoint (upsert)", func(t *testing.T) {
		scan2 := testutil.MustCreateScan(t, db, target.ID)

		// Create initial checkpoint
		checkpoint := &database.ScanCheckpoint{
			ScanID:            scan2.ID,
			LastProcessedPath: "/test/file1.txt",
			FilesProcessed:    50,
		}
		db.Checkpoints.Create(context.Background(), checkpoint)

		initialTime := checkpoint.CheckpointAt

		// Update checkpoint (same scan_id)
		checkpoint.LastProcessedPath = "/test/file2.txt"
		checkpoint.FilesProcessed = 100

		err := db.Checkpoints.Create(context.Background(), checkpoint)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify update
		retrieved, err := db.Checkpoints.Get(context.Background(), scan2.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if retrieved.LastProcessedPath != "/test/file2.txt" {
			t.Errorf("expected path /test/file2.txt, got %s", retrieved.LastProcessedPath)
		}

		if retrieved.FilesProcessed != 100 {
			t.Errorf("expected 100 files processed, got %d", retrieved.FilesProcessed)
		}

		if !retrieved.CheckpointAt.After(initialTime) {
			t.Error("expected checkpoint_at to be updated")
		}
	})
}

func TestCheckpointRepository_Get(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	target := testutil.MustCreateStorageTarget(t, db, "test-target")
	scan := testutil.MustCreateScan(t, db, target.ID)

	checkpoint := &database.ScanCheckpoint{
		ScanID:            scan.ID,
		LastProcessedPath: "/test/file.txt",
		FilesProcessed:    100,
	}
	db.Checkpoints.Create(context.Background(), checkpoint)

	t.Run("retrieves checkpoint by scan ID", func(t *testing.T) {
		retrieved, err := db.Checkpoints.Get(context.Background(), scan.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if retrieved.ScanID != scan.ID {
			t.Errorf("expected scan ID %d, got %d", scan.ID, retrieved.ScanID)
		}

		if retrieved.LastProcessedPath != "/test/file.txt" {
			t.Errorf("expected path /test/file.txt, got %s", retrieved.LastProcessedPath)
		}

		if retrieved.FilesProcessed != 100 {
			t.Errorf("expected 100 files processed, got %d", retrieved.FilesProcessed)
		}
	})

	t.Run("returns nil for non-existent checkpoint", func(t *testing.T) {
		scan2 := testutil.MustCreateScan(t, db, target.ID)

		checkpoint, err := db.Checkpoints.Get(context.Background(), scan2.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if checkpoint != nil {
			t.Error("expected nil checkpoint for scan with no checkpoint")
		}
	})
}

func TestCheckpointRepository_Delete(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	target := testutil.MustCreateStorageTarget(t, db, "test-target")
	scan := testutil.MustCreateScan(t, db, target.ID)

	checkpoint := &database.ScanCheckpoint{
		ScanID:            scan.ID,
		LastProcessedPath: "/test/file.txt",
		FilesProcessed:    100,
	}
	db.Checkpoints.Create(context.Background(), checkpoint)

	t.Run("deletes checkpoint successfully", func(t *testing.T) {
		err := db.Checkpoints.Delete(context.Background(), scan.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify deletion
		retrieved, err := db.Checkpoints.Get(context.Background(), scan.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if retrieved != nil {
			t.Error("expected nil checkpoint after deletion")
		}
	})

	t.Run("succeeds for non-existent checkpoint", func(t *testing.T) {
		scan2 := testutil.MustCreateScan(t, db, target.ID)

		err := db.Checkpoints.Delete(context.Background(), scan2.ID)
		if err != nil {
			t.Errorf("unexpected error for non-existent checkpoint: %v", err)
		}
	})
}
