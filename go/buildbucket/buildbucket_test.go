package buildbucket

import (
	"encoding/json"
	"testing"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/testutils"
)

func TestGetTrybotsForCL(t *testing.T) {
	testutils.MediumTest(t)
	testutils.SkipIfShort(t)

	client := NewClient(httputils.NewTimeoutClient())
	tries, err := client.GetTrybotsForCL(2347, 7, "gerrit", "https://skia-review.googlesource.com")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(tries))
}

func TestSerialize(t *testing.T) {
	testutils.SmallTest(t)
	tc := []struct {
		in  Properties
		out string
	}{
		{
			in:  Properties{},
			out: "{}", // Leave out all empty fields.
		},
		{
			in: Properties{
				Reason: "Triggered by SkiaScheduler",
			},
			out: "{\"reason\":\"Triggered by SkiaScheduler\"}",
		},
	}
	for _, c := range tc {
		b, err := json.Marshal(c.in)
		assert.NoError(t, err)
		assert.Equal(t, []byte(c.out), b)
	}
}
