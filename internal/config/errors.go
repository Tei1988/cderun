package config

import (
	"fmt"
)

// ImageNotFoundError indicates that no image mapping was found for a tool.
type ImageNotFoundError struct {
	Tool string
}

func (e *ImageNotFoundError) Error() string {
	return fmt.Sprintf("no image mapping found for tool: %q", e.Tool)
}

// InvalidConfigError indicates that a configuration value is invalid.
type InvalidConfigError struct {
	Field string
	Value string
	Err   error
}

func (e *InvalidConfigError) Error() string {
	return fmt.Sprintf("invalid %s value %q: %v", e.Field, e.Value, e.Err)
}

func (e *InvalidConfigError) Unwrap() error {
	return e.Err
}

// RegistryMismatchError indicates an internal inconsistency in the option registry.
type RegistryMismatchError struct {
	Option           string
	ExpectedRegistry string
	ActualRegistry   string
	Reason           string
}

func (e *RegistryMismatchError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("registry mismatch: %s", e.Reason)
	}
	return fmt.Sprintf("registry mismatch: option %q expected %s but got %s", e.Option, e.ExpectedRegistry, e.ActualRegistry)
}
