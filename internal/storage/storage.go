package storage

import (
	"context"
	"fmt"
	"io"
	"time"
)

// StorageBackend provides a unified interface for different storage types
type StorageBackend interface {
	// Probe checks if storage is accessible
	Probe(ctx context.Context) error

	// Walk traverses all files in the storage
	Walk(ctx context.Context, fn WalkFunc) error

	// Open opens a file for reading
	Open(ctx context.Context, path string) (io.ReadCloser, error)

	// Stat returns file metadata
	Stat(ctx context.Context, path string) (*FileInfo, error)

	// Close cleans up resources
	Close() error
}

// WalkFunc is called for each file during Walk
type WalkFunc func(path string, info *FileInfo) error

// FileInfo contains file metadata
type FileInfo struct {
	Path    string
	Size    int64
	ModTime time.Time
	IsDir   bool
}

// StorageType represents the type of storage backend
type StorageType string

const (
	TypeLocal StorageType = "local"
	TypeNFS   StorageType = "nfs"
	TypeSMB   StorageType = "smb"
)

// BackendConfig contains configuration for creating a storage backend
type BackendConfig struct {
	Type      StorageType
	Path      string  // Mount path or local directory path
	Server    *string // NFS/SMB server address
	Share     *string // NFS export path or SMB share name
	CredsRef  *string // Optional credentials reference (for future use)
}

// NewBackend creates a new storage backend based on the provided configuration
func NewBackend(cfg BackendConfig) (StorageBackend, error) {
	switch cfg.Type {
	case TypeLocal:
		return NewLocalFSBackend(cfg.Path)

	case TypeNFS:
		if cfg.Server == nil || *cfg.Server == "" {
			return nil, fmt.Errorf("NFS backend requires server address")
		}
		if cfg.Share == nil || *cfg.Share == "" {
			return nil, fmt.Errorf("NFS backend requires share path")
		}
		return NewNFSBackend(*cfg.Server, *cfg.Share, cfg.Path)

	case TypeSMB:
		if cfg.Server == nil || *cfg.Server == "" {
			return nil, fmt.Errorf("SMB backend requires server address")
		}
		if cfg.Share == nil || *cfg.Share == "" {
			return nil, fmt.Errorf("SMB backend requires share name")
		}
		return NewSMBBackend(*cfg.Server, *cfg.Share, cfg.Path)

	default:
		return nil, fmt.Errorf("unsupported storage type: %s", cfg.Type)
	}
}
