package authsession

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/cerebriumai/cerebrium/pkg/config"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testJWT(t *testing.T, exp time.Time) string {
	t.Helper()

	claims, err := json.Marshal(map[string]any{"exp": exp.Unix()})
	require.NoError(t, err)

	return base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`)) + "." +
		base64.RawURLEncoding.EncodeToString(claims) + "." +
		base64.RawURLEncoding.EncodeToString([]byte("signature"))
}

// loadTestConfig returns a config backed by a throwaway file so Save has somewhere to write
func loadTestConfig(t *testing.T) *config.Config {
	t.Helper()

	viper.Reset()
	t.Cleanup(viper.Reset)
	t.Setenv("CEREBRIUM_CONFIG_PATH", filepath.Join(t.TempDir(), "config.yaml"))

	cfg, err := config.Load()
	require.NoError(t, err)

	return cfg
}

func TestToken(t *testing.T) {
	t.Run("returns a valid access token unchanged", func(t *testing.T) {
		cfg := loadTestConfig(t)
		cfg.AccessToken = testJWT(t, time.Now().Add(time.Hour))

		token, err := Token(t.Context(), cfg)

		require.NoError(t, err)
		assert.Equal(t, cfg.AccessToken, token)
	})

	t.Run("reports no stored credentials", func(t *testing.T) {
		cfg := loadTestConfig(t)

		_, err := Token(t.Context(), cfg)

		require.ErrorContains(t, err, "no access token found")
	})

	t.Run("reports an expired token with nothing to refresh with", func(t *testing.T) {
		cfg := loadTestConfig(t)
		cfg.AccessToken = testJWT(t, time.Now().Add(-time.Hour))

		_, err := Token(t.Context(), cfg)

		require.ErrorContains(t, err, "no refresh token available")
	})

	t.Run("refreshes and saves an expired token", func(t *testing.T) {
		fresh := testJWT(t, time.Now().Add(time.Hour))
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"access_token":"` + fresh + `"}`))
		}))
		defer server.Close()

		t.Setenv("AUTH_URL", server.URL)
		cfg := loadTestConfig(t)
		cfg.AccessToken = testJWT(t, time.Now().Add(-time.Hour))
		cfg.RefreshToken = "stored-refresh"

		token, err := Token(t.Context(), cfg)

		require.NoError(t, err)
		assert.Equal(t, fresh, token)
		assert.Equal(t, fresh, cfg.AccessToken)

		reloaded, err := config.Load()
		require.NoError(t, err)
		assert.Equal(t, fresh, reloaded.AccessToken)
	})

	t.Run("keeps credentials when the refresh call fails", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		t.Setenv("AUTH_URL", server.URL)
		cfg := loadTestConfig(t)
		cfg.AccessToken = testJWT(t, time.Now().Add(-time.Hour))
		cfg.RefreshToken = "stored-refresh"

		_, err := Token(t.Context(), cfg)

		require.Error(t, err)
		assert.Equal(t, "stored-refresh", cfg.RefreshToken)
	})

	t.Run("reports an expired service account token", func(t *testing.T) {
		cfg := loadTestConfig(t)
		cfg.ServiceAccountToken = testJWT(t, time.Now().Add(-time.Hour))

		_, err := Token(t.Context(), cfg)

		require.ErrorContains(t, err, "service account token has expired")
	})
}
