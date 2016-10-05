package buildbucket

import (
	"testing"

	"go.skia.org/infra/go/httputils"

	assert "github.com/stretchr/testify/require"
)

func TestGetTrybotsForCL(t *testing.T) {
	client := NewClient(httputils.NewTimeoutClient())
	issueID, patchsetID := int64(2347), int64(7)
	_, err := client.GetTrybotsForCL(issueID, patchsetID, "gerrit", "https://skia-review.googlesource.com")
	assert.NoError(t, err)
}
