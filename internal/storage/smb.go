package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// SMBBackend implements StorageBackend for SMB/CIFS shares
// Assumes the SMB share is already mounted at the specified path
type SMBBackend struct {
	rootPath string
	server   string
	share    string
}

// NewSMBBackend creates a new SMB backend
// In production environments, the SMB share should be mounted before initializing this backend
// For example in Kubernetes, a CIFS FlexVolume or CSI driver would mount the share at rootPath
func NewSMBBackend(server, share, mountPath string) (*SMBBackend, error) {
	if server == "" {
		return nil, fmt.Errorf("SMB server is required")
	}
	if share == "" {
		return nil, fmt.Errorf("SMB share name is required")
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

	return &SMBBackend{
		rootPath: evalPath,
		server:   server,
		share:    share,
	}, nil
}

// Probe checks if the SMB mount is accessible
func (b *SMBBackend) Probe(ctx context.Context) error {
	info, err := os.Stat(b.rootPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("SMB mount point does not exist: %s (server: %s, share: %s)",
				b.rootPath, b.server, b.share)
		}
		return fmt.Errorf("failed to access SMB mount point: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("SMB mount point is not a directory: %s", b.rootPath)
	}

	// Check if we can read the directory
	f, err := os.Open(b.rootPath)
	if err != nil {
		return fmt.Errorf("failed to open SMB mount point: %w", err)
	}
	defer f.Close()

	// Try to read directory contents to verify permissions
	_, err = f.Readdirnames(1)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to read SMB mount point: %w", err)
	}

	return nil
}

// Walk traverses all files in the SMB storage
func (b *SMBBackend) Walk(ctx context.Context, fn WalkFunc) error {
	return filepath.Walk(b.rootPath, func(path string, info os.FileInfo, err error) error {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			// If we can't access this path, skip it and continue
			// This is especially important for SMB where network issues can occur
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

// Open opens a file for reading from SMB
func (b *SMBBackend) Open(ctx context.Context, path string) (io.ReadCloser, error) {
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
		return nil, fmt.Errorf("failed to open file from SMB: %w", err)
	}

	return f, nil
}

// Stat returns file metadata from SMB
func (b *SMBBackend) Stat(ctx context.Context, path string) (*FileInfo, error) {
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
		return nil, fmt.Errorf("failed to stat file on SMB: %w", err)
	}

	return &FileInfo{
		Path:    path,
		Size:    info.Size(),
		ModTime: info.ModTime(),
		IsDir:   info.IsDir(),
	}, nil
}

// Close cleans up resources (no-op for SMB)
func (b *SMBBackend) Close() error {
	return nil
}

// Server returns the SMB server address
func (b *SMBBackend) Server() string {
	return b.server
}

// Share returns the SMB share name
func (b *SMBBackend) Share() string {
	return b.share
}

// RootPath returns the absolute mount path
func (b *SMBBackend) RootPath() string {
	return b.rootPath
}
