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

func testServiceAccountJWT(t *testing.T, exp time.Time, projectID string) string {
	t.Helper()
	claims, err := json.Marshal(map[string]any{"exp": exp.Unix(), "projectId": projectID})
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

		require.ErrorIs(t, err, ErrNotLoggedIn)
	})

	t.Run("names the expiry date and the project's key page for a stored token", func(t *testing.T) {
		cfg := loadTestConfig(t)
		expiry := time.Now().Add(-time.Hour)
		cfg.ServiceAccountToken = testServiceAccountJWT(t, expiry, "p-367e7969")

		_, err := Token(t.Context(), cfg)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "service account token expired on "+expiry.Local().Format("2006-01-02 15:04"))
		assert.Contains(t, err.Error(), "/projects/p-367e7969/api-keys")
		assert.Contains(t, err.Error(), "cerebrium save-auth-config")
	})

	t.Run("distinguishes a stored token it cannot read from an expired one", func(t *testing.T) {
		cfg := loadTestConfig(t)
		cfg.ServiceAccountToken = "not-even-a-jwt"

		_, err := Token(t.Context(), cfg)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "could not be read")
		assert.NotContains(t, err.Error(), "expired")
	})

	t.Run("points an environment token at the environment, not the config file", func(t *testing.T) {
		cfg := loadTestConfig(t)
		t.Setenv("CEREBRIUM_SERVICE_ACCOUNT_TOKEN", testServiceAccountJWT(t, time.Now().Add(-time.Hour), "p-367e7969"))

		_, err := Token(t.Context(), cfg)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "CEREBRIUM_SERVICE_ACCOUNT_TOKEN expired on ")
		assert.Contains(t, err.Error(), "update the environment variable")
		assert.NotContains(t, err.Error(), "save-auth-config")
	})

	t.Run("falls back to the dashboard root when no project can be read", func(t *testing.T) {
		cfg := loadTestConfig(t)
		cfg.ServiceAccountToken = "not-even-a-jwt"

		_, err := Token(t.Context(), cfg)

		require.Error(t, err)
		assert.NotContains(t, err.Error(), "/projects/")
	})

	t.Run("reports an expired token with nothing to refresh with", func(t *testing.T) {
		cfg := loadTestConfig(t)
		cfg.AccessToken = testJWT(t, time.Now().Add(-time.Hour))

		_, err := Token(t.Context(), cfg)

		require.ErrorIs(t, err, ErrSessionExpired)
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

	t.Run("clears credentials the auth server has rejected", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
		}))
		defer server.Close()

		t.Setenv("AUTH_URL", server.URL)
		cfg := loadTestConfig(t)
		cfg.AccessToken = testJWT(t, time.Now().Add(-time.Hour))
		cfg.RefreshToken = "stored-refresh"

		_, err := Token(t.Context(), cfg)

		require.ErrorIs(t, err, ErrSessionExpired)
		assert.Empty(t, cfg.AccessToken)
		assert.Empty(t, cfg.RefreshToken)

		reloaded, err := config.Load()
		require.NoError(t, err)
		assert.Empty(t, reloaded.AccessToken)
		assert.Empty(t, reloaded.RefreshToken)
	})

	t.Run("keeps credentials when the refresh call itself fails", func(t *testing.T) {
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
		assert.NotErrorIs(t, err, ErrSessionExpired)
		assert.Equal(t, "stored-refresh", cfg.RefreshToken)
	})

	t.Run("reports an expired service account token", func(t *testing.T) {
		cfg := loadTestConfig(t)
		cfg.ServiceAccountToken = testJWT(t, time.Now().Add(-time.Hour))

		_, err := Token(t.Context(), cfg)

		require.Error(t, err)
		assert.NotErrorIs(t, err, ErrSessionExpired)
		assert.NotErrorIs(t, err, ErrNotLoggedIn)
	})
}
