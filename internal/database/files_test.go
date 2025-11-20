package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/jeffanddom/fixity/internal/database"
	"github.com/jeffanddom/fixity/tests/testutil"
)

func TestFileRepository_Create(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	target := testutil.MustCreateStorageTarget(t, db, "test-target")

	t.Run("creates file successfully", func(t *testing.T) {
		now := time.Now()
		checksum := "abc123"
		checksumType := "md5"

		file := &database.File{
			StorageTargetID:   target.ID,
			Path:              "/test/file.txt",
			Size:              1024,
			FirstSeen:         now,
			LastSeen:          now,
			CurrentChecksum:   &checksum,
			ChecksumType:      &checksumType,
			LastChecksummedAt: &now,
		}

		err := db.Files.Create(context.Background(), file)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if file.ID == 0 {
			t.Error("expected ID to be set")
		}

		if file.CreatedAt.IsZero() {
			t.Error("expected CreatedAt to be set")
		}

		if file.UpdatedAt.IsZero() {
			t.Error("expected UpdatedAt to be set")
		}
	})

	t.Run("enforces unique constraint on target_id + path", func(t *testing.T) {
		path := "/test/duplicate.txt"
		testutil.MustCreateFile(t, db, target.ID, path)

		// Try to create duplicate
		now := time.Now()
		file := &database.File{
			StorageTargetID: target.ID,
			Path:            path,
			Size:            2048,
			FirstSeen:       now,
			LastSeen:        now,
		}

		err := db.Files.Create(context.Background(), file)
		if err == nil {
			t.Error("expected error for duplicate file, got nil")
		}
	})
}

func TestFileRepository_GetByID(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	target := testutil.MustCreateStorageTarget(t, db, "test-target")
	created := testutil.MustCreateFile(t, db, target.ID, "/test/file.txt")

	t.Run("retrieves file by ID", func(t *testing.T) {
		file, err := db.Files.GetByID(context.Background(), created.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if file.Path != created.Path {
			t.Errorf("expected path %s, got %s", created.Path, file.Path)
		}

		if file.Size != created.Size {
			t.Errorf("expected size %d, got %d", created.Size, file.Size)
		}
	})

	t.Run("returns error for non-existent ID", func(t *testing.T) {
		_, err := db.Files.GetByID(context.Background(), 999999)
		if err == nil {
			t.Error("expected error for non-existent ID, got nil")
		}
	})
}

func TestFileRepository_GetByPath(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	target := testutil.MustCreateStorageTarget(t, db, "test-target")
	path := "/test/file.txt"
	created := testutil.MustCreateFile(t, db, target.ID, path)

	t.Run("retrieves file by path", func(t *testing.T) {
		file, err := db.Files.GetByPath(context.Background(), target.ID, path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if file == nil {
			t.Fatal("expected file, got nil")
		}

		if file.ID != created.ID {
			t.Errorf("expected ID %d, got %d", created.ID, file.ID)
		}
	})

	t.Run("returns nil for non-existent path", func(t *testing.T) {
		file, err := db.Files.GetByPath(context.Background(), target.ID, "/nonexistent")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if file != nil {
			t.Error("expected nil for non-existent file")
		}
	})
}

func TestFileRepository_List(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	target1 := testutil.MustCreateStorageTarget(t, db, "target1")
	target2 := testutil.MustCreateStorageTarget(t, db, "target2")

	// Create files with different characteristics
	testutil.MustCreateFile(t, db, target1.ID, "/small.txt")
	testutil.MustCreateFile(t, db, target1.ID, "/large.txt")
	testutil.MustCreateFile(t, db, target2.ID, "/other.txt")

	// Create a deleted file
	deletedFile := testutil.MustCreateFile(t, db, target1.ID, "/deleted.txt")
	now := time.Now()
	deletedFile.DeletedAt = &now
	if err := db.Files.Update(context.Background(), deletedFile); err != nil {
		t.Fatalf("failed to mark file as deleted: %v", err)
	}

	t.Run("lists all files", func(t *testing.T) {
		files, err := db.Files.List(context.Background(), database.FileFilters{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(files) != 4 {
			t.Errorf("expected 4 files, got %d", len(files))
		}
	})

	t.Run("filters by storage target", func(t *testing.T) {
		files, err := db.Files.List(context.Background(), database.FileFilters{
			StorageTargetID: &target1.ID,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(files) != 3 {
			t.Errorf("expected 3 files for target1, got %d", len(files))
		}
	})

	t.Run("filters active only", func(t *testing.T) {
		files, err := db.Files.List(context.Background(), database.FileFilters{
			StorageTargetID: &target1.ID,
			ActiveOnly:      true,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(files) != 2 {
			t.Errorf("expected 2 active files, got %d", len(files))
		}
	})

	t.Run("filters deleted only", func(t *testing.T) {
		files, err := db.Files.List(context.Background(), database.FileFilters{
			DeletedOnly: true,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(files) != 1 {
			t.Errorf("expected 1 deleted file, got %d", len(files))
		}
	})

	t.Run("respects limit and offset", func(t *testing.T) {
		files, err := db.Files.List(context.Background(), database.FileFilters{
			Limit:  2,
			Offset: 1,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(files) != 2 {
			t.Errorf("expected 2 files, got %d", len(files))
		}
	})
}

func TestFileRepository_Update(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	target := testutil.MustCreateStorageTarget(t, db, "test-target")
	file := testutil.MustCreateFile(t, db, target.ID, "/test/file.txt")

	t.Run("updates file successfully", func(t *testing.T) {
		file.Size = 2048
		newChecksum := "def456"
		file.CurrentChecksum = &newChecksum

		err := db.Files.Update(context.Background(), file)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify update
		updated, err := db.Files.GetByID(context.Background(), file.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if updated.Size != 2048 {
			t.Errorf("expected size 2048, got %d", updated.Size)
		}

		if *updated.CurrentChecksum != "def456" {
			t.Errorf("expected checksum def456, got %s", *updated.CurrentChecksum)
		}
	})

	t.Run("updates updated_at timestamp", func(t *testing.T) {
		original := file.UpdatedAt

		time.Sleep(10 * time.Millisecond)

		file.Size = 4096
		err := db.Files.Update(context.Background(), file)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		updated, err := db.Files.GetByID(context.Background(), file.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !updated.UpdatedAt.After(original) {
			t.Error("expected updated_at to be updated")
		}
	})
}

func TestFileRepository_CreateBatch(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	target := testutil.MustCreateStorageTarget(t, db, "test-target")

	t.Run("creates multiple files in batch", func(t *testing.T) {
		now := time.Now()
		files := []*database.File{
			{
				StorageTargetID: target.ID,
				Path:            "/batch/file1.txt",
				Size:            1024,
				FirstSeen:       now,
				LastSeen:        now,
			},
			{
				StorageTargetID: target.ID,
				Path:            "/batch/file2.txt",
				Size:            2048,
				FirstSeen:       now,
				LastSeen:        now,
			},
			{
				StorageTargetID: target.ID,
				Path:            "/batch/file3.txt",
				Size:            4096,
				FirstSeen:       now,
				LastSeen:        now,
			},
		}

		err := db.Files.CreateBatch(context.Background(), files)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify all files have IDs
		for i, file := range files {
			if file.ID == 0 {
				t.Errorf("file %d missing ID", i)
			}
		}

		// Verify files in database
		count, err := db.Files.Count(context.Background(), database.FileFilters{
			StorageTargetID: &target.ID,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if count != 3 {
			t.Errorf("expected 3 files, got %d", count)
		}
	})

	t.Run("handles empty batch", func(t *testing.T) {
		err := db.Files.CreateBatch(context.Background(), []*database.File{})
		if err != nil {
		t.Fatalf("unexpected error for empty batch: %v", err)
		}
	})
}

func TestFileRepository_GetUnverifiedFiles(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	target := testutil.MustCreateStorageTarget(t, db, "test-target")

	// Create files with different verification times
	now := time.Now()
	old := now.Add(-30 * 24 * time.Hour)
	recent := now.Add(-1 * time.Hour)

	file1 := testutil.MustCreateFile(t, db, target.ID, "/file1.txt")
	file1.LastChecksummedAt = &old
	db.Files.Update(context.Background(), file1)

	file2 := testutil.MustCreateFile(t, db, target.ID, "/file2.txt")
	file2.LastChecksummedAt = &recent
	db.Files.Update(context.Background(), file2)

	file3 := testutil.MustCreateFile(t, db, target.ID, "/file3.txt")
	file3.LastChecksummedAt = nil
	db.Files.Update(context.Background(), file3)

	t.Run("returns files ordered by verification age", func(t *testing.T) {
		files, err := db.Files.GetUnverifiedFiles(context.Background(), target.ID, 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(files) != 3 {
			t.Errorf("expected 3 files, got %d", len(files))
		}

		// First should be never verified (NULL)
		if files[0].LastChecksummedAt != nil {
			t.Error("expected first file to have NULL last_checksummed_at")
		}

		// Second should be oldest
		if files[1].LastChecksummedAt == nil || !files[1].LastChecksummedAt.Equal(old) {
			t.Error("expected second file to have oldest verification time")
		}
	})

	t.Run("respects limit", func(t *testing.T) {
		files, err := db.Files.GetUnverifiedFiles(context.Background(), target.ID, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(files) != 2 {
			t.Errorf("expected 2 files, got %d", len(files))
		}
	})
}

func TestFileRepository_GetVerificationStats(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	target := testutil.MustCreateStorageTarget(t, db, "test-target")

	now := time.Now()

	// Create files with different verification ages
	file1 := testutil.MustCreateFile(t, db, target.ID, "/file1.txt")
	recent := now.Add(-3 * 24 * time.Hour)
	file1.LastChecksummedAt = &recent
	db.Files.Update(context.Background(), file1)

	file2 := testutil.MustCreateFile(t, db, target.ID, "/file2.txt")
	month := now.Add(-15 * 24 * time.Hour)
	file2.LastChecksummedAt = &month
	db.Files.Update(context.Background(), file2)

	file3 := testutil.MustCreateFile(t, db, target.ID, "/file3.txt")
	old := now.Add(-100 * 24 * time.Hour)
	file3.LastChecksummedAt = &old
	db.Files.Update(context.Background(), file3)

	file4 := testutil.MustCreateFile(t, db, target.ID, "/file4.txt")
	file4.LastChecksummedAt = nil
	db.Files.Update(context.Background(), file4)

	t.Run("calculates verification statistics", func(t *testing.T) {
		stats, err := db.Files.GetVerificationStats(context.Background(), target.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if stats.TotalFiles != 4 {
			t.Errorf("expected 4 total files, got %d", stats.TotalFiles)
		}

		if stats.VerifiedLast7Days != 1 {
			t.Errorf("expected 1 file verified in last 7 days, got %d", stats.VerifiedLast7Days)
		}

		if stats.VerifiedLast30Days != 2 {
			t.Errorf("expected 2 files verified in last 30 days, got %d", stats.VerifiedLast30Days)
		}

		if stats.VerifiedLast90Days != 2 {
			t.Errorf("expected 2 files verified in last 90 days, got %d", stats.VerifiedLast90Days)
		}

		if stats.NeverVerified != 1 {
			t.Errorf("expected 1 never verified file, got %d", stats.NeverVerified)
		}

		if stats.OldestVerification == nil {
			t.Error("expected oldest verification to be set")
		}
	})
}
