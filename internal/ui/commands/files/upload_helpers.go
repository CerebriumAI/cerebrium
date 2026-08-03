package files

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"

	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/cerebriumai/cerebrium/internal/upload"
)

// volumeSession brackets a multipart upload into a project's persistent volume.
// The generic transport lives in internal/upload; only these two calls are
// specific to volume storage.
type volumeSession struct {
	client     api.Client
	projectID  string
	region     string
	remotePath string
}

func (s volumeSession) Initiate(ctx context.Context, partCount int) (*api.InitiateUploadResponse, error) {
	return s.client.InitiateUpload(ctx, s.projectID, s.remotePath, s.region, partCount)
}

func (s volumeSession) Complete(ctx context.Context, uploadID string, parts []api.PartInfo) error {
	return s.client.CompleteUpload(ctx, s.projectID, s.remotePath, uploadID, s.region, parts)
}

// uploadSingleFile uploads one file to persistent storage as a multipart upload.
func (m *FileUploadView) uploadSingleFile(file fileToUpload, atomicCounter *atomic.Int64) (int64, error) {
	f, err := os.Open(file.localPath) //nolint:gosec // Path comes from the user's own upload argument
	if err != nil {
		return 0, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close() //nolint:errcheck // Deferred close on a read-only file, error not actionable

	session := volumeSession{
		client:     m.conf.Client,
		projectID:  m.conf.Config.ProjectID,
		region:     m.conf.Region,
		remotePath: file.remotePath,
	}

	err = upload.Do(m.ctx, f, file.size, session, m.conf.Client, upload.Options{
		PartSize:    partSizeBytes,
		Concurrency: maxConcurrentUploads,
		MaxAttempts: maxRetryAttempts,
		RetryDelay:  initialRetryDelay,
		Progress:    atomicCounter,
		Op:          fmt.Sprintf("upload %s", file.remotePath),
	})
	if err != nil {
		return 0, err
	}

	return file.size, nil
}
