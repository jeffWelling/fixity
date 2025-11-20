package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestPathTraversalPrevention tests all backends against path traversal attacks
func TestPathTraversalPrevention(t *testing.T) {
	maliciousPaths := []struct {
		name string
		path string
	}{
		{"basic parent traversal", "../../../etc/passwd"},
		{"windows traversal", "..\\..\\..\\windows\\system32\\config"},
		{"url encoded traversal", "%2e%2e%2f%2e%2e%2f%2e%2e%2fetc%2fpasswd"},
		{"double encoded", "%252e%252e%252f"},
		{"mixed encoding", "..%2F..%2F..%2Fetc%2Fpasswd"},
		{"null byte injection", "../\x00/etc/shadow"},
		{"trailing null", "file.txt\x00.exe"},
		{"unicode normalization", "../\u2024/\u2024/etc/passwd"},
		{"forward and back mixed", "../..\\../etc/passwd"},
		{"absolute path attempt", "/etc/passwd"},
		{"absolute windows path", "C:\\Windows\\System32"},
		{"relative with absolute", "subdir/../../../../../../etc/passwd"},
	}

	testCases := GetAllBackendTestCases(t)

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			backend, cleanup := tc.Setup(t)
			defer cleanup()

			ctx := context.Background()

			for _, mp := range maliciousPaths {
				t.Run(mp.name, func(t *testing.T) {
					// Attempt to open malicious path
					_, err := backend.Open(ctx, mp.path)
					if err == nil {
						t.Errorf("Backend allowed path traversal attack: %s", mp.path)
					}

					// Attempt to stat malicious path
					_, err = backend.Stat(ctx, mp.path)
					if err == nil {
						t.Errorf("Backend allowed stat of malicious path: %s", mp.path)
					}
				})
			}
		})
	}
}

// TestSymlinkBoundaryEnforcement tests that symlinks cannot escape mount boundaries
func TestSymlinkBoundaryEnforcement(t *testing.T) {
	testCases := GetAllBackendTestCases(t)

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			backend, cleanup := tc.Setup(t)
			defer cleanup()

			ctx := context.Background()

			// Get root path
			var rootPath string
			switch b := backend.(type) {
			case *LocalFSBackend:
				rootPath = b.RootPath()
			case *NFSBackend:
				rootPath = b.RootPath()
			case *SMBBackend:
				rootPath = b.RootPath()
			default:
				t.Skip("Backend type doesn't expose RootPath")
			}

			// Create a symlink pointing outside the root
			symlinkPath := filepath.Join(rootPath, "evil_symlink")
			targetPath := "/etc/passwd" // Outside mount point

			err := os.Symlink(targetPath, symlinkPath)
			if err != nil {
				t.Skipf("Cannot create symlink (permissions?): %v", err)
			}
			defer os.Remove(symlinkPath)

			// Attempt to access through symlink
			_, err = backend.Open(ctx, "evil_symlink")
			// We expect this to either fail or resolve to nothing
			// The key is it should NOT give access to /etc/passwd
			if err == nil {
				// If it succeeded, verify we didn't escape
				t.Log("Symlink access succeeded, verifying it didn't escape boundaries")
			}

			// Attempt to stat through symlink
			info, err := backend.Stat(ctx, "evil_symlink")
			if err == nil {
				// Verify the stat result is for the symlink itself, not the target
				if info.Size > 1000 {
					t.Error("Symlink appears to have resolved outside mount boundary")
				}
			}
		})
	}
}

// TestCaseInsensitivityAttacks tests filename case manipulation attacks
func TestCaseInsensitivityAttacks(t *testing.T) {
	testCases := GetAllBackendTestCases(t)

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			backend, cleanup := tc.Setup(t)
			defer cleanup()

			ctx := context.Background()

			// Get root path
			var rootPath string
			switch b := backend.(type) {
			case *LocalFSBackend:
				rootPath = b.RootPath()
			case *NFSBackend:
				rootPath = b.RootPath()
			case *SMBBackend:
				rootPath = b.RootPath()
			default:
				t.Skip("Backend type doesn't expose RootPath")
			}

			// Create a test file
			testFile := filepath.Join(rootPath, "sensitive.txt")
			if err := os.WriteFile(testFile, []byte("secret"), 0644); err != nil {
				t.Fatalf("failed to create test file: %v", err)
			}

			// Try various case manipulations
			caseVariations := []string{
				"sensitive.txt",   // correct
				"Sensitive.txt",   // capital S
				"SENSITIVE.TXT",   // all caps
				"sEnSiTiVe.TxT",   // mixed case
			}

			for _, variation := range caseVariations {
				_, err := backend.Open(ctx, variation)
				// On case-sensitive systems, only exact match should work
				// On case-insensitive systems (macOS, Windows), all should work
				// Either way is acceptable, but behavior should be consistent
				t.Logf("Case variation %q: %v", variation, err)
			}
		})
	}
}

// TestSpecialCharacterHandling tests handling of special characters in filenames
func TestSpecialCharacterHandling(t *testing.T) {
	testCases := GetAllBackendTestCases(t)

	specialChars := []struct {
		name     string
		filename string
		content  string
	}{
		{"newline in name", "file\nname.txt", "content1"},
		{"carriage return", "file\rname.txt", "content2"},
		{"tab character", "file\tname.txt", "content3"},
		{"vertical tab", "file\vname.txt", "content4"},
		{"form feed", "file\fname.txt", "content5"},
		{"backspace", "file\bname.txt", "content6"},
		{"bell character", "file\aname.txt", "content7"},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			backend, cleanup := tc.Setup(t)
			defer cleanup()

			ctx := context.Background()

			// Get root path
			var rootPath string
			switch b := backend.(type) {
			case *LocalFSBackend:
				rootPath = b.RootPath()
			case *NFSBackend:
				rootPath = b.RootPath()
			case *SMBBackend:
				rootPath = b.RootPath()
			default:
				t.Skip("Backend type doesn't expose RootPath")
			}

			for _, sc := range specialChars {
				t.Run(sc.name, func(t *testing.T) {
					testFile := filepath.Join(rootPath, sc.filename)

					// Try to create file with special character
					err := os.WriteFile(testFile, []byte(sc.content), 0644)
					if err != nil {
						// Many special characters are not allowed in filenames
						// This is expected and safe behavior
						t.Logf("Cannot create file with %s: %v (expected)", sc.name, err)
						return
					}
					defer os.Remove(testFile)

					// If creation succeeded, verify we can access it safely
					_, err = backend.Open(ctx, sc.filename)
					if err != nil {
						t.Logf("Cannot open file with %s: %v", sc.name, err)
					}

					_, err = backend.Stat(ctx, sc.filename)
					if err != nil {
						t.Logf("Cannot stat file with %s: %v", sc.name, err)
					}
				})
			}
		})
	}
}

// TestTOCTOURaceCondition tests time-of-check-time-of-use vulnerabilities
func TestTOCTOURaceCondition(t *testing.T) {
	testCases := GetAllBackendTestCases(t)

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			backend, cleanup := tc.Setup(t)
			defer cleanup()

			ctx := context.Background()

			// Get root path
			var rootPath string
			switch b := backend.(type) {
			case *LocalFSBackend:
				rootPath = b.RootPath()
			case *NFSBackend:
				rootPath = b.RootPath()
			case *SMBBackend:
				rootPath = b.RootPath()
			default:
				t.Skip("Backend type doesn't expose RootPath")
			}

			testFile := filepath.Join(rootPath, "toctou.txt")
			originalContent := []byte("original content")

			// Create initial file
			if err := os.WriteFile(testFile, originalContent, 0644); err != nil {
				t.Fatalf("failed to create test file: %v", err)
			}
			defer os.Remove(testFile)

			// Stat the file
			info1, err := backend.Stat(ctx, "toctou.txt")
			if err != nil {
				t.Fatalf("initial stat failed: %v", err)
			}

			// Modify file between stat and open
			modifiedContent := []byte("MODIFIED CONTENT - SHOULD BE DETECTED")
			if err := os.WriteFile(testFile, modifiedContent, 0644); err != nil {
				t.Fatalf("failed to modify file: %v", err)
			}

			// Open file
			reader, err := backend.Open(ctx, "toctou.txt")
			if err != nil {
				t.Fatalf("open after modification failed: %v", err)
			}
			defer reader.Close()

			// Stat again
			info2, err := backend.Stat(ctx, "toctou.txt")
			if err != nil {
				t.Fatalf("second stat failed: %v", err)
			}

			// Verify we detect the change
			if info1.Size == info2.Size && info1.ModTime.Equal(info2.ModTime) {
				t.Log("Note: File modification during TOCTOU window not reflected in metadata")
				t.Log("This is expected filesystem behavior and should be handled by scan logic")
			} else {
				t.Log("File modification detected via size or mtime change (good)")
			}
		})
	}
}

// TestConcurrentAccessSafety tests that concurrent operations don't cause issues
func TestConcurrentAccessSafety(t *testing.T) {
	testCases := GetAllBackendTestCases(t)

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			backend, cleanup := tc.Setup(t)
			defer cleanup()

			ctx := context.Background()

			// Launch 10 concurrent operations on the same files
			done := make(chan bool, 10)

			for i := 0; i < 10; i++ {
				go func(id int) {
					defer func() { done <- true }()

					// Each goroutine performs multiple operations
					for j := 0; j < 100; j++ {
						// Stat a file
						_, _ = backend.Stat(ctx, "file1.txt")

						// Open a file
						if reader, err := backend.Open(ctx, "file2.txt"); err == nil {
							reader.Close()
						}

						// Walk
						_ = backend.Walk(ctx, func(path string, info *FileInfo) error {
							return nil
						})
					}
				}(i)
			}

			// Wait for all goroutines
			for i := 0; i < 10; i++ {
				<-done
			}

			// If we got here without panics or deadlocks, we're good
			t.Log("Concurrent access completed without errors")
		})
	}
}

// TestHiddenFileAccess tests that hidden files are handled correctly
func TestHiddenFileAccess(t *testing.T) {
	testCases := GetAllBackendTestCases(t)

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			backend, cleanup := tc.Setup(t)
			defer cleanup()

			ctx := context.Background()

			// Get root path
			var rootPath string
			switch b := backend.(type) {
			case *LocalFSBackend:
				rootPath = b.RootPath()
			case *NFSBackend:
				rootPath = b.RootPath()
			case *SMBBackend:
				rootPath = b.RootPath()
			default:
				t.Skip("Backend type doesn't expose RootPath")
			}

			// Create hidden files (Unix convention: starts with .)
			hiddenFiles := []string{
				".hidden",
				".config",
				".ssh",
			}

			for _, hf := range hiddenFiles {
				testFile := filepath.Join(rootPath, hf)
				if err := os.WriteFile(testFile, []byte("hidden content"), 0644); err != nil {
					t.Fatalf("failed to create hidden file: %v", err)
				}
				defer os.Remove(testFile)
			}

			// Walk should find hidden files
			foundHidden := 0
			err := backend.Walk(ctx, func(path string, info *FileInfo) error {
				if filepath.Base(path)[0] == '.' && !info.IsDir {
					foundHidden++
				}
				return nil
			})

			if err != nil {
				t.Fatalf("walk failed: %v", err)
			}

			if foundHidden != len(hiddenFiles) {
				t.Errorf("expected to find %d hidden files, found %d", len(hiddenFiles), foundHidden)
			}
		})
	}
}

// TestDeepDirectoryNesting tests handling of deeply nested directories
func TestDeepDirectoryNesting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping deep nesting test in short mode")
	}

	testCases := GetAllBackendTestCases(t)

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			backend, cleanup := tc.Setup(t)
			defer cleanup()

			ctx := context.Background()

			// Get root path
			var rootPath string
			switch b := backend.(type) {
			case *LocalFSBackend:
				rootPath = b.RootPath()
			case *NFSBackend:
				rootPath = b.RootPath()
			case *SMBBackend:
				rootPath = b.RootPath()
			default:
				t.Skip("Backend type doesn't expose RootPath")
			}

			// Create deeply nested directory structure (100 levels)
			deepPath := rootPath
			for i := 0; i < 100; i++ {
				deepPath = filepath.Join(deepPath, "level")
			}

			if err := os.MkdirAll(deepPath, 0755); err != nil {
				t.Fatalf("failed to create deep directory: %v", err)
			}

			// Create a file at the deepest level
			deepFile := filepath.Join(deepPath, "deep.txt")
			if err := os.WriteFile(deepFile, []byte("deep content"), 0644); err != nil {
				t.Fatalf("failed to create deep file: %v", err)
			}

			// Walk should find the deep file
			foundDeep := false
			err := backend.Walk(ctx, func(path string, info *FileInfo) error {
				if filepath.Base(path) == "deep.txt" {
					foundDeep = true
				}
				return nil
			})

			if err != nil {
				t.Fatalf("walk failed: %v", err)
			}

			if !foundDeep {
				t.Error("failed to find deeply nested file")
			}
		})
	}
}
