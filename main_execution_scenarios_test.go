package main

import (
	"errors"
	"testing"

	"cderun/internal/command"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnit_Main_CommandExecutionScenarios(t *testing.T) {
	t.Run("Execute help flag returns nil error", func(t *testing.T) {
		args := []string{"cderun", "--help"}
		err := command.Execute(args)
		require.NoError(t, err)
	})

	t.Run("Execute version flag returns nil error", func(t *testing.T) {
		args := []string{"cderun", "--version"}
		err := command.Execute(args)
		require.NoError(t, err)
	})

	t.Run("Execute dry-run ad-hoc execution", func(t *testing.T) {
		args := []string{"cderun", "--dry-run", "--image=alpine:latest", "echo", "hello"}
		err := command.Execute(args)
		require.NoError(t, err)
	})

	t.Run("Execute invalid argument returns error", func(t *testing.T) {
		args := []string{"cderun", "--nonexistent-flag-xyz"}
		err := command.Execute(args)
		require.Error(t, err)
	})

	t.Run("ExitCodeError unwrapping and formatting", func(t *testing.T) {
		underlyingErr := errors.New("underlying failure")
		exitErr := &command.ExitCodeError{
			Code: 42,
			Err:  underlyingErr,
		}

		assert.Equal(t, 42, exitErr.Code)
		assert.Equal(t, "underlying failure", exitErr.Error())

		var extracted *command.ExitCodeError
		require.ErrorAs(t, exitErr, &extracted)
		assert.Equal(t, 42, extracted.Code)
	})
}
