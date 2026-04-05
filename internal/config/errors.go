package config

import (
	"fmt"
)

// ImageNotFoundError occurs when no image mapping is found for a subcommand.
type ImageNotFoundError struct {
	Tool string
}

func (e *ImageNotFoundError) Error() string {
	return "no image mapping found for tool: " + e.Tool
}

// InvalidConfigError occurs when a configuration value is invalid.
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

// RegistryMismatchError occurs when an internal inconsistency is detected in the option registry.
type RegistryMismatchError struct {
	Option string
	Err    error
}

func (e *RegistryMismatchError) Error() string {
	return fmt.Sprintf("registry mismatch: %v (option: %q)", e.Err, e.Option)
}

func (e *RegistryMismatchError) Unwrap() error {
	return e.Err
}
