package files

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"

	"github.com/avast/retry-go/v4"
	"github.com/cerebriumai/cerebrium/internal/api"
	"golang.org/x/sync/errgroup"
)

// uploadState manages the state for concurrent uploads
type uploadState struct {
	partResults []api.PartInfo
	mu          sync.Mutex
}

// setPartResult safely sets a part result
func (u *uploadState) setPartResult(idx int, info api.PartInfo) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.partResults[idx] = info
}

// uploadSingleFile uploads a file using multipart upload with retry logic
func (m *FileUploadView) uploadSingleFile(file fileToUpload, atomicCounter *atomic.Int64) (int64, error) {
	// Step 1: Initialize the upload
	partCount := m.calculatePartCount(file.size)
	initResp, err := m.initializeUpload(file, partCount)
	if err != nil {
		return 0, err
	}

	// Step 2: Open the file once for all parts
	sharedFile, err := os.Open(file.localPath)
	if err != nil {
		return 0, fmt.Errorf("failed to open file: %w", err)
	}
	//nolint:errcheck // Deferred close, error not actionable
	defer sharedFile.Close()

	// Step 3: Upload all parts concurrently
	uploadState := &uploadState{
		partResults: make([]api.PartInfo, len(initResp.Parts)),
	}

	if err := m.uploadAllParts(initResp.Parts, sharedFile, uploadState, atomicCounter); err != nil {
		return 0, fmt.Errorf("upload failed: %w", err)
	}

	// Step 4: Complete the multipart upload
	if err := m.completeUpload(file, initResp.UploadID, uploadState.partResults); err != nil {
		return 0, err
	}

	return file.size, nil
}

// calculatePartCount determines how many parts are needed for the file
func (m *FileUploadView) calculatePartCount(fileSize int64) int {
	partCount := int((fileSize + partSizeBytes - 1) / partSizeBytes)
	if partCount == 0 {
		partCount = 1
	}
	return partCount
}

// initializeUpload starts a multipart upload session
func (m *FileUploadView) initializeUpload(file fileToUpload, partCount int) (*api.InitiateUploadResponse, error) {
	return m.conf.Client.InitiateUpload(m.ctx, m.conf.Config.ProjectID, file.remotePath, m.conf.Region, partCount)
}

// completeUpload finalizes the multipart upload
func (m *FileUploadView) completeUpload(file fileToUpload, uploadID string, partResults []api.PartInfo) error {
	return m.conf.Client.CompleteUpload(m.ctx, m.conf.Config.ProjectID, file.remotePath, uploadID, m.conf.Region, partResults)
}

// uploadAllParts uploads all parts concurrently with proper error handling
func (m *FileUploadView) uploadAllParts(parts []api.PartURL, sharedFile *os.File, state *uploadState, atomicCounter *atomic.Int64) error {
	eg, ctx := errgroup.WithContext(m.ctx)
	eg.SetLimit(maxConcurrentUploads)

	var fileMu sync.Mutex

	for i, part := range parts {
		idx := i
		p := part

		eg.Go(func() error {
			// Check if context was cancelled (early exit)
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// Upload this part with retry logic
			if err := m.uploadPartWithRetryCtx(ctx, p, idx, sharedFile, &fileMu, state, atomicCounter); err != nil {
				return fmt.Errorf("part %d: %w", p.PartNumber, err)
			}
			return nil
		})
	}

	return eg.Wait()
}

// uploadPartWithRetryCtx uploads a single part with retry logic and context support
func (m *FileUploadView) uploadPartWithRetryCtx(
	ctx context.Context,
	part api.PartURL,
	idx int,
	sharedFile *os.File,
	fileMu *sync.Mutex,
	state *uploadState,
	atomicCounter *atomic.Int64,
) error {
	// Read the part data first (don't retry on read errors)
	partData, bytesRead, err := m.readPartData(part.PartNumber, sharedFile, fileMu)
	if err != nil {
		return err
	}

	// Upload with retry
	var etag string
	err = retry.Do(
		func() error {
			// Check context before each attempt
			select {
			case <-ctx.Done():
				return retry.Unrecoverable(ctx.Err())
			default:
			}

			etag, err = m.conf.Client.UploadPart(ctx, part.URL, partData)
			return err
		},
		retry.Context(ctx),
		retry.Attempts(uint(maxRetryAttempts)),
		retry.Delay(initialRetryDelay),
		retry.DelayType(retry.BackOffDelay),
		retry.RetryIf(func(err error) bool {
			// Don't retry on context cancellation
			return !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
		}),
	)

	if err != nil {
		return fmt.Errorf("failed after retries: %w", err)
	}

	// Success - update state and counter
	state.setPartResult(idx, api.PartInfo{
		PartNumber: part.PartNumber,
		ETag:       etag,
	})

	if atomicCounter != nil {
		atomicCounter.Add(int64(bytesRead))
	}

	return nil
}

// readPartData reads a specific part of the file
func (m *FileUploadView) readPartData(partNumber int, file *os.File, mu *sync.Mutex) ([]byte, int, error) {
	offset := int64(partNumber-1) * partSizeBytes
	partData := make([]byte, partSizeBytes)

	mu.Lock()
	defer mu.Unlock()

	if _, err := file.Seek(offset, 0); err != nil {
		return nil, 0, fmt.Errorf("failed to seek: %w", err)
	}

	n, err := file.Read(partData)
	if err != nil && err != io.EOF {
		return nil, 0, fmt.Errorf("failed to read: %w", err)
	}

	return partData[:n], n, nil
}
