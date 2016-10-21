package buildbucket

import (
	"fmt"
	"testing"

	"github.com/davecgh/go-spew/spew"
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

func TestGetTrybotsForCLxxx(t *testing.T) {
	testutils.SkipIfShort(t)

	client := NewClient(httputils.NewTimeoutClient())
	tries, err := client.GetTrybotsForCL(3787, 2, "gerrit", "https://skia-review.googlesource.com")
	assert.NoError(t, err)
	for i, try := range tries {
		fmt.Printf("\n\nTRY %d: %s\n\n", i, spew.Sprint(try.Parameters))
	}
}
