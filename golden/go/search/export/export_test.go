package export

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
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
					SearchResult: &frontend.SearchResult{
						Digest: "abc-efg",
					},
					URL: fmt.Sprintf(urlTemplate, "https://example.com", "abc-efg"),
				},
			},
		},
	}

	var buf bytes.Buffer
	require.NoError(t, WriteTestRecords(testRecs, &buf))
	found, err := ReadTestRecords(&buf)
	require.NoError(t, err)
	require.Equal(t, testRecs, found)
}
