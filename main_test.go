package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"cderun/internal/command"
)

func TestMain_VersionFlag(t *testing.T) {
	// Verify that the main CLI entrypoint can execute with '--version' cleanly and returns no error.
	args := []string{"cderun", "--version"}
	err := command.Execute(args)
	assert.NoError(t, err)
}

func TestMain_HelpFlag(t *testing.T) {
	// Verify that the main CLI entrypoint can execute with '--help' cleanly and returns no error.
	args := []string{"cderun", "--help"}
	err := command.Execute(args)
	assert.NoError(t, err)
}
