package config

import "fmt"

// RegistryMismatchError is returned when the container registry specified via CLI
// does not match the registry in the tool configuration.
type RegistryMismatchError struct {
	ExpectedRegistry string
	ActualRegistry   string
}

func (e *RegistryMismatchError) Error() string {
	return fmt.Sprintf("container registry mismatch: expected %q, got %q", e.ExpectedRegistry, e.ActualRegistry)
}

// ImageNotFoundError is returned when no image mapping is found for a tool.
type ImageNotFoundError struct {
	Tool string
}

func (e *ImageNotFoundError) Error() string {
	return fmt.Sprintf("no image mapping found for tool: %q", e.Tool)
}

// RuntimeInitError is returned when failed to initialize container runtime.
type RuntimeInitError struct {
	Runtime string
	Err     error
}

func (e *RuntimeInitError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("failed to initialize runtime %q: %v", e.Runtime, e.Err)
	}
	return fmt.Sprintf("failed to initialize runtime %q", e.Runtime)
}

func (e *RuntimeInitError) Unwrap() error {
	return e.Err
}

// InvalidConfigError is returned when a configuration value is invalid.
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
