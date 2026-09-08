// Package authsession resolves the credentials CLI commands authenticate with.
package authsession

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/cerebriumai/cerebrium/internal/auth"
	"github.com/cerebriumai/cerebrium/pkg/config"
)

var (
	// ErrNotLoggedIn reports that no credentials are stored at all.
	ErrNotLoggedIn = errors.New("not logged in. Please run 'cerebrium login', or set CEREBRIUM_SERVICE_ACCOUNT_TOKEN for non-interactive use")

	// ErrSessionExpired reports that stored credentials exist but can no longer be renewed.
	ErrSessionExpired = errors.New("your session has expired. Please run 'cerebrium login' to continue")
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
		return "", errors.New("service account token has expired. Please generate a new one")
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
	if err != nil {
		if errors.Is(err, auth.ErrInvalidGrant) {
			// Keeping a rejected grant only buys a failed round trip on every later command
			if clearErr := cfg.ClearSession(); clearErr != nil {
				slog.Warn("Failed to clear rejected credentials", "error", clearErr)
			}
			return "", ErrSessionExpired
		}
		return "", fmt.Errorf("failed to refresh token: %w", err)
	}

	cfg.SetAccessToken(newToken)
	if err := config.Save(cfg); err != nil {
		return "", fmt.Errorf("failed to save new token: %w", err)
	}

	return newToken, nil
}
