package database

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // PostgreSQL driver
)

// Database provides access to all repositories
type Database struct {
	db *sqlx.DB

	Files             *FileRepository
	Scans             *ScanRepository
	ChangeEvents      *ChangeEventRepository
	StorageTargets    *StorageTargetRepository
	Users             *UserRepository
	Sessions          *SessionRepository
	Webhooks          *WebhookRepository
	WebhookDeliveries *WebhookDeliveryRepository
	Config            *ConfigRepository
	Checkpoints       *CheckpointRepository
}

// ConnectionConfig holds database connection configuration
type ConnectionConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// New creates a new Database instance with connection pooling
func New(cfg ConnectionConfig) (*Database, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.SSLMode,
	)

	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	} else {
		db.SetMaxOpenConns(25) // Default
	}

	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	} else {
		db.SetMaxIdleConns(5) // Default
	}

	if cfg.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	} else {
		db.SetConnMaxLifetime(5 * time.Minute) // Default
	}

	if cfg.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	} else {
		db.SetConnMaxIdleTime(1 * time.Minute) // Default
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Initialize repositories
	d := &Database{db: db}
	d.Files = &FileRepository{db: db}
	d.Scans = &ScanRepository{db: db}
	d.ChangeEvents = &ChangeEventRepository{db: db}
	d.StorageTargets = &StorageTargetRepository{db: db}
	d.Users = &UserRepository{db: db}
	d.Sessions = &SessionRepository{db: db}
	d.Webhooks = &WebhookRepository{db: db}
	d.WebhookDeliveries = &WebhookDeliveryRepository{db: db}
	d.Config = &ConfigRepository{db: db}
	d.Checkpoints = &CheckpointRepository{db: db}

	return d, nil
}

// FromURL creates a new Database instance from a PostgreSQL URL
// URL format: postgres://user:password@host:port/database?sslmode=disable
func FromURL(dbURL string) (*Database, error) {
	parsed, err := url.Parse(dbURL)
	if err != nil {
		return nil, fmt.Errorf("invalid database URL: %w", err)
	}

	cfg := ConnectionConfig{
		Host:     parsed.Hostname(),
		Database: parsed.Path[1:], // Remove leading slash
		SSLMode:  "disable",
	}

	// Parse port
	if portStr := parsed.Port(); portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("invalid port in database URL: %w", err)
		}
		cfg.Port = port
	} else {
		cfg.Port = 5432 // Default PostgreSQL port
	}

	// Parse user/password
	if parsed.User != nil {
		cfg.User = parsed.User.Username()
		if password, ok := parsed.User.Password(); ok {
			cfg.Password = password
		}
	}

	// Parse query parameters
	query := parsed.Query()
	if sslmode := query.Get("sslmode"); sslmode != "" {
		cfg.SSLMode = sslmode
	}

	return New(cfg)
}

// DB returns the underlying database connection for migrations
func (d *Database) DB() *sql.DB {
	return d.db.DB
}

// Close closes the database connection
func (d *Database) Close() error {
	return d.db.Close()
}

// Health checks database connectivity
func (d *Database) Health(ctx context.Context) error {
	return d.db.PingContext(ctx)
}

// BeginTx starts a new transaction
func (d *Database) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sqlx.Tx, error) {
	return d.db.BeginTxx(ctx, opts)
}

// WithinTransaction executes a function within a transaction
// If the function returns an error, the transaction is rolled back
// Otherwise, the transaction is committed
func (d *Database) WithinTransaction(ctx context.Context, fn func(*sqlx.Tx) error) error {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("transaction error: %w, rollback error: %v", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Stats returns database statistics
func (d *Database) Stats() sql.DBStats {
	return d.db.Stats()
}
