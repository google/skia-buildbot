package buildbucket

import (
	"testing"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/testutils"
)

func TestGetTrybotsForCL(t *testing.T) {
	testutils.SkipIfShort(t)

	client := NewClient(httputils.NewTimeoutClient())
	tries, err := client.GetTrybotsForCL(2347, 7, "gerrit", "https://skia-review.googlesource.com")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(tries))
}
