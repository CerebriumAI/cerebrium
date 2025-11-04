package files

import (
	"crypto/md5" //nolint:gosec // MD5 is required for S3 ETag compatibility, not used for security
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// HashFile computes the MD5 hash of a file and returns it as a hex-encoded string
// This format matches S3 ETag format for single-part uploads
func HashFile(filepath string) (string, error) {
	file, err := os.Open(filepath) //nolint:gosec // Path is provided by trusted source
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close() //nolint:errcheck // Deferred close error is non-critical for read operation

	hash := md5.New() //nolint:gosec // MD5 required for S3 ETag compatibility
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to compute hash: %w", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// HashBytes computes the MD5 hash of byte data and returns it as a hex-encoded string
func HashBytes(data []byte) string {
	hash := md5.New() //nolint:gosec // MD5 required for S3 ETag compatibility
	hash.Write(data)
	return hex.EncodeToString(hash.Sum(nil))
}

// HashString computes the MD5 hash of a string and returns it as a hex-encoded string
func HashString(s string) string {
	return HashBytes([]byte(s))
}

// VerifyFileHash checks if a file matches the expected MD5 hash (hex-encoded)
// Returns an error if the hashes don't match or if the file cannot be read
func VerifyFileHash(filepath string, expectedHash string) error {
	actualHash, err := HashFile(filepath)
	if err != nil {
		return err
	}

	if actualHash != expectedHash {
		return fmt.Errorf("hash mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	return nil
}
