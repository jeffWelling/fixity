package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// ScanRepository handles scan operations
type ScanRepository struct {
	db *sqlx.DB
}

// ScanFilters holds filtering options for scan queries
type ScanFilters struct {
	StorageTargetID *int64
	Status          *ScanStatus
	LargeChangeOnly bool
	Limit           int
	Offset          int
}

// GetByID retrieves a scan by ID
func (r *ScanRepository) GetByID(ctx context.Context, id int64) (*Scan, error) {
	var scan Scan
	query := `SELECT * FROM scans WHERE id = $1`
	if err := r.db.GetContext(ctx, &scan, query, id); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("scan not found: %d", id)
		}
		return nil, fmt.Errorf("failed to get scan: %w", err)
	}
	return &scan, nil
}

// List retrieves scans matching the given filters
func (r *ScanRepository) List(ctx context.Context, filters ScanFilters) ([]*Scan, error) {
	query := `SELECT * FROM scans WHERE 1=1`
	args := []interface{}{}
	argNum := 1

	if filters.StorageTargetID != nil {
		query += fmt.Sprintf(" AND storage_target_id = $%d", argNum)
		args = append(args, *filters.StorageTargetID)
		argNum++
	}

	if filters.Status != nil {
		query += fmt.Sprintf(" AND status = $%d", argNum)
		args = append(args, *filters.Status)
		argNum++
	}

	if filters.LargeChangeOnly {
		query += " AND is_large_change = TRUE"
	}

	query += " ORDER BY started_at DESC"

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

	var scans []*Scan
	if err := r.db.SelectContext(ctx, &scans, query, args...); err != nil {
		return nil, fmt.Errorf("failed to list scans: %w", err)
	}

	return scans, nil
}

// GetLatest retrieves the most recent scan for a storage target
func (r *ScanRepository) GetLatest(ctx context.Context, targetID int64) (*Scan, error) {
	var scan Scan
	query := `
		SELECT * FROM scans
		WHERE storage_target_id = $1
		ORDER BY started_at DESC
		LIMIT 1`

	if err := r.db.GetContext(ctx, &scan, query, targetID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No scans yet is not an error
		}
		return nil, fmt.Errorf("failed to get latest scan: %w", err)
	}

	return &scan, nil
}

// Create creates a new scan record
func (r *ScanRepository) Create(ctx context.Context, scan *Scan) error {
	query := `
		INSERT INTO scans (
			storage_target_id, status, started_at, completed_at,
			files_scanned, files_added, files_deleted, files_modified, files_verified,
			errors_count, error_messages, is_large_change, resumed_from,
			created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, NOW()
		) RETURNING id, created_at`

	err := r.db.QueryRowContext(
		ctx, query,
		scan.StorageTargetID, scan.Status, scan.StartedAt, scan.CompletedAt,
		scan.FilesScanned, scan.FilesAdded, scan.FilesDeleted, scan.FilesModified, scan.FilesVerified,
		scan.ErrorsCount, scan.ErrorMessages, scan.IsLargeChange, scan.ResumedFrom,
	).Scan(&scan.ID, &scan.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create scan: %w", err)
	}

	return nil
}

// Update updates an existing scan record
func (r *ScanRepository) Update(ctx context.Context, scan *Scan) error {
	query := `
		UPDATE scans SET
			status = $2,
			completed_at = $3,
			files_scanned = $4,
			files_added = $5,
			files_deleted = $6,
			files_modified = $7,
			files_verified = $8,
			errors_count = $9,
			error_messages = $10,
			is_large_change = $11
		WHERE id = $1`

	result, err := r.db.ExecContext(
		ctx, query,
		scan.ID, scan.Status, scan.CompletedAt,
		scan.FilesScanned, scan.FilesAdded, scan.FilesDeleted, scan.FilesModified, scan.FilesVerified,
		scan.ErrorsCount, scan.ErrorMessages, scan.IsLargeChange,
	)

	if err != nil {
		return fmt.Errorf("failed to update scan: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("scan not found: %d", scan.ID)
	}

	return nil
}

// AddError adds an error message to a scan
func (r *ScanRepository) AddError(ctx context.Context, scanID int64, errorMsg string) error {
	query := `
		UPDATE scans SET
			errors_count = errors_count + 1,
			error_messages = array_append(error_messages, $2)
		WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, scanID, errorMsg)
	if err != nil {
		return fmt.Errorf("failed to add error to scan: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("scan not found: %d", scanID)
	}

	return nil
}

// GetIncomplete retrieves all incomplete scans (status = running)
func (r *ScanRepository) GetIncomplete(ctx context.Context) ([]*Scan, error) {
	query := `SELECT * FROM scans WHERE status = 'running' ORDER BY started_at`

	var scans []*Scan
	if err := r.db.SelectContext(ctx, &scans, query); err != nil {
		return nil, fmt.Errorf("failed to get incomplete scans: %w", err)
	}

	return scans, nil
}
