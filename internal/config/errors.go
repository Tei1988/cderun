package config

import (
	"fmt"
)

// ImageNotFoundError is returned when no image mapping is found for a tool.
type ImageNotFoundError struct {
	Tool string
}

func (e *ImageNotFoundError) Error() string {
	return fmt.Sprintf("no image mapping found for tool: %q", e.Tool)
}

// InvalidConfigError is returned when a configuration value is invalid.
type InvalidConfigError struct {
	Field string
	Value string
	Err   error
}

func (e *InvalidConfigError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("invalid %s value %q: %v", e.Field, e.Value, e.Err)
	}
	return fmt.Sprintf("invalid %s value %q", e.Field, e.Value)
}

func (e *InvalidConfigError) Unwrap() error {
	return e.Err
}

// RegistryMismatchError is returned when there is an inconsistency in the internal configuration registry
// or a mismatch between expected and actual container image registries.
type RegistryMismatchError struct {
	Message          string
	ExpectedRegistry string
	ActualRegistry   string
}

func (e *RegistryMismatchError) Error() string {
	if e.ExpectedRegistry != "" || e.ActualRegistry != "" {
		return fmt.Sprintf("registry mismatch: expected %q, got %q", e.ExpectedRegistry, e.ActualRegistry)
	}
	return "registry mismatch: " + e.Message
}
