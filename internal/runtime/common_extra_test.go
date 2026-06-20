package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnit_Runtime_Common_IsTemporaryAuthError(t *testing.T) {
	assert.False(t, IsTemporaryAuthError(nil))
	assert.False(t, IsTemporaryAuthError(errors.New("permanent failure")))

	refreshableKeywords := []string{
		"token expired",
		"expired token",
		"refresh token",
		"reauthenticate",
		"token refresh",
	}

	for _, kw := range refreshableKeywords {
		t.Run(kw, func(t *testing.T) {
			assert.True(t, IsTemporaryAuthError(errors.New("error: "+kw)))
		})
	}
}

func TestUnit_Runtime_Common_IsRetryablePullError_Canceled(t *testing.T) {
	assert.False(t, IsRetryablePullError(context.Canceled))
}
