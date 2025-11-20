package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// FileRepository handles file operations
type FileRepository struct {
	db *sqlx.DB
}

// FileFilters holds filtering options for file queries
type FileFilters struct {
	StorageTargetID *int64
	PathPattern     *string
	DeletedOnly     bool
	ActiveOnly      bool
	MinSize         *int64
	MaxSize         *int64
	FirstSeenAfter  *time.Time
	FirstSeenBefore *time.Time
	LastSeenAfter   *time.Time
	LastSeenBefore  *time.Time
	Limit           int
	Offset          int
}

// GetByID retrieves a file by ID
func (r *FileRepository) GetByID(ctx context.Context, id int64) (*File, error) {
	var file File
	query := `SELECT * FROM files WHERE id = $1`
	if err := r.db.GetContext(ctx, &file, query, id); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("file not found: %d", id)
		}
		return nil, fmt.Errorf("failed to get file: %w", err)
	}
	return &file, nil
}

// GetByPath retrieves a file by storage target and path
func (r *FileRepository) GetByPath(ctx context.Context, targetID int64, path string) (*File, error) {
	var file File
	query := `SELECT * FROM files WHERE storage_target_id = $1 AND path = $2`
	if err := r.db.GetContext(ctx, &file, query, targetID, path); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found is not an error for path lookups
		}
		return nil, fmt.Errorf("failed to get file by path: %w", err)
	}
	return &file, nil
}

// List retrieves files matching the given filters
func (r *FileRepository) List(ctx context.Context, filters FileFilters) ([]*File, error) {
	query := `SELECT * FROM files WHERE 1=1`
	args := []interface{}{}
	argNum := 1

	if filters.StorageTargetID != nil {
		query += fmt.Sprintf(" AND storage_target_id = $%d", argNum)
		args = append(args, *filters.StorageTargetID)
		argNum++
	}

	if filters.PathPattern != nil {
		query += fmt.Sprintf(" AND path LIKE $%d", argNum)
		args = append(args, *filters.PathPattern)
		argNum++
	}

	if filters.DeletedOnly {
		query += " AND deleted_at IS NOT NULL"
	} else if filters.ActiveOnly {
		query += " AND deleted_at IS NULL"
	}

	if filters.MinSize != nil {
		query += fmt.Sprintf(" AND size >= $%d", argNum)
		args = append(args, *filters.MinSize)
		argNum++
	}

	if filters.MaxSize != nil {
		query += fmt.Sprintf(" AND size <= $%d", argNum)
		args = append(args, *filters.MaxSize)
		argNum++
	}

	if filters.FirstSeenAfter != nil {
		query += fmt.Sprintf(" AND first_seen >= $%d", argNum)
		args = append(args, *filters.FirstSeenAfter)
		argNum++
	}

	if filters.FirstSeenBefore != nil {
		query += fmt.Sprintf(" AND first_seen <= $%d", argNum)
		args = append(args, *filters.FirstSeenBefore)
		argNum++
	}

	if filters.LastSeenAfter != nil {
		query += fmt.Sprintf(" AND last_seen >= $%d", argNum)
		args = append(args, *filters.LastSeenAfter)
		argNum++
	}

	if filters.LastSeenBefore != nil {
		query += fmt.Sprintf(" AND last_seen <= $%d", argNum)
		args = append(args, *filters.LastSeenBefore)
		argNum++
	}

	query += " ORDER BY path"

	if filters.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argNum)
		args = append(args, filters.Limit)
		argNum++
	}

	if filters.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argNum)
		args = append(args, filters.Offset)
		argNum++
	}

	var files []*File
	if err := r.db.SelectContext(ctx, &files, query, args...); err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	return files, nil
}

// Count returns the number of files matching the given filters
func (r *FileRepository) Count(ctx context.Context, filters FileFilters) (int64, error) {
	query := `SELECT COUNT(*) FROM files WHERE 1=1`
	args := []interface{}{}
	argNum := 1

	if filters.StorageTargetID != nil {
		query += fmt.Sprintf(" AND storage_target_id = $%d", argNum)
		args = append(args, *filters.StorageTargetID)
		argNum++
	}

	if filters.DeletedOnly {
		query += " AND deleted_at IS NOT NULL"
	} else if filters.ActiveOnly {
		query += " AND deleted_at IS NULL"
	}

	var count int64
	if err := r.db.GetContext(ctx, &count, query, args...); err != nil {
		return 0, fmt.Errorf("failed to count files: %w", err)
	}

	return count, nil
}

// Create creates a new file record
func (r *FileRepository) Create(ctx context.Context, file *File) error {
	query := `
		INSERT INTO files (
			storage_target_id, path, size, first_seen, last_seen,
			current_checksum, checksum_type, last_checksummed_at,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW()
		) RETURNING id, created_at, updated_at`

	err := r.db.QueryRowContext(
		ctx, query,
		file.StorageTargetID, file.Path, file.Size, file.FirstSeen, file.LastSeen,
		file.CurrentChecksum, file.ChecksumType, file.LastChecksummedAt,
	).Scan(&file.ID, &file.CreatedAt, &file.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	return nil
}

// CreateBatch creates multiple file records in a single transaction
func (r *FileRepository) CreateBatch(ctx context.Context, files []*File) error {
	if len(files) == 0 {
		return nil
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `
		INSERT INTO files (
			storage_target_id, path, size, first_seen, last_seen,
			current_checksum, checksum_type, last_checksummed_at,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW()
		) RETURNING id, created_at, updated_at`

	stmt, err := tx.PreparexContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, file := range files {
		err := stmt.QueryRowContext(
			ctx,
			file.StorageTargetID, file.Path, file.Size, file.FirstSeen, file.LastSeen,
			file.CurrentChecksum, file.ChecksumType, file.LastChecksummedAt,
		).Scan(&file.ID, &file.CreatedAt, &file.UpdatedAt)

		if err != nil {
			return fmt.Errorf("failed to insert file %s: %w", file.Path, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Update updates an existing file record
func (r *FileRepository) Update(ctx context.Context, file *File) error {
	query := `
		UPDATE files SET
			size = $2,
			last_seen = $3,
			current_checksum = $4,
			checksum_type = $5,
			last_checksummed_at = $6,
			deleted_at = $7,
			updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at`

	err := r.db.QueryRowContext(
		ctx, query,
		file.ID, file.Size, file.LastSeen,
		file.CurrentChecksum, file.ChecksumType, file.LastChecksummedAt,
		file.DeletedAt,
	).Scan(&file.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to update file: %w", err)
	}

	return nil
}

// Delete hard deletes a file record (not recommended, use soft delete via Update)
func (r *FileRepository) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM files WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("file not found: %d", id)
	}

	return nil
}

// GetUnverifiedFiles returns files that haven't been checksummed recently
// Used for weighted random sampling
func (r *FileRepository) GetUnverifiedFiles(
	ctx context.Context,
	targetID int64,
	limit int,
) ([]*File, error) {
	query := `
		SELECT * FROM files
		WHERE storage_target_id = $1
		  AND deleted_at IS NULL
		ORDER BY last_checksummed_at ASC NULLS FIRST, path
		LIMIT $2`

	var files []*File
	if err := r.db.SelectContext(ctx, &files, query, targetID, limit); err != nil {
		return nil, fmt.Errorf("failed to get unverified files: %w", err)
	}

	return files, nil
}

// VerificationStats holds statistics about file verification status
type VerificationStats struct {
	TotalFiles          int64 `db:"total_files"`
	VerifiedLast7Days   int64 `db:"verified_last_7_days"`
	VerifiedLast30Days  int64 `db:"verified_last_30_days"`
	VerifiedLast90Days  int64 `db:"verified_last_90_days"`
	NeverVerified       int64 `db:"never_verified"`
	OldestVerification  *time.Time `db:"oldest_verification"`
}

// GetVerificationStats returns verification statistics for a storage target
func (r *FileRepository) GetVerificationStats(
	ctx context.Context,
	targetID int64,
) (*VerificationStats, error) {
	query := `
		SELECT
			COUNT(*) as total_files,
			COUNT(*) FILTER (WHERE last_checksummed_at >= NOW() - INTERVAL '7 days') as verified_last_7_days,
			COUNT(*) FILTER (WHERE last_checksummed_at >= NOW() - INTERVAL '30 days') as verified_last_30_days,
			COUNT(*) FILTER (WHERE last_checksummed_at >= NOW() - INTERVAL '90 days') as verified_last_90_days,
			COUNT(*) FILTER (WHERE last_checksummed_at IS NULL) as never_verified,
			MIN(last_checksummed_at) as oldest_verification
		FROM files
		WHERE storage_target_id = $1 AND deleted_at IS NULL`

	var stats VerificationStats
	if err := r.db.GetContext(ctx, &stats, query, targetID); err != nil {
		return nil, fmt.Errorf("failed to get verification stats: %w", err)
	}

	return &stats, nil
}
