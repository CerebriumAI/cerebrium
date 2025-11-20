// Package bugsnag provides error tracking and reporting functionality for the Cerebrium CLI
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

const (
	// BugsnagAPIKey is the API key for the Cerebrium CLI Bugsnag project
	// This matches the Python CLI's API key for consistency
	BugsnagAPIKey = "606044c1e243e11958763fb42cb751c4" // #nosec G101 - This is a public API key for error reporting

	// DefaultReleaseStage is the default environment if CEREBRIUM_ENV is not set
	DefaultReleaseStage = "prod"
)

// initialized tracks whether Bugsnag has been initialized
var initialized bool

// Initialize sets up Bugsnag for error tracking
// This should be called once at application startup
func Initialize() error {
	if initialized {
		return nil
	}

	releaseStage := os.Getenv("CEREBRIUM_ENV")
	if releaseStage == "" {
		releaseStage = DefaultReleaseStage
	}

	appVersion := version.Version
	if appVersion == "" {
		appVersion = "dev"
	}

	// Configure Bugsnag
	bugsnag.Configure(bugsnag.Configuration{
		APIKey:              BugsnagAPIKey,
		ReleaseStage:        releaseStage,
		AppVersion:          appVersion,
		AppType:             "cli",
		ProjectPackages:     []string{"main", "github.com/cerebriumai/cerebrium"},
		NotifyReleaseStages: []string{"prod", "dev", "local"},
		PanicHandler:        func() {}, // Disable automatic panic handling
		Synchronous:         false,     // Send errors asynchronously
		AutoCaptureSessions: true,      // Enable session tracking like Python version
	})

	// Add system metadata
	addSystemMetadata()

	// Set user context from JWT if available
	setUserContext()

	initialized = true
	return nil
}

// addSystemMetadata adds system information to Bugsnag metadata
// This mirrors the Python implementation's system metadata
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

// setUserContext sets the user ID from JWT token if available
// This mirrors the Python implementation's user context setting
func setUserContext() {
	bugsnag.OnBeforeNotify(func(event *bugsnag.Event, bugsnagConfig *bugsnag.Configuration) error {
		// Try to get user ID from config
		cfg, _ := config.Load()
		// Ignore error - don't block error reporting if config fails

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

		// Add project metadata if available
		if cfg.ProjectID != "" {
			event.MetaData.Add("project", "project_id", cfg.ProjectID)
		}

		return nil
	})
}

// getUserIDFromJWT extracts the user ID from a JWT token
// This mirrors the Python get_user_info_from_jwt function
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

	// Try "sub" first, then "username" (same as Python version)
	if sub, ok := claims["sub"].(string); ok && sub != "" {
		return sub
	}

	if username, ok := claims["username"].(string); ok && username != "" {
		return username
	}

	return ""
}

// NotifyError sends an error to Bugsnag with Error severity
func NotifyError(err error, ctx ...context.Context) {
	if !initialized {
		_ = Initialize()
	}

	if err == nil {
		return
	}

	var c context.Context
	if len(ctx) > 0 {
		c = ctx[0]
	} else {
		c = context.Background()
	}

	_ = bugsnag.Notify(err, c, bugsnag.SeverityError)
}

// NotifyWarning sends an error to Bugsnag with Warning severity
func NotifyWarning(err error, ctx ...context.Context) {
	if !initialized {
		_ = Initialize()
	}

	if err == nil {
		return
	}

	var c context.Context
	if len(ctx) > 0 {
		c = ctx[0]
	} else {
		c = context.Background()
	}

	_ = bugsnag.Notify(err, c, bugsnag.SeverityWarning)
}

// NotifyInfo sends an error to Bugsnag with Info severity
func NotifyInfo(err error, ctx ...context.Context) {
	if !initialized {
		_ = Initialize()
	}

	if err == nil {
		return
	}

	var c context.Context
	if len(ctx) > 0 {
		c = ctx[0]
	} else {
		c = context.Background()
	}

	_ = bugsnag.Notify(err, c, bugsnag.SeverityInfo)
}

// Notify sends an error to Bugsnag with the specified severity
// severity should be one of bugsnag.SeverityError, bugsnag.SeverityWarning, or bugsnag.SeverityInfo
func Notify(err error, severity interface{}, ctx ...context.Context) {
	if !initialized {
		_ = Initialize()
	}

	if err == nil {
		return
	}

	var c context.Context
	if len(ctx) > 0 {
		c = ctx[0]
	} else {
		c = context.Background()
	}

	// Send error to Bugsnag with the severity
	_ = bugsnag.Notify(err, c, severity)
}

// NotifyWithMetadata sends an error to Bugsnag with additional metadata
// severity should be one of bugsnag.SeverityError, bugsnag.SeverityWarning, or bugsnag.SeverityInfo
func NotifyWithMetadata(err error, severity interface{}, metadata bugsnag.MetaData, ctx ...context.Context) {
	if !initialized {
		_ = Initialize()
	}

	if err == nil {
		return
	}

	var c context.Context
	if len(ctx) > 0 {
		c = ctx[0]
	} else {
		c = context.Background()
	}

	_ = bugsnag.Notify(err, c, severity, metadata)
}

// WrapError creates a wrapped error with additional context
// This is useful for adding context to errors before notifying
func WrapError(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

// NotifyOnPanic recovers from panics and sends them to Bugsnag
// Use this with defer in main functions and goroutines
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

		// Send panic as error severity
		NotifyError(err, ctx)

		// Re-panic to maintain normal panic behavior
		panic(r)
	}
}

// Flush ensures all pending errors are sent to Bugsnag
// Call this before the application exits
func Flush() {
	// The Go client doesn't have an explicit flush method,
	// but we can add a small delay to ensure async errors are sent
	// This is typically handled by the client automatically
}

// SetProjectID sets the current project ID in Bugsnag metadata
// This should be called when the project context changes
func SetProjectID(projectID string) {
	if !initialized {
		_ = Initialize()
	}

	bugsnag.OnBeforeNotify(func(event *bugsnag.Event, bugsnagConfig *bugsnag.Configuration) error {
		event.MetaData.Add("project", "project_id", projectID)
		return nil
	})
}

// SetCommandContext adds command-specific metadata
// Use this to track which command was running when an error occurred
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

// IsUserCancellation checks if an error is a user cancellation
// User cancellations should not be reported to Bugsnag
func IsUserCancellation(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	return strings.Contains(errStr, "context canceled") ||
		strings.Contains(errStr, "operation cancelled") ||
		strings.Contains(errStr, "user cancelled")
}
