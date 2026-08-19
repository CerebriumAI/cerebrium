// Package clientenv classifies the environment the CLI is running in
// (AI agent, CI, or interactive) for API request attribution.
// Detection is observational only and must never influence CLI behavior.
package clientenv

import (
	"os"
	"strings"
	"sync"
)

const (
	// HeaderName is the HTTP header carrying the client environment classification.
	HeaderName = "X-Client-Env"

	// ValueCI is the header value emitted when running under a CI system.
	ValueCI = "ci"

	// ValueInteractive is the header value emitted for a human-driven session.
	ValueInteractive = "interactive"

	agentValuePrefix   = "agent:"
	devinMarkerPath    = "/opt/.devin"
	maxAgentNameLength = 64
)

var agentDetectors = []func(env func(string) string) string{
	detectNamedAgent,
	detectCursor,
	detectCursorCLI,
	detectGemini,
	detectCodex,
	detectAntigravity,
	detectAugment,
	detectOpencode,
	detectClaude,
	detectGitHubCopilot,
}

// REPL_ID is deliberately not a detector: it reports that the process is
// running on Replit, not that an agent is driving it, so keying on it would
// classify humans working in the Replit editor as agents. A Replit agent that
// sets AI_AGENT is still detected by detectNamedAgent.
var ciEnvVars = []string{
	"CONTINUOUS_INTEGRATION",
	"GITHUB_ACTIONS",
	"GITLAB_CI",
	"CIRCLECI",
	"TRAVIS",
	"BUILDKITE",
	"JENKINS_URL",
	"TEAMCITY_VERSION",
	"TF_BUILD",
	"DRONE",
	"APPVEYOR",
	"CODEBUILD_BUILD_ID",
	"BITBUCKET_BUILD_NUMBER",
}

var (
	headerValueOnce sync.Once
	headerValue     string
)

// HeaderValue returns the process-wide X-Client-Env value, computed once.
func HeaderValue() string {
	headerValueOnce.Do(func() {
		headerValue = Detect(os.Getenv, markerFileExists)
	})
	return headerValue
}

// Detect classifies the client environment from the supplied environment
// lookup and filesystem probe, returning the X-Client-Env header value.
func Detect(env func(string) string, fileExists func(string) bool) string {
	if name := detectAgent(env, fileExists); name != "" {
		return agentValuePrefix + name
	}
	if isCI(env) {
		return ValueCI
	}
	return ValueInteractive
}

func detectAgent(env func(string) string, fileExists func(string) bool) string {
	for _, detector := range agentDetectors {
		if name := detector(env); name != "" {
			return name
		}
	}
	if fileExists(devinMarkerPath) {
		return "devin"
	}
	return ""
}

func detectNamedAgent(env func(string) string) string {
	raw := strings.TrimSpace(env("AI_AGENT"))
	if raw == "" {
		return ""
	}
	name := sanitizeAgentName(raw)
	if name == "" {
		return "unknown"
	}
	if name == "github-copilot-cli" {
		return "github-copilot"
	}
	return name
}

func detectCursor(env func(string) string) string {
	if env("CURSOR_TRACE_ID") != "" {
		return "cursor"
	}
	return ""
}

func detectCursorCLI(env func(string) string) string {
	if env("CURSOR_AGENT") != "" || env("CURSOR_EXTENSION_HOST_ROLE") == "agent-exec" {
		return "cursor-cli"
	}
	return ""
}

func detectGemini(env func(string) string) string {
	if env("GEMINI_CLI") != "" {
		return "gemini"
	}
	return ""
}

func detectCodex(env func(string) string) string {
	if env("CODEX_SANDBOX") != "" || env("CODEX_CI") != "" || env("CODEX_THREAD_ID") != "" {
		return "codex"
	}
	return ""
}

func detectAntigravity(env func(string) string) string {
	if env("ANTIGRAVITY_AGENT") != "" {
		return "antigravity"
	}
	return ""
}

func detectAugment(env func(string) string) string {
	if env("AUGMENT_AGENT") != "" {
		return "augment-cli"
	}
	return ""
}

func detectOpencode(env func(string) string) string {
	if env("OPENCODE_CLIENT") != "" {
		return "opencode"
	}
	return ""
}

func detectClaude(env func(string) string) string {
	if env("CLAUDECODE") == "" && env("CLAUDE_CODE") == "" {
		return ""
	}
	if env("CLAUDE_CODE_IS_COWORK") != "" {
		return "cowork"
	}
	return "claude"
}

func detectGitHubCopilot(env func(string) string) string {
	if env("COPILOT_MODEL") != "" || env("COPILOT_ALLOW_ALL") != "" || env("COPILOT_GITHUB_TOKEN") != "" {
		return "github-copilot"
	}
	return ""
}

func isCI(env func(string) string) bool {
	if value := env("CI"); value != "" {
		return value != "0" && !strings.EqualFold(value, "false")
	}
	for _, key := range ciEnvVars {
		if env(key) != "" {
			return true
		}
	}
	return false
}

func sanitizeAgentName(raw string) string {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	var builder strings.Builder
	for _, char := range trimmed {
		switch {
		case char >= 'a' && char <= 'z',
			char >= '0' && char <= '9',
			char == '-', char == '.', char == '_':
			builder.WriteRune(char)
		}
	}
	name := builder.String()
	if len(name) > maxAgentNameLength {
		name = name[:maxAgentNameLength]
	}
	return name
}

func markerFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
