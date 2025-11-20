package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// NFSBackend implements StorageBackend for NFS mounts
// Assumes the NFS share is already mounted at the specified path
type NFSBackend struct {
	rootPath string
	server   string
	share    string
}

// NewNFSBackend creates a new NFS backend
// In production environments, the NFS share should be mounted before initializing this backend
// For example in Kubernetes, an NFS PersistentVolume would be mounted at rootPath
func NewNFSBackend(server, share, mountPath string) (*NFSBackend, error) {
	if server == "" {
		return nil, fmt.Errorf("NFS server is required")
	}
	if share == "" {
		return nil, fmt.Errorf("NFS share path is required")
	}
	if mountPath == "" {
		return nil, fmt.Errorf("mount path is required")
	}

	// Clean and validate path
	absPath, err := filepath.Abs(mountPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Resolve any symlinks in the path to ensure security checks work correctly
	// This is especially important on systems where temp directories might be symlinked
	evalPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		// If symlink resolution fails, path might not exist yet - use absPath
		evalPath = absPath
	}

	return &NFSBackend{
		rootPath: evalPath,
		server:   server,
		share:    share,
	}, nil
}

// Probe checks if the NFS mount is accessible
func (b *NFSBackend) Probe(ctx context.Context) error {
	info, err := os.Stat(b.rootPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("NFS mount point does not exist: %s (server: %s, share: %s)",
				b.rootPath, b.server, b.share)
		}
		return fmt.Errorf("failed to access NFS mount point: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("NFS mount point is not a directory: %s", b.rootPath)
	}

	// Check if we can read the directory
	f, err := os.Open(b.rootPath)
	if err != nil {
		return fmt.Errorf("failed to open NFS mount point: %w", err)
	}
	defer f.Close()

	// Try to read directory contents to verify permissions
	_, err = f.Readdirnames(1)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to read NFS mount point: %w", err)
	}

	return nil
}

// Walk traverses all files in the NFS storage
func (b *NFSBackend) Walk(ctx context.Context, fn WalkFunc) error {
	return filepath.Walk(b.rootPath, func(path string, info os.FileInfo, err error) error {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			// If we can't access this path, skip it and continue
			// This is especially important for NFS where network issues can occur
			return nil
		}

		// Convert to relative path from root
		relPath, err := filepath.Rel(b.rootPath, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		// Skip the root directory itself
		if relPath == "." {
			return nil
		}

		// Convert to forward slashes for consistency
		relPath = filepath.ToSlash(relPath)

		fileInfo := &FileInfo{
			Path:    relPath,
			Size:    info.Size(),
			ModTime: info.ModTime(),
			IsDir:   info.IsDir(),
		}

		return fn(relPath, fileInfo)
	})
}

// Open opens a file for reading from NFS
func (b *NFSBackend) Open(ctx context.Context, path string) (io.ReadCloser, error) {
	// Convert from relative to absolute path
	absPath := filepath.Join(b.rootPath, filepath.FromSlash(path))

	// Verify the path is within rootPath (prevent directory traversal)
	if !filepath.HasPrefix(absPath, b.rootPath) {
		return nil, fmt.Errorf("path traversal attempt detected: %s", path)
	}

	// Resolve symlinks and verify they stay within boundaries
	evalPath, err := filepath.EvalSymlinks(absPath)
	if err == nil {
		// If symlink resolution succeeds, check if it escaped
		if !filepath.HasPrefix(evalPath, b.rootPath) {
			return nil, fmt.Errorf("symlink points outside storage boundary: %s", path)
		}
		absPath = evalPath
	}

	f, err := os.Open(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file from NFS: %w", err)
	}

	return f, nil
}

// Stat returns file metadata from NFS
func (b *NFSBackend) Stat(ctx context.Context, path string) (*FileInfo, error) {
	// Convert from relative to absolute path
	absPath := filepath.Join(b.rootPath, filepath.FromSlash(path))

	// Verify the path is within rootPath (prevent directory traversal)
	if !filepath.HasPrefix(absPath, b.rootPath) {
		return nil, fmt.Errorf("path traversal attempt detected: %s", path)
	}

	// Resolve symlinks and verify they stay within boundaries
	evalPath, err := filepath.EvalSymlinks(absPath)
	if err == nil {
		// If symlink resolution succeeds, check if it escaped
		if !filepath.HasPrefix(evalPath, b.rootPath) {
			return nil, fmt.Errorf("symlink points outside storage boundary: %s", path)
		}
		absPath = evalPath
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file on NFS: %w", err)
	}

	return &FileInfo{
		Path:    path,
		Size:    info.Size(),
		ModTime: info.ModTime(),
		IsDir:   info.IsDir(),
	}, nil
}

// Close cleans up resources (no-op for NFS)
func (b *NFSBackend) Close() error {
	return nil
}

// Server returns the NFS server address
func (b *NFSBackend) Server() string {
	return b.server
}

// Share returns the NFS share path
func (b *NFSBackend) Share() string {
	return b.share
}

// RootPath returns the absolute mount path
func (b *NFSBackend) RootPath() string {
	return b.rootPath
}
