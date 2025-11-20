package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// BenchmarkBackend_Walk benchmarks directory traversal across all backends
func BenchmarkBackend_Walk(b *testing.B) {
	fileCounts := []int{10, 100, 1000}
	backends := []struct {
		name string
		typ  StorageType
	}{
		{"Local", TypeLocal},
		{"NFS", TypeNFS},
		{"SMB", TypeSMB},
	}

	for _, fileCount := range fileCounts {
		for _, be := range backends {
			b.Run(fmt.Sprintf("%s/%d_files", be.name, fileCount), func(b *testing.B) {
				// Create custom test directory with specific file count
				testDir := setupBenchDirectory(b, fileCount)
				defer os.RemoveAll(testDir)

				var backend StorageBackend
				var err error

				switch be.typ {
				case TypeLocal:
					backend, err = NewLocalFSBackend(testDir)
				case TypeNFS:
					backend, err = NewNFSBackend("bench-nfs.example.com", "/exports/bench", testDir)
				case TypeSMB:
					backend, err = NewSMBBackend("bench-smb.example.com", "BenchShare", testDir)
				}

				if err != nil {
					b.Fatal(err)
				}
				defer backend.Close()

				ctx := context.Background()

				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					fileCount := 0
					err := backend.Walk(ctx, func(path string, info *FileInfo) error {
						fileCount++
						return nil
					})
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

// BenchmarkBackend_Open benchmarks file opening across all backends
func BenchmarkBackend_Open(b *testing.B) {
	fileSizes := []struct {
		name string
		size int
	}{
		{"1KB", 1024},
		{"10KB", 10 * 1024},
		{"100KB", 100 * 1024},
		{"1MB", 1024 * 1024},
		{"10MB", 10 * 1024 * 1024},
	}

	backends := []struct {
		name string
		typ  StorageType
	}{
		{"Local", TypeLocal},
		{"NFS", TypeNFS},
		{"SMB", TypeSMB},
	}

	for _, size := range fileSizes {
		for _, be := range backends {
			b.Run(fmt.Sprintf("%s/%s", be.name, size.name), func(b *testing.B) {
				testDir := setupBenchDirectory(b, 1)
				defer os.RemoveAll(testDir)

				// Create test file with specific size
				testFile := filepath.Join(testDir, "bench.dat")
				data := make([]byte, size.size)
				for i := range data {
					data[i] = byte(i % 256)
				}
				if err := os.WriteFile(testFile, data, 0644); err != nil {
					b.Fatal(err)
				}

				var backend StorageBackend
				var err error

				switch be.typ {
				case TypeLocal:
					backend, err = NewLocalFSBackend(testDir)
				case TypeNFS:
					backend, err = NewNFSBackend("bench-nfs.example.com", "/exports/bench", testDir)
				case TypeSMB:
					backend, err = NewSMBBackend("bench-smb.example.com", "BenchShare", testDir)
				}

				if err != nil {
					b.Fatal(err)
				}
				defer backend.Close()

				ctx := context.Background()
				b.SetBytes(int64(size.size))
				b.ResetTimer()

				for i := 0; i < b.N; i++ {
					reader, err := backend.Open(ctx, "bench.dat")
					if err != nil {
						b.Fatal(err)
					}
					reader.Close()
				}
			})
		}
	}
}

// BenchmarkBackend_Stat benchmarks file stat operations
func BenchmarkBackend_Stat(b *testing.B) {
	backends := []struct {
		name string
		typ  StorageType
	}{
		{"Local", TypeLocal},
		{"NFS", TypeNFS},
		{"SMB", TypeSMB},
	}

	for _, be := range backends {
		b.Run(be.name, func(b *testing.B) {
			testDir := setupBenchDirectory(b, 10)
			defer os.RemoveAll(testDir)

			var backend StorageBackend
			var err error

			switch be.typ {
			case TypeLocal:
				backend, err = NewLocalFSBackend(testDir)
			case TypeNFS:
				backend, err = NewNFSBackend("bench-nfs.example.com", "/exports/bench", testDir)
			case TypeSMB:
				backend, err = NewSMBBackend("bench-smb.example.com", "BenchShare", testDir)
			}

			if err != nil {
				b.Fatal(err)
			}
			defer backend.Close()

			ctx := context.Background()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := backend.Stat(ctx, "file0000.txt")
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkBackend_ConcurrentWalk benchmarks concurrent directory traversal
func BenchmarkBackend_ConcurrentWalk(b *testing.B) {
	const fileCount = 1000

	backends := []struct {
		name string
		typ  StorageType
	}{
		{"Local", TypeLocal},
		{"NFS", TypeNFS},
		{"SMB", TypeSMB},
	}

	for _, be := range backends {
		b.Run(be.name, func(b *testing.B) {
			testDir := setupBenchDirectory(b, fileCount)
			defer os.RemoveAll(testDir)

			var backend StorageBackend
			var err error

			switch be.typ {
			case TypeLocal:
				backend, err = NewLocalFSBackend(testDir)
			case TypeNFS:
				backend, err = NewNFSBackend("bench-nfs.example.com", "/exports/bench", testDir)
			case TypeSMB:
				backend, err = NewSMBBackend("bench-smb.example.com", "BenchShare", testDir)
			}

			if err != nil {
				b.Fatal(err)
			}
			defer backend.Close()

			ctx := context.Background()
			b.ResetTimer()

			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					fileCount := 0
					err := backend.Walk(ctx, func(path string, info *FileInfo) error {
						fileCount++
						return nil
					})
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}

// BenchmarkBackend_DeepNesting benchmarks deeply nested directory structures
func BenchmarkBackend_DeepNesting(b *testing.B) {
	depths := []int{10, 50, 100}

	backends := []struct {
		name string
		typ  StorageType
	}{
		{"Local", TypeLocal},
		{"NFS", TypeNFS},
		{"SMB", TypeSMB},
	}

	for _, depth := range depths {
		for _, be := range backends {
			b.Run(fmt.Sprintf("%s/%d_levels", be.name, depth), func(b *testing.B) {
				testDir := setupDeepDirectory(b, depth)
				defer os.RemoveAll(testDir)

				var backend StorageBackend
				var err error

				switch be.typ {
				case TypeLocal:
					backend, err = NewLocalFSBackend(testDir)
				case TypeNFS:
					backend, err = NewNFSBackend("bench-nfs.example.com", "/exports/bench", testDir)
				case TypeSMB:
					backend, err = NewSMBBackend("bench-smb.example.com", "BenchShare", testDir)
				}

				if err != nil {
					b.Fatal(err)
				}
				defer backend.Close()

				ctx := context.Background()
				b.ResetTimer()

				for i := 0; i < b.N; i++ {
					fileCount := 0
					err := backend.Walk(ctx, func(path string, info *FileInfo) error {
						fileCount++
						return nil
					})
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

// BenchmarkBackend_ManySmallFiles benchmarks handling many small files
func BenchmarkBackend_ManySmallFiles(b *testing.B) {
	const fileCount = 10000
	const fileSize = 1024 // 1KB each

	backends := []struct {
		name string
		typ  StorageType
	}{
		{"Local", TypeLocal},
		{"NFS", TypeNFS},
		{"SMB", TypeSMB},
	}

	for _, be := range backends {
		b.Run(be.name, func(b *testing.B) {
			testDir := setupBenchDirectory(b, fileCount)
			defer os.RemoveAll(testDir)

			var backend StorageBackend
			var err error

			switch be.typ {
			case TypeLocal:
				backend, err = NewLocalFSBackend(testDir)
			case TypeNFS:
				backend, err = NewNFSBackend("bench-nfs.example.com", "/exports/bench", testDir)
			case TypeSMB:
				backend, err = NewSMBBackend("bench-smb.example.com", "BenchShare", testDir)
			}

			if err != nil {
				b.Fatal(err)
			}
			defer backend.Close()

			ctx := context.Background()
			b.SetBytes(fileCount * fileSize)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				fileCount := 0
				err := backend.Walk(ctx, func(path string, info *FileInfo) error {
					fileCount++
					return nil
				})
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Helper functions for benchmark setup

func setupBenchDirectory(b *testing.B, fileCount int) string {
	b.Helper()

	tempDir, err := os.MkdirTemp("", "fixity-bench-*")
	if err != nil {
		b.Fatalf("failed to create temp dir: %v", err)
	}

	// Create files
	for i := 0; i < fileCount; i++ {
		filename := filepath.Join(tempDir, fmt.Sprintf("file%04d.txt", i))
		content := fmt.Sprintf("Benchmark file %d\n", i)
		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			b.Fatalf("failed to create file: %v", err)
		}
	}

	return tempDir
}

func setupDeepDirectory(b *testing.B, depth int) string {
	b.Helper()

	tempDir, err := os.MkdirTemp("", "fixity-bench-deep-*")
	if err != nil {
		b.Fatalf("failed to create temp dir: %v", err)
	}

	// Create deeply nested structure
	currentPath := tempDir
	for i := 0; i < depth; i++ {
		currentPath = filepath.Join(currentPath, fmt.Sprintf("level%d", i))
		if err := os.Mkdir(currentPath, 0755); err != nil {
			b.Fatalf("failed to create directory: %v", err)
		}

		// Add a file at each level
		filename := filepath.Join(currentPath, fmt.Sprintf("file%d.txt", i))
		content := fmt.Sprintf("File at depth %d\n", i)
		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			b.Fatalf("failed to create file: %v", err)
		}
	}

	return tempDir
}
