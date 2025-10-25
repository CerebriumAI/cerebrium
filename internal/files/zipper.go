package files

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/cerebriumai/cerebrium/pkg/projectconfig"
)

// CreateZip creates a zip file containing all files in fileList with zip bomb protection
func CreateZip(fileList []string, outputPath string, config *projectconfig.ProjectConfig) (int64, error) {
	// Create zip file
	zipFile, err := os.Create(outputPath) //nolint:gosec // Output path is temp file created by CLI
	if err != nil {
		return 0, fmt.Errorf("failed to create zip file: %w", err)
	}
	//nolint:errcheck // Deferred close, error not actionable
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer func() {
		//nolint:staticcheck // Empty branch intentional - see comment
		if err := zipWriter.Close(); err != nil {
			// Unfortunately zip.Writer does not allow us to check closed state before closing.
			// This means the defer might close an already-normally-closed writer, which will
			// emit an error that is not addressable.
			// To avoid spammy logs, just ignore it, it's not the hugest deal because the program
			// quits soon after.
		}
	}()
	// Track uncompressed size for zip bomb protection
	var totalUncompressedSize int64

	// Generate and add dependency files first (requirements.txt, etc.)
	if config != nil {
		depFiles, err := GenerateDependencyFiles(config)
		if err != nil {
			return 0, fmt.Errorf("failed to generate dependency files: %w", err)
		}

		if err := AddDependencyFiles(zipWriter, depFiles); err != nil {
			return 0, fmt.Errorf("failed to add dependency files to zip: %w", err)
		}

		// Track size of dependency files for zip bomb protection
		for _, content := range depFiles {
			totalUncompressedSize += int64(len(content))
		}
	}

	// Add each file to zip
	for _, file := range fileList {
		uncompressedSize, err := addFileToZip(zipWriter, file)
		if err != nil {
			return 0, fmt.Errorf("failed to add %s to zip: %w", file, err)
		}
		totalUncompressedSize += uncompressedSize

		// Zip bomb protection: check compression ratio
		if err := validateCompressionRatio(totalUncompressedSize); err != nil {
			return 0, err
		}
	}

	// Close zip writer to finalize (must be done before getting file stats)
	if err := zipWriter.Close(); err != nil {
		return 0, fmt.Errorf("failed to finalize zip: %w", err)
	}

	// Get zip file size
	zipInfo, err := zipFile.Stat()
	if err != nil {
		return 0, fmt.Errorf("failed to stat zip file: %w", err)
	}

	return zipInfo.Size(), nil
}

// addFileToZip adds a single file to the zip archive with proper UTC timestamp
// Returns the uncompressed size of the file
func addFileToZip(zipWriter *zip.Writer, filePath string) (int64, error) {
	// Open file
	file, err := os.Open(filePath) //nolint:gosec // File path from user's project directory
	if err != nil {
		return 0, err
	}
	//nolint:errcheck // Deferred close, error not actionable
	defer file.Close()

	// Get file info
	info, err := file.Stat()
	if err != nil {
		return 0, err
	}

	// Create zip header with UTC time
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return 0, err
	}

	// Set the name to the relative path
	header.Name = filepath.ToSlash(filePath)

	// Set modified time to UTC
	header.Modified = info.ModTime().UTC()

	// Use deflate compression
	header.Method = zip.Deflate

	// Create writer for this file
	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return 0, err
	}

	// Copy file contents
	_, err = io.Copy(writer, file)
	if err != nil {
		return 0, err
	}

	// Return uncompressed size
	return info.Size(), nil
}

// AddDependencyFiles adds generated dependency files to the zip
func AddDependencyFiles(zipWriter *zip.Writer, files map[string]string) error {
	for filename, content := range files {
		// Create zip header
		header := &zip.FileHeader{
			Name:     filename,
			Method:   zip.Deflate,
			Modified: time.Now().UTC(),
		}

		// Create writer
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("failed to create header for %s: %w", filename, err)
		}

		// Write content
		if _, err := writer.Write([]byte(content)); err != nil {
			return fmt.Errorf("failed to write %s: %w", filename, err)
		}
	}

	return nil
}

// validateCompressionRatio checks for potential zip bomb attacks
func validateCompressionRatio(uncompressedSize int64) error {
	// TODO(wes): More sophisticated zip bomb detection

	// For now, check if uncompressed size is suspiciously large compared to file count
	// A more sophisticated implementation would track actual compressed bytes
	if uncompressedSize > 100*1024*1024*1024 { // 100GB uncompressed
		return fmt.Errorf("zip bomb protection: uncompressed size exceeds 100GB (%d bytes)", uncompressedSize)
	}

	return nil
}

// ValidateZipSize checks if zip size is within acceptable limits
func ValidateZipSize(size int64) (warning string, err error) {
	const (
		warningLimit = 1 * 1024 * 1024 * 1024 // 1GB
		errorLimit   = 2 * 1024 * 1024 * 1024 // 2GB
	)

	if size > errorLimit {
		return "", fmt.Errorf("project zip file is over 2GB (%d bytes). Please use the cerebrium cp command to upload files instead", size)
	}

	if size > warningLimit {
		return fmt.Sprintf("Warning: Project zip file is over 1GB (%d bytes). Your deployment should work but might encounter issues. Consider using the cerebrium cp command if you encounter issues.", size), nil
	}

	return "", nil
}
