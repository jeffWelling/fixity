package database

import (
	"time"

	"github.com/lib/pq"
)

// File represents a tracked file in the system
type File struct {
	ID                int64     `db:"id"`
	StorageTargetID   int64     `db:"storage_target_id"`
	Path              string    `db:"path"`
	Size              int64     `db:"size"`
	FirstSeen         time.Time `db:"first_seen"`
	LastSeen          time.Time `db:"last_seen"`
	CurrentChecksum   *string   `db:"current_checksum"`
	ChecksumType      *string   `db:"checksum_type"`
	LastChecksummedAt *time.Time `db:"last_checksummed_at"`
	DeletedAt         *time.Time `db:"deleted_at"`
	CreatedAt         time.Time `db:"created_at"`
	UpdatedAt         time.Time `db:"updated_at"`
}

// Scan represents a scan execution
type Scan struct {
	ID               int64       `db:"id"`
	StorageTargetID  int64       `db:"storage_target_id"`
	Status           ScanStatus  `db:"status"`
	StartedAt        time.Time   `db:"started_at"`
	CompletedAt      *time.Time  `db:"completed_at"`
	FilesScanned     int64       `db:"files_scanned"`
	FilesAdded       int64       `db:"files_added"`
	FilesDeleted     int64       `db:"files_deleted"`
	FilesModified    int64       `db:"files_modified"`
	FilesVerified    int64       `db:"files_verified"`
	ErrorsCount      int         `db:"errors_count"`
	ErrorMessages    pq.StringArray `db:"error_messages"`
	IsLargeChange    bool        `db:"is_large_change"`
	ResumedFrom      *int64      `db:"resumed_from"`
	CreatedAt        time.Time   `db:"created_at"`
}

// ScanStatus represents the status of a scan
type ScanStatus string

const (
	ScanStatusRunning   ScanStatus = "running"
	ScanStatusCompleted ScanStatus = "completed"
	ScanStatusFailed    ScanStatus = "failed"
	ScanStatusPartial   ScanStatus = "partial"
)

// ChangeEvent represents a file lifecycle event
type ChangeEvent struct {
	ID           int64           `db:"id"`
	ScanID       int64           `db:"scan_id"`
	FileID       int64           `db:"file_id"`
	EventType    ChangeEventType `db:"event_type"`
	DetectedAt   time.Time       `db:"detected_at"`
	OldChecksum  *string         `db:"old_checksum"`
	NewChecksum  *string         `db:"new_checksum"`
	OldSize      *int64          `db:"old_size"`
	NewSize      *int64          `db:"new_size"`
	CreatedAt    time.Time       `db:"created_at"`
}

// ChangeEventType represents the type of change event
type ChangeEventType string

const (
	ChangeEventAdded    ChangeEventType = "added"
	ChangeEventDeleted  ChangeEventType = "deleted"
	ChangeEventModified ChangeEventType = "modified"
	ChangeEventVerified ChangeEventType = "verified"
)

// StorageTarget represents a monitored storage location
type StorageTarget struct {
	ID                              int64          `db:"id"`
	Name                            string         `db:"name"`
	Type                            StorageType    `db:"type"`
	Path                            string         `db:"path"`
	Server                          *string        `db:"server"`
	Share                           *string        `db:"share"`
	CredentialsRef                  *string        `db:"credentials_ref"`
	Enabled                         bool           `db:"enabled"`
	ScanSchedule                    *string        `db:"scan_schedule"`
	ParallelWorkers                 int            `db:"parallel_workers"`
	RandomSamplePercent             float64        `db:"random_sample_percent"`
	ChecksumAlgorithm               string         `db:"checksum_algorithm"`
	CheckpointInterval              int            `db:"checkpoint_interval"`
	BatchSize                       int            `db:"batch_size"`
	LargeChangeThresholdCount       *int           `db:"large_change_threshold_count"`
	LargeChangeThresholdPercent     *float64       `db:"large_change_threshold_percent"`
	LargeChangeThresholdBytes       *int64         `db:"large_change_threshold_bytes"`
	CreatedAt                       time.Time      `db:"created_at"`
	UpdatedAt                       time.Time      `db:"updated_at"`
}

// StorageType represents the type of storage backend
type StorageType string

const (
	StorageTypeLocal StorageType = "local"
	StorageTypeNFS   StorageType = "nfs"
	StorageTypeSMB   StorageType = "smb"
)

// ScanCheckpoint enables scan resumption after interruption
type ScanCheckpoint struct {
	ScanID            int64     `db:"scan_id"`
	LastProcessedPath string    `db:"last_processed_path"`
	FilesProcessed    int64     `db:"files_processed"`
	CheckpointAt      time.Time `db:"checkpoint_at"`
}

// User represents a system user
type User struct {
	ID           int64      `db:"id"`
	Username     string     `db:"username"`
	PasswordHash string     `db:"password_hash"`
	Email        *string    `db:"email"`
	IsAdmin      bool       `db:"is_admin"`
	CreatedAt    time.Time  `db:"created_at"`
	LastLogin    *time.Time `db:"last_login"`
}

// Session represents a user session
type Session struct {
	Token     string    `db:"token"`
	UserID    int64     `db:"user_id"`
	ExpiresAt time.Time `db:"expires_at"`
	CreatedAt time.Time `db:"created_at"`
}

// Webhook represents a webhook configuration
type Webhook struct {
	ID               int64          `db:"id"`
	Name             string         `db:"name"`
	URL              string         `db:"url"`
	Enabled          bool           `db:"enabled"`
	EventIncludes    pq.StringArray `db:"event_includes"`
	EventExcludes    pq.StringArray `db:"event_excludes"`
	RetryAttempts    int            `db:"retry_attempts"`
	RetryBackoffSec  int            `db:"retry_backoff_sec"`
	TimeoutSec       int            `db:"timeout_sec"`
	CreatedAt        time.Time      `db:"created_at"`
	UpdatedAt        time.Time      `db:"updated_at"`
}

// WebhookDelivery represents a webhook delivery attempt
type WebhookDelivery struct {
	ID            int64            `db:"id"`
	WebhookID     int64            `db:"webhook_id"`
	EventType     string           `db:"event_type"`
	Payload       []byte           `db:"payload"`
	Status        DeliveryStatus   `db:"status"`
	Attempt       int              `db:"attempt"`
	LastAttemptAt *time.Time       `db:"last_attempt_at"`
	NextRetryAt   *time.Time       `db:"next_retry_at"`
	DeliveredAt   *time.Time       `db:"delivered_at"`
	ErrorMessage  *string          `db:"error_message"`
	CreatedAt     time.Time        `db:"created_at"`
}

// DeliveryStatus represents the status of a webhook delivery
type DeliveryStatus string

const (
	DeliveryStatusPending   DeliveryStatus = "pending"
	DeliveryStatusDelivered DeliveryStatus = "delivered"
	DeliveryStatusFailed    DeliveryStatus = "failed"
)

// Config represents a configuration key-value pair
type Config struct {
	Key         string     `db:"key"`
	Value       string     `db:"value"`
	Description *string    `db:"description"`
	UpdatedAt   time.Time  `db:"updated_at"`
	UpdatedBy   *int64     `db:"updated_by"`
}
