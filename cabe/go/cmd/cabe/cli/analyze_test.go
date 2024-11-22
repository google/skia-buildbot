package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeCommand(t *testing.T) {
	got := AnalyzeCommand()
	require.NotNil(t, got)
	assert.Equal(t, "analyze", got.Name)
}
