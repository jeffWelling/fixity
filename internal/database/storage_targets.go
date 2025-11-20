package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// StorageTargetRepository handles storage target operations
type StorageTargetRepository struct {
	db *sqlx.DB
}

// GetByID retrieves a storage target by ID
func (r *StorageTargetRepository) GetByID(ctx context.Context, id int64) (*StorageTarget, error) {
	var target StorageTarget
	query := `SELECT * FROM storage_targets WHERE id = $1`
	if err := r.db.GetContext(ctx, &target, query, id); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("storage target not found: %d", id)
		}
		return nil, fmt.Errorf("failed to get storage target: %w", err)
	}
	return &target, nil
}

// GetByName retrieves a storage target by name
func (r *StorageTargetRepository) GetByName(ctx context.Context, name string) (*StorageTarget, error) {
	var target StorageTarget
	query := `SELECT * FROM storage_targets WHERE name = $1`
	if err := r.db.GetContext(ctx, &target, query, name); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("storage target not found: %s", name)
		}
		return nil, fmt.Errorf("failed to get storage target by name: %w", err)
	}
	return &target, nil
}

// ListAll retrieves all storage targets
func (r *StorageTargetRepository) ListAll(ctx context.Context) ([]*StorageTarget, error) {
	query := `SELECT * FROM storage_targets ORDER BY name`

	var targets []*StorageTarget
	if err := r.db.SelectContext(ctx, &targets, query); err != nil {
		return nil, fmt.Errorf("failed to list storage targets: %w", err)
	}

	return targets, nil
}

// ListEnabled retrieves all enabled storage targets
func (r *StorageTargetRepository) ListEnabled(ctx context.Context) ([]*StorageTarget, error) {
	query := `SELECT * FROM storage_targets WHERE enabled = TRUE ORDER BY name`

	var targets []*StorageTarget
	if err := r.db.SelectContext(ctx, &targets, query); err != nil {
		return nil, fmt.Errorf("failed to list enabled storage targets: %w", err)
	}

	return targets, nil
}

// Create creates a new storage target
func (r *StorageTargetRepository) Create(ctx context.Context, target *StorageTarget) error {
	query := `
		INSERT INTO storage_targets (
			name, type, path, server, share, credentials_ref,
			enabled, scan_schedule, parallel_workers, random_sample_percent,
			checksum_algorithm, checkpoint_interval, batch_size,
			large_change_threshold_count, large_change_threshold_percent, large_change_threshold_bytes,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, NOW(), NOW()
		) RETURNING id, created_at, updated_at`

	err := r.db.QueryRowContext(
		ctx, query,
		target.Name, target.Type, target.Path, target.Server, target.Share, target.CredentialsRef,
		target.Enabled, target.ScanSchedule, target.ParallelWorkers, target.RandomSamplePercent,
		target.ChecksumAlgorithm, target.CheckpointInterval, target.BatchSize,
		target.LargeChangeThresholdCount, target.LargeChangeThresholdPercent, target.LargeChangeThresholdBytes,
	).Scan(&target.ID, &target.CreatedAt, &target.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create storage target: %w", err)
	}

	return nil
}

// Update updates an existing storage target
func (r *StorageTargetRepository) Update(ctx context.Context, target *StorageTarget) error {
	query := `
		UPDATE storage_targets SET
			name = $2,
			type = $3,
			path = $4,
			server = $5,
			share = $6,
			credentials_ref = $7,
			enabled = $8,
			scan_schedule = $9,
			parallel_workers = $10,
			random_sample_percent = $11,
			checksum_algorithm = $12,
			checkpoint_interval = $13,
			batch_size = $14,
			large_change_threshold_count = $15,
			large_change_threshold_percent = $16,
			large_change_threshold_bytes = $17,
			updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at`

	err := r.db.QueryRowContext(
		ctx, query,
		target.ID, target.Name, target.Type, target.Path, target.Server, target.Share, target.CredentialsRef,
		target.Enabled, target.ScanSchedule, target.ParallelWorkers, target.RandomSamplePercent,
		target.ChecksumAlgorithm, target.CheckpointInterval, target.BatchSize,
		target.LargeChangeThresholdCount, target.LargeChangeThresholdPercent, target.LargeChangeThresholdBytes,
	).Scan(&target.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to update storage target: %w", err)
	}

	return nil
}

// Delete deletes a storage target
func (r *StorageTargetRepository) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM storage_targets WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete storage target: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("storage target not found: %d", id)
	}

	return nil
}
