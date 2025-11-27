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
// Token precedence:
// 1. ServiceAccountToken (from save-auth-config with JWT or --service-account-token flag)
// 2. AccessToken + RefreshToken (from login)
func GetOrRefreshToken(ctx context.Context, cfg *config.Config) (string, error) {
	// Check service account token first (highest priority after CLI flag)
	if cfg.ServiceAccountToken != "" {
		valid, err := isTokenValid(cfg.ServiceAccountToken)
		if err != nil {
			return "", fmt.Errorf("failed to validate service account token: %w", err)
		}
		if valid {
			return cfg.ServiceAccountToken, nil
		}
		// Service account token has expired - cannot be refreshed
		return "", fmt.Errorf("service account token has expired. Please generate a new one or run 'cerebrium save-auth-config' with a fresh token")
	}

	// Fall back to regular access token
	if cfg.AccessToken == "" {
		return "", fmt.Errorf("no access token found. Please run 'cerebrium login' or provide a service account token")
	}

	valid, err := isTokenValid(cfg.AccessToken)
	if err != nil {
		return "", fmt.Errorf("failed to validate access token: %w", err)
	}
	if valid {
		return cfg.AccessToken, nil
	}

	// Token has expired, try to refresh it
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

// isTokenValid checks if a JWT token is not expired
func isTokenValid(tokenString string) (bool, error) {
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return false, fmt.Errorf("failed to parse JWT token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return false, fmt.Errorf("failed to parse JWT claims")
	}

	exp, ok := claims["exp"].(float64)
	if !ok {
		return false, fmt.Errorf("missing or invalid exp claim in JWT")
	}

	expirationTime := time.Unix(int64(exp), 0)
	return time.Now().Before(expirationTime), nil
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
