package config

import (
	"fmt"
)

// ImageNotFoundError indicates that no image mapping was found for a given tool.
type ImageNotFoundError struct {
	Tool string
}

func (e *ImageNotFoundError) Error() string {
	return "no image mapping found for tool: " + e.Tool
}

// InvalidConfigError indicates that a configuration option has an invalid value.
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

// RegistryMismatchError indicates an internal inconsistency in the configuration registry or reflection logic.
type RegistryMismatchError struct {
	Option string
	Reason string
}

func (e *RegistryMismatchError) Error() string {
	return fmt.Sprintf("registry mismatch: %s for option %q", e.Reason, e.Option)
}
