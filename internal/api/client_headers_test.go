package api

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cerebriumai/cerebrium/internal/clientenv"
	"github.com/cerebriumai/cerebrium/internal/version"
	"github.com/cerebriumai/cerebrium/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func serviceAccountToken() string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"p-test-project"}`))
	return header + "." + payload + ".test-signature"
}

func newTestClient(t *testing.T, serverURL string) *client {
	t.Helper()

	t.Setenv("CEREBRIUM_ENV", "local")
	t.Setenv("REST_API_URL", serverURL)
	t.Setenv("CEREBRIUM_CONFIG_PATH", filepath.Join(t.TempDir(), "config.yaml"))
	t.Setenv("CEREBRIUM_SERVICE_ACCOUNT_TOKEN", serviceAccountToken())

	cfg, err := config.Load()
	require.NoError(t, err)

	return &client{
		config:     cfg,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

func TestRequestPathsEmitAttributionHeaders(t *testing.T) {
	tcs := []struct {
		name string
		call func(t *testing.T, c *client) error
	}{
		{
			name: "json request without auth",
			call: func(t *testing.T, c *client) error {
				t.Helper()
				_, err := c.request(context.Background(), http.MethodGet, "v2/projects", nil, false)
				return err
			},
		},
		{
			name: "json request with auth",
			call: func(t *testing.T, c *client) error {
				t.Helper()
				_, err := c.request(context.Background(), http.MethodGet, "v2/projects", nil, true)
				return err
			},
		},
		{
			name: "multipart run request",
			call: func(t *testing.T, c *client) error {
				t.Helper()
				tarPath := filepath.Join(t.TempDir(), "payload.tar")
				require.NoError(t, os.WriteFile(tarPath, []byte("tar-bytes"), 0o600))
				_, err := c.RunApp(
					context.Background(),
					"p-test-project",
					"p-test-project-app",
					"us-east-1",
					"main.py",
					nil,
					nil,
					map[string]any{},
					tarPath,
					map[string]any{},
				)
				return err
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			var captured http.Header
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				captured = r.Header.Clone()
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{}`))
			}))
			defer server.Close()

			c := newTestClient(t, server.URL)

			require.NoError(t, tc.call(t, c))
			require.NotNil(t, captured, "server never received the request")

			assert.Equal(t, "cli", captured.Get("X-Source"))
			assert.Equal(t, version.Version, captured.Get("X-CLI-Version"))
			assert.Equal(t, clientenv.HeaderValue(), captured.Get(clientenv.HeaderName))
			assert.NotEmpty(t, captured.Get(clientenv.HeaderName))
		})
	}
}
