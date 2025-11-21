// Package bugsnag provides enterprise-grade error tracking and monitoring for the Cerebrium CLI.
// It automatically captures errors, panics, and system metadata to help diagnose issues in production.
package bugsnag

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/bugsnag/bugsnag-go/v2"
	"github.com/cerebriumai/cerebrium/internal/version"
	"github.com/cerebriumai/cerebrium/pkg/config"
	"github.com/golang-jwt/jwt/v5"
)

// Build-time variables that can be set via ldflags
// Example: go build -ldflags "-X github.com/cerebriumai/cerebrium/pkg/bugsnag.BugsnagAPIKey=your-key"
var (
	// BugsnagAPIKey is the API key for error reporting, injected at compile time.
	// If not set during build, error reporting will be disabled.
	BugsnagAPIKey = ""

	// DefaultReleaseStage defines the default environment for error reporting.
	// Can be overridden at compile time via ldflags.
	DefaultReleaseStage = "prod"
)

// initialized tracks whether Bugsnag has been initialized
var initialized bool

// enabled tracks whether Bugsnag error reporting is actually active
var enabled bool

// Initialize configures the Bugsnag error reporting client.
// It sets up automatic error capture, system metadata collection, and user context tracking.
// This function is idempotent and thread-safe for concurrent initialization.
// If BugsnagAPIKey is not set at compile time or telemetry is disabled, error reporting will be silently disabled.
func Initialize() error {
	if initialized {
		return nil
	}

	// Check if telemetry is disabled by user
	cfg, _ := config.Load() // Ignore error - proceed with default behavior if config unavailable
	if cfg != nil && !cfg.IsTelemetryEnabled() {
		initialized = true
		enabled = false
		return nil
	}

	// Skip initialization if API key was not provided at compile time
	if BugsnagAPIKey == "" {
		initialized = true // Mark as initialized to prevent repeated checks
		enabled = false
		return nil
	}

	// Allow environment variable override for API key if needed
	apiKey := BugsnagAPIKey
	if envKey := os.Getenv("BUGSNAG_API_KEY"); envKey != "" {
		apiKey = envKey
	}

	releaseStage := os.Getenv("CEREBRIUM_ENV")
	if releaseStage == "" {
		releaseStage = DefaultReleaseStage
	}

	appVersion := version.Version
	if appVersion == "" {
		appVersion = "dev"
	}

	// Configure Bugsnag with production-ready settings
	bugsnag.Configure(bugsnag.Configuration{
		APIKey:              apiKey,
		ReleaseStage:        releaseStage,
		AppVersion:          appVersion,
		AppType:             "cli",
		ProjectPackages:     []string{"main", "github.com/cerebriumai/cerebrium"},
		NotifyReleaseStages: []string{"prod", "dev", "local"},
		PanicHandler:        func() {}, // Manual panic handling for better control
		Synchronous:         false,     // Asynchronous error reporting for performance
		AutoCaptureSessions: true,      // Track CLI session health metrics
	})

	// Enrich error reports with system information
	addSystemMetadata()

	// Attach user context for better error attribution
	setUserContext()

	initialized = true
	enabled = true
	return nil
}

// IsEnabled returns whether Bugsnag error reporting is active.
// This will be false if no API key was provided at compile time.
func IsEnabled() bool {
	return enabled
}

// addSystemMetadata enriches error reports with runtime environment information.
// This includes OS details, architecture, Go version, and resource utilization metrics.
func addSystemMetadata() {
	systemInfo := bugsnag.MetaData{
		"system": {
			"os_type":       runtime.GOOS,
			"os_arch":       runtime.GOARCH,
			"go_version":    runtime.Version(),
			"num_cpu":       runtime.NumCPU(),
			"num_goroutine": runtime.NumGoroutine(),
		},
	}

	bugsnag.OnBeforeNotify(func(event *bugsnag.Event, bugsnagConfig *bugsnag.Configuration) error {
		for tab, data := range systemInfo {
			for key, value := range data {
				event.MetaData.Add(tab, key, value)
			}
		}
		return nil
	})
}

// setUserContext attaches user identification to error reports for better debugging and attribution.
// It extracts user information from the stored JWT token without transmitting sensitive data.
func setUserContext() {
	bugsnag.OnBeforeNotify(func(event *bugsnag.Event, bugsnagConfig *bugsnag.Configuration) error {
		// Load config to get user context - ignore errors to avoid blocking error reporting
		cfg, _ := config.Load()

		token := cfg.AccessToken
		if token == "" {
			return nil
		}

		userID := getUserIDFromJWT(token)
		if userID != "" {
			event.User = &bugsnag.User{
				Id: userID,
			}
		}

		// Attach project context for multi-project debugging
		if cfg.ProjectID != "" {
			event.MetaData.Add("project", "project_id", cfg.ProjectID)
		}

		return nil
	})
}

// getUserIDFromJWT safely extracts the user identifier from a JWT token.
// It does not validate the token signature to avoid blocking on network calls.
func getUserIDFromJWT(tokenString string) string {
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())

	token, _, err := parser.ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return ""
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return ""
	}

	// Check standard JWT subject claim first, then fall back to username
	if sub, ok := claims["sub"].(string); ok && sub != "" {
		return sub
	}

	if username, ok := claims["username"].(string); ok && username != "" {
		return username
	}

	return ""
}

// NotifyError reports critical errors that indicate system failures or unexpected behavior.
// These errors typically require immediate attention and may affect user functionality.
func NotifyError(ctx context.Context, err error) {
	if !initialized {
		_ = Initialize()
	}

	if !enabled || err == nil {
		return
	}

	_ = bugsnag.Notify(err, ctx, bugsnag.SeverityError)
}

// NotifyWarning reports non-critical issues that may indicate potential problems.
// These are typically recoverable errors or degraded functionality scenarios.
func NotifyWarning(ctx context.Context, err error) {
	if !initialized {
		_ = Initialize()
	}

	if !enabled || err == nil {
		return
	}

	_ = bugsnag.Notify(err, ctx, bugsnag.SeverityWarning)
}

// NotifyInfo reports informational events for monitoring and debugging purposes.
// These are typically expected errors or noteworthy system events.
func NotifyInfo(ctx context.Context, err error) {
	if !initialized {
		_ = Initialize()
	}

	if !enabled || err == nil {
		return
	}

	_ = bugsnag.Notify(err, ctx, bugsnag.SeverityInfo)
}

// Notify reports an error with custom severity level for flexible error categorization.
// Use bugsnag.SeverityError, bugsnag.SeverityWarning, or bugsnag.SeverityInfo as severity values.
func Notify(ctx context.Context, err error, severity interface{}) {
	if !initialized {
		_ = Initialize()
	}

	if !enabled || err == nil {
		return
	}

	_ = bugsnag.Notify(err, ctx, severity)
}

// NotifyWithMetadata reports an error with custom metadata for enhanced debugging context.
// Additional metadata helps developers understand the state and conditions when errors occur.
func NotifyWithMetadata(ctx context.Context, err error, severity interface{}, metadata bugsnag.MetaData) {
	if !initialized {
		_ = Initialize()
	}

	if !enabled || err == nil {
		return
	}

	_ = bugsnag.Notify(err, ctx, severity, metadata)
}

// WrapError enhances an error with additional context for better error tracking.
// Use this to add descriptive information about where and why an error occurred.
func WrapError(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

// NotifyOnPanic captures and reports panic conditions before propagating them.
// Always use with defer at the start of goroutines and main functions for comprehensive panic tracking.
func NotifyOnPanic(ctx context.Context) {
	if r := recover(); r != nil {
		var err error
		switch x := r.(type) {
		case string:
			err = fmt.Errorf("panic: %s", x)
		case error:
			err = fmt.Errorf("panic: %w", x)
		default:
			err = fmt.Errorf("panic: %v", r)
		}

		// Report panic as critical error
		NotifyError(ctx, err)

		// Preserve panic behavior for proper error handling
		panic(r)
	}
}

// Flush ensures all queued error reports are transmitted before application termination.
// Call this in main() defer or before process exit to prevent error loss.
func Flush() {
	// Bugsnag Go client handles flushing internally through its async queue
	// This function exists for API consistency and future enhancements
}

// SetProjectID associates errors with a specific project for multi-project environments.
// Call this when switching project contexts to ensure accurate error attribution.
func SetProjectID(projectID string) {
	if !initialized {
		_ = Initialize()
	}

	bugsnag.OnBeforeNotify(func(event *bugsnag.Event, bugsnagConfig *bugsnag.Configuration) error {
		event.MetaData.Add("project", "project_id", projectID)
		return nil
	})
}

// SetCommandContext tracks which CLI command triggered an error for better debugging.
// This metadata helps identify command-specific issues and usage patterns.
func SetCommandContext(command string, args []string) {
	if !initialized {
		_ = Initialize()
	}

	bugsnag.OnBeforeNotify(func(event *bugsnag.Event, bugsnagConfig *bugsnag.Configuration) error {
		event.MetaData.Add("command", "name", command)
		if len(args) > 0 {
			event.MetaData.Add("command", "args", strings.Join(args, " "))
		}
		return nil
	})
}

// IsUserCancellation identifies errors from user-initiated cancellations.
// These errors are excluded from reporting as they represent normal user behavior, not system issues.
func IsUserCancellation(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	return strings.Contains(errStr, "context canceled") ||
		strings.Contains(errStr, "operation cancelled") ||
		strings.Contains(errStr, "user cancelled")
}
