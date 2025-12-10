package auth

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestJWT creates a test JWT with the given claims
func createTestJWT(claims map[string]any) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))

	claimsJSON, _ := json.Marshal(claims)
	payload := base64.RawURLEncoding.EncodeToString(claimsJSON)

	// Fake signature (not verified anyway)
	signature := base64.RawURLEncoding.EncodeToString([]byte("fake-signature"))

	return header + "." + payload + "." + signature
}

func TestParseClaims(t *testing.T) {
	t.Run("returns all claims", func(t *testing.T) {
		expectedClaims := map[string]any{
			"project_id": "p-123456",
			"sub":        "user@example.com",
			"exp":        float64(1234567890),
		}
		token := createTestJWT(expectedClaims)

		claims, err := ParseClaims(token)

		require.NoError(t, err)
		assert.Equal(t, "p-123456", claims["project_id"])
		assert.Equal(t, "user@example.com", claims["sub"])
		assert.Equal(t, float64(1234567890), claims["exp"])
	})

	t.Run("returns error for invalid token format", func(t *testing.T) {
		tcs := []struct {
			name  string
			token string
		}{
			{
				name:  "no dots",
				token: "invalid-token",
			},
			{
				name:  "one dot",
				token: "header.payload",
			},
			{
				name:  "invalid base64 payload",
				token: "header.!!!invalid!!!.signature",
			},
		}

		for _, tc := range tcs {
			t.Run(tc.name, func(t *testing.T) {
				claims, err := ParseClaims(tc.token)

				require.Error(t, err)
				assert.Nil(t, claims)
			})
		}
	})
}

func TestValidateToken(t *testing.T) {
	t.Run("returns nil for non-expired token", func(t *testing.T) {
		// Token expires in the future (year 2099)
		token := createTestJWT(map[string]any{
			"exp": float64(4102444800), // 2099-01-01
		})

		err := ValidateToken(token)

		require.NoError(t, err)
	})

	t.Run("returns error for expired token", func(t *testing.T) {
		// Token expired in the past
		token := createTestJWT(map[string]any{
			"exp": float64(1000000000), // 2001-09-09
		})

		err := ValidateToken(token)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "expired")
	})

	t.Run("returns nil for missing exp claim", func(t *testing.T) {
		token := createTestJWT(map[string]any{
			"sub": "test",
		})

		err := ValidateToken(token)

		require.NoError(t, err)
	})

	t.Run("returns error for invalid token", func(t *testing.T) {
		err := ValidateToken("invalid-token")

		require.Error(t, err)
	})
}
