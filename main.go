package main

import (
	"fmt"
	"os"

	"cderun/internal/command"
)

func main() {
	if err := command.Execute(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
