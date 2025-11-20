package scanner

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/jeffanddom/fixity/internal/checksum"
	"github.com/jeffanddom/fixity/internal/database"
	"github.com/jeffanddom/fixity/internal/storage"
)

// ChangeSet contains detected changes
type ChangeSet struct {
	Added     []*FileRecord
	Deleted   []*database.File
	Modified  []*FileRecord
	Unchanged []*FileRecord
}

// detectChanges compares current files with previous scan to detect changes
func (e *Engine) detectChanges(
	ctx context.Context,
	current map[string]*FileRecord,
	previous map[string]*database.File,
	scanID int64,
	checksumPool *checksum.WorkerPool,
	backend storage.StorageBackend,
	target *database.StorageTarget,
) (*ChangeSet, error) {
	changes := &ChangeSet{
		Added:     []*FileRecord{},
		Deleted:   []*database.File{},
		Modified:  []*FileRecord{},
		Unchanged: []*FileRecord{},
	}

	// Find added and modified files
	for path, currentFile := range current {
		previousFile, existed := previous[path]

		if !existed {
			// New file
			currentFile.IsNew = true
			changes.Added = append(changes.Added, currentFile)
		} else if e.isModified(currentFile, previousFile) {
			// Modified file
			currentFile.IsModified = true
			currentFile.PreviousChecksum = *previousFile.CurrentChecksum
			changes.Modified = append(changes.Modified, currentFile)
		} else {
			// Unchanged file
			changes.Unchanged = append(changes.Unchanged, currentFile)
		}
	}

	// Find deleted files
	for path, previousFile := range previous {
		if _, exists := current[path]; !exists {
			changes.Deleted = append(changes.Deleted, previousFile)
		}
	}

	// Record change events
	if err := e.recordChanges(ctx, scanID, changes); err != nil {
		return nil, fmt.Errorf("failed to record changes: %w", err)
	}

	// Select random sample for verification
	sampled, err := e.selectRandomSample(ctx, changes.Unchanged, previous, target)
	if err != nil {
		return nil, fmt.Errorf("failed to select random sample: %w", err)
	}

	// Compute checksums for new, modified, and sampled files
	if err := e.computeChecksums(ctx, scanID, changes, sampled, checksumPool, backend, target); err != nil {
		return nil, fmt.Errorf("failed to compute checksums: %w", err)
	}

	return changes, nil
}

// isModified checks if a file has been modified based on size and modtime
func (e *Engine) isModified(current *FileRecord, previous *database.File) bool {
	// Check size change
	if current.Size != previous.Size {
		return true
	}

	// Check modification time (truncate to second for filesystem compatibility)
	currentModTime := current.ModTime.Truncate(1)
	previousModTime := previous.LastSeen.Truncate(1)

	if !currentModTime.Equal(previousModTime) {
		return true
	}

	return false
}

// recordChanges creates change event records in the database
func (e *Engine) recordChanges(ctx context.Context, scanID int64, changes *ChangeSet) error {
	events := []*database.ChangeEvent{}

	// Record additions
	for _, file := range changes.Added {
		// Will be updated with checksum after computation
		events = append(events, &database.ChangeEvent{
			ScanID:      scanID,
			FileID:      0, // Will be set after file record is created
			EventType:   database.ChangeEventAdded,
			DetectedAt:  file.ModTime,
			NewChecksum: nil, // Will be updated after checksum computation
			NewSize:     &file.Size,
		})
	}

	// Record deletions (process immediately since we have file IDs)
	deleteEvents := []*database.ChangeEvent{}
	for _, file := range changes.Deleted {
		oldSize := file.Size
		deleteEvents = append(deleteEvents, &database.ChangeEvent{
			ScanID:      scanID,
			FileID:      file.ID,
			EventType:   database.ChangeEventDeleted,
			DetectedAt:  file.LastSeen,
			OldChecksum: file.CurrentChecksum,
			OldSize:     &oldSize,
		})
	}

	if len(deleteEvents) > 0 {
		if err := e.db.ChangeEvents.CreateBatch(ctx, deleteEvents); err != nil {
			return fmt.Errorf("failed to record deletion events: %w", err)
		}
	}

	// Record modifications
	for _, file := range changes.Modified {
		events = append(events, &database.ChangeEvent{
			ScanID:      scanID,
			FileID:      0, // Will be set after file record is updated
			EventType:   database.ChangeEventModified,
			DetectedAt:  file.ModTime,
			OldChecksum: &file.PreviousChecksum,
			NewChecksum: nil, // Will be updated after checksum computation
			NewSize:     &file.Size,
		})
	}

	// Note: Addition and modification events are stored separately after
	// checksums are computed and file records are created/updated

	return nil
}

// selectRandomSample selects a random sample of unchanged files for verification
// Uses weighted sampling prioritizing files that haven't been checksummed recently
func (e *Engine) selectRandomSample(
	ctx context.Context,
	unchanged []*FileRecord,
	previous map[string]*database.File,
	target *database.StorageTarget,
) ([]*FileRecord, error) {
	if e.config.RandomSamplePercent <= 0 || len(unchanged) == 0 {
		return []*FileRecord{}, nil
	}

	sampleSize := int(float64(len(unchanged)) * e.config.RandomSamplePercent / 100.0)
	if sampleSize == 0 && len(unchanged) > 0 {
		sampleSize = 1 // At least one file if there are any
	}

	// Build list of unverified files from database (prioritize files never checksummed)
	unverifiedFiles, err := e.db.Files.GetUnverifiedFiles(ctx, target.ID, sampleSize*2)
	if err != nil {
		return nil, fmt.Errorf("failed to get unverified files: %w", err)
	}

	// Create map of unverified file paths for quick lookup
	unverifiedPaths := make(map[string]bool)
	for _, f := range unverifiedFiles {
		unverifiedPaths[f.Path] = true
	}

	// Separate unchanged files into unverified (priority) and verified
	priority := []*FileRecord{}
	regular := []*FileRecord{}

	for _, file := range unchanged {
		if unverifiedPaths[file.Path] {
			priority = append(priority, file)
		} else {
			regular = append(regular, file)
		}
	}

	// Select from priority first, then regular if needed
	sampled := []*FileRecord{}
	for i := 0; i < sampleSize && i < len(priority); i++ {
		sampled = append(sampled, priority[i])
	}

	remaining := sampleSize - len(sampled)
	for i := 0; i < remaining && i < len(regular); i++ {
		sampled = append(sampled, regular[i])
	}

	return sampled, nil
}

// computeChecksums computes checksums for files that need verification
func (e *Engine) computeChecksums(
	ctx context.Context,
	scanID int64,
	changes *ChangeSet,
	sampled []*FileRecord,
	checksumPool *checksum.WorkerPool,
	backend storage.StorageBackend,
	target *database.StorageTarget,
) error {
	// Collect all files that need checksums
	toChecksum := make([]*FileRecord, 0)
	toChecksum = append(toChecksum, changes.Added...)
	toChecksum = append(toChecksum, changes.Modified...)
	toChecksum = append(toChecksum, sampled...)

	if len(toChecksum) == 0 {
		return nil
	}

	// Submit jobs to worker pool
	for _, file := range toChecksum {
		job := &checksum.Job{
			Path:      file.Path,
			Algorithm: e.config.ChecksumAlgorithm,
			Opener: func(path string) func() (io.ReadCloser, error) {
				return func() (io.ReadCloser, error) {
					return backend.Open(ctx, path)
				}
			}(file.Path),
			Timeout: e.config.FileTimeout,
		}

		if err := checksumPool.Submit(job); err != nil {
			return fmt.Errorf("failed to submit checksum job for %s: %w", file.Path, err)
		}
	}

	// Collect results
	fileMap := make(map[string]*FileRecord)
	for _, file := range toChecksum {
		fileMap[file.Path] = file
	}

	for i := 0; i < len(toChecksum); i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case result := <-checksumPool.Results():
			file, exists := fileMap[result.Path]
			if !exists {
				continue
			}

			if result.Error != nil {
				// Log error but continue with other files
				continue
			}

			// Update file record with checksum
			file.Checksum = result.Checksum
			file.ChecksumType = string(e.config.ChecksumAlgorithm)
		}
	}

	// Persist file records to database
	if err := e.persistFileRecords(ctx, scanID, toChecksum, target.ID); err != nil {
		return fmt.Errorf("failed to persist file records: %w", err)
	}

	// Create change events for verified files (sampled unchanged files)
	if err := e.createVerificationEvents(ctx, scanID, sampled, target.ID); err != nil {
		return fmt.Errorf("failed to create verification events: %w", err)
	}

	return nil
}

// persistFileRecords creates or updates file records in the database
func (e *Engine) persistFileRecords(
	ctx context.Context,
	scanID int64,
	files []*FileRecord,
	targetID int64,
) error {
	now := time.Now()

	for _, file := range files {
		// Skip files without checksums
		if file.Checksum == "" {
			continue
		}

		dbFile := &database.File{
			StorageTargetID:   targetID,
			Path:              file.Path,
			Size:              file.Size,
			FirstSeen:         now,
			LastSeen:          now,
			CurrentChecksum:   &file.Checksum,
			ChecksumType:      &file.ChecksumType,
			LastChecksummedAt: &now,
		}

		// Check if file already exists
		existing, err := e.db.Files.GetByPath(ctx, targetID, file.Path)
		if err == nil && existing != nil {
			// Update existing file
			dbFile.ID = existing.ID
			dbFile.FirstSeen = existing.FirstSeen
			if err := e.db.Files.Update(ctx, dbFile); err != nil {
				return fmt.Errorf("failed to update file %s: %w", file.Path, err)
			}
		} else {
			// Create new file
			if err := e.db.Files.Create(ctx, dbFile); err != nil {
				return fmt.Errorf("failed to create file %s: %w", file.Path, err)
			}
		}
	}

	return nil
}

// createVerificationEvents creates change events for verified (sampled) files
func (e *Engine) createVerificationEvents(
	ctx context.Context,
	scanID int64,
	sampled []*FileRecord,
	targetID int64,
) error {
	if len(sampled) == 0 {
		return nil
	}

	events := make([]*database.ChangeEvent, 0, len(sampled))
	for _, file := range sampled {
		// Skip files without checksums
		if file.Checksum == "" {
			continue
		}

		// Get file ID from database
		dbFile, err := e.db.Files.GetByPath(ctx, targetID, file.Path)
		if err != nil || dbFile == nil {
			continue
		}

		events = append(events, &database.ChangeEvent{
			ScanID:      scanID,
			FileID:      dbFile.ID,
			EventType:   database.ChangeEventVerified,
			DetectedAt:  file.ModTime,
			NewChecksum: &file.Checksum,
			NewSize:     &file.Size,
		})
	}

	if len(events) > 0 {
		if err := e.db.ChangeEvents.CreateBatch(ctx, events); err != nil {
			return fmt.Errorf("failed to create verification events: %w", err)
		}
	}

	return nil
}
