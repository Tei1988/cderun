package runtime

import (
	"fmt"
)

// RuntimeInitError indicates a failure during the initialization of a container runtime.
type RuntimeInitError struct {
	Runtime string
	Err     error
}

func (e *RuntimeInitError) Error() string {
	return fmt.Sprintf("failed to initialize runtime %q: %v", e.Runtime, e.Err)
}

func (e *RuntimeInitError) Unwrap() error {
	return e.Err
}
