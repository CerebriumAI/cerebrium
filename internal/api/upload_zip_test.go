package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTestZip(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "build.zip")
	require.NoError(t, os.WriteFile(path, []byte("zip-contents"), 0o600))
	return path
}

func TestUploadZipDoesNotRetryClientErrors(t *testing.T) {
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("<Error><Code>AccessDenied</Code></Error>"))
	}))
	defer server.Close()

	c := newTestClient(t, server.URL)

	err := c.UploadZip(context.Background(), server.URL, writeTestZip(t))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "403")
	assert.Equal(t, int32(1), attempts.Load())
}

func TestUploadZipRetriesServerErrors(t *testing.T) {
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := newTestClient(t, server.URL)

	err := c.UploadZip(context.Background(), server.URL, writeTestZip(t))

	require.NoError(t, err)
	assert.Equal(t, int32(2), attempts.Load())
}
