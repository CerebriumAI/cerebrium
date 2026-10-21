package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cerebriumai/cerebrium/internal/clientenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOAuthRequestsEmitAttributionHeaders(t *testing.T) {
	tcs := []struct {
		name string
		call func(ctx context.Context, apiURL string) error
	}{
		{
			name: "device authorization request",
			call: func(ctx context.Context, apiURL string) error {
				_, err := RequestDeviceCode(ctx, apiURL)
				return err
			},
		},
		{
			name: "token polling request",
			call: func(ctx context.Context, apiURL string) error {
				_, err := PollForToken(ctx, apiURL, "device-code-123")
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

			require.NoError(t, tc.call(context.Background(), server.URL))
			require.NotNil(t, captured, "server never received the request")

			assert.Equal(t, "cli", captured.Get("X-Source"))
			assert.Equal(t, clientenv.HeaderValue(), captured.Get(clientenv.HeaderName))
			assert.NotEmpty(t, captured.Get(clientenv.HeaderName))
		})
	}
}
