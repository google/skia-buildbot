package isolate_cache

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.chromium.org/luci/common/isolated"
	"go.skia.org/infra/go/atomic_miss_cache"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/task_scheduler/go/types"
)

func TestIsolateCache(t *testing.T) {
	unittest.LargeTest(t)

	btProject, btInstance, btCleanup := SetupBigTable(t)
	defer btCleanup()

	ctx := context.Background()

	// Compare results of caches with and without backing caches.
	c1, err := New(ctx, btProject, btInstance, nil)
	require.NoError(t, err)
	defer testutils.AssertCloses(t, c1)
	c2 := &Cache{
		cache: atomic_miss_cache.New(nil),
	}

	check := func(rs types.RepoState, isolateFile string, expect *isolated.Isolated, expectErr error) {
		if1, err1 := c1.Get(ctx, rs, isolateFile)
		if2, err2 := c2.Get(ctx, rs, isolateFile)
		require.Equal(t, expectErr, err1)
		require.Equal(t, expectErr, err2)
		require.Equal(t, expect, if1)
		require.Equal(t, expect, if2)
	}

	// The entry does not exist.
	rs1 := types.RepoState{
		Repo:     "fake.git",
		Revision: "abc123",
	}
	i1 := "fake.isolate"
	check(rs1, i1, nil, atomic_miss_cache.ErrNoSuchEntry)

	// Set if unset.
	callCount := 0
	link := "link"
	mode := 777
	size := int64(9000)
	if1 := &isolated.Isolated{
		Algo: "smrt",
		Files: map[string]isolated.File{
			"myfile": {
				Digest: "abc123",
				Link:   &link,
				Mode:   &mode,
				Size:   &size,
				Type:   isolated.Basic,
			},
		},
		Includes: []isolated.HexDigest{"def456"},
		Version:  "NEW!",
	}
	fn := func(ctx context.Context) (*CachedValue, error) {
		callCount++
		return &CachedValue{
			Isolated: map[string]*isolated.Isolated{
				i1: if1,
			},
		}, nil
	}
	require.NoError(t, c1.SetIfUnset(ctx, rs1, fn))
	require.NoError(t, c2.SetIfUnset(ctx, rs1, fn))
	require.Equal(t, 2, callCount)
	check(rs1, i1, if1, nil)

	// Bring up another cache backed by the same BigTable table.
	c3, err := New(ctx, btProject, btInstance, nil)
	require.NoError(t, nil)
	defer testutils.AssertCloses(t, c3)

	check2 := func(rs types.RepoState, isolateFile string, expect *isolated.Isolated, expectErr error) {
		check(rs, isolateFile, expect, expectErr)
		if3, err3 := c3.Get(ctx, rs, isolateFile)
		require.Equal(t, expectErr, err3)
		require.Equal(t, expect, if3)
	}
	check2(rs1, i1, if1, nil)

	// Check for stored errors.
	rs2 := types.RepoState{
		Repo:     "fake.git",
		Revision: "def456",
	}
	require.NoError(t, c1.SetIfUnset(ctx, rs2, func(ctx context.Context) (*CachedValue, error) {
		return &CachedValue{
			Error: "failed to process isolate",
		}, nil
	}))
	_, err = c1.Get(ctx, rs2, i1)
	require.EqualError(t, err, "failed to process isolate")
	_, err = c2.Get(ctx, rs2, i1)
	require.EqualError(t, err, atomic_miss_cache.ErrNoSuchEntry.Error())
	_, err = c3.Get(ctx, rs2, i1)
	require.EqualError(t, err, "failed to process isolate")
}
