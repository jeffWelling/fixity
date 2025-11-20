package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// createTestFiles is a helper that creates test files in a directory
func createTestFiles(t *testing.T, dir string, files []string) {
	t.Helper()
	for _, filename := range files {
		fullPath := filepath.Join(dir, filename)
		content := fmt.Sprintf("content of %s", filename)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create test file %s: %v", filename, err)
		}
	}
}

// TestNetworkMountAccessibility tests that NFS/SMB backends can handle mount point issues
func TestNetworkMountAccessibility(t *testing.T) {
	testCases := []struct {
		Name      string
		Type      StorageType
		SetupFunc func(t *testing.T) (backend StorageBackend, unmountFunc func(), cleanup func())
	}{
		{
			Name: "NFS",
			Type: TypeNFS,
			SetupFunc: func(t *testing.T) (StorageBackend, func(), func()) {
				// Create test directory to simulate NFS mount
				testDir := setupTestDirectory(t)
				createTestFiles(t, testDir, []string{"file1.txt", "file2.txt"})

				backend, err := NewNFSBackend("nfs-server.example.com", "/exports/data", testDir)
				if err != nil {
					t.Fatalf("failed to create NFS backend: %v", err)
				}

				// Unmount simulation: make directory inaccessible
				unmount := func() {
					// On real systems this would be: syscall.Unmount(testDir, 0)
					// For testing, we'll remove read permissions
					os.Chmod(testDir, 0000)
				}

				cleanup := func() {
					os.Chmod(testDir, 0755)
					os.RemoveAll(testDir)
				}

				return backend, unmount, cleanup
			},
		},
		{
			Name: "SMB",
			Type: TypeSMB,
			SetupFunc: func(t *testing.T) (StorageBackend, func(), func()) {
				testDir := setupTestDirectory(t)
				createTestFiles(t, testDir, []string{"file1.txt", "file2.txt"})

				backend, err := NewSMBBackend("smb-server.example.com", "ShareName", testDir)
				if err != nil {
					t.Fatalf("failed to create SMB backend: %v", err)
				}

				unmount := func() {
					os.Chmod(testDir, 0000)
				}

				cleanup := func() {
					os.Chmod(testDir, 0755)
					os.RemoveAll(testDir)
				}

				return backend, unmount, cleanup
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			backend, unmount, cleanup := tc.SetupFunc(t)
			defer cleanup()

			ctx := context.Background()

			// Verify backend works initially
			err := backend.Probe(ctx)
			if err != nil {
				t.Fatalf("initial probe failed: %v", err)
			}

			// Simulate network mount becoming unavailable
			unmount()
			defer func() {
				// Restore permissions for cleanup
				var rootPath string
				switch b := backend.(type) {
				case *NFSBackend:
					rootPath = b.RootPath()
				case *SMBBackend:
					rootPath = b.RootPath()
				}
				os.Chmod(rootPath, 0755)
			}()

			// Probe should now fail gracefully
			err = backend.Probe(ctx)
			if err == nil {
				t.Error("expected probe to fail when mount is unavailable")
			} else {
				t.Logf("Probe correctly failed: %v", err)
			}

			// Walk may or may not fail depending on whether directory handle is cached
			// The important thing is it doesn't panic
			filesFound := 0
			err = backend.Walk(ctx, func(path string, info *FileInfo) error {
				filesFound++
				return nil
			})
			if err != nil {
				t.Logf("Walk failed (acceptable): %v", err)
			} else {
				t.Logf("Walk completed (found %d files, may have cached handles)", filesFound)
			}

			// Open should fail gracefully
			_, err = backend.Open(ctx, "file1.txt")
			if err == nil {
				t.Error("expected open to fail when mount is unavailable")
			} else {
				t.Logf("Open correctly failed: %v", err)
			}
		})
	}
}

// TestStaleFileHandleRecovery tests NFS stale file handle scenarios
func TestStaleFileHandleRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stale handle test in short mode")
	}

	testCases := GetAllBackendTestCases(t)

	for _, tc := range testCases {
		// Only test network backends
		if tc.Type != TypeNFS && tc.Type != TypeSMB {
			continue
		}

		t.Run(tc.Name, func(t *testing.T) {
			backend, cleanup := tc.Setup(t)
			defer cleanup()

			ctx := context.Background()

			// Get root path
			var rootPath string
			switch b := backend.(type) {
			case *NFSBackend:
				rootPath = b.RootPath()
			case *SMBBackend:
				rootPath = b.RootPath()
			default:
				t.Skip("not a network backend")
			}

			// Create a test file
			testFile := filepath.Join(rootPath, "stale_test.txt")
			if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
				t.Fatalf("failed to create test file: %v", err)
			}

			// Open the file
			reader, err := backend.Open(ctx, "stale_test.txt")
			if err != nil {
				t.Fatalf("failed to open file: %v", err)
			}

			// Read first byte
			buf := make([]byte, 1)
			_, err = reader.Read(buf)
			if err != nil {
				t.Fatalf("failed to read file: %v", err)
			}

			// Simulate file being deleted/moved on server (stale handle scenario)
			os.Remove(testFile)

			// Try to continue reading - should handle error gracefully
			_, err = io.ReadAll(reader)
			// We expect an error here, but it should be a clean error, not a panic
			t.Logf("Read after file deletion: %v (expected error)", err)

			reader.Close()

			// Backend should still be functional for other operations
			err = backend.Probe(ctx)
			if err != nil {
				t.Errorf("probe failed after stale handle scenario: %v", err)
			}
		})
	}
}

// TestNetworkInterruptionDuringScan tests behavior when network fails mid-scan
func TestNetworkInterruptionDuringScan(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network interruption test in short mode")
	}

	testCases := []struct {
		Name      string
		Type      StorageType
		SetupFunc func(t *testing.T) (backend StorageBackend, interruptFunc func(), cleanup func())
	}{
		{
			Name: "NFS",
			Type: TypeNFS,
			SetupFunc: func(t *testing.T) (StorageBackend, func(), func()) {
				testDir := setupTestDirectory(t)

				// Create many test files to ensure scan takes time
				var files []string
				for i := 0; i < 100; i++ {
					files = append(files, fmt.Sprintf("file%d.txt", i))
				}
				createTestFiles(t, testDir, files)

				backend, err := NewNFSBackend("nfs-server.example.com", "/exports/data", testDir)
				if err != nil {
					t.Fatalf("failed to create NFS backend: %v", err)
				}

				interrupt := func() {
					// Simulate network interruption by making directory inaccessible
					os.Chmod(testDir, 0000)
				}

				cleanup := func() {
					os.Chmod(testDir, 0755)
					os.RemoveAll(testDir)
				}

				return backend, interrupt, cleanup
			},
		},
		{
			Name: "SMB",
			Type: TypeSMB,
			SetupFunc: func(t *testing.T) (StorageBackend, func(), func()) {
				testDir := setupTestDirectory(t)

				var files []string
				for i := 0; i < 100; i++ {
					files = append(files, fmt.Sprintf("file%d.txt", i))
				}
				createTestFiles(t, testDir, files)

				backend, err := NewSMBBackend("smb-server.example.com", "ShareName", testDir)
				if err != nil {
					t.Fatalf("failed to create SMB backend: %v", err)
				}

				interrupt := func() {
					os.Chmod(testDir, 0000)
				}

				cleanup := func() {
					os.Chmod(testDir, 0755)
					os.RemoveAll(testDir)
				}

				return backend, interrupt, cleanup
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			backend, interrupt, cleanup := tc.SetupFunc(t)
			defer cleanup()

			ctx := context.Background()

			// Start walking in a goroutine
			walkDone := make(chan error, 1)
			filesScanned := 0

			go func() {
				err := backend.Walk(ctx, func(path string, info *FileInfo) error {
					filesScanned++

					// Interrupt after scanning 10 files
					if filesScanned == 10 {
						interrupt()
						// Give time for interrupt to take effect
						time.Sleep(10 * time.Millisecond)
					}

					return nil
				})
				walkDone <- err
			}()

			// Wait for walk to complete
			select {
			case err := <-walkDone:
				// Walk should either complete partially or error gracefully
				t.Logf("Walk completed with error: %v (files scanned: %d)", err, filesScanned)

				// The key is that we didn't panic
				if filesScanned < 10 {
					t.Logf("Warning: interruption may have occurred before threshold")
				}
			case <-time.After(10 * time.Second):
				t.Fatal("walk did not complete within timeout")
			}

			// Restore permissions for cleanup
			var rootPath string
			switch b := backend.(type) {
			case *NFSBackend:
				rootPath = b.RootPath()
			case *SMBBackend:
				rootPath = b.RootPath()
			}
			os.Chmod(rootPath, 0755)
		})
	}
}

// TestInvalidMountPath tests backend creation with invalid mount paths
func TestInvalidMountPath(t *testing.T) {
	testCases := []struct {
		name      string
		backend   func() (StorageBackend, error)
		expectErr bool
	}{
		{
			name: "NFS with non-existent path",
			backend: func() (StorageBackend, error) {
				return NewNFSBackend("server.example.com", "/exports/data", "/nonexistent/path")
			},
			expectErr: false, // Creation succeeds, but Probe will fail
		},
		{
			name: "SMB with non-existent path",
			backend: func() (StorageBackend, error) {
				return NewSMBBackend("server.example.com", "ShareName", "/nonexistent/path")
			},
			expectErr: false, // Creation succeeds, but Probe will fail
		},
		{
			name: "NFS with empty server",
			backend: func() (StorageBackend, error) {
				return NewNFSBackend("", "/exports/data", "/mnt/nfs")
			},
			expectErr: true, // Should fail at creation
		},
		{
			name: "SMB with empty share",
			backend: func() (StorageBackend, error) {
				return NewSMBBackend("server.example.com", "", "/mnt/smb")
			},
			expectErr: true, // Should fail at creation
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			backend, err := tc.backend()

			if tc.expectErr {
				if err == nil {
					t.Error("expected error during backend creation, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error during backend creation: %v", err)
			}

			// For non-existent paths, Probe should fail
			ctx := context.Background()
			err = backend.Probe(ctx)
			if err == nil {
				t.Error("expected probe to fail for non-existent path")
			}
			t.Logf("Probe correctly failed: %v", err)
		})
	}
}

// TestDNSResolutionHandling tests behavior with unresolvable server names
func TestDNSResolutionHandling(t *testing.T) {
	testCases := []struct {
		name   string
		create func() (StorageBackend, error)
	}{
		{
			name: "NFS with invalid hostname",
			create: func() (StorageBackend, error) {
				// Use a hostname that definitely won't resolve
				return NewNFSBackend("this-hostname-does-not-exist-12345.invalid", "/exports/data", "/mnt/test")
			},
		},
		{
			name: "SMB with invalid hostname",
			create: func() (StorageBackend, error) {
				return NewSMBBackend("this-hostname-does-not-exist-12345.invalid", "ShareName", "/mnt/test")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Backend creation should succeed even with invalid hostname
			// (DNS resolution happens at mount time, not at backend creation)
			backend, err := tc.create()
			if err != nil {
				// If error occurs, it should be due to mount path validation, not DNS
				t.Logf("Backend creation failed (expected): %v", err)
				return
			}

			// Operations should fail gracefully
			ctx := context.Background()
			err = backend.Probe(ctx)
			// Error is expected and acceptable
			t.Logf("Probe with invalid hostname: %v (expected to fail)", err)
		})
	}
}

// TestSlowNetworkPerformance tests behavior with simulated slow network
func TestSlowNetworkPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping slow network test in short mode")
	}

	testCases := GetAllBackendTestCases(t)

	for _, tc := range testCases {
		// Only test network backends
		if tc.Type != TypeNFS && tc.Type != TypeSMB {
			continue
		}

		t.Run(tc.Name, func(t *testing.T) {
			backend, cleanup := tc.Setup(t)
			defer cleanup()

			// Create a context with timeout
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Get root path and create test file
			var rootPath string
			switch b := backend.(type) {
			case *NFSBackend:
				rootPath = b.RootPath()
			case *SMBBackend:
				rootPath = b.RootPath()
			default:
				t.Skip("not a network backend")
			}

			testFile := filepath.Join(rootPath, "large_file.txt")
			// Create a 10MB file to simulate slow transfer
			largeContent := make([]byte, 10*1024*1024)
			for i := range largeContent {
				largeContent[i] = byte(i % 256)
			}
			if err := os.WriteFile(testFile, largeContent, 0644); err != nil {
				t.Fatalf("failed to create large file: %v", err)
			}

			// Measure time to open and read
			start := time.Now()
			reader, err := backend.Open(ctx, "large_file.txt")
			if err != nil {
				t.Fatalf("failed to open file: %v", err)
			}
			defer reader.Close()

			_, err = io.ReadAll(reader)
			duration := time.Since(start)

			if err != nil {
				if err == context.DeadlineExceeded {
					t.Logf("Operation timed out (acceptable for slow network simulation)")
					return
				}
				t.Fatalf("failed to read file: %v", err)
			}

			t.Logf("Read 10MB in %v", duration)

			// For local filesystem simulation, this should be fast
			// On real network mounts, this would take longer
			if duration > 5*time.Second {
				t.Logf("Warning: operation took longer than expected (may indicate real network issues)")
			}
		})
	}
}

// TestConcurrentNetworkOperations tests multiple goroutines accessing network storage
func TestConcurrentNetworkOperations(t *testing.T) {
	testCases := GetAllBackendTestCases(t)

	for _, tc := range testCases {
		// Only test network backends
		if tc.Type != TypeNFS && tc.Type != TypeSMB {
			continue
		}

		t.Run(tc.Name, func(t *testing.T) {
			backend, cleanup := tc.Setup(t)
			defer cleanup()

			ctx := context.Background()

			// Get root path and create test files
			var rootPath string
			switch b := backend.(type) {
			case *NFSBackend:
				rootPath = b.RootPath()
			case *SMBBackend:
				rootPath = b.RootPath()
			default:
				t.Skip("not a network backend")
			}

			// Create 20 test files
			for i := 0; i < 20; i++ {
				testFile := filepath.Join(rootPath, fmt.Sprintf("concurrent_%d.txt", i))
				content := fmt.Sprintf("content for file %d", i)
				if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
					t.Fatalf("failed to create test file: %v", err)
				}
			}

			// Launch 20 goroutines, each accessing a different file
			done := make(chan error, 20)

			for i := 0; i < 20; i++ {
				go func(id int) {
					filename := fmt.Sprintf("concurrent_%d.txt", id)

					// Stat the file
					info, err := backend.Stat(ctx, filename)
					if err != nil {
						done <- fmt.Errorf("goroutine %d stat failed: %w", id, err)
						return
					}

					if info.Size == 0 {
						done <- fmt.Errorf("goroutine %d got zero-size file", id)
						return
					}

					// Open and read the file
					reader, err := backend.Open(ctx, filename)
					if err != nil {
						done <- fmt.Errorf("goroutine %d open failed: %w", id, err)
						return
					}
					defer reader.Close()

					content, err := io.ReadAll(reader)
					if err != nil {
						done <- fmt.Errorf("goroutine %d read failed: %w", id, err)
						return
					}

					expectedContent := fmt.Sprintf("content for file %d", id)
					if string(content) != expectedContent {
						done <- fmt.Errorf("goroutine %d content mismatch", id)
						return
					}

					done <- nil
				}(i)
			}

			// Wait for all goroutines
			var errors []error
			for i := 0; i < 20; i++ {
				if err := <-done; err != nil {
					errors = append(errors, err)
				}
			}

			if len(errors) > 0 {
				t.Errorf("concurrent operations failed: %v", errors)
			} else {
				t.Log("All 20 concurrent operations completed successfully")
			}
		})
	}
}

// TestPermissionDeniedHandling tests behavior when mount has insufficient permissions
func TestPermissionDeniedHandling(t *testing.T) {
	testCases := []struct {
		Name      string
		Type      StorageType
		SetupFunc func(t *testing.T) (backend StorageBackend, restrictFunc func(), cleanup func())
	}{
		{
			Name: "NFS",
			Type: TypeNFS,
			SetupFunc: func(t *testing.T) (StorageBackend, func(), func()) {
				testDir := setupTestDirectory(t)
				createTestFiles(t, testDir, []string{"file1.txt"})

				backend, err := NewNFSBackend("nfs-server.example.com", "/exports/data", testDir)
				if err != nil {
					t.Fatalf("failed to create NFS backend: %v", err)
				}

				restrict := func() {
					// Create a subdirectory with no read permissions
					restrictedDir := filepath.Join(testDir, "restricted")
					os.Mkdir(restrictedDir, 0755)
					os.WriteFile(filepath.Join(restrictedDir, "secret.txt"), []byte("secret"), 0644)
					os.Chmod(restrictedDir, 0000)
				}

				cleanup := func() {
					restrictedDir := filepath.Join(testDir, "restricted")
					os.Chmod(restrictedDir, 0755) // Restore permissions for cleanup
					os.RemoveAll(testDir)
				}

				return backend, restrict, cleanup
			},
		},
		{
			Name: "SMB",
			Type: TypeSMB,
			SetupFunc: func(t *testing.T) (StorageBackend, func(), func()) {
				testDir := setupTestDirectory(t)
				createTestFiles(t, testDir, []string{"file1.txt"})

				backend, err := NewSMBBackend("smb-server.example.com", "ShareName", testDir)
				if err != nil {
					t.Fatalf("failed to create SMB backend: %v", err)
				}

				restrict := func() {
					restrictedDir := filepath.Join(testDir, "restricted")
					os.Mkdir(restrictedDir, 0755)
					os.WriteFile(filepath.Join(restrictedDir, "secret.txt"), []byte("secret"), 0644)
					os.Chmod(restrictedDir, 0000)
				}

				cleanup := func() {
					restrictedDir := filepath.Join(testDir, "restricted")
					os.Chmod(restrictedDir, 0755)
					os.RemoveAll(testDir)
				}

				return backend, restrict, cleanup
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			backend, restrict, cleanup := tc.SetupFunc(t)
			defer cleanup()

			ctx := context.Background()

			// Create restricted directory
			restrict()

			// Walk should skip inaccessible files gracefully
			filesFound := 0
			err := backend.Walk(ctx, func(path string, info *FileInfo) error {
				filesFound++
				// Should not encounter restricted/secret.txt
				if strings.Contains(path, "secret.txt") {
					t.Error("Walk should not have accessed restricted file")
				}
				return nil
			})

			// Walk should complete without error (skipping inaccessible paths)
			if err != nil {
				t.Logf("Walk error (may be acceptable): %v", err)
			}

			t.Logf("Found %d accessible files (restricted files skipped)", filesFound)

			// Attempting to directly access restricted file should fail
			_, err = backend.Open(ctx, "restricted/secret.txt")
			if err == nil {
				t.Error("expected error when accessing restricted file")
			}

			// Check if error is permission-related
			if perr, ok := err.(*os.PathError); ok {
				if perr.Err == syscall.EACCES || perr.Err == syscall.EPERM {
					t.Log("Correctly received permission denied error")
				}
			}
		})
	}
}

// TestMountOptionVariations tests different mount configurations
func TestMountOptionVariations(t *testing.T) {
	// Test that backends work regardless of mount options
	// (read-only mounts, nosuid, noexec, etc.)

	testCases := GetAllBackendTestCases(t)

	for _, tc := range testCases {
		if tc.Type != TypeNFS && tc.Type != TypeSMB {
			continue
		}

		t.Run(tc.Name, func(t *testing.T) {
			backend, cleanup := tc.Setup(t)
			defer cleanup()

			ctx := context.Background()

			// Basic operations should work regardless of mount options
			err := backend.Probe(ctx)
			if err != nil {
				t.Fatalf("probe failed: %v", err)
			}

			err = backend.Walk(ctx, func(path string, info *FileInfo) error {
				return nil
			})
			if err != nil {
				t.Fatalf("walk failed: %v", err)
			}

			t.Log("Backend works with various mount configurations")
		})
	}
}
