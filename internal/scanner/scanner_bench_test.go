package scanner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jeffanddom/fixity/internal/database"
	"github.com/jeffanddom/fixity/internal/storage"
	"github.com/jmoiron/sqlx"
)

// Benchmark-compatible database helpers

func newBenchDB(b *testing.B) *database.Database {
	b.Helper()

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
		b.Fatalf("failed to create test database: %v", err)
	}

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.Health(ctx); err != nil {
		b.Fatalf("failed to connect to test database: %v", err)
	}

	return db
}

func cleanupBenchDB(b *testing.B, db *database.Database) {
	b.Helper()

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
	}

	for _, table := range tables {
		query := fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", table)
		err := db.WithinTransaction(ctx, func(tx *sqlx.Tx) error {
			_, err := tx.ExecContext(ctx, query)
			return err
		})
		if err != nil {
			b.Fatalf("failed to truncate table %s: %v", table, err)
		}
	}
}

// BenchmarkFullScan benchmarks complete scan workflows
func BenchmarkFullScan(b *testing.B) {
	fileCounts := []int{10, 100, 1000}

	for _, fileCount := range fileCounts {
		b.Run(fmt.Sprintf("%d_files", fileCount), func(b *testing.B) {
			db := newBenchDB(b)
			defer cleanupBenchDB(b, db)
			defer db.Close()

			// Create test directory
			testDir := setupScanBenchDirectory(b, fileCount)
			defer os.RemoveAll(testDir)

			// Create storage target
			target := &database.StorageTarget{
				Name:                "benchmark-target",
				Type:                database.StorageTypeLocal,
				Path:                testDir,
				Enabled:             true,
				ParallelWorkers:     4,
				ChecksumAlgorithm:   "blake3",
				RandomSamplePercent: 1.0,
				CheckpointInterval:  1000,
				BatchSize:           100,
			}

			if err := db.StorageTargets.Create(context.Background(), target); err != nil {
				b.Fatalf("failed to create target: %v", err)
			}

			// Create storage backend
			backend, err := storage.NewLocalFSBackend(testDir)
			if err != nil {
				b.Fatalf("failed to create backend: %v", err)
			}
			defer backend.Close()

			// Create scanner
			config := Config{
				ParallelWorkers:    4,
				CheckpointInterval: 1000,
				BatchSize:          100,
			}
			engine := NewEngine(db, config)

			ctx := context.Background()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				result, err := engine.Scan(ctx, target.ID, backend)
				if err != nil {
					b.Fatalf("scan failed: %v", err)
				}
				if result.FilesScanned != int64(fileCount) {
					b.Fatalf("expected %d files, got %d", fileCount, result.FilesScanned)
				}

				// Clean up files for next iteration
				if i < b.N-1 {
					cleanupScanResults(b, db, target.ID)
				}
			}
		})
	}
}

// BenchmarkIncrementalScan benchmarks change detection
func BenchmarkIncrementalScan(b *testing.B) {
	db := newBenchDB(b)
	defer cleanupBenchDB(b, db)
	defer db.Close()

	testDir := setupScanBenchDirectory(b, 100)
	defer os.RemoveAll(testDir)

	// Create storage target
	target := &database.StorageTarget{
		Name:                "benchmark-target",
		Type:                database.StorageTypeLocal,
		Path:                testDir,
		Enabled:             true,
		ParallelWorkers:     4,
		ChecksumAlgorithm:   "blake3",
		RandomSamplePercent: 1.0,
		CheckpointInterval:  1000,
		BatchSize:           100,
	}

	if err := db.StorageTargets.Create(context.Background(), target); err != nil {
		b.Fatalf("failed to create target: %v", err)
	}

	backend, err := storage.NewLocalFSBackend(testDir)
	if err != nil {
		b.Fatalf("failed to create backend: %v", err)
	}
	defer backend.Close()

	config := Config{
		ParallelWorkers:    4,
		CheckpointInterval: 1000,
		BatchSize:          100,
	}
	engine := NewEngine(db, config)

	ctx := context.Background()

	// Run initial scan
	_, err = engine.Scan(ctx, target.ID, backend)
	if err != nil {
		b.Fatalf("initial scan failed: %v", err)
	}

	// Modify 10% of files
	modifyFiles(b, testDir, 10)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result, err := engine.Scan(ctx, target.ID, backend)
		if err != nil {
			b.Fatalf("scan failed: %v", err)
		}
		if result.FilesScanned != 100 {
			b.Fatalf("expected 100 files, got %d", result.FilesScanned)
		}
	}
}

// BenchmarkChangeDetection benchmarks change detection algorithms
func BenchmarkChangeDetection(b *testing.B) {
	db := newBenchDB(b)
	defer cleanupBenchDB(b, db)
	defer db.Close()

	fileCounts := []int{100, 1000, 10000}

	for _, fileCount := range fileCounts {
		b.Run(fmt.Sprintf("%d_files", fileCount), func(b *testing.B) {
			testDir := setupScanBenchDirectory(b, fileCount)
			defer os.RemoveAll(testDir)

			target := &database.StorageTarget{
				Name:                "benchmark-target",
				Type:                database.StorageTypeLocal,
				Path:                testDir,
				Enabled:             true,
				ParallelWorkers:     4,
				ChecksumAlgorithm:   "blake3",
				RandomSamplePercent: 1.0,
				CheckpointInterval:  1000,
				BatchSize:           100,
			}

			if err := db.StorageTargets.Create(context.Background(), target); err != nil {
				b.Fatalf("failed to create target: %v", err)
			}

			backend, err := storage.NewLocalFSBackend(testDir)
			if err != nil {
				b.Fatalf("failed to create backend: %v", err)
			}
			defer backend.Close()

			config := Config{
				ParallelWorkers:    4,
				CheckpointInterval: 1000,
				BatchSize:          100,
			}
			engine := NewEngine(db, config)

			ctx := context.Background()

			// Run initial scan
			_, err = engine.Scan(ctx, target.ID, backend)
			if err != nil {
				b.Fatalf("initial scan failed: %v", err)
			}

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				// Load previous files
				previousFiles, err := engine.loadPreviousFiles(ctx, target.ID)
				if err != nil {
					b.Fatalf("failed to load previous files: %v", err)
				}

				if len(previousFiles) != fileCount {
					b.Fatalf("expected %d previous files, got %d", fileCount, len(previousFiles))
				}
			}
		})
	}
}

// BenchmarkParallelWorkers benchmarks different worker pool sizes
func BenchmarkParallelWorkers(b *testing.B) {
	workerCounts := []int{1, 2, 4, 8, 16}

	for _, workers := range workerCounts {
		b.Run(fmt.Sprintf("%d_workers", workers), func(b *testing.B) {
			db := newBenchDB(b)
			defer cleanupBenchDB(b, db)
			defer db.Close()

			testDir := setupScanBenchDirectory(b, 1000)
			defer os.RemoveAll(testDir)

			target := &database.StorageTarget{
				Name:                "benchmark-target",
				Type:                database.StorageTypeLocal,
				Path:                testDir,
				Enabled:             true,
				ParallelWorkers:     workers,
				ChecksumAlgorithm:   "blake3",
				RandomSamplePercent: 1.0,
				CheckpointInterval:  1000,
				BatchSize:           100,
			}

			if err := db.StorageTargets.Create(context.Background(), target); err != nil {
				b.Fatalf("failed to create target: %v", err)
			}

			backend, err := storage.NewLocalFSBackend(testDir)
			if err != nil {
				b.Fatalf("failed to create backend: %v", err)
			}
			defer backend.Close()

			config := Config{
				ParallelWorkers:    workers,
				CheckpointInterval: 1000,
				BatchSize:          100,
			}
			engine := NewEngine(db, config)

			ctx := context.Background()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				result, err := engine.Scan(ctx, target.ID, backend)
				if err != nil {
					b.Fatalf("scan failed: %v", err)
				}
				if result.FilesScanned != 1000 {
					b.Fatalf("expected 1000 files, got %d", result.FilesScanned)
				}

				if i < b.N-1 {
					cleanupScanResults(b, db, target.ID)
				}
			}
		})
	}
}

// BenchmarkBatchSizes benchmarks different database batch sizes
func BenchmarkBatchSizes(b *testing.B) {
	batchSizes := []int{10, 50, 100, 500, 1000}

	for _, batchSize := range batchSizes {
		b.Run(fmt.Sprintf("batch_%d", batchSize), func(b *testing.B) {
			db := newBenchDB(b)
			defer cleanupBenchDB(b, db)
			defer db.Close()

			testDir := setupScanBenchDirectory(b, 1000)
			defer os.RemoveAll(testDir)

			target := &database.StorageTarget{
				Name:                "benchmark-target",
				Type:                database.StorageTypeLocal,
				Path:                testDir,
				Enabled:             true,
				ParallelWorkers:     4,
				ChecksumAlgorithm:   "blake3",
				RandomSamplePercent: 1.0,
				CheckpointInterval:  1000,
				BatchSize:           batchSize,
			}

			if err := db.StorageTargets.Create(context.Background(), target); err != nil {
				b.Fatalf("failed to create target: %v", err)
			}

			backend, err := storage.NewLocalFSBackend(testDir)
			if err != nil {
				b.Fatalf("failed to create backend: %v", err)
			}
			defer backend.Close()

			config := Config{
				ParallelWorkers:    4,
				CheckpointInterval: 1000,
				BatchSize:          batchSize,
			}
			engine := NewEngine(db, config)

			ctx := context.Background()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				result, err := engine.Scan(ctx, target.ID, backend)
				if err != nil {
					b.Fatalf("scan failed: %v", err)
				}
				if result.FilesScanned != 1000 {
					b.Fatalf("expected 1000 files, got %d", result.FilesScanned)
				}

				if i < b.N-1 {
					cleanupScanResults(b, db, target.ID)
				}
			}
		})
	}
}

// Helper functions

func setupScanBenchDirectory(b *testing.B, fileCount int) string {
	b.Helper()

	tempDir, err := os.MkdirTemp("", "fixity-scan-bench-*")
	if err != nil {
		b.Fatalf("failed to create temp dir: %v", err)
	}

	// Create files
	for i := 0; i < fileCount; i++ {
		filename := filepath.Join(tempDir, fmt.Sprintf("file%04d.txt", i))
		content := fmt.Sprintf("Benchmark file %d content that is somewhat longer to make it realistic\n", i)
		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			b.Fatalf("failed to create file: %v", err)
		}
	}

	return tempDir
}

func modifyFiles(b *testing.B, dir string, count int) {
	b.Helper()

	for i := 0; i < count; i++ {
		filename := filepath.Join(dir, fmt.Sprintf("file%04d.txt", i))
		content := fmt.Sprintf("Modified content %d\n", i)
		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			b.Fatalf("failed to modify file: %v", err)
		}
	}
}

func cleanupScanResults(b *testing.B, db *database.Database, targetID int64) {
	b.Helper()

	ctx := context.Background()

	// Delete all files for this target
	err := db.WithinTransaction(ctx, func(tx *sqlx.Tx) error {
		_, err := tx.ExecContext(ctx, "DELETE FROM files WHERE storage_target_id = $1", targetID)
		return err
	})
	if err != nil {
		b.Fatalf("failed to cleanup files: %v", err)
	}

	// Delete all scans for this target
	err = db.WithinTransaction(ctx, func(tx *sqlx.Tx) error {
		_, err := tx.ExecContext(ctx, "DELETE FROM scans WHERE storage_target_id = $1", targetID)
		return err
	})
	if err != nil {
		b.Fatalf("failed to cleanup scans: %v", err)
	}
}
