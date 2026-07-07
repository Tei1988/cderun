package main

import (
	"errors"
	"fmt"
	"os"

	"cderun/internal/command"
)

func main() {
	if err := command.Execute(os.Args); err != nil {
		var exitErr *command.ExitCodeError
		if errors.As(err, &exitErr) {
			if exitErr.Err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", exitErr.Err)
			}
			os.Exit(exitErr.Code)
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
