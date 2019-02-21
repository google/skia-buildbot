package isolate_cache

import (
	"context"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/isolate"
	"go.skia.org/infra/go/set_if_unset_cache"
	"go.skia.org/infra/task_scheduler/go/types"
)

func TestIsolateCache(t *testing.T) {
	testutils.LargeCache(t)

	btProject, btInstance, btCleanup := SetupBigTable(t)
	defer btCleanup()

	ctx := context.Background()

	// Compare results of caches with and without backing caches.
	c1, err := New(ctx, btProject, btInstance, nil)
	assert.NoError(t, err)
	c2 := &Cache{
		cache: set_if_unset_cache.New(nil),
	}

	check := func(rs types.RepoState, isolateFile string, expect *isolate.IsolatedFile, expectErr error) {
		if1, err1 := c1.Get(ctx, rs, isolateFile)
		if2, err2 := c2.Get(ctx, rs, isolateFile)
		assert.Equal(t, expectErr, err1)
		assert.Equal(t, expectErr, err2)
		assert.Equal(t, expect, if1)
		assert.Equal(t, expect, if2)
	}

	// The entry does not exist.
	rs1 := types.RepoState{
		Repo:     "fake.git",
		Revision: "abc123",
	}
	i1 := "fake.isolate"
	check(rs1, i1, nil, set_if_unset_cache.ErrNoSuchEntry)

	// Set if unset.
	callCount := 0
	if1 := &isolate.IsolatedFile{
		Algo:    "smrt",
		Command: []string{"sit", "stay"},
		Files: map[string]interface{}{
			"myfile": "blahblah",
		},
		Includes:    []string{"other"},
		RelativeCwd: "dot",
		Version:     "NEW!",
	}
	fn := func(ctx context.Context) (*CachedValue, error) {
		callCount++
		return &CachedValue{
			Isolated: map[string]*isolate.IsolatedFile{
				i1: if1,
			},
		}, nil
	}
	assert.NoError(t, c1.SetIfUnset(ctx, rs1, fn))
	assert.NoError(t, c2.SetIfUnset(ctx, rs1, fn))
	assert.Equal(t, 2, callCount)
	check(rs1, i1, if1, nil)

	// Bring up another cache backed by the same BigTable table.
	c3, err := New(ctx, btProject, btInstance, nil)
	assert.NoError(t, nil)

	check2 := func(rs types.RepoState, isolateFile string, expect *isolate.IsolatedFile, expectErr error) {
		check(rs, isolateFile, expect, expectErr)
		if3, err3 := c3.Get(ctx, rs, isolateFile)
		assert.Equal(t, expectErr, err3)
		assert.Equal(t, expect, if3)
	}
	check2(rs1, i1, if1, nil)

	// Check for stored errors.
	rs2 := types.RepoState{
		Repo:     "fake.git",
		Revision: "def456",
	}
	assert.NoError(t, c1.SetIfUnset(ctx, rs2, func(ctx context.Context) (*CachedValue, error) {
		return &CachedValue{
			Error: "failed to process isolate",
		}, nil
	}))
	_, err = c1.Get(ctx, rs2, i1)
	assert.EqualError(t, err, "failed to process isolate")
	_, err = c2.Get(ctx, rs2, i1)
	assert.EqualError(t, err, set_if_unset_cache.ErrNoSuchEntry.Error())
	_, err = c3.Get(ctx, rs2, i1)
	assert.EqualError(t, err, "failed to process isolate")
}
