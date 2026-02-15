package actions

import (
	"errors"
	"fmt"
)

// Common errors for action handling.
var (
	ErrCancelled      = errors.New("cancelled")
	ErrNotInstalled   = errors.New("dnstc is not installed")
	ErrTunnelNotFound = errors.New("tunnel not found")
	ErrTunnelExists   = errors.New("tunnel already exists")
	ErrNoTunnels      = errors.New("no tunnels configured")
)

// ActionError represents a structured error with a hint.
type ActionError struct {
	Message string
	Hint    string
	Err     error
}

func (e *ActionError) Error() string {
	if e.Hint != "" {
		return fmt.Sprintf("%s\n%s", e.Message, e.Hint)
	}
	return e.Message
}

func (e *ActionError) Unwrap() error {
	return e.Err
}

// NewActionError creates a new ActionError.
func NewActionError(message, hint string) *ActionError {
	return &ActionError{Message: message, Hint: hint}
}

// WrapError wraps an error with a message and hint.
func WrapError(err error, message, hint string) *ActionError {
	return &ActionError{Message: message, Hint: hint, Err: err}
}

// TunnelNotFoundError creates a tunnel not found error.
func TunnelNotFoundError(tag string) *ActionError {
	return &ActionError{
		Message: fmt.Sprintf("tunnel '%s' not found", tag),
		Hint:    "Use 'dnstc tunnel list' to see available tunnels",
		Err:     ErrTunnelNotFound,
	}
}

// TunnelExistsError creates a tunnel already exists error.
func TunnelExistsError(tag string) *ActionError {
	return &ActionError{
		Message: fmt.Sprintf("tunnel '%s' already exists", tag),
		Hint:    "Choose a different tag or remove the existing tunnel",
		Err:     ErrTunnelExists,
	}
}

// NoTunnelsError returns an error indicating no tunnels exist.
func NoTunnelsError() *ActionError {
	return &ActionError{
		Message: "no tunnels configured",
		Hint:    "Use 'dnstc tunnel add' to create one",
		Err:     ErrNoTunnels,
	}
}
