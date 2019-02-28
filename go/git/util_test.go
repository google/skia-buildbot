package git

import (
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
)

func TestNormURL(t *testing.T) {
	testutils.SmallTest(t)
	httpURL := "https://github.com/skia-dev/textfiles.git"
	gitURL := "ssh://git@github.com/skia-dev/textfiles"
	gitURLWithExt := "ssh://git@github.com:skia-dev/textfiles.git"
	normHTTP, err := NormalizeURL(httpURL)
	assert.NoError(t, err)
	normGit, err := NormalizeURL(gitURL)
	assert.NoError(t, err)
	normGitWithExt, err := NormalizeURL(gitURLWithExt)
	assert.NoError(t, err)

	// Make sure they all match.
	assert.Equal(t, "github.com/skia-dev/textfiles", normHTTP)
	assert.Equal(t, normHTTP, normGit)
	assert.Equal(t, normHTTP, normGitWithExt)
}
