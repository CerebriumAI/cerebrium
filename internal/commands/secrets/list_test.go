package secrets

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Hiding values is the whole reason this payload has its own shape. If someone
// later collapses jsonSecret into the raw map, JSON output starts printing every
// secret in the project while the table still hides them.
func TestNewJSONSecretsHidesValuesByDefault(t *testing.T) {
	keys := []string{"API_KEY", "DB_PASSWORD"}
	secrets := map[string]string{"API_KEY": "sk-live-abc123", "DB_PASSWORD": "hunter2"}

	out, err := json.Marshal(newJSONSecrets(keys, secrets, false))
	require.NoError(t, err)

	assert.JSONEq(t, `[{"name":"API_KEY"},{"name":"DB_PASSWORD"}]`, string(out))
	assert.NotContains(t, string(out), "sk-live-abc123")
	assert.NotContains(t, string(out), "hunter2")
}

func TestNewJSONSecretsIncludesValuesWhenAsked(t *testing.T) {
	keys := []string{"API_KEY"}
	secrets := map[string]string{"API_KEY": "sk-live-abc123"}

	out, err := json.Marshal(newJSONSecrets(keys, secrets, true))
	require.NoError(t, err)

	assert.JSONEq(t, `[{"name":"API_KEY","value":"sk-live-abc123"}]`, string(out))
}

// A secret genuinely set to "" is still a secret that exists. Encoding it through
// a pointer keeps it distinguishable from a hidden value rather than omitted.
func TestNewJSONSecretsKeepsEmptyValue(t *testing.T) {
	out, err := json.Marshal(newJSONSecrets([]string{"EMPTY"}, map[string]string{"EMPTY": ""}, true))
	require.NoError(t, err)

	assert.JSONEq(t, `[{"name":"EMPTY","value":""}]`, string(out))
}

func TestNewJSONSecretsPreservesKeyOrder(t *testing.T) {
	keys := []string{"A", "B", "C"}
	secrets := map[string]string{"C": "3", "A": "1", "B": "2"}

	result := newJSONSecrets(keys, secrets, false)

	require.Len(t, result, 3)
	assert.Equal(t, "A", result[0].Name)
	assert.Equal(t, "B", result[1].Name)
	assert.Equal(t, "C", result[2].Name)
}

// No secrets must encode as [] rather than null, so a consumer can iterate the
// result without a nil check.
func TestNewJSONSecretsEncodesEmptyAsArray(t *testing.T) {
	out, err := json.Marshal(newJSONSecrets(nil, map[string]string{}, false))
	require.NoError(t, err)

	assert.Equal(t, "[]", string(out))
}
