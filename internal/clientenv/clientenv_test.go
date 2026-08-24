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

func ttyPresent() bool { return true }

func ttyAbsent() bool { return false }

// TestAgentDetectorsMatchRegistryOrder pins the matrix to vercel/detect-agent
// agents.json (schema version 1). Identifiers and order are both load-bearing:
// the identifiers make our attribution comparable with the registry, and the
// order decides which of two simultaneous markers wins. Update this list only
// alongside a deliberate registry sync.
func TestAgentDetectorsMatchRegistryOrder(t *testing.T) {
	expected := []string{
		"cursor",
		"cursor-cli",
		"kimi",
		"grok",
		"gemini_cli",
		"cline",
		"codex_cli",
		"antigravity",
		"augment-cli",
		"open_code",
		"goose",
		"junie",
		"pi",
		"cowork",
		"claude_code",
		"replit",
		"github-copilot",
		"kiro",
		"openclaw",
		"devin",
	}

	actual := make([]string, 0, len(agentDetectors))
	for _, d := range agentDetectors {
		actual = append(actual, d.name)
	}

	assert.Equal(t, expected, actual)
}

func TestDetect(t *testing.T) {
	tcs := []struct {
		name       string
		vars       map[string]string
		markerFile bool
		noTTY      bool
		expected   string
	}{
		{
			name:     "empty environment is interactive",
			vars:     map[string]string{},
			expected: ValueInteractive,
		},

		// One case per registry entry, in registry order.
		{
			name:     "CURSOR_TRACE_ID",
			vars:     map[string]string{"CURSOR_TRACE_ID": "trace-1"},
			expected: "agent:cursor",
		},
		{
			name:     "CURSOR_AGENT",
			vars:     map[string]string{"CURSOR_AGENT": "1"},
			expected: "agent:cursor-cli",
		},
		{
			name:     "CURSOR_EXTENSION_HOST_ROLE agent-exec",
			vars:     map[string]string{"CURSOR_EXTENSION_HOST_ROLE": "agent-exec"},
			expected: "agent:cursor-cli",
		},
		{
			name:     "CURSOR_EXTENSION_HOST_ROLE other value is not an agent",
			vars:     map[string]string{"CURSOR_EXTENSION_HOST_ROLE": "editor"},
			expected: ValueInteractive,
		},
		{
			name:     "KIMI_PLUGIN_ROOT",
			vars:     map[string]string{"KIMI_PLUGIN_ROOT": "/root"},
			expected: "agent:kimi",
		},
		{
			name:     "GROK_PLUGIN_ROOT",
			vars:     map[string]string{"GROK_PLUGIN_ROOT": "/root"},
			expected: "agent:grok",
		},
		{
			name:     "GROK_PLUGIN_DATA",
			vars:     map[string]string{"GROK_PLUGIN_DATA": "/data"},
			expected: "agent:grok",
		},
		{
			name:     "GEMINI_CLI",
			vars:     map[string]string{"GEMINI_CLI": "1"},
			expected: "agent:gemini_cli",
		},
		{
			name:     "CLINE_ACTIVE",
			vars:     map[string]string{"CLINE_ACTIVE": "1"},
			expected: "agent:cline",
		},
		{
			name:     "CODEX_SANDBOX",
			vars:     map[string]string{"CODEX_SANDBOX": "seatbelt"},
			expected: "agent:codex_cli",
		},
		{
			name:     "CODEX_CI",
			vars:     map[string]string{"CODEX_CI": "1"},
			expected: "agent:codex_cli",
		},
		{
			name:     "CODEX_THREAD_ID",
			vars:     map[string]string{"CODEX_THREAD_ID": "thread-1"},
			expected: "agent:codex_cli",
		},
		{
			name:     "CODEX_SANDBOX_NETWORK_DISABLED",
			vars:     map[string]string{"CODEX_SANDBOX_NETWORK_DISABLED": "1"},
			expected: "agent:codex_cli",
		},
		{
			name:     "ANTIGRAVITY_AGENT",
			vars:     map[string]string{"ANTIGRAVITY_AGENT": "1"},
			expected: "agent:antigravity",
		},
		{
			name:     "ANTIGRAVITY_CLI_ALIAS",
			vars:     map[string]string{"ANTIGRAVITY_CLI_ALIAS": "ag"},
			expected: "agent:antigravity",
		},
		{
			name:     "AUGMENT_AGENT",
			vars:     map[string]string{"AUGMENT_AGENT": "1"},
			expected: "agent:augment-cli",
		},
		{
			name:     "OPENCODE_CLIENT",
			vars:     map[string]string{"OPENCODE_CLIENT": "cli"},
			expected: "agent:open_code",
		},
		{
			name:     "OPENCODE",
			vars:     map[string]string{"OPENCODE": "1"},
			expected: "agent:open_code",
		},
		{
			name:     "GOOSE_PROVIDER",
			vars:     map[string]string{"GOOSE_PROVIDER": "anthropic"},
			expected: "agent:goose",
		},
		{
			name:     "JUNIE_DATA",
			vars:     map[string]string{"JUNIE_DATA": "/data"},
			expected: "agent:junie",
		},
		{
			name:     "JUNIE_SHIM_PATH",
			vars:     map[string]string{"JUNIE_SHIM_PATH": "/shim"},
			expected: "agent:junie",
		},
		{
			name:     "PATH with a .pi/agent segment",
			vars:     map[string]string{"PATH": "/usr/bin:/home/u/.pi/agent/bin"},
			expected: "agent:pi",
		},
		{
			name:     "PATH with a windows .pi agent segment",
			vars:     map[string]string{"PATH": `C:\Users\u\.pi\agent\bin`},
			expected: "agent:pi",
		},
		{
			name:     "ordinary PATH is not pi",
			vars:     map[string]string{"PATH": "/usr/local/bin:/usr/bin"},
			expected: ValueInteractive,
		},
		{
			name:     "CLAUDE_CODE_IS_COWORK with a claude marker",
			vars:     map[string]string{"CLAUDE_CODE_IS_COWORK": "1", "CLAUDECODE": "1"},
			expected: "agent:cowork",
		},
		{
			name:     "CLAUDE_CODE_IS_COWORK alone is not cowork",
			vars:     map[string]string{"CLAUDE_CODE_IS_COWORK": "1"},
			expected: ValueInteractive,
		},
		{
			name:     "CLAUDECODE",
			vars:     map[string]string{"CLAUDECODE": "1"},
			expected: "agent:claude_code",
		},
		{
			name:     "CLAUDE_CODE",
			vars:     map[string]string{"CLAUDE_CODE": "1"},
			expected: "agent:claude_code",
		},
		{
			name:     "COPILOT_MODEL",
			vars:     map[string]string{"COPILOT_MODEL": "gpt"},
			expected: "agent:github-copilot",
		},
		{
			name:     "COPILOT_ALLOW_ALL",
			vars:     map[string]string{"COPILOT_ALLOW_ALL": "1"},
			expected: "agent:github-copilot",
		},
		{
			name:     "COPILOT_GITHUB_TOKEN",
			vars:     map[string]string{"COPILOT_GITHUB_TOKEN": "tok"},
			expected: "agent:github-copilot",
		},
		{
			name:     "OPENCLAW_SHELL",
			vars:     map[string]string{"OPENCLAW_SHELL": "1"},
			expected: "agent:openclaw",
		},
		{
			name:       "devin marker file",
			vars:       map[string]string{},
			markerFile: true,
			expected:   "agent:devin",
		},

		// Host variables that need corroborating evidence of an agent. Both are
		// set for humans working in the vendor's editor, so a TTY means a person.
		{
			name:     "REPL_ID without a TTY is the replit agent",
			vars:     map[string]string{"REPL_ID": "repl-1"},
			noTTY:    true,
			expected: "agent:replit",
		},
		{
			name:     "REPL_ID with a TTY is a human on Replit",
			vars:     map[string]string{"REPL_ID": "repl-1"},
			expected: ValueInteractive,
		},
		{
			name:     "TERM_PROGRAM kiro without a TTY is the kiro agent",
			vars:     map[string]string{"TERM_PROGRAM": "kiro"},
			noTTY:    true,
			expected: "agent:kiro",
		},
		{
			name:     "TERM_PROGRAM kiro with a TTY is a human at the kiro terminal",
			vars:     map[string]string{"TERM_PROGRAM": "kiro"},
			expected: ValueInteractive,
		},
		{
			name:     "unrelated TERM_PROGRAM without a TTY is not kiro",
			vars:     map[string]string{"TERM_PROGRAM": "iTerm.app"},
			noTTY:    true,
			expected: ValueInteractive,
		},

		// Ordering. The registry evaluates in array order and the first match wins.
		{
			name:     "cowork beats claude_code",
			vars:     map[string]string{"CLAUDECODE": "1", "CLAUDE_CODE_IS_COWORK": "1"},
			expected: "agent:cowork",
		},
		{
			name:     "grok beats claude_code on shared plugin markers",
			vars:     map[string]string{"GROK_PLUGIN_ROOT": "/root", "CLAUDECODE": "1"},
			expected: "agent:grok",
		},
		{
			name:     "cursor beats a later agent",
			vars:     map[string]string{"CURSOR_TRACE_ID": "trace-1", "CLAUDECODE": "1"},
			expected: "agent:cursor",
		},

		// AI_AGENT self-declaration.
		{
			name:     "AI_AGENT generic name",
			vars:     map[string]string{"AI_AGENT": "my-bot"},
			expected: "agent:my-bot",
		},
		{
			name:     "AI_AGENT wins over a detected agent",
			vars:     map[string]string{"AI_AGENT": "my-wrapper", "CLAUDECODE": "1"},
			expected: "agent:my-wrapper",
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
			name:     "AI_AGENT sanitizing to empty still counts as a declaration",
			vars:     map[string]string{"AI_AGENT": "!!!"},
			expected: "agent:unknown",
		},
		{
			name:     "AI_AGENT truncated to the length cap",
			vars:     map[string]string{"AI_AGENT": strings.Repeat("a", maxAgentNameLength+10)},
			expected: agentValuePrefix + strings.Repeat("a", maxAgentNameLength),
		},
		{
			name:     "AI_AGENT whitespace only is not a declaration",
			vars:     map[string]string{"AI_AGENT": "   "},
			expected: ValueInteractive,
		},

		// CI.
		{
			name:     "CI truthy",
			vars:     map[string]string{"CI": "true"},
			expected: ValueCI,
		},
		{
			name:     "CI false is not CI",
			vars:     map[string]string{"CI": "false"},
			expected: ValueInteractive,
		},
		{
			name:     "CI zero is not CI",
			vars:     map[string]string{"CI": "0"},
			expected: ValueInteractive,
		},
		{
			name:     "GITHUB_ACTIONS",
			vars:     map[string]string{"GITHUB_ACTIONS": "true"},
			expected: ValueCI,
		},
		{
			name:     "TF_BUILD",
			vars:     map[string]string{"TF_BUILD": "True"},
			expected: ValueCI,
		},
		{
			name:     "CODEBUILD_BUILD_ID",
			vars:     map[string]string{"CODEBUILD_BUILD_ID": "build:1"},
			expected: ValueCI,
		},
		{
			name:     "an agent inside CI reports the agent",
			vars:     map[string]string{"CI": "true", "CLAUDECODE": "1"},
			expected: "agent:claude_code",
		},
		{
			name:     "codex in CI reports codex, not ci",
			vars:     map[string]string{"CI": "true", "CODEX_CI": "1"},
			expected: "agent:codex_cli",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			fileExists := noMarkerFile
			if tc.markerFile {
				fileExists = func(path string) bool { return path == "/opt/.devin" }
			}

			isTTY := ttyPresent
			if tc.noTTY {
				isTTY = ttyAbsent
			}

			assert.Equal(t, tc.expected, Detect(envFrom(tc.vars), fileExists, isTTY))
		})
	}
}

func TestDetectNeverReturnsABareAgentPrefix(t *testing.T) {
	for _, d := range agentDetectors {
		assert.NotEmpty(t, d.name, "detector with an empty name would emit a bare %q", agentValuePrefix)
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
