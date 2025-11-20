package checksum_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jeffanddom/fixity/internal/checksum"
)

func TestComputeMD5(t *testing.T) {
	t.Run("computes correct MD5 for known input", func(t *testing.T) {
		input := strings.NewReader("hello world")
		hash, err := checksum.ComputeMD5(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// "hello world" MD5: 5eb63bbbe01eeed093cb22bb8f5acdc3
		expected := "5eb63bbbe01eeed093cb22bb8f5acdc3"
		if hash != expected {
			t.Errorf("expected %s, got %s", expected, hash)
		}
	})

	t.Run("computes MD5 for empty input", func(t *testing.T) {
		input := strings.NewReader("")
		hash, err := checksum.ComputeMD5(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Empty string MD5: d41d8cd98f00b204e9800998ecf8427e
		expected := "d41d8cd98f00b204e9800998ecf8427e"
		if hash != expected {
			t.Errorf("expected %s, got %s", expected, hash)
		}
	})

	t.Run("computes MD5 for large input", func(t *testing.T) {
		// Create 10MB of data
		data := bytes.Repeat([]byte("a"), 10*1024*1024)
		input := bytes.NewReader(data)

		hash, err := checksum.ComputeMD5(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if hash == "" {
			t.Error("expected non-empty hash")
		}

		// Verify hash is 32 hex characters
		if len(hash) != 32 {
			t.Errorf("expected 32 character hash, got %d", len(hash))
		}
	})
}

func TestComputeSHA256(t *testing.T) {
	t.Run("computes correct SHA256 for known input", func(t *testing.T) {
		input := strings.NewReader("hello world")
		hash, err := checksum.ComputeSHA256(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// "hello world" SHA256: b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9
		expected := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
		if hash != expected {
			t.Errorf("expected %s, got %s", expected, hash)
		}
	})

	t.Run("computes SHA256 for empty input", func(t *testing.T) {
		input := strings.NewReader("")
		hash, err := checksum.ComputeSHA256(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Empty string SHA256: e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
		expected := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
		if hash != expected {
			t.Errorf("expected %s, got %s", expected, hash)
		}
	})

	t.Run("computes SHA256 for large input", func(t *testing.T) {
		// Create 10MB of data
		data := bytes.Repeat([]byte("a"), 10*1024*1024)
		input := bytes.NewReader(data)

		hash, err := checksum.ComputeSHA256(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if hash == "" {
			t.Error("expected non-empty hash")
		}

		// Verify hash is 64 hex characters
		if len(hash) != 64 {
			t.Errorf("expected 64 character hash, got %d", len(hash))
		}
	})
}

func TestComputeBLAKE3(t *testing.T) {
	t.Run("computes BLAKE3 for known input", func(t *testing.T) {
		input := strings.NewReader("hello world")
		hash, err := checksum.ComputeBLAKE3(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// "hello world" BLAKE3: d74981efa70a0c880b8d8c1985d075dbcbf679b99a5f9914e5aaf96b831a9e24
		expected := "d74981efa70a0c880b8d8c1985d075dbcbf679b99a5f9914e5aaf96b831a9e24"
		if hash != expected {
			t.Errorf("expected %s, got %s", expected, hash)
		}
	})

	t.Run("computes BLAKE3 for empty input", func(t *testing.T) {
		input := strings.NewReader("")
		hash, err := checksum.ComputeBLAKE3(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Empty string BLAKE3: af1349b9f5f9a1a6a0404dea36dcc9499bcb25c9adc112b7cc9a93cae41f3262
		expected := "af1349b9f5f9a1a6a0404dea36dcc9499bcb25c9adc112b7cc9a93cae41f3262"
		if hash != expected {
			t.Errorf("expected %s, got %s", expected, hash)
		}
	})

	t.Run("computes BLAKE3 for large input", func(t *testing.T) {
		// Create 10MB of data
		data := bytes.Repeat([]byte("a"), 10*1024*1024)
		input := bytes.NewReader(data)

		hash, err := checksum.ComputeBLAKE3(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if hash == "" {
			t.Error("expected non-empty hash")
		}

		// Verify hash is 64 hex characters
		if len(hash) != 64 {
			t.Errorf("expected 64 character hash, got %d", len(hash))
		}
	})
}

func TestCompute(t *testing.T) {
	input := "test data"

	t.Run("computes MD5 via generic function", func(t *testing.T) {
		reader := strings.NewReader(input)
		hash, err := checksum.Compute(checksum.AlgorithmMD5, reader)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if hash == "" {
			t.Error("expected non-empty hash")
		}

		if len(hash) != 32 {
			t.Errorf("expected 32 character MD5, got %d", len(hash))
		}
	})

	t.Run("computes SHA256 via generic function", func(t *testing.T) {
		reader := strings.NewReader(input)
		hash, err := checksum.Compute(checksum.AlgorithmSHA256, reader)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if hash == "" {
			t.Error("expected non-empty hash")
		}

		if len(hash) != 64 {
			t.Errorf("expected 64 character SHA256, got %d", len(hash))
		}
	})

	t.Run("computes BLAKE3 via generic function", func(t *testing.T) {
		reader := strings.NewReader(input)
		hash, err := checksum.Compute(checksum.AlgorithmBLAKE3, reader)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if hash == "" {
			t.Error("expected non-empty hash")
		}

		if len(hash) != 64 {
			t.Errorf("expected 64 character BLAKE3, got %d", len(hash))
		}
	})

	t.Run("returns error for unsupported algorithm", func(t *testing.T) {
		reader := strings.NewReader(input)
		_, err := checksum.Compute("invalid", reader)
		if err == nil {
			t.Error("expected error for invalid algorithm")
		}

		if !strings.Contains(err.Error(), "unsupported") {
			t.Errorf("expected 'unsupported' error, got: %v", err)
		}
	})
}

func TestValidateAlgorithm(t *testing.T) {
	t.Run("validates MD5", func(t *testing.T) {
		err := checksum.ValidateAlgorithm(checksum.AlgorithmMD5)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("validates SHA256", func(t *testing.T) {
		err := checksum.ValidateAlgorithm(checksum.AlgorithmSHA256)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("validates BLAKE3", func(t *testing.T) {
		err := checksum.ValidateAlgorithm(checksum.AlgorithmBLAKE3)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("rejects invalid algorithm", func(t *testing.T) {
		err := checksum.ValidateAlgorithm("invalid")
		if err == nil {
			t.Error("expected error for invalid algorithm")
		}
	})
}

func TestWorkerPool(t *testing.T) {
	t.Run("processes single job", func(t *testing.T) {
		pool := checksum.NewWorkerPool(1)
		pool.Start()
		defer pool.Stop()

		job := &checksum.Job{
			Path:      "test.txt",
			Algorithm: checksum.AlgorithmMD5,
			Opener: func() (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader("hello world")), nil
			},
		}

		err := pool.Submit(job)
		if err != nil {
			t.Fatalf("failed to submit job: %v", err)
		}

		result := <-pool.Results()
		if result.Error != nil {
			t.Fatalf("unexpected error: %v", result.Error)
		}

		if result.Path != "test.txt" {
			t.Errorf("expected path test.txt, got %s", result.Path)
		}

		expected := "5eb63bbbe01eeed093cb22bb8f5acdc3"
		if result.Checksum != expected {
			t.Errorf("expected checksum %s, got %s", expected, result.Checksum)
		}

		if result.Duration == 0 {
			t.Error("expected non-zero duration")
		}
	})

	t.Run("processes multiple jobs in parallel", func(t *testing.T) {
		pool := checksum.NewWorkerPool(4)
		pool.Start()
		defer pool.Stop()

		jobCount := 10
		for i := 0; i < jobCount; i++ {
			job := &checksum.Job{
				Path:      filepath.Join("file", string(rune(i))+"txt"),
				Algorithm: checksum.AlgorithmBLAKE3,
				Opener: func() (io.ReadCloser, error) {
					return io.NopCloser(strings.NewReader("test data")), nil
				},
			}

			err := pool.Submit(job)
			if err != nil {
				t.Fatalf("failed to submit job: %v", err)
			}
		}

		successCount := 0
		for i := 0; i < jobCount; i++ {
			result := <-pool.Results()
			if result.Error == nil {
				successCount++
			}
		}

		if successCount != jobCount {
			t.Errorf("expected %d successful results, got %d", jobCount, successCount)
		}
	})

	t.Run("handles file open errors", func(t *testing.T) {
		pool := checksum.NewWorkerPool(1)
		pool.Start()
		defer pool.Stop()

		job := &checksum.Job{
			Path:      "nonexistent.txt",
			Algorithm: checksum.AlgorithmMD5,
			Opener: func() (io.ReadCloser, error) {
				return nil, os.ErrNotExist
			},
		}

		pool.Submit(job)

		result := <-pool.Results()
		if result.Error == nil {
			t.Error("expected error for failed file open")
		}

		if !strings.Contains(result.Error.Error(), "failed to open") {
			t.Errorf("expected 'failed to open' error, got: %v", result.Error)
		}
	})

	t.Run("respects timeout", func(t *testing.T) {
		pool := checksum.NewWorkerPool(1)
		pool.Start()
		defer pool.Stop()

		// Create a slow reader that delays reads
		slowReader := &slowReader{
			delay: 100 * time.Millisecond,
			data:  bytes.Repeat([]byte("a"), 1024*1024), // 1MB
		}

		job := &checksum.Job{
			Path:      "slow.txt",
			Algorithm: checksum.AlgorithmMD5,
			Opener: func() (io.ReadCloser, error) {
				return io.NopCloser(slowReader), nil
			},
			Timeout: 10 * time.Millisecond, // Short timeout
		}

		pool.Submit(job)

		result := <-pool.Results()
		if result.Error == nil {
			t.Error("expected timeout error")
		}

		if !strings.Contains(result.Error.Error(), "timeout") && !strings.Contains(result.Error.Error(), "cancelled") {
			t.Errorf("expected timeout/cancelled error, got: %v", result.Error)
		}
	})

	t.Run("computes real file checksum", func(t *testing.T) {
		// Create temporary file
		tmpDir := t.TempDir()
		tmpFile := filepath.Join(tmpDir, "test.txt")
		content := []byte("hello from file")
		err := os.WriteFile(tmpFile, content, 0644)
		if err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		pool := checksum.NewWorkerPool(1)
		pool.Start()
		defer pool.Stop()

		job := &checksum.Job{
			Path:      tmpFile,
			Algorithm: checksum.AlgorithmSHA256,
			Opener: func() (io.ReadCloser, error) {
				return os.Open(tmpFile)
			},
		}

		pool.Submit(job)

		result := <-pool.Results()
		if result.Error != nil {
			t.Fatalf("unexpected error: %v", result.Error)
		}

		// Verify checksum is consistent
		file, _ := os.Open(tmpFile)
		defer file.Close()
		expected, _ := checksum.ComputeSHA256(file)

		if result.Checksum != expected {
			t.Errorf("expected checksum %s, got %s", expected, result.Checksum)
		}
	})
}

// slowReader is a test helper that delays reads
type slowReader struct {
	delay time.Duration
	data  []byte
	pos   int
}

func (r *slowReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}

	time.Sleep(r.delay)

	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
