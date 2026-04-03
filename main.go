package main

import (
	"os"

	"cderun/internal/command"
)

func main() {
	_ = command.Execute(os.Args)
}
