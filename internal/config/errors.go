package config

import (
	"fmt"
)

// ImageNotFoundError is returned when no image mapping is found for a tool.
type ImageNotFoundError struct {
	Tool string
}

func (e *ImageNotFoundError) Error() string {
	return "no image mapping found for tool: " + e.Tool
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

// RegistryMismatchError is returned when the internal registry does not match CLIOptions fields.
type RegistryMismatchError struct {
	Option string
	Message string
}

func (e *RegistryMismatchError) Error() string {
	return e.Message
}
