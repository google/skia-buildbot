package dataframe

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vcsinfo"
)

func TestFromIndexCommit(t *testing.T) {
	testutils.SmallTest(t)

	ts0 = time.Unix(1406721642, 0).UTC()
	ts1 = time.Unix(1406721715, 0).UTC()

	commits = []*vcsinfo.IndexCommit{
		{
			Hash:      "7a669cfa3f4cd3482a4fd03989f75efcc7595f7f",
			Index:     0,
			Timestamp: ts0,
		},
		{
			Hash:      "8652a6df7dc8a7e6addee49f6ed3c2308e36bd18",
			Index:     1,
			Timestamp: ts1,
		},
	}
	expected_headers := []*ColumnHeader{
		{
			Source:    "master",
			Offset:    0,
			Timestamp: ts0.Unix(),
		},
		{
			Source:    "master",
			Offset:    1,
			Timestamp: ts1.Unix(),
		},
	}
	expected_indices := []int32{0, 1}

	headers, pcommits, _ := fromIndexCommit(commits, 0)
	assert.Equal(t, 2, len(headers))
	assert.Equal(t, 2, len(pcommits))
	deepequal.AssertDeepEqual(t, expected_headers, headers)
	deepequal.AssertDeepEqual(t, expected_indices, pcommits)

	headers, pcommits, _ = fromIndexCommit([]*vcsinfo.IndexCommit{}, 0)
	assert.Equal(t, 0, len(headers))
	assert.Equal(t, 0, len(pcommits))
}
