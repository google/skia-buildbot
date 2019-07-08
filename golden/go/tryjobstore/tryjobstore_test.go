package tryjobstore

import (
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
)

func TestTryjobJsonCodec(t *testing.T) {
	unittest.SmallTest(t)

	tryjob_1 := &Tryjob{
		IssueID:       12345,
		PatchsetID:    9,
		Builder:       "Test-Builder-1",
		BuildBucketID: 45409309403,
		Status:        TRYJOB_RUNNING,
	}

	// Create a codec like we do for firing events and test the round trip.
	codec := util.JSONCodec(&Tryjob{})
	jsonBytes, err := codec.Encode(tryjob_1)
	assert.NoError(t, err)

	foundInterface, err := codec.Decode(jsonBytes)
	assert.NoError(t, err)
	assert.IsType(t, foundInterface, &Tryjob{})
	assert.Equal(t, tryjob_1, foundInterface.(*Tryjob))
}
