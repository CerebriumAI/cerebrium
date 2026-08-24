// Package clientenv classifies the environment the CLI is running in (AI agent,
// CI, or interactive) so API requests can be attributed. Detection is
// observational only and must never influence CLI behavior.
//
// The agent matrix mirrors the vercel/detect-agent registry (agents.json
// schema version 1), which promotes AI_AGENT as the cross-vendor
// self-declaration variable and evaluates agents in array order, first match
// wins. Identifiers are kept byte-identical to that registry, including its
// inconsistent underscores (claude_code, gemini_cli, codex_cli, open_code)
// alongside its hyphens (cursor-cli, augment-cli, github-copilot), because
// matching the registry verbatim is what makes our attribution comparable with
// anyone else reading the same list.
package clientenv

import (
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/mattn/go-isatty"
)

const (
	// HeaderName is the HTTP header carrying the client environment classification.
	HeaderName = "X-Client-Env"

	// ValueCI is the header value emitted when running under a CI system.
	ValueCI = "ci"

	// ValueInteractive is the header value emitted for a human-driven session.
	ValueInteractive = "interactive"

	agentValuePrefix   = "agent:"
	maxAgentNameLength = 64
)

// piPathPattern and kiroTermPattern mirror the registry's env_matches patterns
// verbatim.
var (
	piPathPattern   = regexp.MustCompile(`\.pi[\\/]agent`)
	kiroTermPattern = regexp.MustCompile(`kiro`)
)

// probe supplies the readings a condition needs. Every lookup is injected so
// the whole matrix is testable without mutating the real process environment.
type probe struct {
	env        func(string) string
	fileExists func(string) bool
	stdinIsTTY func() bool
}

type condition func(p probe) bool

type detector struct {
	name  string
	match condition
}

// agentDetectors mirrors vercel/detect-agent agents.json in array order. Order
// is load-bearing: grok precedes claude_code because Grok supports Claude Code
// plugins and may expose its markers, and cowork precedes claude_code because
// it is the more specific marker.
//
// Two deliberate divergences from the registry, both narrowing:
//
//   - replit is gated on the absence of a TTY. REPL_ID is set for every process
//     on Replit, including a human in the Replit editor, so the bare variable
//     evidences the host rather than an agent. This applies the registry's own
//     stated reasoning for kiro ("set by both the IDE terminal and the CLI
//     agent, so gate on no_tty to avoid misdetecting a human at the integrated
//     terminal") to the variable that has the same problem.
//   - AI_AGENT is evaluated first rather than as a fallback, so an operator
//     wrapping a known agent can name their own harness and have that win.
var agentDetectors = []detector{
	{name: "cursor", match: envSet("CURSOR_TRACE_ID")},
	{name: "cursor-cli", match: anyOf(envSet("CURSOR_AGENT"), envEquals("CURSOR_EXTENSION_HOST_ROLE", "agent-exec"))},
	{name: "kimi", match: envSet("KIMI_PLUGIN_ROOT")},
	{name: "grok", match: anyOf(envSet("GROK_PLUGIN_ROOT"), envSet("GROK_PLUGIN_DATA"))},
	{name: "gemini_cli", match: envSet("GEMINI_CLI")},
	{name: "cline", match: envSet("CLINE_ACTIVE")},
	{name: "codex_cli", match: anyOf(
		envSet("CODEX_SANDBOX"),
		envSet("CODEX_CI"),
		envSet("CODEX_THREAD_ID"),
		envSet("CODEX_SANDBOX_NETWORK_DISABLED"),
	)},
	{name: "antigravity", match: anyOf(envSet("ANTIGRAVITY_AGENT"), envSet("ANTIGRAVITY_CLI_ALIAS"))},
	{name: "augment-cli", match: envSet("AUGMENT_AGENT")},
	{name: "open_code", match: anyOf(envSet("OPENCODE_CLIENT"), envSet("OPENCODE"))},
	{name: "goose", match: envSet("GOOSE_PROVIDER")},
	{name: "junie", match: anyOf(envSet("JUNIE_DATA"), envSet("JUNIE_SHIM_PATH"))},
	{name: "pi", match: envMatches("PATH", piPathPattern)},
	{name: "cowork", match: allOf(
		envSet("CLAUDE_CODE_IS_COWORK"),
		anyOf(envSet("CLAUDECODE"), envSet("CLAUDE_CODE")),
	)},
	{name: "claude_code", match: anyOf(envSet("CLAUDECODE"), envSet("CLAUDE_CODE"))},
	{name: "replit", match: allOf(envSet("REPL_ID"), noTTY())},
	{name: "github-copilot", match: anyOf(
		envSet("COPILOT_MODEL"),
		envSet("COPILOT_ALLOW_ALL"),
		envSet("COPILOT_GITHUB_TOKEN"),
	)},
	{name: "kiro", match: allOf(envMatches("TERM_PROGRAM", kiroTermPattern), noTTY())},
	{name: "openclaw", match: envSet("OPENCLAW_SHELL")},
	{name: "devin", match: fileExists("/opt/.devin")},
}

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
		headerValue = Detect(os.Getenv, markerFileExists, stdinIsTTY)
	})
	return headerValue
}

// Detect classifies the client environment from the supplied environment
// lookup, filesystem probe and TTY probe, returning the X-Client-Env value.
// An agent classification wins over CI because an agent running inside a CI
// job is more specifically an agent, and the hosted agent surfaces all set CI.
func Detect(env func(string) string, fileExists func(string) bool, stdinIsTTY func() bool) string {
	p := probe{env: env, fileExists: fileExists, stdinIsTTY: stdinIsTTY}

	if name := detectAgent(p); name != "" {
		return agentValuePrefix + name
	}
	if isCI(env) {
		return ValueCI
	}
	return ValueInteractive
}

func detectAgent(p probe) string {
	if name, declared := declaredAgent(p.env); declared {
		return name
	}
	for _, d := range agentDetectors {
		if d.match(p) {
			return d.name
		}
	}
	return ""
}

// declaredAgent reads the registry's AI_AGENT self-declaration variable. The
// value is caller-supplied, so it is lowercased, restricted to an identifier
// charset and capped. A value that sanitizes away entirely still counts as a
// declaration, reported as "unknown", so a declared-but-unparseable agent
// stays in the agent bucket instead of silently degrading to interactive.
func declaredAgent(env func(string) string) (string, bool) {
	raw := strings.TrimSpace(env("AI_AGENT"))
	if raw == "" {
		return "", false
	}

	name := sanitizeAgentName(raw)
	if name == "" {
		return "unknown", true
	}
	if name == "github-copilot-cli" {
		return "github-copilot", true
	}
	return name, true
}

func envSet(name string) condition {
	return func(p probe) bool { return p.env(name) != "" }
}

func envEquals(name, value string) condition {
	return func(p probe) bool { return p.env(name) == value }
}

func envMatches(name string, pattern *regexp.Regexp) condition {
	return func(p probe) bool {
		value := p.env(name)
		return value != "" && pattern.MatchString(value)
	}
}

func fileExists(path string) condition {
	return func(p probe) bool { return p.fileExists(path) }
}

func noTTY() condition {
	return func(p probe) bool { return !p.stdinIsTTY() }
}

func anyOf(conditions ...condition) condition {
	return func(p probe) bool {
		for _, c := range conditions {
			if c(p) {
				return true
			}
		}
		return false
	}
}

func allOf(conditions ...condition) condition {
	return func(p probe) bool {
		for _, c := range conditions {
			if !c(p) {
				return false
			}
		}
		return true
	}
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

func stdinIsTTY() bool {
	return isatty.IsTerminal(os.Stdin.Fd())
}
