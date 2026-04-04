package runtime

import (
	"fmt"
)

// RuntimeInitError indicates a failure during container runtime initialization.
type RuntimeInitError struct {
	Runtime string
	Err     error
}

func (e *RuntimeInitError) Error() string {
	return fmt.Sprintf("failed to initialize runtime: %v", e.Err)
}

func (e *RuntimeInitError) Unwrap() error {
	return e.Err
}
