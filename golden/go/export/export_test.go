package export

import (
	"fmt"
	"testing"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/search"
)

const TEST_FILE_NAME = "gold_index.json"

func TestWriteRead(t *testing.T) {
	testutils.MediumTest(t)
	testRecs := []*TestRecord{
		{
			TestName: "test-1",
			Digests: []*DigestInfo{
				{
					SRDigest: &search.SRDigest{
						Digest: "abc-efg",
					},
					URL: fmt.Sprintf(urlTemplate, "https://example.com", "abc-efg"),
				},
			},
		},
	}

	assert.NoError(t, WriteTestRecords(TEST_FILE_NAME, testRecs))
	defer util.Remove(TEST_FILE_NAME)
	found, err := ReadTestRecords(TEST_FILE_NAME)
	assert.NoError(t, err)
	assert.Equal(t, testRecs, found)
}
