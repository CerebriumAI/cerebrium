package commands

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newUploadTestView(t *testing.T, uploadURL string) *DeployView {
	t.Helper()

	zipPath := filepath.Join(t.TempDir(), "deployment.zip")
	require.NoError(t, os.WriteFile(zipPath, []byte("zip-contents"), 0o600))

	return &DeployView{
		ctx:                 context.Background(),
		zipPath:             zipPath,
		appResponse:         &api.CreateAppResponse{UploadURL: uploadURL},
		atomicBytesUploaded: &atomic.Int64{},
	}
}

func TestUploadZipWithProgressDoesNotRetryClientErrors(t *testing.T) {
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("<Error><Code>AccessDenied</Code></Error>"))
	}))
	defer server.Close()

	err := newUploadTestView(t, server.URL).uploadZipWithProgress()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "403")
	assert.Equal(t, int32(1), attempts.Load())
}

func TestUploadZipWithProgressRetriesServerErrors(t *testing.T) {
	var attempts atomic.Int32
	var received atomic.Int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		n, _ := io.Copy(io.Discard, r.Body)
		received.Store(n)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	m := newUploadTestView(t, server.URL)
	err := m.uploadZipWithProgress()

	require.NoError(t, err)
	assert.Equal(t, int32(2), attempts.Load())
	assert.Equal(t, int64(len("zip-contents")), received.Load())
	assert.Equal(t, int64(len("zip-contents")), m.atomicBytesUploaded.Load())
}
