package files

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// HashFile computes the MD5 hash of a file (matches S3 ETag format)
func HashFile(filepath string) (string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to compute hash: %w", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// HashBytes computes the MD5 hash of byte data
func HashBytes(data []byte) string {
	hash := md5.New()
	hash.Write(data)
	return hex.EncodeToString(hash.Sum(nil))
}

// HashString computes the MD5 hash of a string
func HashString(s string) string {
	return HashBytes([]byte(s))
}

// VerifyFileHash checks if a file matches the expected hash
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
