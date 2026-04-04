package config

import "fmt"

// ImageNotFoundError indicates that no image mapping was found for a tool.
type ImageNotFoundError struct {
	Tool string
}

func (e *ImageNotFoundError) Error() string {
	return "no image mapping found for tool: " + e.Tool
}

// InvalidConfigError indicates that a configuration value is invalid.
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

// RegistryMismatchError indicates an internal inconsistency in the option registry.
type RegistryMismatchError struct {
	Option string
	Reason string
}

func (e *RegistryMismatchError) Error() string {
	return fmt.Sprintf("registry mismatch: %s", e.Reason)
}

// Validation Error compat helpers to match existing tests
func (e *ImageNotFoundError) Is(target error) bool {
	_, ok := target.(*ImageNotFoundError)
	return ok
}

func (e *InvalidConfigError) Is(target error) bool {
	_, ok := target.(*InvalidConfigError)
	return ok
}

func (e *RegistryMismatchError) Is(target error) bool {
	_, ok := target.(*RegistryMismatchError)
	return ok
}
