package scanner

import (
	"context"
	"fmt"
	"time"

	"github.com/jeffanddom/fixity/internal/checksum"
	"github.com/jeffanddom/fixity/internal/database"
	"github.com/jeffanddom/fixity/internal/storage"
)

// Engine orchestrates file scanning and change detection
type Engine struct {
	db     *database.Database
	config Config
}

// Config holds scanner configuration
type Config struct {
	ChecksumAlgorithm   checksum.Algorithm
	ParallelWorkers     int
	RandomSamplePercent float64
	CheckpointInterval  int // Checkpoint every N files
	BatchSize           int // Database batch size
	FileTimeout         time.Duration
}

// ScanResult contains the results of a scan
type ScanResult struct {
	ScanID        int64
	FilesScanned  int64
	FilesAdded    int64
	FilesDeleted  int64
	FilesModified int64
	FilesVerified int64
	ErrorsCount   int
	Errors        []string
	IsLargeChange bool
	Duration      time.Duration
}

// FileRecord represents a file discovered during scanning
type FileRecord struct {
	Path             string
	Size             int64
	ModTime          time.Time
	Checksum         string
	ChecksumType     string
	IsNew            bool
	IsDeleted        bool
	IsModified       bool
	IsVerified       bool
	PreviousChecksum string
}

// NewEngine creates a new scanner engine
func NewEngine(db *database.Database, config Config) *Engine {
	// Set defaults
	if config.ParallelWorkers <= 0 {
		config.ParallelWorkers = 4
	}
	if config.RandomSamplePercent <= 0 {
		config.RandomSamplePercent = 1.0
	}
	if config.CheckpointInterval <= 0 {
		config.CheckpointInterval = 1000
	}
	if config.BatchSize <= 0 {
		config.BatchSize = 1000
	}
	if config.FileTimeout <= 0 {
		config.FileTimeout = 5 * time.Minute
	}
	if config.ChecksumAlgorithm == "" {
		config.ChecksumAlgorithm = checksum.AlgorithmSHA256
	}

	return &Engine{
		db:     db,
		config: config,
	}
}

// Scan performs a full scan of a storage target
func (e *Engine) Scan(ctx context.Context, targetID int64, backend storage.StorageBackend) (*ScanResult, error) {
	start := time.Now()

	// Get storage target configuration
	target, err := e.db.StorageTargets.GetByID(ctx, targetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get storage target: %w", err)
	}

	// Probe backend to ensure it's accessible
	if err := backend.Probe(ctx); err != nil {
		return nil, fmt.Errorf("storage backend not accessible: %w", err)
	}

	// Create scan record
	scan := &database.Scan{
		StorageTargetID: targetID,
		Status:          database.ScanStatusRunning,
		StartedAt:       time.Now(),
	}
	if err := e.db.Scans.Create(ctx, scan); err != nil {
		return nil, fmt.Errorf("failed to create scan record: %w", err)
	}

	result := &ScanResult{
		ScanID: scan.ID,
		Errors: []string{},
	}

	// Create and start checksum worker pool for this scan
	checksumPool := checksum.NewWorkerPool(e.config.ParallelWorkers)
	checksumPool.Start()
	defer checksumPool.Stop()

	// Scan directory tree
	currentFiles, err := e.scanDirectory(ctx, backend, scan.ID, result)
	if err != nil {
		e.finalizeScan(ctx, scan, result, database.ScanStatusFailed)
		return nil, fmt.Errorf("failed to scan directory: %w", err)
	}

	// Load previous scan data
	previousFiles, err := e.loadPreviousFiles(ctx, targetID)
	if err != nil {
		e.finalizeScan(ctx, scan, result, database.ScanStatusFailed)
		return nil, fmt.Errorf("failed to load previous files: %w", err)
	}

	// Detect changes
	changes, err := e.detectChanges(ctx, currentFiles, previousFiles, scan.ID, checksumPool, backend, target)
	if err != nil {
		e.finalizeScan(ctx, scan, result, database.ScanStatusFailed)
		return nil, fmt.Errorf("failed to detect changes: %w", err)
	}

	// Update counters
	result.FilesAdded = int64(len(changes.Added))
	result.FilesDeleted = int64(len(changes.Deleted))
	result.FilesModified = int64(len(changes.Modified))

	// Check for large changes
	result.IsLargeChange = e.isLargeChange(target, changes, len(currentFiles))

	// Finalize scan
	result.Duration = time.Since(start)
	e.finalizeScan(ctx, scan, result, database.ScanStatusCompleted)

	return result, nil
}

// scanDirectory walks the directory tree and discovers all files
func (e *Engine) scanDirectory(ctx context.Context, backend storage.StorageBackend, scanID int64, result *ScanResult) (map[string]*FileRecord, error) {
	files := make(map[string]*FileRecord)
	fileCount := 0

	err := backend.Walk(ctx, func(path string, info *storage.FileInfo) error {
		// Skip directories
		if info.IsDir {
			return nil
		}

		fileCount++
		result.FilesScanned++

		// Create file record
		files[path] = &FileRecord{
			Path:    path,
			Size:    info.Size,
			ModTime: info.ModTime,
		}

		// Checkpoint periodically
		if fileCount%e.config.CheckpointInterval == 0 {
			if err := e.saveCheckpoint(ctx, scanID, path, int64(fileCount)); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("checkpoint error: %v", err))
				result.ErrorsCount++
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

// loadPreviousFiles loads file records from the previous scan
func (e *Engine) loadPreviousFiles(ctx context.Context, targetID int64) (map[string]*database.File, error) {
	files, err := e.db.Files.List(ctx, database.FileFilters{
		StorageTargetID: &targetID,
		ActiveOnly:      true,
	})
	if err != nil {
		return nil, err
	}

	fileMap := make(map[string]*database.File)
	for _, f := range files {
		fileMap[f.Path] = f
	}

	return fileMap, nil
}

// saveCheckpoint saves scan progress for resumability
func (e *Engine) saveCheckpoint(ctx context.Context, scanID int64, lastPath string, filesProcessed int64) error {
	checkpoint := &database.ScanCheckpoint{
		ScanID:            scanID,
		LastProcessedPath: lastPath,
		FilesProcessed:    filesProcessed,
	}
	return e.db.Checkpoints.Create(ctx, checkpoint)
}

// finalizeScan updates the scan record with final statistics
func (e *Engine) finalizeScan(ctx context.Context, scan *database.Scan, result *ScanResult, status database.ScanStatus) {
	now := time.Now()
	scan.Status = status
	scan.CompletedAt = &now
	scan.FilesScanned = result.FilesScanned
	scan.FilesAdded = result.FilesAdded
	scan.FilesDeleted = result.FilesDeleted
	scan.FilesModified = result.FilesModified
	scan.FilesVerified = result.FilesVerified
	scan.ErrorsCount = result.ErrorsCount
	scan.IsLargeChange = result.IsLargeChange

	// Store errors
	if len(result.Errors) > 0 {
		scan.ErrorMessages = make([]string, len(result.Errors))
		copy(scan.ErrorMessages, result.Errors)
	}

	e.db.Scans.Update(ctx, scan)
}

// isLargeChange determines if the changes exceed configured thresholds
func (e *Engine) isLargeChange(target *database.StorageTarget, changes *ChangeSet, totalFiles int) bool {
	totalChanges := len(changes.Added) + len(changes.Deleted) + len(changes.Modified)

	// Check count threshold
	if target.LargeChangeThresholdCount != nil && totalChanges > *target.LargeChangeThresholdCount {
		return true
	}

	// Check percentage threshold
	if target.LargeChangeThresholdPercent != nil && totalFiles > 0 {
		changePercent := float64(totalChanges) / float64(totalFiles) * 100
		if changePercent > *target.LargeChangeThresholdPercent {
			return true
		}
	}

	// Check bytes threshold (would need to sum file sizes)
	// Not implemented in this simplified version

	return false
}
