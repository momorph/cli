package errors

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
)

// ExitCode represents CLI exit codes
type ExitCode int

const (
	// ExitSuccess indicates successful execution
	ExitSuccess ExitCode = 0
	// ExitError indicates a general error
	ExitError ExitCode = 1
	// ExitUsageError indicates invalid command usage
	ExitUsageError ExitCode = 2
	// ExitAuthError indicates an authentication error
	ExitAuthError ExitCode = 3
	// ExitNetworkError indicates a network error
	ExitNetworkError ExitCode = 4
)

// CLIError represents a CLI error with user-friendly message and exit code
type CLIError struct {
	// TechnicalError is the underlying technical error (for logging)
	TechnicalError error
	// UserMsg is the user-friendly error message
	UserMsg string
	// ExitCode is the exit code to return
	ExitCode ExitCode
	// StackTrace contains the call stack when debug mode is enabled
	StackTrace string
}

// Error implements the error interface
func (e *CLIError) Error() string {
	if e.TechnicalError != nil {
		return fmt.Sprintf("%s: %v", e.UserMsg, e.TechnicalError)
	}
	return e.UserMsg
}

// Unwrap returns the underlying error
func (e *CLIError) Unwrap() error {
	return e.TechnicalError
}

// WithStackTrace adds a stack trace to the error
func (e *CLIError) WithStackTrace() *CLIError {
	e.StackTrace = captureStackTrace(2) // Skip 2 frames: WithStackTrace and its caller
	return e
}

// NewCLIError creates a new CLIError
func NewCLIError(technicalErr error, userMsg string, exitCode ExitCode) *CLIError {
	return &CLIError{
		TechnicalError: technicalErr,
		UserMsg:        userMsg,
		ExitCode:       exitCode,
	}
}

// NewError creates a CLIError with ExitError code
func NewError(technicalErr error, userMsg string) *CLIError {
	return NewCLIError(technicalErr, userMsg, ExitError)
}

// NewUsageError creates a CLIError with ExitUsageError code
func NewUsageError(userMsg string) *CLIError {
	return NewCLIError(nil, userMsg, ExitUsageError)
}

// NewAuthError creates a CLIError for authentication failures
func NewAuthError(technicalErr error, userMsg string) *CLIError {
	return NewCLIError(technicalErr, userMsg, ExitAuthError)
}

// NewNetworkError creates a CLIError for network failures
func NewNetworkError(technicalErr error, userMsg string) *CLIError {
	return NewCLIError(technicalErr, userMsg, ExitNetworkError)
}

// Wrap wraps an error with additional context
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

// Wrapf wraps an error with formatted context
func Wrapf(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), err)
}

// Is reports whether any error in err's chain matches target
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// As finds the first error in err's chain that matches target
func As(err error, target interface{}) bool {
	return errors.As(err, target)
}

// captureStackTrace captures the current call stack
func captureStackTrace(skip int) string {
	const maxDepth = 32
	var pcs [maxDepth]uintptr
	n := runtime.Callers(skip+1, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])

	var sb strings.Builder
	for {
		frame, more := frames.Next()
		if !more {
			break
		}
		// Skip runtime internal frames
		if strings.Contains(frame.Function, "runtime.") {
			continue
		}
		fmt.Fprintf(&sb, "  %s\n    %s:%d\n", frame.Function, frame.File, frame.Line)
	}
	return sb.String()
}

// FormatError formats an error for display, optionally including stack trace
func FormatError(err error, includeStack bool) string {
	var sb strings.Builder

	// Check if it's a CLIError with stack trace
	var cliErr *CLIError
	if errors.As(err, &cliErr) {
		sb.WriteString(cliErr.UserMsg)
		if cliErr.TechnicalError != nil && includeStack {
			sb.WriteString("\n\nTechnical details:\n  ")
			sb.WriteString(cliErr.TechnicalError.Error())
		}
		if cliErr.StackTrace != "" && includeStack {
			sb.WriteString("\n\nStack trace:\n")
			sb.WriteString(cliErr.StackTrace)
		}
	} else {
		sb.WriteString(err.Error())
	}

	return sb.String()
}
