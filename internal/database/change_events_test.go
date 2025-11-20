package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/jeffanddom/fixity/internal/database"
	"github.com/jeffanddom/fixity/tests/testutil"
)

func TestChangeEventRepository_Create(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	target := testutil.MustCreateStorageTarget(t, db, "test-target")
	file := testutil.MustCreateFile(t, db, target.ID, "/test/file.txt")
	scan := testutil.MustCreateScan(t, db, target.ID)

	t.Run("creates added event successfully", func(t *testing.T) {
		event := &database.ChangeEvent{
			ScanID:     scan.ID,
			FileID:     file.ID,
			EventType:  database.ChangeEventAdded,
			DetectedAt: time.Now(),
		}

		err := db.ChangeEvents.Create(context.Background(), event)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if event.ID == 0 {
			t.Error("expected ID to be set")
		}

		if event.CreatedAt.IsZero() {
			t.Error("expected CreatedAt to be set")
		}
	})

	t.Run("creates deleted event with checksums", func(t *testing.T) {
		oldChecksum := "abc123"
		oldSize := int64(1024)

		event := &database.ChangeEvent{
			ScanID:      scan.ID,
			FileID:      file.ID,
			EventType:   database.ChangeEventDeleted,
			DetectedAt:  time.Now(),
			OldChecksum: &oldChecksum,
			OldSize:     &oldSize,
		}

		err := db.ChangeEvents.Create(context.Background(), event)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify checksums persisted
		retrieved, err := db.ChangeEvents.GetByID(context.Background(), event.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if retrieved.OldChecksum == nil || *retrieved.OldChecksum != oldChecksum {
			t.Errorf("expected old checksum %s, got %v", oldChecksum, retrieved.OldChecksum)
		}

		if retrieved.OldSize == nil || *retrieved.OldSize != oldSize {
			t.Errorf("expected old size %d, got %v", oldSize, retrieved.OldSize)
		}
	})

	t.Run("creates modified event with before/after data", func(t *testing.T) {
		oldChecksum := "old123"
		newChecksum := "new456"
		oldSize := int64(1024)
		newSize := int64(2048)

		event := &database.ChangeEvent{
			ScanID:      scan.ID,
			FileID:      file.ID,
			EventType:   database.ChangeEventModified,
			DetectedAt:  time.Now(),
			OldChecksum: &oldChecksum,
			NewChecksum: &newChecksum,
			OldSize:     &oldSize,
			NewSize:     &newSize,
		}

		err := db.ChangeEvents.Create(context.Background(), event)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify before/after data
		retrieved, err := db.ChangeEvents.GetByID(context.Background(), event.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if retrieved.OldChecksum == nil || *retrieved.OldChecksum != oldChecksum {
			t.Errorf("expected old checksum %s, got %v", oldChecksum, retrieved.OldChecksum)
		}

		if retrieved.NewChecksum == nil || *retrieved.NewChecksum != newChecksum {
			t.Errorf("expected new checksum %s, got %v", newChecksum, retrieved.NewChecksum)
		}
	})

	t.Run("creates verified event", func(t *testing.T) {
		checksum := "verified123"

		event := &database.ChangeEvent{
			ScanID:      scan.ID,
			FileID:      file.ID,
			EventType:   database.ChangeEventVerified,
			DetectedAt:  time.Now(),
			NewChecksum: &checksum,
		}

		err := db.ChangeEvents.Create(context.Background(), event)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if event.ID == 0 {
			t.Error("expected ID to be set")
		}
	})
}

func TestChangeEventRepository_CreateBatch(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	target := testutil.MustCreateStorageTarget(t, db, "test-target")
	file1 := testutil.MustCreateFile(t, db, target.ID, "/test/file1.txt")
	file2 := testutil.MustCreateFile(t, db, target.ID, "/test/file2.txt")
	file3 := testutil.MustCreateFile(t, db, target.ID, "/test/file3.txt")
	scan := testutil.MustCreateScan(t, db, target.ID)

	t.Run("creates multiple events in batch", func(t *testing.T) {
		events := []*database.ChangeEvent{
			{
				ScanID:     scan.ID,
				FileID:     file1.ID,
				EventType:  database.ChangeEventAdded,
				DetectedAt: time.Now(),
			},
			{
				ScanID:     scan.ID,
				FileID:     file2.ID,
				EventType:  database.ChangeEventModified,
				DetectedAt: time.Now(),
			},
			{
				ScanID:     scan.ID,
				FileID:     file3.ID,
				EventType:  database.ChangeEventVerified,
				DetectedAt: time.Now(),
			},
		}

		err := db.ChangeEvents.CreateBatch(context.Background(), events)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify all events got IDs
		for i, event := range events {
			if event.ID == 0 {
				t.Errorf("event %d: expected ID to be set", i)
			}
			if event.CreatedAt.IsZero() {
				t.Errorf("event %d: expected CreatedAt to be set", i)
			}
		}

		// Verify all events were persisted
		retrieved, err := db.ChangeEvents.GetByScan(context.Background(), scan.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(retrieved) != 3 {
			t.Errorf("expected 3 events, got %d", len(retrieved))
		}
	})

	t.Run("handles empty batch", func(t *testing.T) {
		events := []*database.ChangeEvent{}

		err := db.ChangeEvents.CreateBatch(context.Background(), events)
		if err != nil {
			t.Fatalf("unexpected error for empty batch: %v", err)
		}
	})

	t.Run("rolls back on error", func(t *testing.T) {
		events := []*database.ChangeEvent{
			{
				ScanID:     scan.ID,
				FileID:     file1.ID,
				EventType:  database.ChangeEventAdded,
				DetectedAt: time.Now(),
			},
			{
				ScanID:     999999, // Invalid scan ID
				FileID:     file2.ID,
				EventType:  database.ChangeEventAdded,
				DetectedAt: time.Now(),
			},
		}

		initialCount := len(testutil.MustGetAllChangeEvents(t, db, scan.ID))

		err := db.ChangeEvents.CreateBatch(context.Background(), events)
		if err == nil {
			t.Error("expected error for invalid scan ID")
		}

		// Verify rollback - no events should be created
		finalCount := len(testutil.MustGetAllChangeEvents(t, db, scan.ID))
		if finalCount != initialCount {
			t.Errorf("expected count to remain %d after rollback, got %d", initialCount, finalCount)
		}
	})
}

func TestChangeEventRepository_GetByID(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	target := testutil.MustCreateStorageTarget(t, db, "test-target")
	file := testutil.MustCreateFile(t, db, target.ID, "/test/file.txt")
	scan := testutil.MustCreateScan(t, db, target.ID)
	event := testutil.MustCreateChangeEvent(t, db, scan.ID, file.ID, database.ChangeEventAdded)

	t.Run("retrieves event by ID", func(t *testing.T) {
		retrieved, err := db.ChangeEvents.GetByID(context.Background(), event.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if retrieved.ScanID != scan.ID {
			t.Errorf("expected scan ID %d, got %d", scan.ID, retrieved.ScanID)
		}

		if retrieved.FileID != file.ID {
			t.Errorf("expected file ID %d, got %d", file.ID, retrieved.FileID)
		}

		if retrieved.EventType != database.ChangeEventAdded {
			t.Errorf("expected event type added, got %s", retrieved.EventType)
		}
	})

	t.Run("returns error for non-existent ID", func(t *testing.T) {
		_, err := db.ChangeEvents.GetByID(context.Background(), 999999)
		if err == nil {
			t.Error("expected error for non-existent ID")
		}
	})
}

func TestChangeEventRepository_List(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	target := testutil.MustCreateStorageTarget(t, db, "test-target")
	file1 := testutil.MustCreateFile(t, db, target.ID, "/test/file1.txt")
	file2 := testutil.MustCreateFile(t, db, target.ID, "/test/file2.txt")
	scan1 := testutil.MustCreateScan(t, db, target.ID)
	scan2 := testutil.MustCreateScan(t, db, target.ID)

	// Create events for different scans and files
	testutil.MustCreateChangeEvent(t, db, scan1.ID, file1.ID, database.ChangeEventAdded)
	testutil.MustCreateChangeEvent(t, db, scan1.ID, file2.ID, database.ChangeEventModified)
	testutil.MustCreateChangeEvent(t, db, scan2.ID, file1.ID, database.ChangeEventVerified)
	testutil.MustCreateChangeEvent(t, db, scan2.ID, file2.ID, database.ChangeEventDeleted)

	t.Run("lists all events without filters", func(t *testing.T) {
		events, err := db.ChangeEvents.List(context.Background(), database.ChangeEventFilters{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(events) != 4 {
			t.Errorf("expected 4 events, got %d", len(events))
		}
	})

	t.Run("filters by scan ID", func(t *testing.T) {
		events, err := db.ChangeEvents.List(context.Background(), database.ChangeEventFilters{
			ScanID: &scan1.ID,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(events) != 2 {
			t.Errorf("expected 2 events for scan1, got %d", len(events))
		}

		for _, event := range events {
			if event.ScanID != scan1.ID {
				t.Errorf("expected scan ID %d, got %d", scan1.ID, event.ScanID)
			}
		}
	})

	t.Run("filters by file ID", func(t *testing.T) {
		events, err := db.ChangeEvents.List(context.Background(), database.ChangeEventFilters{
			FileID: &file1.ID,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(events) != 2 {
			t.Errorf("expected 2 events for file1, got %d", len(events))
		}

		for _, event := range events {
			if event.FileID != file1.ID {
				t.Errorf("expected file ID %d, got %d", file1.ID, event.FileID)
			}
		}
	})

	t.Run("filters by event types", func(t *testing.T) {
		events, err := db.ChangeEvents.List(context.Background(), database.ChangeEventFilters{
			EventTypes: []database.ChangeEventType{
				database.ChangeEventAdded,
				database.ChangeEventDeleted,
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(events) != 2 {
			t.Errorf("expected 2 events (added/deleted), got %d", len(events))
		}

		for _, event := range events {
			if event.EventType != database.ChangeEventAdded && event.EventType != database.ChangeEventDeleted {
				t.Errorf("unexpected event type: %s", event.EventType)
			}
		}
	})

	t.Run("combines multiple filters", func(t *testing.T) {
		events, err := db.ChangeEvents.List(context.Background(), database.ChangeEventFilters{
			ScanID: &scan1.ID,
			EventTypes: []database.ChangeEventType{
				database.ChangeEventAdded,
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(events) != 1 {
			t.Errorf("expected 1 event, got %d", len(events))
		}

		if len(events) > 0 {
			if events[0].ScanID != scan1.ID {
				t.Errorf("expected scan ID %d, got %d", scan1.ID, events[0].ScanID)
			}
			if events[0].EventType != database.ChangeEventAdded {
				t.Errorf("expected event type added, got %s", events[0].EventType)
			}
		}
	})

	t.Run("respects limit", func(t *testing.T) {
		events, err := db.ChangeEvents.List(context.Background(), database.ChangeEventFilters{
			Limit: 2,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(events) != 2 {
			t.Errorf("expected 2 events, got %d", len(events))
		}
	})

	t.Run("respects offset", func(t *testing.T) {
		// Get all events ordered by detected_at DESC
		allEvents, err := db.ChangeEvents.List(context.Background(), database.ChangeEventFilters{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Get events with offset
		offsetEvents, err := db.ChangeEvents.List(context.Background(), database.ChangeEventFilters{
			Offset: 2,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(offsetEvents) != 2 {
			t.Errorf("expected 2 events after offset, got %d", len(offsetEvents))
		}

		// Verify offset skipped first 2 events
		if len(allEvents) >= 3 && len(offsetEvents) >= 1 {
			if offsetEvents[0].ID != allEvents[2].ID {
				t.Error("offset did not skip correct events")
			}
		}
	})

	t.Run("orders by detected_at DESC", func(t *testing.T) {
		events, err := db.ChangeEvents.List(context.Background(), database.ChangeEventFilters{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify descending order
		for i := 0; i < len(events)-1; i++ {
			if events[i].DetectedAt.Before(events[i+1].DetectedAt) {
				t.Error("events not ordered by detected_at DESC")
			}
		}
	})
}

func TestChangeEventRepository_GetByScan(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	target := testutil.MustCreateStorageTarget(t, db, "test-target")
	file1 := testutil.MustCreateFile(t, db, target.ID, "/test/file1.txt")
	file2 := testutil.MustCreateFile(t, db, target.ID, "/test/file2.txt")
	scan1 := testutil.MustCreateScan(t, db, target.ID)
	scan2 := testutil.MustCreateScan(t, db, target.ID)

	testutil.MustCreateChangeEvent(t, db, scan1.ID, file1.ID, database.ChangeEventAdded)
	testutil.MustCreateChangeEvent(t, db, scan1.ID, file2.ID, database.ChangeEventModified)
	testutil.MustCreateChangeEvent(t, db, scan2.ID, file1.ID, database.ChangeEventVerified)

	t.Run("retrieves all events for a scan", func(t *testing.T) {
		events, err := db.ChangeEvents.GetByScan(context.Background(), scan1.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(events) != 2 {
			t.Errorf("expected 2 events for scan1, got %d", len(events))
		}

		for _, event := range events {
			if event.ScanID != scan1.ID {
				t.Errorf("expected scan ID %d, got %d", scan1.ID, event.ScanID)
			}
		}
	})

	t.Run("returns empty list for scan with no events", func(t *testing.T) {
		scan3 := testutil.MustCreateScan(t, db, target.ID)

		events, err := db.ChangeEvents.GetByScan(context.Background(), scan3.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(events) != 0 {
			t.Errorf("expected 0 events for scan3, got %d", len(events))
		}
	})
}

func TestChangeEventRepository_GetByFile(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	target := testutil.MustCreateStorageTarget(t, db, "test-target")
	file1 := testutil.MustCreateFile(t, db, target.ID, "/test/file1.txt")
	file2 := testutil.MustCreateFile(t, db, target.ID, "/test/file2.txt")
	scan1 := testutil.MustCreateScan(t, db, target.ID)
	scan2 := testutil.MustCreateScan(t, db, target.ID)

	// Create file lifecycle: added, verified, modified
	testutil.MustCreateChangeEvent(t, db, scan1.ID, file1.ID, database.ChangeEventAdded)
	time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	testutil.MustCreateChangeEvent(t, db, scan1.ID, file1.ID, database.ChangeEventVerified)
	time.Sleep(10 * time.Millisecond)
	testutil.MustCreateChangeEvent(t, db, scan2.ID, file1.ID, database.ChangeEventModified)
	testutil.MustCreateChangeEvent(t, db, scan2.ID, file2.ID, database.ChangeEventAdded)

	t.Run("retrieves complete file lifecycle history", func(t *testing.T) {
		events, err := db.ChangeEvents.GetByFile(context.Background(), file1.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(events) != 3 {
			t.Errorf("expected 3 lifecycle events for file1, got %d", len(events))
		}

		for _, event := range events {
			if event.FileID != file1.ID {
				t.Errorf("expected file ID %d, got %d", file1.ID, event.FileID)
			}
		}

		// Verify chronological order (DESC)
		expectedTypes := []database.ChangeEventType{
			database.ChangeEventModified,
			database.ChangeEventVerified,
			database.ChangeEventAdded,
		}

		for i, event := range events {
			if event.EventType != expectedTypes[i] {
				t.Errorf("event %d: expected type %s, got %s", i, expectedTypes[i], event.EventType)
			}
		}
	})

	t.Run("returns empty list for file with no events", func(t *testing.T) {
		file3 := testutil.MustCreateFile(t, db, target.ID, "/test/file3.txt")

		events, err := db.ChangeEvents.GetByFile(context.Background(), file3.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(events) != 0 {
			t.Errorf("expected 0 events for file3, got %d", len(events))
		}
	})
}

func TestChangeEventRepository_CountByType(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	target := testutil.MustCreateStorageTarget(t, db, "test-target")
	file1 := testutil.MustCreateFile(t, db, target.ID, "/test/file1.txt")
	file2 := testutil.MustCreateFile(t, db, target.ID, "/test/file2.txt")
	file3 := testutil.MustCreateFile(t, db, target.ID, "/test/file3.txt")
	file4 := testutil.MustCreateFile(t, db, target.ID, "/test/file4.txt")
	scan := testutil.MustCreateScan(t, db, target.ID)

	// Create events with different types
	testutil.MustCreateChangeEvent(t, db, scan.ID, file1.ID, database.ChangeEventAdded)
	testutil.MustCreateChangeEvent(t, db, scan.ID, file2.ID, database.ChangeEventAdded)
	testutil.MustCreateChangeEvent(t, db, scan.ID, file3.ID, database.ChangeEventModified)
	testutil.MustCreateChangeEvent(t, db, scan.ID, file4.ID, database.ChangeEventVerified)
	testutil.MustCreateChangeEvent(t, db, scan.ID, file1.ID, database.ChangeEventVerified)

	t.Run("counts events by type", func(t *testing.T) {
		counts, err := db.ChangeEvents.CountByType(context.Background(), scan.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expectedCounts := map[database.ChangeEventType]int64{
			database.ChangeEventAdded:    2,
			database.ChangeEventModified: 1,
			database.ChangeEventVerified: 2,
		}

		if len(counts) != len(expectedCounts) {
			t.Errorf("expected %d event types, got %d", len(expectedCounts), len(counts))
		}

		for eventType, expectedCount := range expectedCounts {
			if counts[eventType] != expectedCount {
				t.Errorf("event type %s: expected count %d, got %d", eventType, expectedCount, counts[eventType])
			}
		}
	})

	t.Run("returns empty map for scan with no events", func(t *testing.T) {
		scan2 := testutil.MustCreateScan(t, db, target.ID)

		counts, err := db.ChangeEvents.CountByType(context.Background(), scan2.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(counts) != 0 {
			t.Errorf("expected empty map for scan with no events, got %d types", len(counts))
		}
	})

	t.Run("counts only for specified scan", func(t *testing.T) {
		scan2 := testutil.MustCreateScan(t, db, target.ID)
		testutil.MustCreateChangeEvent(t, db, scan2.ID, file1.ID, database.ChangeEventDeleted)

		counts, err := db.ChangeEvents.CountByType(context.Background(), scan2.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(counts) != 1 {
			t.Errorf("expected 1 event type for scan2, got %d", len(counts))
		}

		if counts[database.ChangeEventDeleted] != 1 {
			t.Errorf("expected 1 deleted event, got %d", counts[database.ChangeEventDeleted])
		}
	})
}
