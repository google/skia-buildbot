package db

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/trace/db/perftypes"
	"go.skia.org/infra/go/util"
)

func TestAdd(t *testing.T) {
	testutils.MediumTest(t)
	ts, cleanup := setupClientServerForTesting(t.Fatalf)
	defer cleanup()

	now := time.Unix(100, 0)

	commitIDs := []*CommitID{
		{
			Timestamp: now.Unix(),
			ID:        "abc123",
			Source:    "master",
		},
		{
			Timestamp: now.Add(time.Hour).Unix(),
			ID:        "xyz789",
			Source:    "master",
		},
	}

	entries := map[tiling.TraceId]*Entry{
		"key:8888:android": {
			Params: map[string]string{
				"config":   "8888",
				"platform": "android",
				"type":     "skp",
			},
			Value: perftypes.BytesFromFloat64(0.01),
		},
		"key:gpu:win8": {
			Params: map[string]string{
				"config":   "gpu",
				"platform": "win8",
				"type":     "skp",
			},
			Value: perftypes.BytesFromFloat64(1.234),
		},
	}

	err := ts.Add(commitIDs[0], entries)

	assert.NoError(t, err)
	tile, hashes, err := ts.TileFromCommits(commitIDs)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(tile.Traces))
	assert.Equal(t, 2, len(tile.Commits))
	assert.Equal(t, 2, len(hashes))
	assert.True(t, util.In("d41d8cd98f00b204e9800998ecf8427e", hashes))
	assert.NotEqual(t, hashes[0], hashes[1])

	tr := tile.Traces["key:8888:android"].(*perftypes.PerfTrace)
	assert.Equal(t, 0.01, tr.Values[0])
	assert.True(t, tr.IsMissing(1))
	assert.Equal(t, "8888", tr.Params()["config"])

	tr = tile.Traces["key:gpu:win8"].(*perftypes.PerfTrace)
	assert.Equal(t, 1.234, tr.Values[0])
	assert.True(t, tr.IsMissing(1))

	assert.Equal(t, "abc123", tile.Commits[0].Hash)
	assert.Equal(t, "xyz789", tile.Commits[1].Hash)

	foundCommits, err := ts.List(now, now.Add(time.Hour))
	assert.NoError(t, err)
	assert.Equal(t, 1, len(foundCommits))
}
