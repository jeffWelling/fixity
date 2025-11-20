package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// WebhookRepository handles webhook operations
type WebhookRepository struct {
	db *sqlx.DB
}

// GetByID retrieves a webhook by ID
func (r *WebhookRepository) GetByID(ctx context.Context, id int64) (*Webhook, error) {
	var webhook Webhook
	query := `SELECT * FROM webhooks WHERE id = $1`
	if err := r.db.GetContext(ctx, &webhook, query, id); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("webhook not found: %d", id)
		}
		return nil, fmt.Errorf("failed to get webhook: %w", err)
	}
	return &webhook, nil
}

// ListAll retrieves all webhooks
func (r *WebhookRepository) ListAll(ctx context.Context) ([]*Webhook, error) {
	query := `SELECT * FROM webhooks ORDER BY name`

	var webhooks []*Webhook
	if err := r.db.SelectContext(ctx, &webhooks, query); err != nil {
		return nil, fmt.Errorf("failed to list webhooks: %w", err)
	}

	return webhooks, nil
}

// ListEnabled retrieves all enabled webhooks
func (r *WebhookRepository) ListEnabled(ctx context.Context) ([]*Webhook, error) {
	query := `SELECT * FROM webhooks WHERE enabled = TRUE ORDER BY name`

	var webhooks []*Webhook
	if err := r.db.SelectContext(ctx, &webhooks, query); err != nil {
		return nil, fmt.Errorf("failed to list enabled webhooks: %w", err)
	}

	return webhooks, nil
}

// Create creates a new webhook
func (r *WebhookRepository) Create(ctx context.Context, webhook *Webhook) error {
	query := `
		INSERT INTO webhooks (
			name, url, enabled, event_includes, event_excludes,
			retry_attempts, retry_backoff_sec, timeout_sec,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW()
		) RETURNING id, created_at, updated_at`

	err := r.db.QueryRowContext(
		ctx, query,
		webhook.Name, webhook.URL, webhook.Enabled,
		webhook.EventIncludes, webhook.EventExcludes,
		webhook.RetryAttempts, webhook.RetryBackoffSec, webhook.TimeoutSec,
	).Scan(&webhook.ID, &webhook.CreatedAt, &webhook.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create webhook: %w", err)
	}

	return nil
}

// Update updates an existing webhook
func (r *WebhookRepository) Update(ctx context.Context, webhook *Webhook) error {
	query := `
		UPDATE webhooks SET
			name = $2,
			url = $3,
			enabled = $4,
			event_includes = $5,
			event_excludes = $6,
			retry_attempts = $7,
			retry_backoff_sec = $8,
			timeout_sec = $9,
			updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at`

	err := r.db.QueryRowContext(
		ctx, query,
		webhook.ID, webhook.Name, webhook.URL, webhook.Enabled,
		webhook.EventIncludes, webhook.EventExcludes,
		webhook.RetryAttempts, webhook.RetryBackoffSec, webhook.TimeoutSec,
	).Scan(&webhook.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to update webhook: %w", err)
	}

	return nil
}

// Delete deletes a webhook
func (r *WebhookRepository) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM webhooks WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete webhook: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("webhook not found: %d", id)
	}

	return nil
}

// WebhookDeliveryRepository handles webhook delivery tracking
type WebhookDeliveryRepository struct {
	db *sqlx.DB
}

// Create creates a new webhook delivery record
func (r *WebhookDeliveryRepository) Create(ctx context.Context, delivery *WebhookDelivery) error {
	query := `
		INSERT INTO webhook_deliveries (
			webhook_id, event_type, payload, status, attempt,
			created_at
		) VALUES (
			$1, $2, $3, $4, $5, NOW()
		) RETURNING id, created_at`

	err := r.db.QueryRowContext(
		ctx, query,
		delivery.WebhookID, delivery.EventType, delivery.Payload,
		delivery.Status, delivery.Attempt,
	).Scan(&delivery.ID, &delivery.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create webhook delivery: %w", err)
	}

	return nil
}

// Update updates a webhook delivery record
func (r *WebhookDeliveryRepository) Update(ctx context.Context, delivery *WebhookDelivery) error {
	query := `
		UPDATE webhook_deliveries SET
			status = $2,
			attempt = $3,
			last_attempt_at = $4,
			next_retry_at = $5,
			delivered_at = $6,
			error_message = $7
		WHERE id = $1`

	result, err := r.db.ExecContext(
		ctx, query,
		delivery.ID, delivery.Status, delivery.Attempt,
		delivery.LastAttemptAt, delivery.NextRetryAt,
		delivery.DeliveredAt, delivery.ErrorMessage,
	)

	if err != nil {
		return fmt.Errorf("failed to update webhook delivery: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("webhook delivery not found: %d", delivery.ID)
	}

	return nil
}

// GetPending retrieves pending webhook deliveries ready for retry
func (r *WebhookDeliveryRepository) GetPending(ctx context.Context, limit int) ([]*WebhookDelivery, error) {
	query := `
		SELECT * FROM webhook_deliveries
		WHERE status = 'pending'
		  AND (next_retry_at IS NULL OR next_retry_at <= NOW())
		ORDER BY created_at
		LIMIT $1`

	var deliveries []*WebhookDelivery
	if err := r.db.SelectContext(ctx, &deliveries, query, limit); err != nil {
		return nil, fmt.Errorf("failed to get pending deliveries: %w", err)
	}

	return deliveries, nil
}
