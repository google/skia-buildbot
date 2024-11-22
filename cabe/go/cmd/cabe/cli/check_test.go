package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckCommand(t *testing.T) {
	got := CheckCommand()
	require.NotNil(t, got)
	assert.Equal(t, "check", got.Name)
}
