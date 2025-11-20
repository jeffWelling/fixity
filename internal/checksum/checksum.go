package checksum

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/zeebo/blake3"
)

// Algorithm represents a checksum algorithm
type Algorithm string

const (
	AlgorithmMD5     Algorithm = "md5"
	AlgorithmSHA256  Algorithm = "sha256"
	AlgorithmBLAKE3  Algorithm = "blake3"
)

// BufferSize is the chunk size for streaming reads (1MB)
const BufferSize = 1024 * 1024

// Compute computes a checksum for the given reader using the specified algorithm
func Compute(algorithm Algorithm, reader io.Reader) (string, error) {
	switch algorithm {
	case AlgorithmMD5:
		return ComputeMD5(reader)
	case AlgorithmSHA256:
		return ComputeSHA256(reader)
	case AlgorithmBLAKE3:
		return ComputeBLAKE3(reader)
	default:
		return "", fmt.Errorf("unsupported algorithm: %s", algorithm)
	}
}

// ComputeMD5 computes MD5 checksum
func ComputeMD5(reader io.Reader) (string, error) {
	hasher := md5.New()
	if _, err := io.Copy(hasher, reader); err != nil {
		return "", fmt.Errorf("failed to compute MD5: %w", err)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// ComputeSHA256 computes SHA-256 checksum
func ComputeSHA256(reader io.Reader) (string, error) {
	hasher := sha256.New()
	if _, err := io.Copy(hasher, reader); err != nil {
		return "", fmt.Errorf("failed to compute SHA256: %w", err)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// ComputeBLAKE3 computes BLAKE3 checksum
func ComputeBLAKE3(reader io.Reader) (string, error) {
	hasher := blake3.New()
	if _, err := io.Copy(hasher, reader); err != nil {
		return "", fmt.Errorf("failed to compute BLAKE3: %w", err)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// ValidateAlgorithm checks if the algorithm is supported
func ValidateAlgorithm(algorithm Algorithm) error {
	switch algorithm {
	case AlgorithmMD5, AlgorithmSHA256, AlgorithmBLAKE3:
		return nil
	default:
		return fmt.Errorf("unsupported algorithm: %s", algorithm)
	}
}
