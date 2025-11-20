package database

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// ChangeEventRepository handles change event operations
type ChangeEventRepository struct {
	db *sqlx.DB
}

// ChangeEventFilters holds filtering options for change event queries
type ChangeEventFilters struct {
	ScanID     *int64
	FileID     *int64
	EventTypes []ChangeEventType
	Limit      int
	Offset     int
}

// GetByID retrieves a change event by ID
func (r *ChangeEventRepository) GetByID(ctx context.Context, id int64) (*ChangeEvent, error) {
	var event ChangeEvent
	query := `SELECT * FROM change_events WHERE id = $1`
	if err := r.db.GetContext(ctx, &event, query, id); err != nil {
		return nil, fmt.Errorf("failed to get change event: %w", err)
	}
	return &event, nil
}

// List retrieves change events matching the given filters
func (r *ChangeEventRepository) List(ctx context.Context, filters ChangeEventFilters) ([]*ChangeEvent, error) {
	query := `SELECT * FROM change_events WHERE 1=1`
	args := []interface{}{}
	argNum := 1

	if filters.ScanID != nil {
		query += fmt.Sprintf(" AND scan_id = $%d", argNum)
		args = append(args, *filters.ScanID)
		argNum++
	}

	if filters.FileID != nil {
		query += fmt.Sprintf(" AND file_id = $%d", argNum)
		args = append(args, *filters.FileID)
		argNum++
	}

	if len(filters.EventTypes) > 0 {
		query += fmt.Sprintf(" AND event_type = ANY($%d)", argNum)
		args = append(args, pq.Array(filters.EventTypes))
		argNum++
	}

	query += " ORDER BY detected_at DESC"

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

	var events []*ChangeEvent
	if err := r.db.SelectContext(ctx, &events, query, args...); err != nil {
		return nil, fmt.Errorf("failed to list change events: %w", err)
	}

	return events, nil
}

// GetByScan retrieves all change events for a scan
func (r *ChangeEventRepository) GetByScan(ctx context.Context, scanID int64) ([]*ChangeEvent, error) {
	return r.List(ctx, ChangeEventFilters{ScanID: &scanID})
}

// GetByFile retrieves all change events for a file (lifecycle history)
func (r *ChangeEventRepository) GetByFile(ctx context.Context, fileID int64) ([]*ChangeEvent, error) {
	return r.List(ctx, ChangeEventFilters{FileID: &fileID})
}

// Create creates a new change event record
func (r *ChangeEventRepository) Create(ctx context.Context, event *ChangeEvent) error {
	query := `
		INSERT INTO change_events (
			scan_id, file_id, event_type, detected_at,
			old_checksum, new_checksum, old_size, new_size,
			created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, NOW()
		) RETURNING id, created_at`

	err := r.db.QueryRowContext(
		ctx, query,
		event.ScanID, event.FileID, event.EventType, event.DetectedAt,
		event.OldChecksum, event.NewChecksum, event.OldSize, event.NewSize,
	).Scan(&event.ID, &event.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create change event: %w", err)
	}

	return nil
}

// CreateBatch creates multiple change event records in a single transaction
func (r *ChangeEventRepository) CreateBatch(ctx context.Context, events []*ChangeEvent) error {
	if len(events) == 0 {
		return nil
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `
		INSERT INTO change_events (
			scan_id, file_id, event_type, detected_at,
			old_checksum, new_checksum, old_size, new_size,
			created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, NOW()
		) RETURNING id, created_at`

	stmt, err := tx.PreparexContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, event := range events {
		err := stmt.QueryRowContext(
			ctx,
			event.ScanID, event.FileID, event.EventType, event.DetectedAt,
			event.OldChecksum, event.NewChecksum, event.OldSize, event.NewSize,
		).Scan(&event.ID, &event.CreatedAt)

		if err != nil {
			return fmt.Errorf("failed to insert change event: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// CountByType returns counts of change events by type for a scan
func (r *ChangeEventRepository) CountByType(ctx context.Context, scanID int64) (map[ChangeEventType]int64, error) {
	query := `
		SELECT event_type, COUNT(*) as count
		FROM change_events
		WHERE scan_id = $1
		GROUP BY event_type`

	rows, err := r.db.QueryContext(ctx, query, scanID)
	if err != nil {
		return nil, fmt.Errorf("failed to count events by type: %w", err)
	}
	defer rows.Close()

	counts := make(map[ChangeEventType]int64)
	for rows.Next() {
		var eventType ChangeEventType
		var count int64
		if err := rows.Scan(&eventType, &count); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		counts[eventType] = count
	}

	return counts, nil
}
