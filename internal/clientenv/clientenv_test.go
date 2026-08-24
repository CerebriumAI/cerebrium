package clientenv

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// agentMarkerVars lists the environment variables from the detect-agent
// registry that could plausibly be set where this suite runs (a developer's
// agent session, an editor terminal, CI). detect-agent reads the real process
// environment, so each test neutralizes these for a deterministic baseline.
var agentMarkerVars = []string{
	"AI_AGENT",
	"CLAUDECODE",
	"CLAUDE_CODE",
	"CLAUDE_CODE_IS_COWORK",
	"CURSOR_TRACE_ID",
	"CURSOR_AGENT",
	"CURSOR_EXTENSION_HOST_ROLE",
	"GEMINI_CLI",
	"CODEX_SANDBOX",
	"CODEX_CI",
	"CODEX_THREAD_ID",
	"CODEX_SANDBOX_NETWORK_DISABLED",
	"COPILOT_MODEL",
	"COPILOT_ALLOW_ALL",
	"COPILOT_GITHUB_TOKEN",
	"TERM_PROGRAM",
	"REPL_ID",
}

func resetEnvironment(t *testing.T) {
	t.Helper()
	for _, key := range agentMarkerVars {
		t.Setenv(key, "")
	}
	t.Setenv("CI", "")
	for _, key := range ciEnvVars {
		t.Setenv(key, "")
	}
}

func TestDetect(t *testing.T) {
	tcs := []struct {
		name     string
		vars     map[string]string
		expected string
	}{
		{
			name:     "clean environment is interactive",
			expected: ValueInteractive,
		},
		{
			name:     "detected agent",
			vars:     map[string]string{"CLAUDECODE": "1"},
			expected: "agent:claude_code",
		},
		{
			name:     "declared agent is sanitized for the header, keeping the version suffix",
			vars:     map[string]string{"AI_AGENT": " My Bot@2.0\r\n"},
			expected: "agent:mybot@2.0",
		},
		{
			name:     "declared agent sanitizing to empty reports unknown",
			vars:     map[string]string{"AI_AGENT": "!!!"},
			expected: "agent:unknown",
		},
		{
			name:     "CI",
			vars:     map[string]string{"CI": "true"},
			expected: ValueCI,
		},
		{
			name:     "agent wins over CI",
			vars:     map[string]string{"CLAUDECODE": "1", "CI": "true"},
			expected: "agent:claude_code",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			resetEnvironment(t)
			for key, value := range tc.vars {
				t.Setenv(key, value)
			}

			assert.Equal(t, tc.expected, detect())
		})
	}
}

func TestHeaderValueIsStableAndWellFormed(t *testing.T) {
	first := HeaderValue()
	second := HeaderValue()

	assert.Equal(t, first, second)
	assert.True(
		t,
		first == ValueCI || first == ValueInteractive || strings.HasPrefix(first, agentValuePrefix),
		"unexpected header value %q", first,
	)
}
