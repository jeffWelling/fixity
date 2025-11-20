package coordinator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jeffanddom/fixity/internal/checksum"
	"github.com/jeffanddom/fixity/internal/database"
	"github.com/jeffanddom/fixity/internal/scanner"
	"github.com/jeffanddom/fixity/internal/storage"
)

// Coordinator orchestrates scans across multiple storage targets
type Coordinator struct {
	db                *database.Database
	maxConcurrentSans int
	mu                sync.Mutex
	runningScans      map[int64]context.CancelFunc // targetID -> cancel function
}

// Config holds coordinator configuration
type Config struct {
	MaxConcurrentScans int // Maximum number of concurrent scans
}

// ScanRequest represents a request to scan a storage target
type ScanRequest struct {
	TargetID int64
}

// ScanStatus represents the status of a running scan
type ScanStatus struct {
	TargetID    int64
	TargetName  string
	ScanID      int64
	Status      database.ScanStatus
	StartedAt   time.Time
	CompletedAt *time.Time
	Progress    ScanProgress
}

// ScanProgress tracks scan progress
type ScanProgress struct {
	FilesScanned  int64
	FilesAdded    int64
	FilesDeleted  int64
	FilesModified int64
	FilesVerified int64
	ErrorsCount   int
}

// NewCoordinator creates a new scan coordinator
func NewCoordinator(db *database.Database, config Config) *Coordinator {
	if config.MaxConcurrentScans <= 0 {
		config.MaxConcurrentScans = 3 // Default: 3 concurrent scans
	}

	return &Coordinator{
		db:                db,
		maxConcurrentSans: config.MaxConcurrentScans,
		runningScans:      make(map[int64]context.CancelFunc),
	}
}

// ScanTarget triggers a scan for a specific storage target
func (c *Coordinator) ScanTarget(ctx context.Context, targetID int64) (*scanner.ScanResult, error) {
	// Load target configuration
	target, err := c.db.StorageTargets.GetByID(ctx, targetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get storage target: %w", err)
	}

	if !target.Enabled {
		return nil, fmt.Errorf("storage target %d is disabled", targetID)
	}

	// Check if already running
	c.mu.Lock()
	if _, running := c.runningScans[targetID]; running {
		c.mu.Unlock()
		return nil, fmt.Errorf("scan already running for target %d", targetID)
	}

	// Check concurrent scan limit
	if len(c.runningScans) >= c.maxConcurrentSans {
		c.mu.Unlock()
		return nil, fmt.Errorf("concurrent scan limit reached (%d)", c.maxConcurrentSans)
	}

	// Register scan
	scanCtx, cancel := context.WithCancel(ctx)
	c.runningScans[targetID] = cancel
	c.mu.Unlock()

	// Ensure cleanup
	defer func() {
		c.mu.Lock()
		delete(c.runningScans, targetID)
		c.mu.Unlock()
	}()

	// Create storage backend
	backend, err := c.createBackend(target)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage backend: %w", err)
	}

	// Create scanner with target-specific configuration
	scannerConfig := scanner.Config{
		ChecksumAlgorithm:   checksum.Algorithm(target.ChecksumAlgorithm),
		ParallelWorkers:     target.ParallelWorkers,
		RandomSamplePercent: target.RandomSamplePercent,
		CheckpointInterval:  target.CheckpointInterval,
		BatchSize:           target.BatchSize,
		FileTimeout:         5 * time.Minute,
	}

	engine := scanner.NewEngine(c.db, scannerConfig)

	// Execute scan
	result, err := engine.Scan(scanCtx, targetID, backend)
	if err != nil {
		return nil, fmt.Errorf("scan failed: %w", err)
	}

	return result, nil
}

// CancelScan cancels a running scan
func (c *Coordinator) CancelScan(targetID int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	cancel, running := c.runningScans[targetID]
	if !running {
		return fmt.Errorf("no scan running for target %d", targetID)
	}

	cancel()
	return nil
}

// GetRunningSans returns the list of currently running scans
func (c *Coordinator) GetRunningSans(ctx context.Context) ([]*ScanStatus, error) {
	c.mu.Lock()
	runningTargetIDs := make([]int64, 0, len(c.runningScans))
	for targetID := range c.runningScans {
		runningTargetIDs = append(runningTargetIDs, targetID)
	}
	c.mu.Unlock()

	statuses := make([]*ScanStatus, 0, len(runningTargetIDs))

	for _, targetID := range runningTargetIDs {
		// Get target info
		target, err := c.db.StorageTargets.GetByID(ctx, targetID)
		if err != nil {
			continue // Skip if target was deleted
		}

		// Get most recent scan for this target
		runningStatus := database.ScanStatusRunning
		scans, err := c.db.Scans.List(ctx, database.ScanFilters{
			StorageTargetID: &targetID,
			Status:          &runningStatus,
			Limit:           1,
		})
		if err != nil || len(scans) == 0 {
			continue
		}

		scan := scans[0]
		statuses = append(statuses, &ScanStatus{
			TargetID:    targetID,
			TargetName:  target.Name,
			ScanID:      scan.ID,
			Status:      scan.Status,
			StartedAt:   scan.StartedAt,
			CompletedAt: scan.CompletedAt,
			Progress: ScanProgress{
				FilesScanned:  scan.FilesScanned,
				FilesAdded:    scan.FilesAdded,
				FilesDeleted:  scan.FilesDeleted,
				FilesModified: scan.FilesModified,
				FilesVerified: scan.FilesVerified,
				ErrorsCount:   scan.ErrorsCount,
			},
		})
	}

	return statuses, nil
}

// ScanAll triggers scans for all enabled storage targets (respecting concurrency limits)
func (c *Coordinator) ScanAll(ctx context.Context) ([]*scanner.ScanResult, []error) {
	// Get all enabled targets
	targets, err := c.db.StorageTargets.ListEnabled(ctx)
	if err != nil {
		return nil, []error{fmt.Errorf("failed to list storage targets: %w", err)}
	}

	results := make([]*scanner.ScanResult, 0)
	errors := make([]error, 0)

	// Use semaphore pattern for concurrency control
	sem := make(chan struct{}, c.maxConcurrentSans)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, target := range targets {
		wg.Add(1)
		go func(t *database.StorageTarget) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			result, err := c.ScanTarget(ctx, t.ID)

			mu.Lock()
			if err != nil {
				errors = append(errors, fmt.Errorf("target %s: %w", t.Name, err))
			} else {
				results = append(results, result)
			}
			mu.Unlock()
		}(target)
	}

	wg.Wait()

	return results, errors
}

// createBackend creates a storage backend for the given target
func (c *Coordinator) createBackend(target *database.StorageTarget) (storage.StorageBackend, error) {
	// Convert database.StorageType to storage.StorageType
	var storageType storage.StorageType
	switch target.Type {
	case database.StorageTypeLocal:
		storageType = storage.TypeLocal
	case database.StorageTypeNFS:
		storageType = storage.TypeNFS
	case database.StorageTypeSMB:
		storageType = storage.TypeSMB
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", target.Type)
	}

	// Create backend configuration
	config := storage.BackendConfig{
		Type:     storageType,
		Path:     target.Path,
		Server:   target.Server,
		Share:    target.Share,
		CredsRef: target.CredentialsRef,
	}

	return storage.NewBackend(config)
}
