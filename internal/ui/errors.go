package ui

import "fmt"

// ErrorType defines the category of error for proper handling
type ErrorType int

const (
	ErrorTypeUserCancelled ErrorType = iota // Ctrl+C, 'q' - silent exit
	ErrorTypeValidation                     // Pre-flight checks - show error, no usage
	ErrorTypeAPI                            // Network/API - show error, no usage
	ErrorTypeFileSystem                     // File operations - show error, no usage
	ErrorTypeConfiguration                  // Config issues - show error, no usage
	ErrorTypeInternal                       // Unexpected - show error, no usage
)

// UIError defines a structured error type for communication between Bubbletea and Cobra.
// It provides metadata about how the error should be handled and presented to the user.
type UIError struct {
	Err           error
	Type          ErrorType
	SuppressUsage bool // Don't show Cobra usage message
	SilentExit    bool // Don't show error message (already rendered in UI or should be silent)
}

func (e *UIError) Error() string {
	return e.Err.Error()
}

func (e *UIError) Unwrap() error {
	return e.Err
}

// Constructor helpers

func NewUserCancelledError() *UIError {
	return &UIError{
		Err:           fmt.Errorf("cancelled by user"),
		Type:          ErrorTypeUserCancelled,
		SuppressUsage: true,
		SilentExit:    true,
	}
}

func NewValidationError(err error) *UIError {
	return &UIError{
		Err:           err,
		Type:          ErrorTypeValidation,
		SuppressUsage: true,
		SilentExit:    false,
	}
}

func NewAPIError(err error) *UIError {
	return &UIError{
		Err:           err,
		Type:          ErrorTypeAPI,
		SuppressUsage: true,
		SilentExit:    false, // Caller decides if shown in UI
	}
}

func NewFileSystemError(err error) *UIError {
	return &UIError{
		Err:           err,
		Type:          ErrorTypeFileSystem,
		SuppressUsage: true,
		SilentExit:    false, // Caller decides if shown in UI
	}
}

func NewConfigurationError(err error) *UIError {
	return &UIError{
		Err:           err,
		Type:          ErrorTypeConfiguration,
		SuppressUsage: true,
		SilentExit:    false, // Caller decides if shown in UI
	}
}

func NewInternalError(err error) *UIError {
	return &UIError{
		Err:           err,
		Type:          ErrorTypeInternal,
		SuppressUsage: true,
		SilentExit:    false, // Caller decides if shown in UI
	}
}
