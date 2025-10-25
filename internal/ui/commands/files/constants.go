package files

import "time"

// Upload constants
const (
	// maxConcurrentUploads is the maximum number of parts that can be uploaded simultaneously
	maxConcurrentUploads = 10

	// maxRetryAttempts is the maximum number of times to retry a failed upload
	maxRetryAttempts = 3

	// partSizeMB is the size of each multipart upload part in megabytes
	// Using 5MB for quicker initial feedback on progress
	partSizeMB = 5

	// partSizeBytes is the size of each multipart upload part in bytes
	partSizeBytes = partSizeMB * 1024 * 1024

	// initialRetryDelay is the initial delay for exponential backoff
	initialRetryDelay = 2 * time.Second
)

// Download constants
const (
	// downloadBufferSize is the buffer size for downloading files
	downloadBufferSize = 32 * 1024 // 32KB

	// progressUpdateInterval is how often to update the progress display
	progressUpdateInterval = 100 * time.Millisecond
)

// UI constants
const (
	// fastProgressUpdate is for upload progress (more responsive)
	fastProgressUpdate = 10 * time.Millisecond
)

// File size units for formatting
const (
	byteUnit = 1024
)
