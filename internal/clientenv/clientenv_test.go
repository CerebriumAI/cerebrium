package clientenv

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func envFrom(vars map[string]string) func(string) string {
	return func(key string) string {
		return vars[key]
	}
}

func noMarkerFile(string) bool { return false }

func TestDetect(t *testing.T) {
	tcs := []struct {
		name       string
		vars       map[string]string
		markerFile bool
		expected   string
	}{
		{
			name:     "empty environment - interactive",
			vars:     map[string]string{},
			expected: "interactive",
		},
		{
			name:     "CLAUDECODE - claude",
			vars:     map[string]string{"CLAUDECODE": "1"},
			expected: "agent:claude",
		},
		{
			name:     "CLAUDE_CODE - claude",
			vars:     map[string]string{"CLAUDE_CODE": "1"},
			expected: "agent:claude",
		},
		{
			name:     "CLAUDECODE with cowork marker - cowork",
			vars:     map[string]string{"CLAUDECODE": "1", "CLAUDE_CODE_IS_COWORK": "1"},
			expected: "agent:cowork",
		},
		{
			name:     "CURSOR_TRACE_ID - cursor",
			vars:     map[string]string{"CURSOR_TRACE_ID": "trace-123"},
			expected: "agent:cursor",
		},
		{
			name:     "CURSOR_AGENT - cursor-cli",
			vars:     map[string]string{"CURSOR_AGENT": "1"},
			expected: "agent:cursor-cli",
		},
		{
			name:     "CURSOR_EXTENSION_HOST_ROLE agent-exec - cursor-cli",
			vars:     map[string]string{"CURSOR_EXTENSION_HOST_ROLE": "agent-exec"},
			expected: "agent:cursor-cli",
		},
		{
			name:     "CURSOR_EXTENSION_HOST_ROLE other value - interactive",
			vars:     map[string]string{"CURSOR_EXTENSION_HOST_ROLE": "editor"},
			expected: "interactive",
		},
		{
			name:     "GEMINI_CLI - gemini",
			vars:     map[string]string{"GEMINI_CLI": "1"},
			expected: "agent:gemini",
		},
		{
			name:     "CODEX_SANDBOX - codex",
			vars:     map[string]string{"CODEX_SANDBOX": "seatbelt"},
			expected: "agent:codex",
		},
		{
			name:     "CODEX_CI - codex",
			vars:     map[string]string{"CODEX_CI": "1"},
			expected: "agent:codex",
		},
		{
			name:     "CODEX_THREAD_ID - codex",
			vars:     map[string]string{"CODEX_THREAD_ID": "thread-1"},
			expected: "agent:codex",
		},
		{
			name:     "ANTIGRAVITY_AGENT - antigravity",
			vars:     map[string]string{"ANTIGRAVITY_AGENT": "1"},
			expected: "agent:antigravity",
		},
		{
			name:     "AUGMENT_AGENT - augment-cli",
			vars:     map[string]string{"AUGMENT_AGENT": "1"},
			expected: "agent:augment-cli",
		},
		{
			name:     "OPENCODE_CLIENT - opencode",
			vars:     map[string]string{"OPENCODE_CLIENT": "1"},
			expected: "agent:opencode",
		},
		{
			name:     "REPL_ID alone is a host, not an agent",
			vars:     map[string]string{"REPL_ID": "repl-1"},
			expected: ValueInteractive,
		},
		{
			name:     "REPL_ID with AI_AGENT reports the agent",
			vars:     map[string]string{"REPL_ID": "repl-1", "AI_AGENT": "replit-agent"},
			expected: "agent:replit-agent",
		},
		{
			name:     "COPILOT_MODEL - github-copilot",
			vars:     map[string]string{"COPILOT_MODEL": "gpt"},
			expected: "agent:github-copilot",
		},
		{
			name:     "COPILOT_ALLOW_ALL - github-copilot",
			vars:     map[string]string{"COPILOT_ALLOW_ALL": "1"},
			expected: "agent:github-copilot",
		},
		{
			name:     "COPILOT_GITHUB_TOKEN - github-copilot",
			vars:     map[string]string{"COPILOT_GITHUB_TOKEN": "tok"},
			expected: "agent:github-copilot",
		},
		{
			name:     "AI_AGENT generic name",
			vars:     map[string]string{"AI_AGENT": "my-bot"},
			expected: "agent:my-bot",
		},
		{
			name:     "AI_AGENT github-copilot-cli normalized",
			vars:     map[string]string{"AI_AGENT": "github-copilot-cli"},
			expected: "agent:github-copilot",
		},
		{
			name:     "AI_AGENT trimmed and lowercased",
			vars:     map[string]string{"AI_AGENT": "  My-Bot "},
			expected: "agent:my-bot",
		},
		{
			name:     "AI_AGENT unsafe characters stripped",
			vars:     map[string]string{"AI_AGENT": "sneaky\r\nagent!"},
			expected: "agent:sneakyagent",
		},
		{
			name:     "AI_AGENT sanitizing to empty still counts as agent",
			vars:     map[string]string{"AI_AGENT": "!!!"},
			expected: "agent:unknown",
		},
		{
			name:     "AI_AGENT overlong name truncated",
			vars:     map[string]string{"AI_AGENT": strings.Repeat("a", 100)},
			expected: "agent:" + strings.Repeat("a", 64),
		},
		{
			name:     "AI_AGENT whitespace only falls through to CI",
			vars:     map[string]string{"AI_AGENT": "   ", "CI": "true"},
			expected: "ci",
		},
		{
			name:     "AI_AGENT takes precedence over cursor",
			vars:     map[string]string{"AI_AGENT": "custom", "CURSOR_TRACE_ID": "trace-123"},
			expected: "agent:custom",
		},
		{
			name:     "cursor takes precedence over claude",
			vars:     map[string]string{"CURSOR_TRACE_ID": "trace-123", "CLAUDECODE": "1"},
			expected: "agent:cursor",
		},
		{
			name:     "agent takes precedence over CI",
			vars:     map[string]string{"CLAUDECODE": "1", "CI": "true"},
			expected: "agent:claude",
		},
		{
			name:       "devin marker file - devin",
			vars:       map[string]string{},
			markerFile: true,
			expected:   "agent:devin",
		},
		{
			name:       "env agent takes precedence over devin marker",
			vars:       map[string]string{"GEMINI_CLI": "1"},
			markerFile: true,
			expected:   "agent:gemini",
		},
		{
			name:     "CI true - ci",
			vars:     map[string]string{"CI": "true"},
			expected: "ci",
		},
		{
			name:     "CI 1 - ci",
			vars:     map[string]string{"CI": "1"},
			expected: "ci",
		},
		{
			name:     "CI false - interactive",
			vars:     map[string]string{"CI": "false"},
			expected: "interactive",
		},
		{
			name:     "CI 0 - interactive",
			vars:     map[string]string{"CI": "0"},
			expected: "interactive",
		},
		{
			name:     "GITHUB_ACTIONS - ci",
			vars:     map[string]string{"GITHUB_ACTIONS": "true"},
			expected: "ci",
		},
		{
			name:     "GITLAB_CI - ci",
			vars:     map[string]string{"GITLAB_CI": "true"},
			expected: "ci",
		},
		{
			name:     "JENKINS_URL - ci",
			vars:     map[string]string{"JENKINS_URL": "https://jenkins.example.com"},
			expected: "ci",
		},
		{
			name:     "BUILDKITE - ci",
			vars:     map[string]string{"BUILDKITE": "true"},
			expected: "ci",
		},
		{
			name:     "TF_BUILD - ci",
			vars:     map[string]string{"TF_BUILD": "True"},
			expected: "ci",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			fileExists := noMarkerFile
			if tc.markerFile {
				fileExists = func(path string) bool { return path == devinMarkerPath }
			}

			assert.Equal(t, tc.expected, Detect(envFrom(tc.vars), fileExists))
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
