package database_test

import (
	"context"
	"testing"

	"github.com/jeffanddom/fixity/internal/database"
	"github.com/jeffanddom/fixity/tests/testutil"
)

func TestStorageTargetRepository_Create(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	t.Run("creates storage target successfully", func(t *testing.T) {
		target := &database.StorageTarget{
			Name:                "test-target",
			Type:                database.StorageTypeLocal,
			Path:                "/tmp/test",
			Enabled:             true,
			ParallelWorkers:     2,
			RandomSamplePercent: 1.5,
			ChecksumAlgorithm:   "sha256",
			CheckpointInterval:  500,
			BatchSize:           2000,
		}

		err := db.StorageTargets.Create(context.Background(), target)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if target.ID == 0 {
			t.Error("expected ID to be set")
		}

		if target.CreatedAt.IsZero() {
			t.Error("expected CreatedAt to be set")
		}

		if target.UpdatedAt.IsZero() {
			t.Error("expected UpdatedAt to be set")
		}
	})

	t.Run("enforces unique name constraint", func(t *testing.T) {
		name := "duplicate-target"
		testutil.MustCreateStorageTarget(t, db, name)

		// Try to create duplicate
		duplicate := &database.StorageTarget{
			Name:                name,
			Type:                database.StorageTypeLocal,
			Path:                "/tmp/other",
			Enabled:             true,
			ParallelWorkers:     1,
			RandomSamplePercent: 1.0,
			ChecksumAlgorithm:   "md5",
			CheckpointInterval:  1000,
			BatchSize:           1000,
		}

		err := db.StorageTargets.Create(context.Background(), duplicate)
		if err == nil {
			t.Error("expected error for duplicate name")
		}
	})

	t.Run("creates SMB target with credentials", func(t *testing.T) {
		server := "nas.example.com"
		share := "media"
		credRef := "file:/etc/fixity/creds/nas.yaml"

		target := &database.StorageTarget{
			Name:                "smb-target",
			Type:                database.StorageTypeSMB,
			Path:                "/mnt/smb",
			Server:              &server,
			Share:               &share,
			CredentialsRef:      &credRef,
			Enabled:             true,
			ParallelWorkers:     1,
			RandomSamplePercent: 1.0,
			ChecksumAlgorithm:   "md5",
			CheckpointInterval:  1000,
			BatchSize:           1000,
		}

		err := db.StorageTargets.Create(context.Background(), target)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify SMB-specific fields
		retrieved, err := db.StorageTargets.GetByID(context.Background(), target.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if retrieved.Server == nil || *retrieved.Server != server {
			t.Errorf("expected server %s, got %v", server, retrieved.Server)
		}

		if retrieved.Share == nil || *retrieved.Share != share {
			t.Errorf("expected share %s, got %v", share, retrieved.Share)
		}
	})

	t.Run("creates target with thresholds", func(t *testing.T) {
		count := 1000
		percent := 5.0
		bytes := int64(100000000)

		target := &database.StorageTarget{
			Name:                            "target-with-thresholds",
			Type:                            database.StorageTypeLocal,
			Path:                            "/tmp/test",
			Enabled:                         true,
			ParallelWorkers:                 1,
			RandomSamplePercent:             1.0,
			ChecksumAlgorithm:               "md5",
			CheckpointInterval:              1000,
			BatchSize:                       1000,
			LargeChangeThresholdCount:       &count,
			LargeChangeThresholdPercent:     &percent,
			LargeChangeThresholdBytes:       &bytes,
		}

		err := db.StorageTargets.Create(context.Background(), target)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify thresholds
		retrieved, err := db.StorageTargets.GetByID(context.Background(), target.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if retrieved.LargeChangeThresholdCount == nil || *retrieved.LargeChangeThresholdCount != count {
			t.Errorf("expected threshold count %d, got %v", count, retrieved.LargeChangeThresholdCount)
		}
	})
}

func TestStorageTargetRepository_GetByID(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	target := testutil.MustCreateStorageTarget(t, db, "test-target")

	t.Run("retrieves target by ID", func(t *testing.T) {
		retrieved, err := db.StorageTargets.GetByID(context.Background(), target.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if retrieved.Name != target.Name {
			t.Errorf("expected name %s, got %s", target.Name, retrieved.Name)
		}

		if retrieved.Type != target.Type {
			t.Errorf("expected type %s, got %s", target.Type, retrieved.Type)
		}
	})

	t.Run("returns error for non-existent ID", func(t *testing.T) {
		_, err := db.StorageTargets.GetByID(context.Background(), 999999)
		if err == nil {
			t.Error("expected error for non-existent ID")
		}
	})
}

func TestStorageTargetRepository_GetByName(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	name := "unique-target"
	target := testutil.MustCreateStorageTarget(t, db, name)

	t.Run("retrieves target by name", func(t *testing.T) {
		retrieved, err := db.StorageTargets.GetByName(context.Background(), name)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if retrieved.ID != target.ID {
			t.Errorf("expected ID %d, got %d", target.ID, retrieved.ID)
		}
	})

	t.Run("returns error for non-existent name", func(t *testing.T) {
		_, err := db.StorageTargets.GetByName(context.Background(), "nonexistent")
		if err == nil {
			t.Error("expected error for non-existent name")
		}
	})
}

func TestStorageTargetRepository_ListAll(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	testutil.MustCreateStorageTarget(t, db, "target1")
	testutil.MustCreateStorageTarget(t, db, "target2")

	// Create disabled target
	disabled := testutil.MustCreateStorageTarget(t, db, "disabled-target")
	disabled.Enabled = false
	db.StorageTargets.Update(context.Background(), disabled)

	t.Run("lists all targets", func(t *testing.T) {
		targets, err := db.StorageTargets.ListAll(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(targets) != 3 {
			t.Errorf("expected 3 targets, got %d", len(targets))
		}
	})

	t.Run("returns empty list when no targets", func(t *testing.T) {
		testutil.CleanupDB(t, db)

		targets, err := db.StorageTargets.ListAll(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(targets) != 0 {
			t.Errorf("expected 0 targets, got %d", len(targets))
		}
	})
}

func TestStorageTargetRepository_ListEnabled(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	testutil.MustCreateStorageTarget(t, db, "enabled1")
	testutil.MustCreateStorageTarget(t, db, "enabled2")

	// Create disabled target
	disabled := testutil.MustCreateStorageTarget(t, db, "disabled")
	disabled.Enabled = false
	db.StorageTargets.Update(context.Background(), disabled)

	t.Run("lists only enabled targets", func(t *testing.T) {
		targets, err := db.StorageTargets.ListEnabled(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(targets) != 2 {
			t.Errorf("expected 2 enabled targets, got %d", len(targets))
		}

		for _, target := range targets {
			if !target.Enabled {
				t.Errorf("expected all targets to be enabled, found disabled: %s", target.Name)
			}
		}
	})
}

func TestStorageTargetRepository_Update(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	target := testutil.MustCreateStorageTarget(t, db, "test-target")

	t.Run("updates target successfully", func(t *testing.T) {
		target.Name = "updated-target"
		target.ParallelWorkers = 4
		target.ChecksumAlgorithm = "blake3"
		target.Enabled = false

		err := db.StorageTargets.Update(context.Background(), target)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify update
		updated, err := db.StorageTargets.GetByID(context.Background(), target.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if updated.Name != "updated-target" {
			t.Errorf("expected name updated-target, got %s", updated.Name)
		}

		if updated.ParallelWorkers != 4 {
			t.Errorf("expected 4 workers, got %d", updated.ParallelWorkers)
		}

		if updated.ChecksumAlgorithm != "blake3" {
			t.Errorf("expected blake3, got %s", updated.ChecksumAlgorithm)
		}

		if updated.Enabled {
			t.Error("expected target to be disabled")
		}

		if !updated.UpdatedAt.After(target.CreatedAt) {
			t.Error("expected updated_at to be after created_at")
		}
	})

	t.Run("returns error for non-existent target", func(t *testing.T) {
		nonExistent := &database.StorageTarget{
			ID:   999999,
			Name: "nonexistent",
		}

		err := db.StorageTargets.Update(context.Background(), nonExistent)
		if err == nil {
			t.Error("expected error for non-existent target")
		}
	})
}

func TestStorageTargetRepository_Delete(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	target := testutil.MustCreateStorageTarget(t, db, "test-target")

	t.Run("deletes target successfully", func(t *testing.T) {
		err := db.StorageTargets.Delete(context.Background(), target.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify deletion
		_, err = db.StorageTargets.GetByID(context.Background(), target.ID)
		if err == nil {
			t.Error("expected error when getting deleted target")
		}
	})

	t.Run("returns error for non-existent target", func(t *testing.T) {
		err := db.StorageTargets.Delete(context.Background(), 999999)
		if err == nil {
			t.Error("expected error for non-existent target")
		}
	})

	t.Run("cascades deletion to related records", func(t *testing.T) {
		// Create target with files
		target := testutil.MustCreateStorageTarget(t, db, "cascade-target")
		testutil.MustCreateFile(t, db, target.ID, "/test/file.txt")

		// Delete target
		err := db.StorageTargets.Delete(context.Background(), target.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify files are also deleted (CASCADE)
		files, err := db.Files.List(context.Background(), database.FileFilters{
			StorageTargetID: &target.ID,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(files) != 0 {
			t.Errorf("expected 0 files after cascade delete, got %d", len(files))
		}
	})
}
