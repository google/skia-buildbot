package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadCASCommand(t *testing.T) {
	got := ReadCASCommand()
	require.NotNil(t, got)
	assert.Equal(t, "readcas", got.Name)
}
