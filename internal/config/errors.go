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
