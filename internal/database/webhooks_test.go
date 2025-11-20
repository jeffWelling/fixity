package database_test

import (
	"context"
	"testing"

	"github.com/jeffanddom/fixity/internal/database"
	"github.com/jeffanddom/fixity/tests/testutil"
	"github.com/lib/pq"
)

func TestWebhookRepository_Create(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	t.Run("creates webhook successfully", func(t *testing.T) {
		webhook := &database.Webhook{
			Name:             "test-webhook",
			URL:              "https://example.com/webhook",
			Enabled:          true,
			EventIncludes:    pq.StringArray{"file.added", "file.deleted"},
			EventExcludes:    pq.StringArray{},
			RetryAttempts:    3,
			RetryBackoffSec:  60,
			TimeoutSec:       30,
		}

		err := db.Webhooks.Create(context.Background(), webhook)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if webhook.ID == 0 {
			t.Error("expected ID to be set")
		}

		if webhook.CreatedAt.IsZero() {
			t.Error("expected CreatedAt to be set")
		}

		if webhook.UpdatedAt.IsZero() {
			t.Error("expected UpdatedAt to be set")
		}
	})

	t.Run("creates webhook with event filters", func(t *testing.T) {
		webhook := &database.Webhook{
			Name:             "filtered-webhook",
			URL:              "https://example.com/filtered",
			Enabled:          true,
			EventIncludes:    pq.StringArray{"file.added", "file.modified"},
			EventExcludes:    pq.StringArray{"file.verified"},
			RetryAttempts:    5,
			RetryBackoffSec:  120,
			TimeoutSec:       60,
		}

		err := db.Webhooks.Create(context.Background(), webhook)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify event filters persisted
		retrieved, err := db.Webhooks.GetByID(context.Background(), webhook.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(retrieved.EventIncludes) != 2 {
			t.Errorf("expected 2 event includes, got %d", len(retrieved.EventIncludes))
		}

		if len(retrieved.EventExcludes) != 1 {
			t.Errorf("expected 1 event exclude, got %d", len(retrieved.EventExcludes))
		}
	})

	t.Run("creates disabled webhook", func(t *testing.T) {
		webhook := &database.Webhook{
			Name:             "disabled-webhook",
			URL:              "https://example.com/disabled",
			Enabled:          false,
			EventIncludes:    pq.StringArray{},
			EventExcludes:    pq.StringArray{},
			RetryAttempts:    3,
			RetryBackoffSec:  60,
			TimeoutSec:       30,
		}

		err := db.Webhooks.Create(context.Background(), webhook)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		retrieved, err := db.Webhooks.GetByID(context.Background(), webhook.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if retrieved.Enabled {
			t.Error("expected webhook to be disabled")
		}
	})
}

func TestWebhookRepository_GetByID(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	webhook := &database.Webhook{
		Name:             "test-webhook",
		URL:              "https://example.com/webhook",
		Enabled:          true,
		EventIncludes:    pq.StringArray{"file.added"},
		EventExcludes:    pq.StringArray{},
		RetryAttempts:    3,
		RetryBackoffSec:  60,
		TimeoutSec:       30,
	}
	db.Webhooks.Create(context.Background(), webhook)

	t.Run("retrieves webhook by ID", func(t *testing.T) {
		retrieved, err := db.Webhooks.GetByID(context.Background(), webhook.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if retrieved.Name != webhook.Name {
			t.Errorf("expected name %s, got %s", webhook.Name, retrieved.Name)
		}

		if retrieved.URL != webhook.URL {
			t.Errorf("expected URL %s, got %s", webhook.URL, retrieved.URL)
		}

		if retrieved.RetryAttempts != webhook.RetryAttempts {
			t.Errorf("expected retry attempts %d, got %d", webhook.RetryAttempts, retrieved.RetryAttempts)
		}
	})

	t.Run("returns error for non-existent ID", func(t *testing.T) {
		_, err := db.Webhooks.GetByID(context.Background(), 999999)
		if err == nil {
			t.Error("expected error for non-existent ID")
		}
	})
}

func TestWebhookRepository_ListAll(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	// Create webhooks with different states
	webhook1 := &database.Webhook{
		Name:             "webhook-a",
		URL:              "https://example.com/a",
		Enabled:          true,
		EventIncludes:    pq.StringArray{},
		EventExcludes:    pq.StringArray{},
		RetryAttempts:    3,
		RetryBackoffSec:  60,
		TimeoutSec:       30,
	}
	db.Webhooks.Create(context.Background(), webhook1)

	webhook2 := &database.Webhook{
		Name:             "webhook-b",
		URL:              "https://example.com/b",
		Enabled:          false,
		EventIncludes:    pq.StringArray{},
		EventExcludes:    pq.StringArray{},
		RetryAttempts:    3,
		RetryBackoffSec:  60,
		TimeoutSec:       30,
	}
	db.Webhooks.Create(context.Background(), webhook2)

	t.Run("lists all webhooks", func(t *testing.T) {
		webhooks, err := db.Webhooks.ListAll(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(webhooks) != 2 {
			t.Errorf("expected 2 webhooks, got %d", len(webhooks))
		}
	})

	t.Run("orders webhooks by name", func(t *testing.T) {
		webhooks, err := db.Webhooks.ListAll(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(webhooks) >= 2 {
			if webhooks[0].Name > webhooks[1].Name {
				t.Error("webhooks not ordered by name")
			}
		}
	})

	t.Run("returns empty list when no webhooks", func(t *testing.T) {
		testutil.CleanupDB(t, db)

		webhooks, err := db.Webhooks.ListAll(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(webhooks) != 0 {
			t.Errorf("expected 0 webhooks, got %d", len(webhooks))
		}
	})
}

func TestWebhookRepository_ListEnabled(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	// Create enabled webhooks
	enabled1 := &database.Webhook{
		Name:             "enabled-1",
		URL:              "https://example.com/enabled1",
		Enabled:          true,
		EventIncludes:    pq.StringArray{},
		EventExcludes:    pq.StringArray{},
		RetryAttempts:    3,
		RetryBackoffSec:  60,
		TimeoutSec:       30,
	}
	db.Webhooks.Create(context.Background(), enabled1)

	enabled2 := &database.Webhook{
		Name:             "enabled-2",
		URL:              "https://example.com/enabled2",
		Enabled:          true,
		EventIncludes:    pq.StringArray{},
		EventExcludes:    pq.StringArray{},
		RetryAttempts:    3,
		RetryBackoffSec:  60,
		TimeoutSec:       30,
	}
	db.Webhooks.Create(context.Background(), enabled2)

	// Create disabled webhook
	disabled := &database.Webhook{
		Name:             "disabled",
		URL:              "https://example.com/disabled",
		Enabled:          false,
		EventIncludes:    pq.StringArray{},
		EventExcludes:    pq.StringArray{},
		RetryAttempts:    3,
		RetryBackoffSec:  60,
		TimeoutSec:       30,
	}
	db.Webhooks.Create(context.Background(), disabled)

	t.Run("lists only enabled webhooks", func(t *testing.T) {
		webhooks, err := db.Webhooks.ListEnabled(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(webhooks) != 2 {
			t.Errorf("expected 2 enabled webhooks, got %d", len(webhooks))
		}

		for _, webhook := range webhooks {
			if !webhook.Enabled {
				t.Errorf("expected all webhooks to be enabled, found disabled: %s", webhook.Name)
			}
		}
	})
}

func TestWebhookRepository_Update(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	webhook := &database.Webhook{
		Name:             "test-webhook",
		URL:              "https://example.com/webhook",
		Enabled:          true,
		EventIncludes:    pq.StringArray{"file.added"},
		EventExcludes:    pq.StringArray{},
		RetryAttempts:    3,
		RetryBackoffSec:  60,
		TimeoutSec:       30,
	}
	db.Webhooks.Create(context.Background(), webhook)

	t.Run("updates webhook successfully", func(t *testing.T) {
		webhook.Name = "updated-webhook"
		webhook.URL = "https://example.com/updated"
		webhook.Enabled = false
		webhook.EventIncludes = pq.StringArray{"file.added", "file.deleted"}
		webhook.EventExcludes = pq.StringArray{"file.verified"}
		webhook.RetryAttempts = 5
		webhook.RetryBackoffSec = 120
		webhook.TimeoutSec = 60

		err := db.Webhooks.Update(context.Background(), webhook)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify update
		updated, err := db.Webhooks.GetByID(context.Background(), webhook.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if updated.Name != "updated-webhook" {
			t.Errorf("expected name updated-webhook, got %s", updated.Name)
		}

		if updated.URL != "https://example.com/updated" {
			t.Errorf("expected updated URL, got %s", updated.URL)
		}

		if updated.Enabled {
			t.Error("expected webhook to be disabled")
		}

		if len(updated.EventIncludes) != 2 {
			t.Errorf("expected 2 event includes, got %d", len(updated.EventIncludes))
		}

		if updated.RetryAttempts != 5 {
			t.Errorf("expected 5 retry attempts, got %d", updated.RetryAttempts)
		}

		if !updated.UpdatedAt.After(webhook.CreatedAt) {
			t.Error("expected updated_at to be after created_at")
		}
	})

	t.Run("returns error for non-existent webhook", func(t *testing.T) {
		nonExistent := &database.Webhook{
			ID:   999999,
			Name: "nonexistent",
		}

		err := db.Webhooks.Update(context.Background(), nonExistent)
		if err == nil {
			t.Error("expected error for non-existent webhook")
		}
	})
}

func TestWebhookRepository_Delete(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	webhook := &database.Webhook{
		Name:             "test-webhook",
		URL:              "https://example.com/webhook",
		Enabled:          true,
		EventIncludes:    pq.StringArray{},
		EventExcludes:    pq.StringArray{},
		RetryAttempts:    3,
		RetryBackoffSec:  60,
		TimeoutSec:       30,
	}
	db.Webhooks.Create(context.Background(), webhook)

	t.Run("deletes webhook successfully", func(t *testing.T) {
		err := db.Webhooks.Delete(context.Background(), webhook.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify deletion
		_, err = db.Webhooks.GetByID(context.Background(), webhook.ID)
		if err == nil {
			t.Error("expected error when getting deleted webhook")
		}
	})

	t.Run("returns error for non-existent webhook", func(t *testing.T) {
		err := db.Webhooks.Delete(context.Background(), 999999)
		if err == nil {
			t.Error("expected error for non-existent webhook")
		}
	})
}

func TestWebhookDeliveryRepository_Create(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	webhook := &database.Webhook{
		Name:             "test-webhook",
		URL:              "https://example.com/webhook",
		Enabled:          true,
		EventIncludes:    pq.StringArray{},
		EventExcludes:    pq.StringArray{},
		RetryAttempts:    3,
		RetryBackoffSec:  60,
		TimeoutSec:       30,
	}
	db.Webhooks.Create(context.Background(), webhook)

	t.Run("creates delivery successfully", func(t *testing.T) {
		delivery := &database.WebhookDelivery{
			WebhookID: webhook.ID,
			EventType: "file.added",
			Payload:   []byte(`{"file": "/test/file.txt", "size": 1024}`),
			Status:    database.DeliveryStatusPending,
			Attempt:   1,
		}

		err := db.WebhookDeliveries.Create(context.Background(), delivery)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if delivery.ID == 0 {
			t.Error("expected ID to be set")
		}

		if delivery.CreatedAt.IsZero() {
			t.Error("expected CreatedAt to be set")
		}
	})

	t.Run("creates delivery with pending status", func(t *testing.T) {
		delivery := &database.WebhookDelivery{
			WebhookID: webhook.ID,
			EventType: "file.deleted",
			Payload:   []byte(`{"file": "/test/deleted.txt"}`),
			Status:    database.DeliveryStatusPending,
			Attempt:   1,
		}

		err := db.WebhookDeliveries.Create(context.Background(), delivery)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if delivery.Status != database.DeliveryStatusPending {
			t.Errorf("expected pending status, got %s", delivery.Status)
		}
	})
}

func TestWebhookDeliveryRepository_Update(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	webhook := &database.Webhook{
		Name:             "test-webhook",
		URL:              "https://example.com/webhook",
		Enabled:          true,
		EventIncludes:    pq.StringArray{},
		EventExcludes:    pq.StringArray{},
		RetryAttempts:    3,
		RetryBackoffSec:  60,
		TimeoutSec:       30,
	}
	db.Webhooks.Create(context.Background(), webhook)

	delivery := &database.WebhookDelivery{
		WebhookID: webhook.ID,
		EventType: "file.added",
		Payload:   []byte(`{"file": "/test/file.txt"}`),
		Status:    database.DeliveryStatusPending,
		Attempt:   1,
	}
	db.WebhookDeliveries.Create(context.Background(), delivery)

	t.Run("updates delivery status", func(t *testing.T) {
		now := testutil.TimeNow()
		delivery.Status = database.DeliveryStatusDelivered
		delivery.DeliveredAt = &now

		err := db.WebhookDeliveries.Update(context.Background(), delivery)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("updates delivery with error", func(t *testing.T) {
		now := testutil.TimeNow()
		errorMsg := "connection timeout"
		delivery.Status = database.DeliveryStatusFailed
		delivery.Attempt = 2
		delivery.LastAttemptAt = &now
		delivery.ErrorMessage = &errorMsg

		err := db.WebhookDeliveries.Update(context.Background(), delivery)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("returns error for non-existent delivery", func(t *testing.T) {
		nonExistent := &database.WebhookDelivery{
			ID:     999999,
			Status: database.DeliveryStatusDelivered,
		}

		err := db.WebhookDeliveries.Update(context.Background(), nonExistent)
		if err == nil {
			t.Error("expected error for non-existent delivery")
		}
	})
}

func TestWebhookDeliveryRepository_GetPending(t *testing.T) {
	db := testutil.NewTestDB(t)
	defer db.Close()
	defer testutil.CleanupDB(t, db)

	webhook := &database.Webhook{
		Name:             "test-webhook",
		URL:              "https://example.com/webhook",
		Enabled:          true,
		EventIncludes:    pq.StringArray{},
		EventExcludes:    pq.StringArray{},
		RetryAttempts:    3,
		RetryBackoffSec:  60,
		TimeoutSec:       30,
	}
	db.Webhooks.Create(context.Background(), webhook)

	// Create pending deliveries
	for i := 0; i < 3; i++ {
		delivery := &database.WebhookDelivery{
			WebhookID: webhook.ID,
			EventType: "file.added",
			Payload:   []byte(`{"file": "/test/file.txt"}`),
			Status:    database.DeliveryStatusPending,
			Attempt:   1,
		}
		db.WebhookDeliveries.Create(context.Background(), delivery)
	}

	// Create delivered delivery (should not be returned)
	now := testutil.TimeNow()
	delivered := &database.WebhookDelivery{
		WebhookID:   webhook.ID,
		EventType:   "file.added",
		Payload:     []byte(`{"file": "/test/file2.txt"}`),
		Status:      database.DeliveryStatusDelivered,
		Attempt:     1,
		DeliveredAt: &now,
	}
	db.WebhookDeliveries.Create(context.Background(), delivered)

	t.Run("retrieves pending deliveries", func(t *testing.T) {
		pending, err := db.WebhookDeliveries.GetPending(context.Background(), 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(pending) != 3 {
			t.Errorf("expected 3 pending deliveries, got %d", len(pending))
		}

		for _, delivery := range pending {
			if delivery.Status != database.DeliveryStatusPending {
				t.Errorf("expected pending status, got %s", delivery.Status)
			}
		}
	})

	t.Run("respects limit", func(t *testing.T) {
		pending, err := db.WebhookDeliveries.GetPending(context.Background(), 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(pending) != 2 {
			t.Errorf("expected 2 pending deliveries (limited), got %d", len(pending))
		}
	})

	t.Run("orders by created_at", func(t *testing.T) {
		pending, err := db.WebhookDeliveries.GetPending(context.Background(), 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify chronological order
		for i := 0; i < len(pending)-1; i++ {
			if pending[i].CreatedAt.After(pending[i+1].CreatedAt) {
				t.Error("pending deliveries not ordered by created_at")
			}
		}
	})
}
