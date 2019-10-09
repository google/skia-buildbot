package git

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestNormURL(t *testing.T) {
	unittest.SmallTest(t)
	httpURL := "https://github.com/skia-dev/textfiles.git"
	gitURL := "ssh://git@github.com/skia-dev/textfiles"
	gitURLWithExt := "ssh://git@github.com:skia-dev/textfiles.git"
	normHTTP, err := NormalizeURL(httpURL)
	require.NoError(t, err)
	normGit, err := NormalizeURL(gitURL)
	require.NoError(t, err)
	normGitWithExt, err := NormalizeURL(gitURLWithExt)
	require.NoError(t, err)

	// Make sure they all match.
	require.Equal(t, "github.com/skia-dev/textfiles", normHTTP)
	require.Equal(t, normHTTP, normGit)
	require.Equal(t, normHTTP, normGitWithExt)
}
