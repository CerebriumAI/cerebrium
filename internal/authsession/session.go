// Package authsession resolves the credentials CLI commands authenticate with.
package authsession

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/cerebriumai/cerebrium/internal/auth"
	"github.com/cerebriumai/cerebrium/pkg/config"
)

// Token returns the token to authenticate with, refreshing and persisting the
// stored access token when it has expired.
func Token(ctx context.Context, cfg *config.Config) (string, error) {
	// 1. Try service account token from environment variable first
	if token := os.Getenv(config.ServiceAccountEnvVar); token != "" {
		if err := auth.ValidateToken(token); err != nil {
			return "", serviceAccountError(err, token, cfg, true)
		}
		return token, nil
	}

	// 2. Try stored service account token
	if token := cfg.GetServiceAccountToken(); token != "" {
		if err := auth.ValidateToken(token); err != nil {
			return "", serviceAccountError(err, token, cfg, false)
		}
		return token, nil
	}

	// 3. Try access token
	token := cfg.GetAccessToken()
	if token == "" {
		return "", ErrNotLoggedIn
	}

	if err := auth.ValidateToken(token); err == nil {
		return token, nil
	}

	// 4. Access token expired, try to refresh
	refreshToken := cfg.GetRefreshToken()
	if refreshToken == "" {
		return "", ErrSessionExpired
	}

	envConfig := cfg.GetEnvConfig()
	newToken, err := auth.RefreshToken(ctx, envConfig.AuthUrl, envConfig.ClientID, refreshToken)
	if errors.Is(err, auth.ErrInvalidGrant) {
		// Keeping a rejected grant only buys a failed round trip on every later command
		if clearErr := cfg.ClearSession(); clearErr != nil {
			slog.Warn("Failed to clear rejected credentials", "error", clearErr)
		}
		return "", ErrSessionExpired
	} else if err != nil {
		return "", fmt.Errorf("failed to refresh token: %w", err)
	}

	cfg.SetAccessToken(newToken)
	if err := config.Save(cfg); err != nil {
		return "", fmt.Errorf("failed to save new token: %w", err)
	}

	return newToken, nil
}
