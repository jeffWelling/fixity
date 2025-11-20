package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// ConfigRepository handles configuration operations
type ConfigRepository struct {
	db *sqlx.DB
}

// Get retrieves a configuration value by key
func (r *ConfigRepository) Get(ctx context.Context, key string) (*Config, error) {
	var config Config
	query := `SELECT * FROM config WHERE key = $1`
	if err := r.db.GetContext(ctx, &config, query, key); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("config key not found: %s", key)
		}
		return nil, fmt.Errorf("failed to get config: %w", err)
	}
	return &config, nil
}

// GetValue retrieves just the value for a configuration key
func (r *ConfigRepository) GetValue(ctx context.Context, key string) (string, error) {
	var value string
	query := `SELECT value FROM config WHERE key = $1`
	if err := r.db.GetContext(ctx, &value, query, key); err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("config key not found: %s", key)
		}
		return "", fmt.Errorf("failed to get config value: %w", err)
	}
	return value, nil
}

// ListAll retrieves all configuration entries
func (r *ConfigRepository) ListAll(ctx context.Context) ([]*Config, error) {
	query := `SELECT * FROM config ORDER BY key`

	var configs []*Config
	if err := r.db.SelectContext(ctx, &configs, query); err != nil {
		return nil, fmt.Errorf("failed to list config: %w", err)
	}

	return configs, nil
}

// Set creates or updates a configuration value
func (r *ConfigRepository) Set(ctx context.Context, key, value string, updatedBy *int64) error {
	query := `
		INSERT INTO config (key, value, updated_by, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (key) DO UPDATE SET
			value = EXCLUDED.value,
			updated_by = EXCLUDED.updated_by,
			updated_at = NOW()`

	_, err := r.db.ExecContext(ctx, query, key, value, updatedBy)
	if err != nil {
		return fmt.Errorf("failed to set config: %w", err)
	}

	return nil
}

// Delete deletes a configuration entry
func (r *ConfigRepository) Delete(ctx context.Context, key string) error {
	query := `DELETE FROM config WHERE key = $1`
	result, err := r.db.ExecContext(ctx, query, key)
	if err != nil {
		return fmt.Errorf("failed to delete config: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("config key not found: %s", key)
	}

	return nil
}

// CheckpointRepository handles scan checkpoint operations
type CheckpointRepository struct {
	db *sqlx.DB
}

// Get retrieves a checkpoint for a scan
func (r *CheckpointRepository) Get(ctx context.Context, scanID int64) (*ScanCheckpoint, error) {
	var checkpoint ScanCheckpoint
	query := `SELECT * FROM scan_checkpoints WHERE scan_id = $1`
	if err := r.db.GetContext(ctx, &checkpoint, query, scanID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No checkpoint is not an error
		}
		return nil, fmt.Errorf("failed to get checkpoint: %w", err)
	}
	return &checkpoint, nil
}

// Create creates or updates a checkpoint
func (r *CheckpointRepository) Create(ctx context.Context, checkpoint *ScanCheckpoint) error {
	query := `
		INSERT INTO scan_checkpoints (scan_id, last_processed_path, files_processed, checkpoint_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (scan_id) DO UPDATE SET
			last_processed_path = EXCLUDED.last_processed_path,
			files_processed = EXCLUDED.files_processed,
			checkpoint_at = NOW()
		RETURNING checkpoint_at`

	err := r.db.QueryRowContext(
		ctx, query,
		checkpoint.ScanID, checkpoint.LastProcessedPath, checkpoint.FilesProcessed,
	).Scan(&checkpoint.CheckpointAt)

	if err != nil {
		return fmt.Errorf("failed to create checkpoint: %w", err)
	}

	return nil
}

// Delete deletes a checkpoint
func (r *CheckpointRepository) Delete(ctx context.Context, scanID int64) error {
	query := `DELETE FROM scan_checkpoints WHERE scan_id = $1`
	_, err := r.db.ExecContext(ctx, query, scanID)
	if err != nil {
		return fmt.Errorf("failed to delete checkpoint: %w", err)
	}
	return nil
}
