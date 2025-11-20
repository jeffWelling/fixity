package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/jeffanddom/fixity/internal/database"
	"github.com/jeffanddom/fixity/tests/testutil"
)

func TestScanRepository_Create(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	target := testutil.MustCreateStorageTarget(t, db, "test-target")

	t.Run("creates scan successfully", func(t *testing.T) {
		scan := &database.Scan{
			StorageTargetID: target.ID,
			Status:          database.ScanStatusRunning,
			StartedAt:       time.Now(),
		}

		err := db.Scans.Create(context.Background(), scan)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if scan.ID == 0 {
			t.Error("expected ID to be set")
		}

		if scan.CreatedAt.IsZero() {
			t.Error("expected CreatedAt to be set")
		}
	})

	t.Run("creates scan with counters", func(t *testing.T) {
		now := time.Now()
		completed := now

		scan := &database.Scan{
			StorageTargetID: target.ID,
			Status:          database.ScanStatusCompleted,
			StartedAt:       now,
			CompletedAt:     &completed,
			FilesScanned:    1000,
			FilesAdded:      10,
			FilesDeleted:    5,
			FilesModified:   3,
			FilesVerified:   982,
		}

		err := db.Scans.Create(context.Background(), scan)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify
		retrieved, err := db.Scans.GetByID(context.Background(), scan.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if retrieved.FilesScanned != 1000 {
			t.Errorf("expected 1000 files scanned, got %d", retrieved.FilesScanned)
		}

		if retrieved.FilesAdded != 10 {
			t.Errorf("expected 10 files added, got %d", retrieved.FilesAdded)
		}
	})
}

func TestScanRepository_GetByID(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	target := testutil.MustCreateStorageTarget(t, db, "test-target")
	scan := testutil.MustCreateScan(t, db, target.ID)

	t.Run("retrieves scan by ID", func(t *testing.T) {
		retrieved, err := db.Scans.GetByID(context.Background(), scan.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if retrieved.StorageTargetID != target.ID {
			t.Errorf("expected target ID %d, got %d", target.ID, retrieved.StorageTargetID)
		}

		if retrieved.Status != database.ScanStatusRunning {
			t.Errorf("expected status running, got %s", retrieved.Status)
		}
	})

	t.Run("returns error for non-existent ID", func(t *testing.T) {
		_, err := db.Scans.GetByID(context.Background(), 999999)
		if err == nil {
			t.Error("expected error for non-existent ID")
		}
	})
}

func TestScanRepository_List(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	target1 := testutil.MustCreateStorageTarget(t, db, "target1")
	target2 := testutil.MustCreateStorageTarget(t, db, "target2")

	// Create scans with different characteristics
	scan1 := testutil.MustCreateScan(t, db, target1.ID)
	scan1.Status = database.ScanStatusCompleted
	scan1.IsLargeChange = true
	now := time.Now()
	scan1.CompletedAt = &now
	db.Scans.Update(context.Background(), scan1)

	testutil.MustCreateScan(t, db, target1.ID)
	testutil.MustCreateScan(t, db, target2.ID)

	t.Run("lists all scans", func(t *testing.T) {
		scans, err := db.Scans.List(context.Background(), database.ScanFilters{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(scans) != 3 {
			t.Errorf("expected 3 scans, got %d", len(scans))
		}
	})

	t.Run("filters by storage target", func(t *testing.T) {
		scans, err := db.Scans.List(context.Background(), database.ScanFilters{
			StorageTargetID: &target1.ID,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(scans) != 2 {
			t.Errorf("expected 2 scans for target1, got %d", len(scans))
		}
	})

	t.Run("filters by status", func(t *testing.T) {
		status := database.ScanStatusRunning
		scans, err := db.Scans.List(context.Background(), database.ScanFilters{
			Status: &status,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(scans) != 2 {
			t.Errorf("expected 2 running scans, got %d", len(scans))
		}
	})

	t.Run("filters large changes only", func(t *testing.T) {
		scans, err := db.Scans.List(context.Background(), database.ScanFilters{
			LargeChangeOnly: true,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(scans) != 1 {
			t.Errorf("expected 1 large change scan, got %d", len(scans))
		}
	})

	t.Run("respects limit and offset", func(t *testing.T) {
		scans, err := db.Scans.List(context.Background(), database.ScanFilters{
			Limit:  2,
			Offset: 1,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(scans) != 2 {
			t.Errorf("expected 2 scans, got %d", len(scans))
		}
	})
}

func TestScanRepository_GetLatest(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	target := testutil.MustCreateStorageTarget(t, db, "test-target")

	t.Run("returns nil when no scans exist", func(t *testing.T) {
		scan, err := db.Scans.GetLatest(context.Background(), target.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if scan != nil {
			t.Error("expected nil for no scans")
		}
	})

	t.Run("returns latest scan", func(t *testing.T) {
		// Create older scan
		older := testutil.MustCreateScan(t, db, target.ID)
		time.Sleep(10 * time.Millisecond)

		// Create newer scan
		newer := testutil.MustCreateScan(t, db, target.ID)

		latest, err := db.Scans.GetLatest(context.Background(), target.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if latest.ID != newer.ID {
			t.Errorf("expected latest scan ID %d, got %d", newer.ID, latest.ID)
		}

		if !latest.StartedAt.After(older.StartedAt) {
			t.Error("expected latest scan to have later start time")
		}
	})
}

func TestScanRepository_Update(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	target := testutil.MustCreateStorageTarget(t, db, "test-target")
	scan := testutil.MustCreateScan(t, db, target.ID)

	t.Run("updates scan successfully", func(t *testing.T) {
		scan.Status = database.ScanStatusCompleted
		scan.FilesScanned = 100
		scan.FilesAdded = 10
		now := time.Now()
		scan.CompletedAt = &now

		err := db.Scans.Update(context.Background(), scan)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify
		updated, err := db.Scans.GetByID(context.Background(), scan.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if updated.Status != database.ScanStatusCompleted {
			t.Errorf("expected status completed, got %s", updated.Status)
		}

		if updated.FilesScanned != 100 {
			t.Errorf("expected 100 files scanned, got %d", updated.FilesScanned)
		}

		if updated.CompletedAt == nil {
			t.Error("expected completed_at to be set")
		}
	})

	t.Run("returns error for non-existent scan", func(t *testing.T) {
		nonExistent := &database.Scan{
			ID:     999999,
			Status: database.ScanStatusCompleted,
		}

		err := db.Scans.Update(context.Background(), nonExistent)
		if err == nil {
			t.Error("expected error for non-existent scan")
		}
	})
}

func TestScanRepository_AddError(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	target := testutil.MustCreateStorageTarget(t, db, "test-target")
	scan := testutil.MustCreateScan(t, db, target.ID)

	t.Run("adds error to scan", func(t *testing.T) {
		err := db.Scans.AddError(context.Background(), scan.ID, "test error 1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		err = db.Scans.AddError(context.Background(), scan.ID, "test error 2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify
		updated, err := db.Scans.GetByID(context.Background(), scan.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if updated.ErrorsCount != 2 {
			t.Errorf("expected 2 errors, got %d", updated.ErrorsCount)
		}

		if len(updated.ErrorMessages) != 2 {
			t.Errorf("expected 2 error messages, got %d", len(updated.ErrorMessages))
		}

		if updated.ErrorMessages[0] != "test error 1" {
			t.Errorf("expected first error message 'test error 1', got '%s'", updated.ErrorMessages[0])
		}
	})
}

func TestScanRepository_GetIncomplete(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	target := testutil.MustCreateStorageTarget(t, db, "test-target")

	t.Run("returns only running scans", func(t *testing.T) {
		// Create running scan
		running1 := testutil.MustCreateScan(t, db, target.ID)

		// Create completed scan
		completed := testutil.MustCreateScan(t, db, target.ID)
		completed.Status = database.ScanStatusCompleted
		now := time.Now()
		completed.CompletedAt = &now
		db.Scans.Update(context.Background(), completed)

		// Create another running scan
		running2 := testutil.MustCreateScan(t, db, target.ID)

		// Get incomplete
		incomplete, err := db.Scans.GetIncomplete(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(incomplete) != 2 {
			t.Errorf("expected 2 incomplete scans, got %d", len(incomplete))
		}

		// Verify they're the running ones
		found1, found2 := false, false
		for _, scan := range incomplete {
			if scan.ID == running1.ID {
				found1 = true
			}
			if scan.ID == running2.ID {
				found2 = true
			}
		}

		if !found1 || !found2 {
			t.Error("expected to find both running scans")
		}
	})

	t.Run("returns empty when no incomplete scans", func(t *testing.T) {
		testutil.CleanupDB(t, db)

		incomplete, err := db.Scans.GetIncomplete(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(incomplete) != 0 {
			t.Errorf("expected 0 incomplete scans, got %d", len(incomplete))
		}
	})
}
