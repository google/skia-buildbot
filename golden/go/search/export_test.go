package search

import (
	"bytes"
	"fmt"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
)

func TestWriteReadExport(t *testing.T) {
	testutils.SmallTest(t)
	testRecs := []*ExportTestRecord{
		{
			TestName: "test-1",
			Digests: []*ExportDigestInfo{
				{
					SRDigest: &SRDigest{
						Digest: "abc-efg",
					},
					URL: fmt.Sprintf(urlTemplate, "https://example.com", "abc-efg"),
				},
			},
		},
	}

	var buf bytes.Buffer
	assert.NoError(t, WriteExportTestRecords(testRecs, &buf))
	found, err := ReadExportTestRecords(&buf)
	assert.NoError(t, err)
	assert.Equal(t, testRecs, found)
}
