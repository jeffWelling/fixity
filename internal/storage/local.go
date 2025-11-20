package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// LocalFSBackend implements StorageBackend for local filesystem
type LocalFSBackend struct {
	rootPath string
}

// NewLocalFSBackend creates a new local filesystem backend
func NewLocalFSBackend(rootPath string) (*LocalFSBackend, error) {
	// Clean and validate path
	absPath, err := filepath.Abs(rootPath)
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

	return &LocalFSBackend{
		rootPath: evalPath,
	}, nil
}

// Probe checks if the storage root is accessible
func (b *LocalFSBackend) Probe(ctx context.Context) error {
	info, err := os.Stat(b.rootPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("storage root does not exist: %s", b.rootPath)
		}
		return fmt.Errorf("failed to access storage root: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("storage root is not a directory: %s", b.rootPath)
	}

	// Check if we can read the directory
	f, err := os.Open(b.rootPath)
	if err != nil {
		return fmt.Errorf("failed to open storage root: %w", err)
	}
	defer f.Close()

	// Try to read directory contents to verify permissions
	_, err = f.Readdirnames(1)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to read storage root: %w", err)
	}

	return nil
}

// Walk traverses all files in the storage
func (b *LocalFSBackend) Walk(ctx context.Context, fn WalkFunc) error {
	return filepath.Walk(b.rootPath, func(path string, info os.FileInfo, err error) error {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			// If we can't access this path, skip it and continue
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

// Open opens a file for reading
func (b *LocalFSBackend) Open(ctx context.Context, path string) (io.ReadCloser, error) {
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
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return f, nil
}

// Stat returns file metadata
func (b *LocalFSBackend) Stat(ctx context.Context, path string) (*FileInfo, error) {
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
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	return &FileInfo{
		Path:    path,
		Size:    info.Size(),
		ModTime: info.ModTime(),
		IsDir:   info.IsDir(),
	}, nil
}

// Close cleans up resources (no-op for local filesystem)
func (b *LocalFSBackend) Close() error {
	return nil
}

// RootPath returns the absolute root path of this backend
func (b *LocalFSBackend) RootPath() string {
	return b.rootPath
}
