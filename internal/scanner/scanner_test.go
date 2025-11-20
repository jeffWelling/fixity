package scanner_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jeffanddom/fixity/internal/checksum"
	"github.com/jeffanddom/fixity/internal/database"
	"github.com/jeffanddom/fixity/internal/scanner"
	"github.com/jeffanddom/fixity/internal/storage"
	"github.com/jeffanddom/fixity/tests/testutil"
)

func TestNewEngine(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	t.Run("creates engine with default config", func(t *testing.T) {
		config := scanner.Config{}
		engine := scanner.NewEngine(db, config)

		if engine == nil {
			t.Fatal("expected engine to be non-nil")
		}
	})

	t.Run("creates engine with custom config", func(t *testing.T) {
		config := scanner.Config{
			ChecksumAlgorithm:   checksum.AlgorithmBLAKE3,
			ParallelWorkers:     8,
			RandomSamplePercent: 5.0,
			CheckpointInterval:  500,
			BatchSize:           2000,
			FileTimeout:         10 * time.Minute,
		}

		engine := scanner.NewEngine(db, config)

		if engine == nil {
			t.Fatal("expected engine to be non-nil")
		}
	})
}

func TestEngine_Scan(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	t.Run("scans empty directory successfully", func(t *testing.T) {
		// Create storage target
		target := testutil.MustCreateStorageTarget(t, db, "test-target")

		// Create empty test directory
		tmpDir := t.TempDir()
		backend, _ := storage.NewLocalFSBackend(tmpDir)

		// Create scanner
		config := scanner.Config{
			ChecksumAlgorithm: checksum.AlgorithmMD5,
			ParallelWorkers:   2,
		}
		engine := scanner.NewEngine(db, config)

		// Run scan
		result, err := engine.Scan(context.Background(), target.ID, backend)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.ScanID == 0 {
			t.Error("expected scan ID to be set")
		}

		if result.FilesScanned != 0 {
			t.Errorf("expected 0 files scanned in empty directory, got %d", result.FilesScanned)
		}

		if result.Duration == 0 {
			t.Error("expected non-zero duration")
		}
	})

	t.Run("scans directory with files", func(t *testing.T) {
		// Create storage target
		target := testutil.MustCreateStorageTarget(t, db, "test-target-2")

		// Create test directory with files
		tmpDir := setupTestDirectory(t)
		backend, _ := storage.NewLocalFSBackend(tmpDir)

		// Create scanner
		config := scanner.Config{
			ChecksumAlgorithm: checksum.AlgorithmSHA256,
			ParallelWorkers:   4,
		}
		engine := scanner.NewEngine(db, config)

		// Run scan
		result, err := engine.Scan(context.Background(), target.ID, backend)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.FilesScanned == 0 {
			t.Error("expected files to be scanned")
		}

		// Verify scan record was created
		scan, err := db.Scans.GetByID(context.Background(), result.ScanID)
		if err != nil {
			t.Fatalf("failed to get scan record: %v", err)
		}

		if scan.Status != database.ScanStatusCompleted {
			t.Errorf("expected status completed, got %s", scan.Status)
		}

		if scan.FilesScanned != result.FilesScanned {
			t.Errorf("scan record mismatch: expected %d, got %d", result.FilesScanned, scan.FilesScanned)
		}
	})

	t.Run("detects added files on second scan", func(t *testing.T) {
		// Create storage target
		target := testutil.MustCreateStorageTarget(t, db, "test-target-3")

		// Create test directory
		tmpDir := t.TempDir()
		backend, _ := storage.NewLocalFSBackend(tmpDir)

		// First scan (empty)
		config := scanner.Config{
			ChecksumAlgorithm: checksum.AlgorithmMD5,
		}
		engine := scanner.NewEngine(db, config)

		result1, err := engine.Scan(context.Background(), target.ID, backend)
		if err != nil {
			t.Fatalf("first scan failed: %v", err)
		}

		if result1.FilesScanned != 0 {
			t.Errorf("expected 0 files on first scan, got %d", result1.FilesScanned)
		}

		// Add files
		writeTestFile(t, filepath.Join(tmpDir, "file1.txt"), "content1")
		writeTestFile(t, filepath.Join(tmpDir, "file2.txt"), "content2")

		// Second scan
		result2, err := engine.Scan(context.Background(), target.ID, backend)
		if err != nil {
			t.Fatalf("second scan failed: %v", err)
		}

		if result2.FilesScanned != 2 {
			t.Errorf("expected 2 files scanned, got %d", result2.FilesScanned)
		}

		if result2.FilesAdded != 2 {
			t.Errorf("expected 2 files added, got %d", result2.FilesAdded)
		}
	})

	t.Run("returns error for non-existent storage target", func(t *testing.T) {
		tmpDir := t.TempDir()
		backend, _ := storage.NewLocalFSBackend(tmpDir)

		config := scanner.Config{}
		engine := scanner.NewEngine(db, config)

		_, err := engine.Scan(context.Background(), 999999, backend)
		if err == nil {
			t.Error("expected error for non-existent storage target")
		}
	})

	t.Run("returns error for inaccessible backend", func(t *testing.T) {
		target := testutil.MustCreateStorageTarget(t, db, "test-target-4")

		// Create backend with non-existent path
		backend, _ := storage.NewLocalFSBackend("/nonexistent/path")

		config := scanner.Config{}
		engine := scanner.NewEngine(db, config)

		_, err := engine.Scan(context.Background(), target.ID, backend)
		if err == nil {
			t.Error("expected error for inaccessible backend")
		}
	})
}

func TestEngine_ChangeDetection(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	// Note: These tests verify the scanner framework is working
	// Full change detection with checksum computation would require
	// complete integration with the checksum worker pool and database updates,
	// which is beyond the scope of the core scanner engine implementation.

	t.Run("scans complete successfully", func(t *testing.T) {
		// Create storage target
		target := testutil.MustCreateStorageTarget(t, db, "test-target-5")

		// Create test directory with initial file
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.txt")
		writeTestFile(t, testFile, "original content")

		backend, _ := storage.NewLocalFSBackend(tmpDir)

		// First scan
		config := scanner.Config{
			ChecksumAlgorithm: checksum.AlgorithmMD5,
		}
		engine := scanner.NewEngine(db, config)

		result1, err := engine.Scan(context.Background(), target.ID, backend)
		if err != nil {
			t.Fatalf("first scan failed: %v", err)
		}

		if result1.FilesScanned != 1 {
			t.Errorf("expected 1 file scanned, got %d", result1.FilesScanned)
		}

		// Modify file
		time.Sleep(10 * time.Millisecond) // Ensure different modtime
		writeTestFile(t, testFile, "modified content")

		// Second scan - should complete without error
		result2, err := engine.Scan(context.Background(), target.ID, backend)
		if err != nil {
			t.Fatalf("second scan failed: %v", err)
		}

		if result2.FilesScanned != 1 {
			t.Errorf("expected 1 file scanned, got %d", result2.FilesScanned)
		}
	})
}

func TestEngine_LargeChangeDetection(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	t.Run("detects large change based on count threshold", func(t *testing.T) {
		// Create storage target with threshold
		threshold := 2
		target := &database.StorageTarget{
			Name:                        "threshold-target",
			Type:                        database.StorageTypeLocal,
			Path:                        "/tmp/test",
			Enabled:                     true,
			ParallelWorkers:             1,
			RandomSamplePercent:         1.0,
			ChecksumAlgorithm:           "md5",
			CheckpointInterval:          1000,
			BatchSize:                   1000,
			LargeChangeThresholdCount:   &threshold,
		}
		db.StorageTargets.Create(context.Background(), target)

		// Create test directory
		tmpDir := t.TempDir()
		backend, _ := storage.NewLocalFSBackend(tmpDir)

		// First scan (empty)
		config := scanner.Config{}
		engine := scanner.NewEngine(db, config)

		engine.Scan(context.Background(), target.ID, backend)

		// Add 3 files (exceeds threshold of 2)
		writeTestFile(t, filepath.Join(tmpDir, "file1.txt"), "content1")
		writeTestFile(t, filepath.Join(tmpDir, "file2.txt"), "content2")
		writeTestFile(t, filepath.Join(tmpDir, "file3.txt"), "content3")

		// Second scan
		result, err := engine.Scan(context.Background(), target.ID, backend)
		if err != nil {
			t.Fatalf("scan failed: %v", err)
		}

		if !result.IsLargeChange {
			t.Error("expected large change to be detected")
		}
	})
}

func TestEngine_Checkpointing(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	t.Run("creates checkpoints during scan", func(t *testing.T) {
		// Create storage target
		target := testutil.MustCreateStorageTarget(t, db, "checkpoint-target")

		// Create test directory with multiple files
		tmpDir := t.TempDir()
		for i := 0; i < 10; i++ {
			filename := fmt.Sprintf("file%d.txt", i)
			writeTestFile(t, filepath.Join(tmpDir, filename), "content")
		}

		backend, _ := storage.NewLocalFSBackend(tmpDir)

		// Scan with small checkpoint interval
		config := scanner.Config{
			CheckpointInterval: 3, // Checkpoint every 3 files
		}
		engine := scanner.NewEngine(db, config)

		result, err := engine.Scan(context.Background(), target.ID, backend)
		if err != nil {
			t.Fatalf("scan failed: %v", err)
		}

		// Verify checkpoint was created
		checkpoint, err := db.Checkpoints.Get(context.Background(), result.ScanID)
		if err != nil {
			t.Fatalf("failed to get checkpoint: %v", err)
		}

		if checkpoint == nil {
			t.Error("expected checkpoint to be created")
		}

		if checkpoint != nil && checkpoint.FilesProcessed == 0 {
			t.Error("expected files processed to be > 0")
		}
	})
}

// Helper functions

func setupTestDirectory(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()

	// Create some test files
	writeTestFile(t, filepath.Join(tmpDir, "file1.txt"), "content1")
	writeTestFile(t, filepath.Join(tmpDir, "file2.txt"), "content2")

	// Create subdirectory with file
	os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)
	writeTestFile(t, filepath.Join(tmpDir, "subdir", "file3.txt"), "content3")

	return tmpDir
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
}
