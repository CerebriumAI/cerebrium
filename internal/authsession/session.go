// Package authsession resolves the credentials CLI commands authenticate with.
package authsession

import (
	"context"
	"fmt"

	"github.com/cerebriumai/cerebrium/internal/auth"
	"github.com/cerebriumai/cerebrium/pkg/config"
)

// Token returns the token to authenticate with, refreshing and persisting the
// stored access token when it has expired.
func Token(ctx context.Context, cfg *config.Config) (string, error) {
	// 1. Try service account token from environment variable first
	serviceToken, err := config.GetServiceAccountTokenFromEnv()
	if err != nil {
		return "", fmt.Errorf("service account token error: %w", err)
	}
	if serviceToken != "" {
		return serviceToken, nil
	}

	// 2. Try stored service account token
	if token := cfg.GetServiceAccountToken(); token != "" {
		if err := auth.ValidateToken(token); err == nil {
			return token, nil
		}
		return "", fmt.Errorf("service account token has expired. Please generate a new one")
	}

	// 3. Try access token
	token := cfg.GetAccessToken()
	if token == "" {
		return "", fmt.Errorf("no access token found. Please run 'cerebrium login' or provide a service account token")
	}

	// Check if access token is still valid
	if err := auth.ValidateToken(token); err == nil {
		return token, nil
	}

	// 4. Access token expired, try to refresh
	refreshToken := cfg.GetRefreshToken()
	if refreshToken == "" {
		return "", fmt.Errorf("access token has expired and no refresh token available. Please run 'cerebrium login'")
	}

	envConfig := cfg.GetEnvConfig()
	newToken, err := auth.RefreshToken(ctx, envConfig.AuthUrl, envConfig.ClientID, refreshToken)
	if err != nil {
		return "", fmt.Errorf("failed to refresh token: %w", err)
	}

	// Save the new token
	cfg.SetAccessToken(newToken)
	if err := config.Save(cfg); err != nil {
		return "", fmt.Errorf("failed to save new token: %w", err)
	}

	return newToken, nil
}
