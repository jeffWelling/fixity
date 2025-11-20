package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// BackendTestCase defines a backend configuration for testing
type BackendTestCase struct {
	Name    string
	Type    StorageType
	Setup   func(t *testing.T) (backend StorageBackend, cleanup func())
}

// setupTestDirectory creates a test directory with sample files
func setupTestDirectory(t *testing.T) string {
	t.Helper()

	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "fixity-backend-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Create test file structure
	files := map[string]string{
		"file1.txt":         "This is file 1",
		"file2.txt":         "This is file 2",
		"subdir/file3.txt":  "This is file 3 in subdir",
		"subdir/file4.txt":  "This is file 4 in subdir",
		"subdir2/file5.txt": "This is file 5 in subdir2",
		"empty.txt":         "",
	}

	for path, content := range files {
		fullPath := filepath.Join(tempDir, path)
		dir := filepath.Dir(fullPath)

		// Create directory if needed
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}

		// Write file
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file %s: %v", fullPath, err)
		}
	}

	return tempDir
}

// GetAllBackendTestCases returns test cases for all backend types
func GetAllBackendTestCases(t *testing.T) []BackendTestCase {
	return []BackendTestCase{
		{
			Name: "Local",
			Type: TypeLocal,
			Setup: func(t *testing.T) (StorageBackend, func()) {
				testDir := setupTestDirectory(t)
				backend, err := NewLocalFSBackend(testDir)
				if err != nil {
					t.Fatalf("failed to create local backend: %v", err)
				}
				cleanup := func() {
					backend.Close()
					os.RemoveAll(testDir)
				}
				return backend, cleanup
			},
		},
		{
			Name: "NFS",
			Type: TypeNFS,
			Setup: func(t *testing.T) (StorageBackend, func()) {
				testDir := setupTestDirectory(t)
				// For testing, we treat the test directory as an "NFS mount"
				backend, err := NewNFSBackend("nfs-server.example.com", "/exports/test", testDir)
				if err != nil {
					t.Fatalf("failed to create NFS backend: %v", err)
				}
				cleanup := func() {
					backend.Close()
					os.RemoveAll(testDir)
				}
				return backend, cleanup
			},
		},
		{
			Name: "SMB",
			Type: TypeSMB,
			Setup: func(t *testing.T) (StorageBackend, func()) {
				testDir := setupTestDirectory(t)
				// For testing, we treat the test directory as an "SMB mount"
				backend, err := NewSMBBackend("smb-server.example.com", "testshare", testDir)
				if err != nil {
					t.Fatalf("failed to create SMB backend: %v", err)
				}
				cleanup := func() {
					backend.Close()
					os.RemoveAll(testDir)
				}
				return backend, cleanup
			},
		},
	}
}

// TestBackend_Probe tests the Probe method for all backends
func TestBackend_Probe(t *testing.T) {
	testCases := GetAllBackendTestCases(t)

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			backend, cleanup := tc.Setup(t)
			defer cleanup()

			ctx := context.Background()

			// Test successful probe
			err := backend.Probe(ctx)
			if err != nil {
				t.Errorf("Probe() failed: %v", err)
			}
		})
	}
}

// TestBackend_ProbeNonExistent tests probing non-existent paths
func TestBackend_ProbeNonExistent(t *testing.T) {
	testCases := []struct {
		name    string
		backend func() (StorageBackend, error)
	}{
		{
			name: "Local",
			backend: func() (StorageBackend, error) {
				return NewLocalFSBackend("/nonexistent/path/12345")
			},
		},
		{
			name: "NFS",
			backend: func() (StorageBackend, error) {
				return NewNFSBackend("nfs-server.example.com", "/exports/test", "/nonexistent/path/12345")
			},
		},
		{
			name: "SMB",
			backend: func() (StorageBackend, error) {
				return NewSMBBackend("smb-server.example.com", "testshare", "/nonexistent/path/12345")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			backend, err := tc.backend()
			if err != nil {
				t.Fatalf("failed to create backend: %v", err)
			}
			defer backend.Close()

			ctx := context.Background()
			err = backend.Probe(ctx)
			if err == nil {
				t.Error("Probe() should fail for non-existent path")
			}
		})
	}
}

// TestBackend_Walk tests the Walk method for all backends
func TestBackend_Walk(t *testing.T) {
	testCases := GetAllBackendTestCases(t)

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			backend, cleanup := tc.Setup(t)
			defer cleanup()

			ctx := context.Background()

			// Collect all files found
			var foundFiles []string
			err := backend.Walk(ctx, func(path string, info *FileInfo) error {
				if !info.IsDir {
					foundFiles = append(foundFiles, path)
				}
				return nil
			})

			if err != nil {
				t.Fatalf("Walk() failed: %v", err)
			}

			// Verify we found all expected files
			expectedFiles := []string{
				"empty.txt",
				"file1.txt",
				"file2.txt",
				"subdir/file3.txt",
				"subdir/file4.txt",
				"subdir2/file5.txt",
			}

			if len(foundFiles) != len(expectedFiles) {
				t.Errorf("Walk() found %d files, expected %d", len(foundFiles), len(expectedFiles))
			}

			// Check that all expected files were found
			foundMap := make(map[string]bool)
			for _, f := range foundFiles {
				foundMap[f] = true
			}

			for _, expected := range expectedFiles {
				if !foundMap[expected] {
					t.Errorf("Walk() did not find expected file: %s", expected)
				}
			}
		})
	}
}

// TestBackend_WalkWithCancellation tests Walk context cancellation
func TestBackend_WalkWithCancellation(t *testing.T) {
	testCases := GetAllBackendTestCases(t)

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			backend, cleanup := tc.Setup(t)
			defer cleanup()

			ctx, cancel := context.WithCancel(context.Background())

			// Cancel after first file
			fileCount := 0
			err := backend.Walk(ctx, func(path string, info *FileInfo) error {
				fileCount++
				if fileCount == 1 {
					cancel()
				}
				return nil
			})

			if err != context.Canceled {
				t.Errorf("Walk() should return context.Canceled, got: %v", err)
			}
		})
	}
}

// TestBackend_Open tests the Open method for all backends
func TestBackend_Open(t *testing.T) {
	testCases := GetAllBackendTestCases(t)

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			backend, cleanup := tc.Setup(t)
			defer cleanup()

			ctx := context.Background()

			// Test opening a file
			reader, err := backend.Open(ctx, "file1.txt")
			if err != nil {
				t.Fatalf("Open() failed: %v", err)
			}
			defer reader.Close()

			// Read contents
			content, err := io.ReadAll(reader)
			if err != nil {
				t.Fatalf("failed to read file: %v", err)
			}

			expected := "This is file 1"
			if string(content) != expected {
				t.Errorf("Open() content = %q, want %q", string(content), expected)
			}
		})
	}
}

// TestBackend_OpenNonExistent tests opening non-existent files
func TestBackend_OpenNonExistent(t *testing.T) {
	testCases := GetAllBackendTestCases(t)

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			backend, cleanup := tc.Setup(t)
			defer cleanup()

			ctx := context.Background()

			_, err := backend.Open(ctx, "nonexistent.txt")
			if err == nil {
				t.Error("Open() should fail for non-existent file")
			}
		})
	}
}

// TestBackend_OpenPathTraversal tests directory traversal prevention
func TestBackend_OpenPathTraversal(t *testing.T) {
	testCases := GetAllBackendTestCases(t)

	maliciousPaths := []string{
		"../../../etc/passwd",
		"..\\..\\..\\windows\\system32",
		"subdir/../../etc/passwd",
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			backend, cleanup := tc.Setup(t)
			defer cleanup()

			ctx := context.Background()

			for _, maliciousPath := range maliciousPaths {
				_, err := backend.Open(ctx, maliciousPath)
				if err == nil {
					t.Errorf("Open() should prevent path traversal for: %s", maliciousPath)
				}
			}
		})
	}
}

// TestBackend_Stat tests the Stat method for all backends
func TestBackend_Stat(t *testing.T) {
	testCases := GetAllBackendTestCases(t)

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			backend, cleanup := tc.Setup(t)
			defer cleanup()

			ctx := context.Background()

			// Test stat on a file
			info, err := backend.Stat(ctx, "file1.txt")
			if err != nil {
				t.Fatalf("Stat() failed: %v", err)
			}

			if info.Path != "file1.txt" {
				t.Errorf("Stat() path = %q, want %q", info.Path, "file1.txt")
			}

			if info.Size != 14 { // "This is file 1" = 14 bytes
				t.Errorf("Stat() size = %d, want 14", info.Size)
			}

			if info.IsDir {
				t.Error("Stat() IsDir should be false for a file")
			}

			if info.ModTime.IsZero() {
				t.Error("Stat() ModTime should not be zero")
			}
		})
	}
}

// TestBackend_StatDirectory tests Stat on directories
func TestBackend_StatDirectory(t *testing.T) {
	testCases := GetAllBackendTestCases(t)

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			backend, cleanup := tc.Setup(t)
			defer cleanup()

			ctx := context.Background()

			// Test stat on a directory
			info, err := backend.Stat(ctx, "subdir")
			if err != nil {
				t.Fatalf("Stat() failed: %v", err)
			}

			if !info.IsDir {
				t.Error("Stat() IsDir should be true for a directory")
			}
		})
	}
}

// TestBackend_StatNonExistent tests Stat on non-existent files
func TestBackend_StatNonExistent(t *testing.T) {
	testCases := GetAllBackendTestCases(t)

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			backend, cleanup := tc.Setup(t)
			defer cleanup()

			ctx := context.Background()

			_, err := backend.Stat(ctx, "nonexistent.txt")
			if err == nil {
				t.Error("Stat() should fail for non-existent file")
			}
		})
	}
}

// TestBackend_Close tests the Close method for all backends
func TestBackend_Close(t *testing.T) {
	testCases := GetAllBackendTestCases(t)

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			backend, cleanup := tc.Setup(t)
			defer cleanup()

			err := backend.Close()
			if err != nil {
				t.Errorf("Close() failed: %v", err)
			}
		})
	}
}

// TestNewBackendFactory tests the factory function
func TestNewBackendFactory(t *testing.T) {
	testDir := setupTestDirectory(t)
	defer os.RemoveAll(testDir)

	tests := []struct {
		name    string
		config  BackendConfig
		wantErr bool
	}{
		{
			name: "Local backend",
			config: BackendConfig{
				Type: TypeLocal,
				Path: testDir,
			},
			wantErr: false,
		},
		{
			name: "NFS backend with server and share",
			config: BackendConfig{
				Type:   TypeNFS,
				Path:   testDir,
				Server: stringPtr("nfs-server.example.com"),
				Share:  stringPtr("/exports/test"),
			},
			wantErr: false,
		},
		{
			name: "NFS backend missing server",
			config: BackendConfig{
				Type:  TypeNFS,
				Path:  testDir,
				Share: stringPtr("/exports/test"),
			},
			wantErr: true,
		},
		{
			name: "NFS backend missing share",
			config: BackendConfig{
				Type:   TypeNFS,
				Path:   testDir,
				Server: stringPtr("nfs-server.example.com"),
			},
			wantErr: true,
		},
		{
			name: "SMB backend with server and share",
			config: BackendConfig{
				Type:   TypeSMB,
				Path:   testDir,
				Server: stringPtr("smb-server.example.com"),
				Share:  stringPtr("testshare"),
			},
			wantErr: false,
		},
		{
			name: "SMB backend missing server",
			config: BackendConfig{
				Type:  TypeSMB,
				Path:  testDir,
				Share: stringPtr("testshare"),
			},
			wantErr: true,
		},
		{
			name: "SMB backend missing share",
			config: BackendConfig{
				Type:   TypeSMB,
				Path:   testDir,
				Server: stringPtr("smb-server.example.com"),
			},
			wantErr: true,
		},
		{
			name: "Unsupported type",
			config: BackendConfig{
				Type: "unsupported",
				Path: testDir,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend, err := NewBackend(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewBackend() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				defer backend.Close()

				// Verify backend can probe
				ctx := context.Background()
				if err := backend.Probe(ctx); err != nil {
					t.Errorf("Backend.Probe() failed: %v", err)
				}
			}
		})
	}
}

// TestBackend_LargeFileHandling tests handling of larger files
func TestBackend_LargeFileHandling(t *testing.T) {
	testCases := GetAllBackendTestCases(t)

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			backend, cleanup := tc.Setup(t)
			defer cleanup()

			// Get the root path based on backend type
			var rootPath string
			switch b := backend.(type) {
			case *LocalFSBackend:
				rootPath = b.RootPath()
			case *NFSBackend:
				rootPath = b.RootPath()
			case *SMBBackend:
				rootPath = b.RootPath()
			}

			// Create a larger file (1MB)
			largeFilePath := filepath.Join(rootPath, "large.bin")
			largeContent := make([]byte, 1024*1024) // 1MB
			for i := range largeContent {
				largeContent[i] = byte(i % 256)
			}
			if err := os.WriteFile(largeFilePath, largeContent, 0644); err != nil {
				t.Fatalf("failed to create large file: %v", err)
			}

			ctx := context.Background()

			// Test Stat
			info, err := backend.Stat(ctx, "large.bin")
			if err != nil {
				t.Fatalf("Stat() failed: %v", err)
			}
			if info.Size != 1024*1024 {
				t.Errorf("Stat() size = %d, want %d", info.Size, 1024*1024)
			}

			// Test Open and read
			reader, err := backend.Open(ctx, "large.bin")
			if err != nil {
				t.Fatalf("Open() failed: %v", err)
			}
			defer reader.Close()

			readContent, err := io.ReadAll(reader)
			if err != nil {
				t.Fatalf("failed to read large file: %v", err)
			}

			if len(readContent) != 1024*1024 {
				t.Errorf("Read %d bytes, want %d", len(readContent), 1024*1024)
			}
		})
	}
}

// TestBackend_SpecialCharactersInFilenames tests handling of special characters
func TestBackend_SpecialCharactersInFilenames(t *testing.T) {
	testCases := GetAllBackendTestCases(t)

	specialNames := []string{
		"file with spaces.txt",
		"file-with-dashes.txt",
		"file_with_underscores.txt",
		"file.multiple.dots.txt",
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			backend, cleanup := tc.Setup(t)
			defer cleanup()

			// Get the root path
			var rootPath string
			switch b := backend.(type) {
			case *LocalFSBackend:
				rootPath = b.RootPath()
			case *NFSBackend:
				rootPath = b.RootPath()
			case *SMBBackend:
				rootPath = b.RootPath()
			}

			ctx := context.Background()

			// Create files with special names
			for _, name := range specialNames {
				filePath := filepath.Join(rootPath, name)
				content := fmt.Sprintf("Content of %s", name)
				if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
					t.Fatalf("failed to create file %s: %v", name, err)
				}

				// Test Stat
				_, err := backend.Stat(ctx, name)
				if err != nil {
					t.Errorf("Stat() failed for %s: %v", name, err)
				}

				// Test Open
				reader, err := backend.Open(ctx, name)
				if err != nil {
					t.Errorf("Open() failed for %s: %v", name, err)
				} else {
					reader.Close()
				}
			}

			// Test Walk finds all special named files
			var foundNames []string
			err := backend.Walk(ctx, func(path string, info *FileInfo) error {
				if !info.IsDir {
					for _, special := range specialNames {
						if strings.Contains(path, special) {
							foundNames = append(foundNames, path)
							break
						}
					}
				}
				return nil
			})

			if err != nil {
				t.Fatalf("Walk() failed: %v", err)
			}

			if len(foundNames) != len(specialNames) {
				t.Errorf("Walk() found %d special files, want %d", len(foundNames), len(specialNames))
			}
		})
	}
}

// TestBackend_EmptyFiles tests handling of empty files
func TestBackend_EmptyFiles(t *testing.T) {
	testCases := GetAllBackendTestCases(t)

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			backend, cleanup := tc.Setup(t)
			defer cleanup()

			ctx := context.Background()

			// Test Stat on empty file
			info, err := backend.Stat(ctx, "empty.txt")
			if err != nil {
				t.Fatalf("Stat() failed: %v", err)
			}

			if info.Size != 0 {
				t.Errorf("Stat() size = %d, want 0", info.Size)
			}

			// Test Open on empty file
			reader, err := backend.Open(ctx, "empty.txt")
			if err != nil {
				t.Fatalf("Open() failed: %v", err)
			}
			defer reader.Close()

			content, err := io.ReadAll(reader)
			if err != nil {
				t.Fatalf("failed to read empty file: %v", err)
			}

			if len(content) != 0 {
				t.Errorf("Read %d bytes from empty file, want 0", len(content))
			}
		})
	}
}

// TestBackend_WalkFileMetadata verifies Walk returns correct metadata
func TestBackend_WalkFileMetadata(t *testing.T) {
	testCases := GetAllBackendTestCases(t)

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			backend, cleanup := tc.Setup(t)
			defer cleanup()

			ctx := context.Background()

			// Record file metadata from Walk
			filesMetadata := make(map[string]*FileInfo)
			err := backend.Walk(ctx, func(path string, info *FileInfo) error {
				if !info.IsDir {
					filesMetadata[path] = info
				}
				return nil
			})

			if err != nil {
				t.Fatalf("Walk() failed: %v", err)
			}

			// Verify metadata is correct by comparing with Stat
			for path, walkInfo := range filesMetadata {
				statInfo, err := backend.Stat(ctx, path)
				if err != nil {
					t.Errorf("Stat() failed for %s: %v", path, err)
					continue
				}

				if walkInfo.Size != statInfo.Size {
					t.Errorf("Walk() size for %s = %d, Stat() = %d", path, walkInfo.Size, statInfo.Size)
				}

				if walkInfo.IsDir != statInfo.IsDir {
					t.Errorf("Walk() IsDir for %s = %v, Stat() = %v", path, walkInfo.IsDir, statInfo.IsDir)
				}

				// ModTime should be within 1 second (to account for filesystem precision)
				timeDiff := walkInfo.ModTime.Sub(statInfo.ModTime)
				if timeDiff < -time.Second || timeDiff > time.Second {
					t.Errorf("Walk() ModTime for %s differs from Stat() by %v", path, timeDiff)
				}
			}
		})
	}
}

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}
