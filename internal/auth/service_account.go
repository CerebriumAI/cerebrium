package auth

import (
	"fmt"
	"os"
	"strings"
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

// ExtractProjectIDFromJWT extracts the project_id from a JWT token.
// It checks multiple claim names to match Python CLI behavior:
// 1. project_id
// 2. projectId
// 3. sub (if it looks like a project ID)
// 4. project
// 5. custom.project_id, custom.projectId, custom.project (nested claims)
func ExtractProjectIDFromJWT(tokenString string) (string, error) {
	// Parse without verification - we just need to extract claims
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return "", fmt.Errorf("invalid JWT token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("failed to parse JWT claims")
	}

	// Try top-level claims in order of preference (matching Python CLI)
	topLevelClaims := []string{"project_id", "projectId", "sub", "project"}
	for _, claimName := range topLevelClaims {
		if value, ok := claims[claimName].(string); ok && isValidProjectID(value) {
			return value, nil
		}
	}

	// Check nested custom claims (matching Python CLI)
	if customClaims, ok := claims["custom"].(map[string]interface{}); ok {
		customClaimNames := []string{"project_id", "projectId", "project"}
		for _, claimName := range customClaimNames {
			if value, ok := customClaims[claimName].(string); ok && isValidProjectID(value) {
				return value, nil
			}
		}
	}

	return "", fmt.Errorf("JWT token does not contain a valid project_id claim")
}

// isValidProjectID checks if a string looks like a valid project ID
func isValidProjectID(projectID string) bool {
	return strings.HasPrefix(projectID, "p-") || strings.HasPrefix(projectID, "dev-p-")
}

// GetJWTClaims returns all claims from a JWT token for debugging purposes
func GetJWTClaims(tokenString string) (map[string]interface{}, error) {
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("invalid JWT token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("failed to parse JWT claims")
	}

	return claims, nil
}
