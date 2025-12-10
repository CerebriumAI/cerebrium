package auth

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestJWT creates a test JWT with the given claims
func createTestJWT(claims map[string]interface{}) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))

	claimsJSON, _ := json.Marshal(claims)
	payload := base64.RawURLEncoding.EncodeToString(claimsJSON)

	// Fake signature (not verified anyway)
	signature := base64.RawURLEncoding.EncodeToString([]byte("fake-signature"))

	return header + "." + payload + "." + signature
}

func TestExtractProjectIDFromJWT(t *testing.T) {
	tcs := []struct {
		name           string
		claims         map[string]interface{}
		expectedID     string
		expectedErrMsg string
	}{
		{
			name: "extracts project_id claim",
			claims: map[string]interface{}{
				"project_id": "p-123456",
				"sub":        "user@example.com",
			},
			expectedID: "p-123456",
		},
		{
			name: "extracts projectId claim",
			claims: map[string]interface{}{
				"projectId": "p-abcdef",
			},
			expectedID: "p-abcdef",
		},
		{
			name: "extracts sub claim when it looks like project ID",
			claims: map[string]interface{}{
				"sub": "p-from-sub",
			},
			expectedID: "p-from-sub",
		},
		{
			name: "extracts project claim",
			claims: map[string]interface{}{
				"project": "p-proj-claim",
			},
			expectedID: "p-proj-claim",
		},
		{
			name: "prefers project_id over other claims",
			claims: map[string]interface{}{
				"project_id": "p-preferred",
				"projectId":  "p-not-this",
				"sub":        "p-not-this-either",
				"project":    "p-nor-this",
			},
			expectedID: "p-preferred",
		},
		{
			name: "extracts from custom.project_id",
			claims: map[string]interface{}{
				"custom": map[string]interface{}{
					"project_id": "p-custom-claim",
				},
			},
			expectedID: "p-custom-claim",
		},
		{
			name: "extracts from custom.projectId",
			claims: map[string]interface{}{
				"custom": map[string]interface{}{
					"projectId": "p-custom-camel",
				},
			},
			expectedID: "p-custom-camel",
		},
		{
			name: "extracts from custom.project",
			claims: map[string]interface{}{
				"custom": map[string]interface{}{
					"project": "p-custom-project",
				},
			},
			expectedID: "p-custom-project",
		},
		{
			name: "prefers top-level over custom claims",
			claims: map[string]interface{}{
				"project_id": "p-top-level",
				"custom": map[string]interface{}{
					"project_id": "p-custom",
				},
			},
			expectedID: "p-top-level",
		},
		{
			name: "handles dev project IDs",
			claims: map[string]interface{}{
				"project_id": "dev-p-123456",
			},
			expectedID: "dev-p-123456",
		},
		{
			name: "ignores sub that is not a valid project ID",
			claims: map[string]interface{}{
				"sub":     "user@example.com",
				"project": "p-valid-project",
			},
			expectedID: "p-valid-project",
		},
		{
			name: "returns error when no valid project ID found",
			claims: map[string]interface{}{
				"sub":   "user@example.com",
				"email": "user@example.com",
			},
			expectedErrMsg: "does not contain a valid project_id claim",
		},
		{
			name:           "returns error for empty claims",
			claims:         map[string]interface{}{},
			expectedErrMsg: "does not contain a valid project_id claim",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			token := createTestJWT(tc.claims)

			projectID, err := ExtractProjectIDFromJWT(token)

			if tc.expectedErrMsg != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				assert.Empty(t, projectID)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedID, projectID)
			}
		})
	}
}

func TestExtractProjectIDFromJWT_InvalidToken(t *testing.T) {
	tcs := []struct {
		name           string
		token          string
		expectedErrMsg string
	}{
		{
			name:           "invalid token format - no dots",
			token:          "invalid-token",
			expectedErrMsg: "invalid JWT token",
		},
		{
			name:           "invalid token format - one dot",
			token:          "header.payload",
			expectedErrMsg: "invalid JWT token",
		},
		{
			name:           "invalid base64 payload",
			token:          "header.!!!invalid!!!.signature",
			expectedErrMsg: "invalid JWT token",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			projectID, err := ExtractProjectIDFromJWT(tc.token)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedErrMsg)
			assert.Empty(t, projectID)
		})
	}
}

func Test_isValidProjectID(t *testing.T) {
	tcs := []struct {
		name      string
		projectID string
		expected  bool
	}{
		{
			name:      "valid prod project ID",
			projectID: "p-123456",
			expected:  true,
		},
		{
			name:      "valid dev project ID",
			projectID: "dev-p-123456",
			expected:  true,
		},
		{
			name:      "invalid - no prefix",
			projectID: "123456",
			expected:  false,
		},
		{
			name:      "invalid - wrong prefix",
			projectID: "project-123456",
			expected:  false,
		},
		{
			name:      "invalid - email address",
			projectID: "user@example.com",
			expected:  false,
		},
		{
			name:      "empty string",
			projectID: "",
			expected:  false,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			result := isValidProjectID(tc.projectID)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGetJWTClaims(t *testing.T) {
	t.Run("returns all claims", func(t *testing.T) {
		expectedClaims := map[string]interface{}{
			"project_id": "p-123456",
			"sub":        "user@example.com",
			"exp":        float64(1234567890),
		}
		token := createTestJWT(expectedClaims)

		claims, err := GetJWTClaims(token)

		require.NoError(t, err)
		assert.Equal(t, "p-123456", claims["project_id"])
		assert.Equal(t, "user@example.com", claims["sub"])
		assert.Equal(t, float64(1234567890), claims["exp"])
	})

	t.Run("returns error for invalid token", func(t *testing.T) {
		claims, err := GetJWTClaims("invalid-token")

		require.Error(t, err)
		assert.Nil(t, claims)
	})
}
