// Package clientenv classifies the environment the CLI is running in (AI agent,
// CI, or interactive) so API requests can be attributed. Detection is
// observational only and must never influence CLI behavior.
//
// Agent detection is delegated to github.com/vercel/detect-agent, which
// evaluates its vendored registry (the AI_AGENT self-declaration variable
// first, then the agents.json specs in array order, first match wins). This
// package only classifies the result and makes the agent name safe to emit in
// an HTTP header: detect-agent returns the raw trimmed AI_AGENT value for
// self-declared agents, which is caller-supplied and may contain
// header-hostile characters.
package clientenv

import (
	"os"
	"strings"
	"sync"

	detectagent "github.com/vercel/detect-agent"
)

const (
	// HeaderName is the HTTP header carrying the client environment classification.
	HeaderName = "X-Client-Env"

	// ValueCI is the header value emitted when running under a CI system.
	ValueCI = "ci"

	// ValueInteractive is the header value emitted for a human-driven session.
	ValueInteractive = "interactive"

	agentValuePrefix   = "agent:"
	unknownAgentName   = "unknown"
	maxAgentNameLength = 64
)

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
		headerValue = detect()
	})
	return headerValue
}

// detect classifies the client environment, returning the X-Client-Env value.
// An agent classification wins over CI because an agent running inside a CI
// job is more specifically an agent, and the hosted agent surfaces all set CI.
// Any detection error is treated as no agent so classification can never
// affect CLI behavior.
func detect() string {
	if agent, err := detectagent.Detect(); err == nil {
		return agentValuePrefix + headerSafeAgentName(agent.Name)
	}
	if isCI() {
		return ValueCI
	}
	return ValueInteractive
}

// headerSafeAgentName makes a detected agent name safe for an HTTP header.
// Registry identifiers pass through unchanged; AI_AGENT self-declarations are
// lowercased, restricted to an identifier charset and capped. A name that
// sanitizes away entirely is reported as "unknown" so a detected agent stays
// in the agent bucket instead of silently degrading to interactive.
func headerSafeAgentName(raw string) string {
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
	if name == "" {
		return unknownAgentName
	}
	return name
}

func isCI() bool {
	if value := os.Getenv("CI"); value != "" {
		return value != "0" && !strings.EqualFold(value, "false")
	}
	for _, key := range ciEnvVars {
		if os.Getenv(key) != "" {
			return true
		}
	}
	return false
}
