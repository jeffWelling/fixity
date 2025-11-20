package storage_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jeffanddom/fixity/internal/coordinator"
	"github.com/jeffanddom/fixity/internal/database"
	"github.com/jeffanddom/fixity/internal/scanner"
	"github.com/jeffanddom/fixity/internal/storage"
	"github.com/jeffanddom/fixity/tests/testutil"
)

// IntegrationTestCase defines a backend configuration for integration testing
type IntegrationTestCase struct {
	Name       string
	BackendType database.StorageType
	SetupTarget func(t *testing.T, db *database.Database) (*database.StorageTarget, string, func())
}

// GetAllIntegrationTestCases returns integration test cases for all backend types
func GetAllIntegrationTestCases(t *testing.T, db *database.Database) []IntegrationTestCase {
	return []IntegrationTestCase{
		{
			Name:       "Local",
			BackendType: database.StorageTypeLocal,
			SetupTarget: func(t *testing.T, db *database.Database) (*database.StorageTarget, string, func()) {
				// Create test directory with files
				testDir, err := os.MkdirTemp("", "fixity-integration-local-*")
				if err != nil {
					t.Fatalf("failed to create temp dir: %v", err)
				}

				// Create test files
				files := map[string]string{
					"file1.txt":        "content1",
					"file2.txt":        "content2",
					"subdir/file3.txt": "content3",
				}
				for path, content := range files {
					fullPath := filepath.Join(testDir, path)
					dir := filepath.Dir(fullPath)
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatalf("failed to create dir: %v", err)
					}
					if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
						t.Fatalf("failed to write file: %v", err)
					}
				}

				// Create storage target
				target := &database.StorageTarget{
					Name:                "Local Test Target",
					Type:                database.StorageTypeLocal,
					Path:                testDir,
					Enabled:             true,
					ParallelWorkers:     1,
					RandomSamplePercent: 1.0,
					ChecksumAlgorithm:   "md5",
					CheckpointInterval:  1000,
					BatchSize:           1000,
				}

				if err := db.StorageTargets.Create(context.Background(), target); err != nil {
					t.Fatalf("failed to create target: %v", err)
				}

				cleanup := func() {
					os.RemoveAll(testDir)
					db.StorageTargets.Delete(context.Background(), target.ID)
				}

				return target, testDir, cleanup
			},
		},
		{
			Name:       "NFS",
			BackendType: database.StorageTypeNFS,
			SetupTarget: func(t *testing.T, db *database.Database) (*database.StorageTarget, string, func()) {
				// For testing, we simulate an NFS mount using a local directory
				testDir, err := os.MkdirTemp("", "fixity-integration-nfs-*")
				if err != nil {
					t.Fatalf("failed to create temp dir: %v", err)
				}

				// Create test files
				files := map[string]string{
					"nfs-file1.txt":        "nfs-content1",
					"nfs-file2.txt":        "nfs-content2",
					"nfs-subdir/file3.txt": "nfs-content3",
				}
				for path, content := range files {
					fullPath := filepath.Join(testDir, path)
					dir := filepath.Dir(fullPath)
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatalf("failed to create dir: %v", err)
					}
					if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
						t.Fatalf("failed to write file: %v", err)
					}
				}

				// Create storage target with NFS configuration
				server := "nfs-server.example.com"
				share := "/exports/test"
				target := &database.StorageTarget{
					Name:                "NFS Test Target",
					Type:                database.StorageTypeNFS,
					Path:                testDir, // Mount point
					Server:              &server,
					Share:               &share,
					Enabled:             true,
					ParallelWorkers:     1,
					RandomSamplePercent: 1.0,
					ChecksumAlgorithm:   "md5",
					CheckpointInterval:  1000,
					BatchSize:           1000,
				}

				if err := db.StorageTargets.Create(context.Background(), target); err != nil {
					t.Fatalf("failed to create target: %v", err)
				}

				cleanup := func() {
					os.RemoveAll(testDir)
					db.StorageTargets.Delete(context.Background(), target.ID)
				}

				return target, testDir, cleanup
			},
		},
		{
			Name:       "SMB",
			BackendType: database.StorageTypeSMB,
			SetupTarget: func(t *testing.T, db *database.Database) (*database.StorageTarget, string, func()) {
				// For testing, we simulate an SMB mount using a local directory
				testDir, err := os.MkdirTemp("", "fixity-integration-smb-*")
				if err != nil {
					t.Fatalf("failed to create temp dir: %v", err)
				}

				// Create test files
				files := map[string]string{
					"smb-file1.txt":        "smb-content1",
					"smb-file2.txt":        "smb-content2",
					"smb-subdir/file3.txt": "smb-content3",
				}
				for path, content := range files {
					fullPath := filepath.Join(testDir, path)
					dir := filepath.Dir(fullPath)
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatalf("failed to create dir: %v", err)
					}
					if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
						t.Fatalf("failed to write file: %v", err)
					}
				}

				// Create storage target with SMB configuration
				server := "smb-server.example.com"
				share := "TestShare"
				target := &database.StorageTarget{
					Name:                "SMB Test Target",
					Type:                database.StorageTypeSMB,
					Path:                testDir, // Mount point
					Server:              &server,
					Share:               &share,
					Enabled:             true,
					ParallelWorkers:     1,
					RandomSamplePercent: 1.0,
					ChecksumAlgorithm:   "md5",
					CheckpointInterval:  1000,
					BatchSize:           1000,
				}

				if err := db.StorageTargets.Create(context.Background(), target); err != nil {
					t.Fatalf("failed to create target: %v", err)
				}

				cleanup := func() {
					os.RemoveAll(testDir)
					db.StorageTargets.Delete(context.Background(), target.ID)
				}

				return target, testDir, cleanup
			},
		},
	}
}

// TestIntegration_ScanAllBackendTypes tests scanning with all backend types
func TestIntegration_ScanAllBackendTypes(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer testutil.CleanupDB(t, db)
	defer db.Close()

	testCases := GetAllIntegrationTestCases(t, db)

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			target, _, cleanup := tc.SetupTarget(t, db)
			defer cleanup()

			// Create scanner with target configuration
			scannerCfg := scanner.Config{
				ChecksumAlgorithm:   "md5",
				ParallelWorkers:     1,
				RandomSamplePercent: 1.0,
				CheckpointInterval:  1000,
				BatchSize:           1000,
			}
			engine := scanner.NewEngine(db, scannerCfg)

			// Create backend based on target type
			var backend storage.StorageBackend
			var err error

			switch tc.BackendType {
			case database.StorageTypeLocal:
				backend, err = storage.NewLocalFSBackend(target.Path)
			case database.StorageTypeNFS:
				backend, err = storage.NewNFSBackend(*target.Server, *target.Share, target.Path)
			case database.StorageTypeSMB:
				backend, err = storage.NewSMBBackend(*target.Server, *target.Share, target.Path)
			default:
				t.Fatalf("unsupported backend type: %s", tc.BackendType)
			}

			if err != nil {
				t.Fatalf("failed to create backend: %v", err)
			}

			// Run scan
			ctx := context.Background()
			result, err := engine.Scan(ctx, target.ID, backend)
			if err != nil {
				t.Fatalf("scan failed: %v", err)
			}

			// Verify results
			if result.FilesScanned != 3 {
				t.Errorf("expected 3 files scanned, got %d", result.FilesScanned)
			}

			if result.FilesAdded != 3 {
				t.Errorf("expected 3 files added, got %d", result.FilesAdded)
			}

			if result.ErrorsCount != 0 {
				t.Errorf("expected 0 errors, got %d: %v", result.ErrorsCount, result.Errors)
			}

			// Verify files were stored in database
			files, err := db.Files.List(ctx, database.FileFilters{
				StorageTargetID: &target.ID,
			})
			if err != nil {
				t.Fatalf("failed to get files: %v", err)
			}

			if len(files) != 3 {
				t.Errorf("expected 3 files in database, got %d", len(files))
			}

			// Verify all files have checksums
			for _, file := range files {
				if file.CurrentChecksum == nil || *file.CurrentChecksum == "" {
					t.Errorf("file %s has no checksum", file.Path)
				}
			}
		})
	}
}

// TestIntegration_CoordinatorWithAllBackends tests the coordinator with all backend types
func TestIntegration_CoordinatorWithAllBackends(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer testutil.CleanupDB(t, db)
	defer db.Close()

	coord := coordinator.NewCoordinator(db, coordinator.Config{
		MaxConcurrentScans: 3,
	})

	testCases := GetAllIntegrationTestCases(t, db)

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			target, _, cleanup := tc.SetupTarget(t, db)
			defer cleanup()

			// Use coordinator to scan target
			ctx := context.Background()
			result, err := coord.ScanTarget(ctx, target.ID)
			if err != nil {
				t.Fatalf("coordinator scan failed: %v", err)
			}

			// Verify results
			if result.FilesScanned != 3 {
				t.Errorf("expected 3 files scanned, got %d", result.FilesScanned)
			}

			if result.FilesAdded != 3 {
				t.Errorf("expected 3 files added, got %d", result.FilesAdded)
			}

			// Verify scan record was created
			scans, err := db.Scans.List(ctx, database.ScanFilters{
				StorageTargetID: &target.ID,
			})
			if err != nil {
				t.Fatalf("failed to list scans: %v", err)
			}

			if len(scans) != 1 {
				t.Errorf("expected 1 scan record, got %d", len(scans))
			}

			scan := scans[0]
			if scan.Status != database.ScanStatusCompleted {
				t.Errorf("expected scan status completed, got %s", scan.Status)
			}

			if scan.FilesScanned != 3 {
				t.Errorf("scan record: expected 3 files scanned, got %d", scan.FilesScanned)
			}
		})
	}
}

// TestIntegration_ChangeDetectionAllBackends tests change detection with all backend types
func TestIntegration_ChangeDetectionAllBackends(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer testutil.CleanupDB(t, db)
	defer db.Close()

	testCases := GetAllIntegrationTestCases(t, db)

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			target, testDir, cleanup := tc.SetupTarget(t, db)
			defer cleanup()

			coord := coordinator.NewCoordinator(db, coordinator.Config{
				MaxConcurrentScans: 1,
			})

			ctx := context.Background()

			// First scan
			result1, err := coord.ScanTarget(ctx, target.ID)
			if err != nil {
				t.Fatalf("first scan failed: %v", err)
			}

			if result1.FilesAdded != 3 {
				t.Errorf("first scan: expected 3 files added, got %d", result1.FilesAdded)
			}

			// Wait briefly to ensure distinct timestamps
			time.Sleep(100 * time.Millisecond)

			// Modify a file
			modifyPath := filepath.Join(testDir, "file1.txt")
			if tc.BackendType == database.StorageTypeNFS {
				modifyPath = filepath.Join(testDir, "nfs-file1.txt")
			} else if tc.BackendType == database.StorageTypeSMB {
				modifyPath = filepath.Join(testDir, "smb-file1.txt")
			}

			if err := os.WriteFile(modifyPath, []byte("MODIFIED CONTENT"), 0644); err != nil {
				t.Fatalf("failed to modify file: %v", err)
			}

			// Second scan
			result2, err := coord.ScanTarget(ctx, target.ID)
			if err != nil {
				t.Fatalf("second scan failed: %v", err)
			}

			if result2.FilesModified != 1 {
				t.Errorf("second scan: expected 1 file modified, got %d", result2.FilesModified)
			}

			if result2.FilesVerified != 2 {
				t.Errorf("second scan: expected 2 files verified, got %d", result2.FilesVerified)
			}

			// Verify change events were created
			events, err := db.ChangeEvents.List(ctx, database.ChangeEventFilters{})
			if err != nil {
				t.Fatalf("failed to list change events: %v", err)
			}

			// Should have 3 added events + 1 modified event + 2 verified events = 6 events
			if len(events) != 6 {
				t.Errorf("expected 6 change events, got %d", len(events))
			}

			// Count event types
			addedCount := 0
			modifiedCount := 0
			verifiedCount := 0

			for _, event := range events {
				switch event.EventType {
				case database.ChangeEventAdded:
					addedCount++
				case database.ChangeEventModified:
					modifiedCount++
				case database.ChangeEventVerified:
					verifiedCount++
				}
			}

			if addedCount != 3 {
				t.Errorf("expected 3 added events, got %d", addedCount)
			}
			if modifiedCount != 1 {
				t.Errorf("expected 1 modified event, got %d", modifiedCount)
			}
			if verifiedCount != 2 {
				t.Errorf("expected 2 verified events, got %d", verifiedCount)
			}
		})
	}
}

// TestIntegration_BackendProbeFailure tests handling of backend probe failures
func TestIntegration_BackendProbeFailure(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer testutil.CleanupDB(t, db)
	defer db.Close()

	// Create a target with non-existent path
	target := &database.StorageTarget{
		Name:                "Bad Target",
		Type:                database.StorageTypeLocal,
		Path:                "/nonexistent/path/12345",
		Enabled:             true,
		ParallelWorkers:     1,
		RandomSamplePercent: 1.0,
		ChecksumAlgorithm:   "md5",
		CheckpointInterval:  1000,
		BatchSize:           1000,
	}

	if err := db.StorageTargets.Create(context.Background(), target); err != nil {
		t.Fatalf("failed to create target: %v", err)
	}
	defer db.StorageTargets.Delete(context.Background(), target.ID)

	// Attempt to scan should fail at probe stage
	coord := coordinator.NewCoordinator(db, coordinator.Config{})
	ctx := context.Background()

	_, err := coord.ScanTarget(ctx, target.ID)
	if err == nil {
		t.Error("expected scan to fail for inaccessible backend")
	}
}

// TestIntegration_BackendParity verifies identical behavior across all backends
func TestIntegration_BackendParity(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer testutil.CleanupDB(t, db)
	defer db.Close()

	testCases := GetAllIntegrationTestCases(t, db)

	// Collect results from each backend type
	results := make(map[string]*scanner.ScanResult)

	for _, tc := range testCases {
		target, _, cleanup := tc.SetupTarget(t, db)
		defer cleanup()

		coord := coordinator.NewCoordinator(db, coordinator.Config{})
		ctx := context.Background()

		result, err := coord.ScanTarget(ctx, target.ID)
		if err != nil {
			t.Fatalf("%s: scan failed: %v", tc.Name, err)
		}

		results[tc.Name] = result
	}

	// Verify all backends produced identical results
	localResult := results["Local"]

	for name, result := range results {
		if name == "Local" {
			continue
		}

		if result.FilesScanned != localResult.FilesScanned {
			t.Errorf("%s: FilesScanned = %d, want %d (same as Local)",
				name, result.FilesScanned, localResult.FilesScanned)
		}

		if result.FilesAdded != localResult.FilesAdded {
			t.Errorf("%s: FilesAdded = %d, want %d (same as Local)",
				name, result.FilesAdded, localResult.FilesAdded)
		}

		if result.ErrorsCount != localResult.ErrorsCount {
			t.Errorf("%s: ErrorsCount = %d, want %d (same as Local)",
				name, result.ErrorsCount, localResult.ErrorsCount)
		}
	}
}
