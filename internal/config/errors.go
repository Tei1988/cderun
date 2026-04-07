package config

import (
	"fmt"
)

// ImageNotFoundError indicates that no image mapping was found for a given tool.
type ImageNotFoundError struct {
	Tool string
}

func (e *ImageNotFoundError) Error() string {
	return fmt.Sprintf("no image mapping found for tool: %q", e.Tool)
}

// InvalidConfigError indicates that a configuration field has an invalid value.
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

// RegistryMismatchError indicates an inconsistency between the options registry and the internal structures.
type RegistryMismatchError struct {
	ExpectedRegistry string
	ActualRegistry   string
	Message          string
}

func (e *RegistryMismatchError) Error() string {
	if e.Message != "" {
		return "registry mismatch: " + e.Message
	}
	return fmt.Sprintf("registry mismatch: expected %q, but got %q", e.ExpectedRegistry, e.ActualRegistry)
}
