package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestNormURL(t *testing.T) {
	unittest.SmallTest(t)
	httpURL := "https://github.com/skia-dev/textfiles.git"
	normHTTP, err := NormalizeURL(httpURL)
	require.NoError(t, err)
	assert.Equal(t, "github.com/skia-dev/textfiles", normHTTP)

	gitURL := "ssh://git@github.com/skia-dev/textfiles"
	normGit, err := NormalizeURL(gitURL)
	require.NoError(t, err)
	assert.Equal(t, "github.com/skia-dev/textfiles", normGit)

	gitURLWithExt := "ssh://git@github.com:skia-dev/textfiles.git"
	normGitWithExt, err := NormalizeURL(gitURLWithExt)
	require.NoError(t, err)
	assert.Equal(t, "github.com/skia-dev/textfiles", normGitWithExt)
}
