package checksum_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/jeffanddom/fixity/internal/checksum"
)

// Benchmark small files (1KB)
func BenchmarkMD5_1KB(b *testing.B) {
	data := bytes.Repeat([]byte("a"), 1024)
	b.ResetTimer()
	b.SetBytes(1024)

	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		_, err := checksum.ComputeMD5(reader)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSHA256_1KB(b *testing.B) {
	data := bytes.Repeat([]byte("a"), 1024)
	b.ResetTimer()
	b.SetBytes(1024)

	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		_, err := checksum.ComputeSHA256(reader)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBLAKE3_1KB(b *testing.B) {
	data := bytes.Repeat([]byte("a"), 1024)
	b.ResetTimer()
	b.SetBytes(1024)

	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		_, err := checksum.ComputeBLAKE3(reader)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark medium files (1MB)
func BenchmarkMD5_1MB(b *testing.B) {
	data := bytes.Repeat([]byte("a"), 1024*1024)
	b.ResetTimer()
	b.SetBytes(1024 * 1024)

	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		_, err := checksum.ComputeMD5(reader)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSHA256_1MB(b *testing.B) {
	data := bytes.Repeat([]byte("a"), 1024*1024)
	b.ResetTimer()
	b.SetBytes(1024 * 1024)

	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		_, err := checksum.ComputeSHA256(reader)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBLAKE3_1MB(b *testing.B) {
	data := bytes.Repeat([]byte("a"), 1024*1024)
	b.ResetTimer()
	b.SetBytes(1024 * 1024)

	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		_, err := checksum.ComputeBLAKE3(reader)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark large files (100MB)
func BenchmarkMD5_100MB(b *testing.B) {
	data := bytes.Repeat([]byte("a"), 100*1024*1024)
	b.ResetTimer()
	b.SetBytes(100 * 1024 * 1024)

	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		_, err := checksum.ComputeMD5(reader)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSHA256_100MB(b *testing.B) {
	data := bytes.Repeat([]byte("a"), 100*1024*1024)
	b.ResetTimer()
	b.SetBytes(100 * 1024 * 1024)

	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		_, err := checksum.ComputeSHA256(reader)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBLAKE3_100MB(b *testing.B) {
	data := bytes.Repeat([]byte("a"), 100*1024*1024)
	b.ResetTimer()
	b.SetBytes(100 * 1024 * 1024)

	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		_, err := checksum.ComputeBLAKE3(reader)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark real file operations
func BenchmarkRealFile_MD5_10MB(b *testing.B) {
	tmpDir := b.TempDir()
	tmpFile := filepath.Join(tmpDir, "benchmark.dat")

	// Create 10MB test file
	data := bytes.Repeat([]byte("a"), 10*1024*1024)
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.SetBytes(10 * 1024 * 1024)

	for i := 0; i < b.N; i++ {
		file, err := os.Open(tmpFile)
		if err != nil {
			b.Fatal(err)
		}

		_, err = checksum.ComputeMD5(file)
		file.Close()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRealFile_SHA256_10MB(b *testing.B) {
	tmpDir := b.TempDir()
	tmpFile := filepath.Join(tmpDir, "benchmark.dat")

	// Create 10MB test file
	data := bytes.Repeat([]byte("a"), 10*1024*1024)
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.SetBytes(10 * 1024 * 1024)

	for i := 0; i < b.N; i++ {
		file, err := os.Open(tmpFile)
		if err != nil {
			b.Fatal(err)
		}

		_, err = checksum.ComputeSHA256(file)
		file.Close()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRealFile_BLAKE3_10MB(b *testing.B) {
	tmpDir := b.TempDir()
	tmpFile := filepath.Join(tmpDir, "benchmark.dat")

	// Create 10MB test file
	data := bytes.Repeat([]byte("a"), 10*1024*1024)
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.SetBytes(10 * 1024 * 1024)

	for i := 0; i < b.N; i++ {
		file, err := os.Open(tmpFile)
		if err != nil {
			b.Fatal(err)
		}

		_, err = checksum.ComputeBLAKE3(file)
		file.Close()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark worker pool throughput
func BenchmarkWorkerPool_1Worker(b *testing.B) {
	pool := checksum.NewWorkerPool(1)
	pool.Start()
	defer pool.Stop()

	data := bytes.Repeat([]byte("a"), 1024*1024) // 1MB
	b.ResetTimer()
	b.SetBytes(1024 * 1024)

	for i := 0; i < b.N; i++ {
		job := &checksum.Job{
			Path:      "test.dat",
			Algorithm: checksum.AlgorithmBLAKE3,
			Opener: func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(data)), nil
			},
		}

		pool.Submit(job)
		result := <-pool.Results()
		if result.Error != nil {
			b.Fatal(result.Error)
		}
	}
}

func BenchmarkWorkerPool_4Workers(b *testing.B) {
	pool := checksum.NewWorkerPool(4)
	pool.Start()
	defer pool.Stop()

	data := bytes.Repeat([]byte("a"), 1024*1024) // 1MB
	b.ResetTimer()
	b.SetBytes(1024 * 1024)

	for i := 0; i < b.N; i++ {
		job := &checksum.Job{
			Path:      "test.dat",
			Algorithm: checksum.AlgorithmBLAKE3,
			Opener: func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(data)), nil
			},
		}

		pool.Submit(job)
		result := <-pool.Results()
		if result.Error != nil {
			b.Fatal(result.Error)
		}
	}
}

func BenchmarkWorkerPool_8Workers(b *testing.B) {
	pool := checksum.NewWorkerPool(8)
	pool.Start()
	defer pool.Stop()

	data := bytes.Repeat([]byte("a"), 1024*1024) // 1MB
	b.ResetTimer()
	b.SetBytes(1024 * 1024)

	for i := 0; i < b.N; i++ {
		job := &checksum.Job{
			Path:      "test.dat",
			Algorithm: checksum.AlgorithmBLAKE3,
			Opener: func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(data)), nil
			},
		}

		pool.Submit(job)
		result := <-pool.Results()
		if result.Error != nil {
			b.Fatal(result.Error)
		}
	}
}
