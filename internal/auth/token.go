package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/cerebriumai/cerebrium/pkg/config"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// GetOrRefreshToken returns a valid access token, refreshing if necessary
func GetOrRefreshToken(ctx context.Context, cfg *config.Config) (string, error) {
	// Check if we have an access token
	if cfg.AccessToken == "" {
		return "", fmt.Errorf("no access token found. Please run 'cerebrium login'")
	}

	// Parse the token without verification to check expiration
	token, _, err := new(jwt.Parser).ParseUnverified(cfg.AccessToken, jwt.MapClaims{})
	if err != nil {
		return "", fmt.Errorf("failed to parse JWT token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("failed to parse JWT claims")
	}

	// Check expiration
	exp, ok := claims["exp"].(float64)
	if !ok {
		return "", fmt.Errorf("missing or invalid exp claim in JWT")
	}

	expirationTime := time.Unix(int64(exp), 0)
	if time.Now().Before(expirationTime) {
		// Token is still valid
		return cfg.AccessToken, nil
	}

	// Token has expired, refresh it
	if cfg.RefreshToken == "" {
		return "", fmt.Errorf("access token has expired and no refresh token available. Please run 'cerebrium login'")
	}

	newToken, err := refreshToken(ctx, cfg)
	if err != nil {
		return "", fmt.Errorf("failed to refresh token: %w", err)
	}

	// Save the new token
	cfg.AccessToken = newToken
	if err := config.Save(cfg); err != nil {
		return "", fmt.Errorf("failed to save new token: %w", err)
	}

	return newToken, nil
}

// refreshToken exchanges a refresh token for a new access token
func refreshToken(ctx context.Context, cfg *config.Config) (string, error) {
	envCfg := cfg.GetEnvConfig()

	// Create form-encoded data
	formData := url.Values{}
	formData.Set("grant_type", "refresh_token")
	formData.Set("client_id", envCfg.ClientID)
	formData.Set("refresh_token", cfg.RefreshToken)

	req, err := http.NewRequestWithContext(ctx, "POST", envCfg.AuthUrl, bytes.NewBufferString(formData.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // Deferred close, error not actionable

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token refresh failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	accessToken, ok := result["access_token"].(string)
	if !ok {
		return "", fmt.Errorf("missing access_token in response")
	}

	return accessToken, nil
}
