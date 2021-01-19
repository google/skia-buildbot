package bazel

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInBazel(t *testing.T) {
	BazelTest(t)
	require.True(t, InBazel())
}
