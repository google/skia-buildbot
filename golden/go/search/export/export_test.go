package export

import (
	"bytes"
	"fmt"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/search/frontend"
)

func TestWriteReadExport(t *testing.T) {
	unittest.SmallTest(t)
	testRecs := []*TestRecord{
		{
			TestName: "test-1",
			Digests: []*DigestInfo{
				{
					SRDigest: &frontend.SRDigest{
						Digest: "abc-efg",
					},
					URL: fmt.Sprintf(urlTemplate, "https://example.com", "abc-efg"),
				},
			},
		},
	}

	var buf bytes.Buffer
	assert.NoError(t, WriteTestRecords(testRecs, &buf))
	found, err := ReadTestRecords(&buf)
	assert.NoError(t, err)
	assert.Equal(t, testRecs, found)
}
