package authsession

import (
	"errors"
	"fmt"
	"time"

	"github.com/cerebriumai/cerebrium/internal/auth"
	"github.com/cerebriumai/cerebrium/pkg/config"
)

// serviceAccountError describes why a service account token cannot be used, naming the
// source it came from so the remedy matches: a token in the environment is not fixed by
// writing one to the config file, and vice versa.
func serviceAccountError(err error, token string, cfg *config.Config, fromEnv bool) error {
	source := "service account token"
	remedy := "then save it with 'cerebrium save-auth-config <token>'"
	if fromEnv {
		source = config.ServiceAccountEnvVar
		remedy = "then update the environment variable"
	}

	cause := fmt.Sprintf("could not be read (%v)", err)
	if errors.Is(err, auth.ErrTokenExpired) {
		cause = "has expired"
		if expiry, ok := tokenExpiry(token); ok {
			cause = fmt.Sprintf("expired on %s", expiry.Local().Format("2006-01-02 15:04"))
		}
	}

	return fmt.Errorf("%s %s. Create a new one at %s, %s", source, cause, apiKeysURL(token, cfg), remedy)
}

// apiKeysURL points at the dashboard page that issues service account tokens, deep-linked
// to the project the token names when that can still be read off it.
func apiKeysURL(token string, cfg *config.Config) string {
	base := cfg.GetEnvConfig().DashboardUrl
	claims, err := auth.ParseClaims(token)
	if err != nil {
		return base
	}
	projectID := config.ExtractProjectIDFromClaims(claims)
	if projectID == "" {
		return base
	}
	return fmt.Sprintf("%s/projects/%s/api-keys", base, projectID)
}

// tokenExpiry reports the token's expiry, and whether it carries one at all.
func tokenExpiry(token string) (time.Time, bool) {
	claims, err := auth.ParseClaims(token)
	if err != nil {
		return time.Time{}, false
	}
	exp, ok := claims["exp"].(float64)
	if !ok {
		return time.Time{}, false
	}
	return time.Unix(int64(exp), 0), true
}
