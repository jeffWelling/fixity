package storage_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jeffanddom/fixity/internal/storage"
)

// setupTestDir creates a temporary directory structure for testing
func setupTestDir(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()

	// Create test directory structure:
	// /
	// ├── file1.txt
	// ├── file2.txt
	// ├── subdir/
	// │   ├── file3.txt
	// │   └── nested/
	// │       └── file4.txt
	// └── empty/

	// Create files
	writeFile(t, filepath.Join(tmpDir, "file1.txt"), "content1")
	writeFile(t, filepath.Join(tmpDir, "file2.txt"), "content2")

	// Create subdirectory with files
	os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)
	writeFile(t, filepath.Join(tmpDir, "subdir", "file3.txt"), "content3")

	// Create nested subdirectory
	os.MkdirAll(filepath.Join(tmpDir, "subdir", "nested"), 0755)
	writeFile(t, filepath.Join(tmpDir, "subdir", "nested", "file4.txt"), "content4")

	// Create empty directory
	os.Mkdir(filepath.Join(tmpDir, "empty"), 0755)

	return tmpDir
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
}

func TestNewLocalFSBackend(t *testing.T) {
	t.Run("creates backend with absolute path", func(t *testing.T) {
		tmpDir := t.TempDir()

		backend, err := storage.NewLocalFSBackend(tmpDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if backend == nil {
			t.Fatal("expected backend to be non-nil")
		}

		// Verify path is absolute
		rootPath := backend.RootPath()
		if !filepath.IsAbs(rootPath) {
			t.Errorf("expected absolute path, got: %s", rootPath)
		}
	})

	t.Run("resolves relative path to absolute", func(t *testing.T) {
		tmpDir := t.TempDir()
		relPath := filepath.Base(tmpDir)

		// Change to parent directory temporarily
		originalWd, _ := os.Getwd()
		defer os.Chdir(originalWd)
		os.Chdir(filepath.Dir(tmpDir))

		backend, err := storage.NewLocalFSBackend(relPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		rootPath := backend.RootPath()
		if !filepath.IsAbs(rootPath) {
			t.Errorf("expected absolute path, got: %s", rootPath)
		}
	})
}

func TestLocalFSBackend_Probe(t *testing.T) {
	t.Run("probes accessible directory successfully", func(t *testing.T) {
		tmpDir := setupTestDir(t)
		backend, _ := storage.NewLocalFSBackend(tmpDir)

		err := backend.Probe(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("returns error for non-existent directory", func(t *testing.T) {
		backend, _ := storage.NewLocalFSBackend("/nonexistent/path/that/does/not/exist")

		err := backend.Probe(context.Background())
		if err == nil {
			t.Error("expected error for non-existent directory")
		}

		if !strings.Contains(err.Error(), "does not exist") {
			t.Errorf("expected 'does not exist' error, got: %v", err)
		}
	})

	t.Run("returns error when root is a file not directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "regular-file.txt")
		writeFile(t, filePath, "content")

		backend, _ := storage.NewLocalFSBackend(filePath)

		err := backend.Probe(context.Background())
		if err == nil {
			t.Error("expected error when root is a file")
		}

		if !strings.Contains(err.Error(), "not a directory") {
			t.Errorf("expected 'not a directory' error, got: %v", err)
		}
	})
}

func TestLocalFSBackend_Walk(t *testing.T) {
	t.Run("walks all files in directory tree", func(t *testing.T) {
		tmpDir := setupTestDir(t)
		backend, _ := storage.NewLocalFSBackend(tmpDir)

		var paths []string
		var dirs []string

		err := backend.Walk(context.Background(), func(path string, info *storage.FileInfo) error {
			if info.IsDir {
				dirs = append(dirs, path)
			} else {
				paths = append(paths, path)
			}
			return nil
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify files
		expectedFiles := []string{"file1.txt", "file2.txt", "subdir/file3.txt", "subdir/nested/file4.txt"}
		if len(paths) != len(expectedFiles) {
			t.Errorf("expected %d files, got %d: %v", len(expectedFiles), len(paths), paths)
		}

		for _, expected := range expectedFiles {
			found := false
			for _, path := range paths {
				if path == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected file not found: %s", expected)
			}
		}

		// Verify directories
		expectedDirs := []string{"subdir", "subdir/nested", "empty"}
		if len(dirs) != len(expectedDirs) {
			t.Errorf("expected %d directories, got %d: %v", len(expectedDirs), len(dirs), dirs)
		}
	})

	t.Run("provides correct file metadata", func(t *testing.T) {
		tmpDir := setupTestDir(t)
		backend, _ := storage.NewLocalFSBackend(tmpDir)

		var fileInfo *storage.FileInfo

		err := backend.Walk(context.Background(), func(path string, info *storage.FileInfo) error {
			if path == "file1.txt" {
				fileInfo = info
			}
			return nil
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if fileInfo == nil {
			t.Fatal("expected to find file1.txt")
		}

		if fileInfo.Path != "file1.txt" {
			t.Errorf("expected path file1.txt, got %s", fileInfo.Path)
		}

		if fileInfo.Size != 8 { // "content1" = 8 bytes
			t.Errorf("expected size 8, got %d", fileInfo.Size)
		}

		if fileInfo.ModTime.IsZero() {
			t.Error("expected ModTime to be set")
		}

		if fileInfo.IsDir {
			t.Error("expected IsDir to be false for file")
		}
	})

	t.Run("uses forward slashes in paths", func(t *testing.T) {
		tmpDir := setupTestDir(t)
		backend, _ := storage.NewLocalFSBackend(tmpDir)

		var foundNestedFile bool

		err := backend.Walk(context.Background(), func(path string, info *storage.FileInfo) error {
			// Verify no backslashes (Windows path separators)
			if strings.Contains(path, "\\") {
				t.Errorf("path contains backslash: %s", path)
			}

			if path == "subdir/nested/file4.txt" {
				foundNestedFile = true
			}
			return nil
		})

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !foundNestedFile {
			t.Error("expected to find nested file with forward slashes")
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		tmpDir := setupTestDir(t)
		backend, _ := storage.NewLocalFSBackend(tmpDir)

		ctx, cancel := context.WithCancel(context.Background())
		fileCount := 0

		err := backend.Walk(ctx, func(path string, info *storage.FileInfo) error {
			fileCount++
			if fileCount >= 2 {
				cancel() // Cancel after processing 2 files
			}
			return nil
		})

		if err != context.Canceled {
			t.Errorf("expected context.Canceled error, got: %v", err)
		}
	})

	t.Run("continues walk on permission errors", func(t *testing.T) {
		tmpDir := t.TempDir()
		backend, _ := storage.NewLocalFSBackend(tmpDir)

		// Create accessible file
		writeFile(t, filepath.Join(tmpDir, "accessible.txt"), "content")

		// Create inaccessible directory (if not running as root)
		restrictedDir := filepath.Join(tmpDir, "restricted")
		os.Mkdir(restrictedDir, 0000)
		defer os.Chmod(restrictedDir, 0755) // Cleanup

		var accessibleFound bool

		err := backend.Walk(context.Background(), func(path string, info *storage.FileInfo) error {
			if path == "accessible.txt" {
				accessibleFound = true
			}
			return nil
		})

		// Walk should complete despite permission errors
		if err != nil && err != context.Canceled {
			t.Logf("walk completed with: %v", err)
		}

		if !accessibleFound {
			t.Error("expected to find accessible file despite permission errors")
		}
	})
}

func TestLocalFSBackend_Open(t *testing.T) {
	t.Run("opens file successfully", func(t *testing.T) {
		tmpDir := setupTestDir(t)
		backend, _ := storage.NewLocalFSBackend(tmpDir)

		reader, err := backend.Open(context.Background(), "file1.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer reader.Close()

		content, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}

		if string(content) != "content1" {
			t.Errorf("expected content1, got %s", string(content))
		}
	})

	t.Run("opens nested file successfully", func(t *testing.T) {
		tmpDir := setupTestDir(t)
		backend, _ := storage.NewLocalFSBackend(tmpDir)

		reader, err := backend.Open(context.Background(), "subdir/nested/file4.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer reader.Close()

		content, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}

		if string(content) != "content4" {
			t.Errorf("expected content4, got %s", string(content))
		}
	})

	t.Run("returns error for non-existent file", func(t *testing.T) {
		tmpDir := setupTestDir(t)
		backend, _ := storage.NewLocalFSBackend(tmpDir)

		_, err := backend.Open(context.Background(), "nonexistent.txt")
		if err == nil {
			t.Error("expected error for non-existent file")
		}
	})

	t.Run("prevents directory traversal attacks", func(t *testing.T) {
		tmpDir := t.TempDir()
		backend, _ := storage.NewLocalFSBackend(tmpDir)

		// Try to access file outside root using ../
		_, err := backend.Open(context.Background(), "../../../etc/passwd")
		if err == nil {
			t.Error("expected error for directory traversal attempt")
		}

		if !strings.Contains(err.Error(), "traversal") {
			t.Errorf("expected 'traversal' error, got: %v", err)
		}
	})
}

func TestLocalFSBackend_Stat(t *testing.T) {
	t.Run("stats file successfully", func(t *testing.T) {
		tmpDir := setupTestDir(t)
		backend, _ := storage.NewLocalFSBackend(tmpDir)

		info, err := backend.Stat(context.Background(), "file1.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if info.Path != "file1.txt" {
			t.Errorf("expected path file1.txt, got %s", info.Path)
		}

		if info.Size != 8 { // "content1" = 8 bytes
			t.Errorf("expected size 8, got %d", info.Size)
		}

		if info.ModTime.IsZero() {
			t.Error("expected ModTime to be set")
		}

		if info.IsDir {
			t.Error("expected IsDir to be false")
		}
	})

	t.Run("stats directory successfully", func(t *testing.T) {
		tmpDir := setupTestDir(t)
		backend, _ := storage.NewLocalFSBackend(tmpDir)

		info, err := backend.Stat(context.Background(), "subdir")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !info.IsDir {
			t.Error("expected IsDir to be true")
		}
	})

	t.Run("stats nested file successfully", func(t *testing.T) {
		tmpDir := setupTestDir(t)
		backend, _ := storage.NewLocalFSBackend(tmpDir)

		info, err := backend.Stat(context.Background(), "subdir/nested/file4.txt")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if info.Size != 8 { // "content4" = 8 bytes
			t.Errorf("expected size 8, got %d", info.Size)
		}
	})

	t.Run("returns error for non-existent file", func(t *testing.T) {
		tmpDir := setupTestDir(t)
		backend, _ := storage.NewLocalFSBackend(tmpDir)

		_, err := backend.Stat(context.Background(), "nonexistent.txt")
		if err == nil {
			t.Error("expected error for non-existent file")
		}
	})

	t.Run("prevents directory traversal attacks", func(t *testing.T) {
		tmpDir := t.TempDir()
		backend, _ := storage.NewLocalFSBackend(tmpDir)

		_, err := backend.Stat(context.Background(), "../../../etc/passwd")
		if err == nil {
			t.Error("expected error for directory traversal attempt")
		}

		if !strings.Contains(err.Error(), "traversal") {
			t.Errorf("expected 'traversal' error, got: %v", err)
		}
	})
}

func TestLocalFSBackend_Close(t *testing.T) {
	t.Run("closes without error", func(t *testing.T) {
		tmpDir := setupTestDir(t)
		backend, _ := storage.NewLocalFSBackend(tmpDir)

		err := backend.Close()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestLocalFSBackend_Integration(t *testing.T) {
	t.Run("full workflow: probe, walk, stat, open", func(t *testing.T) {
		tmpDir := setupTestDir(t)
		backend, err := storage.NewLocalFSBackend(tmpDir)
		if err != nil {
			t.Fatalf("failed to create backend: %v", err)
		}
		defer backend.Close()

		// Probe
		if err := backend.Probe(context.Background()); err != nil {
			t.Fatalf("probe failed: %v", err)
		}

		// Walk to find files
		var files []string
		err = backend.Walk(context.Background(), func(path string, info *storage.FileInfo) error {
			if !info.IsDir {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk failed: %v", err)
		}

		if len(files) == 0 {
			t.Fatal("expected to find files")
		}

		// Stat each file
		for _, path := range files {
			info, err := backend.Stat(context.Background(), path)
			if err != nil {
				t.Errorf("stat failed for %s: %v", path, err)
			}

			if info.Size == 0 {
				t.Errorf("expected non-zero size for %s", path)
			}

			// Verify ModTime is recent (within last minute)
			if time.Since(info.ModTime) > time.Minute {
				t.Errorf("expected recent ModTime for %s", path)
			}
		}

		// Open and read first file
		if len(files) > 0 {
			reader, err := backend.Open(context.Background(), files[0])
			if err != nil {
				t.Fatalf("open failed: %v", err)
			}
			defer reader.Close()

			content, err := io.ReadAll(reader)
			if err != nil {
				t.Fatalf("read failed: %v", err)
			}

			if len(content) == 0 {
				t.Error("expected non-empty content")
			}
		}
	})
}
