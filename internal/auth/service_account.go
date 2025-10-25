package auth

import (
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// GetServiceAccountToken checks for a service account token from environment variable
func GetServiceAccountToken() (string, error) {
	token := os.Getenv("CEREBRIUM_SERVICE_ACCOUNT_TOKEN")
	if token == "" {
		return "", nil // No service account token configured
	}

	// Validate the token
	if err := ValidateServiceAccountToken(token); err != nil {
		return "", err
	}

	return token, nil
}

// ValidateServiceAccountToken checks if a service account token is valid
func ValidateServiceAccountToken(token string) error {
	// Parse without verification to check expiration
	parsed, _, err := new(jwt.Parser).ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		return fmt.Errorf("failed to parse service account token: %w", err)
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return fmt.Errorf("failed to parse JWT claims")
	}

	// Check expiration if present
	if exp, ok := claims["exp"].(float64); ok {
		expirationTime := time.Unix(int64(exp), 0)
		if time.Now().After(expirationTime) {
			return fmt.Errorf("service account token has expired")
		}
	}

	return nil
}
