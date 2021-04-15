package dataframe

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/config"
	perfgit "go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/git/gittest"
	"go.skia.org/infra/perf/go/progress"
	"go.skia.org/infra/perf/go/types"
)

func TestMerge(t *testing.T) {
	unittest.SmallTest(t)
	// Simple
	a := []*ColumnHeader{
		{Offset: 1},
		{Offset: 2},
		{Offset: 4},
	}
	b := []*ColumnHeader{
		{Offset: 3},
		{Offset: 4},
	}
	m, aMap, bMap := merge(a, b)
	expected := []*ColumnHeader{
		{Offset: 1},
		{Offset: 2},
		{Offset: 3},
		{Offset: 4},
	}
	assert.Equal(t, m, expected)
	assert.Equal(t, map[int]int{0: 0, 1: 1, 2: 3}, aMap)
	assert.Equal(t, map[int]int{0: 2, 1: 3}, bMap)

	// Skips
	a = []*ColumnHeader{
		{Offset: 1},
		{Offset: 2},
		{Offset: 4},
	}
	b = []*ColumnHeader{
		{Offset: 5},
		{Offset: 7},
	}
	m, aMap, bMap = merge(a, b)
	expected = []*ColumnHeader{
		{Offset: 1},
		{Offset: 2},
		{Offset: 4},
		{Offset: 5},
		{Offset: 7},
	}
	assert.Equal(t, m, expected)
	assert.Equal(t, map[int]int{0: 0, 1: 1, 2: 2}, aMap)
	assert.Equal(t, map[int]int{0: 3, 1: 4}, bMap)

	// Empty b
	a = []*ColumnHeader{
		{Offset: 1},
		{Offset: 2},
		{Offset: 4},
	}
	b = []*ColumnHeader{}
	m, aMap, bMap = merge(a, b)
	expected = []*ColumnHeader{
		{Offset: 1},
		{Offset: 2},
		{Offset: 4},
	}
	assert.Equal(t, m, expected)
	assert.Equal(t, map[int]int{0: 0, 1: 1, 2: 2}, aMap)
	assert.Equal(t, map[int]int{}, bMap)

	// Empty a
	a = []*ColumnHeader{}
	b = []*ColumnHeader{
		{Offset: 1},
		{Offset: 2},
		{Offset: 4},
	}
	m, aMap, bMap = merge(a, b)
	expected = []*ColumnHeader{
		{Offset: 1},
		{Offset: 2},
		{Offset: 4},
	}
	assert.Equal(t, m, expected)
	assert.Equal(t, map[int]int{}, aMap)
	assert.Equal(t, map[int]int{0: 0, 1: 1, 2: 2}, bMap)

	// Empty a and b.
	a = []*ColumnHeader{}
	b = []*ColumnHeader{}
	m, aMap, bMap = merge(a, b)
	expected = []*ColumnHeader{}
	assert.Equal(t, m, expected)
	assert.Equal(t, map[int]int{}, aMap)
	assert.Equal(t, map[int]int{}, bMap)
}

func TestDFAppend(t *testing.T) {
	unittest.SmallTest(t)
	a := DataFrame{
		Header: []*ColumnHeader{
			{Offset: 1},
			{Offset: 2},
			{Offset: 4},
		},
		TraceSet: types.TraceSet{
			",config=8888,arch=x86,": []float32{0.1, 0.2, 0.4},
			",config=8888,arch=arm,": []float32{1.1, 1.2, 1.4},
		},
	}
	b := DataFrame{
		Header: []*ColumnHeader{
			{Offset: 3},
			{Offset: 4},
		},
		TraceSet: types.TraceSet{
			",config=565,arch=x86,": []float32{3.3, 3.4},
			",config=565,arch=arm,": []float32{4.3, 4.4},
		},
	}
	a.BuildParamSet()
	b.BuildParamSet()
	r := Join(&a, &b)

	expectedHeader := []*ColumnHeader{
		{Offset: 1},
		{Offset: 2},
		{Offset: 3},
		{Offset: 4},
	}

	assert.Equal(t, expectedHeader, r.Header)
	assert.Len(t, r.TraceSet, 4)
	e := vec32.MissingDataSentinel
	assert.Equal(t, types.Trace{0.1, 0.2, e, 0.4}, r.TraceSet[",config=8888,arch=x86,"])
	assert.Equal(t, types.Trace{1.1, 1.2, e, 1.4}, r.TraceSet[",config=8888,arch=arm,"])
	assert.Equal(t, types.Trace{e, e, 4.3, 4.4}, r.TraceSet[",config=565,arch=arm,"])
	assert.Equal(t, types.Trace{e, e, 3.3, 3.4}, r.TraceSet[",config=565,arch=x86,"])
}

func TestGetSkps_Success(t *testing.T) {
	unittest.LargeTest(t)
	ctx, db, _, _, instanceConfig, cleanup := gittest.NewForTest(t)
	defer cleanup()
	g, err := perfgit.New(ctx, true, db, instanceConfig)
	require.NoError(t, err)

	instanceConfig.GitRepoConfig.FileChangeMarker = "bar.txt"
	config.Config = instanceConfig

	skps, err := getSkps(ctx, []*ColumnHeader{
		{
			Offset: 0,
		},
		{
			Offset: 7,
		},
	}, g)
	require.NoError(t, err)
	assert.Equal(t, []int{3, 6}, skps)
}

func TestGetSkps_SuccessIfFileChangeMarkerNotSet(t *testing.T) {
	unittest.LargeTest(t)
	ctx, db, _, _, instanceConfig, cleanup := gittest.NewForTest(t)
	defer cleanup()
	g, err := perfgit.New(ctx, true, db, instanceConfig)
	require.NoError(t, err)

	instanceConfig.GitRepoConfig.FileChangeMarker = ""
	config.Config = instanceConfig

	skps, err := getSkps(ctx, []*ColumnHeader{
		{
			Offset: 0,
		},
		{
			Offset: 7,
		},
	}, g)
	require.NoError(t, err)
	assert.Empty(t, skps)
}

func TestGetSkps_ErrOnBadCommitNumber(t *testing.T) {
	unittest.LargeTest(t)
	ctx, db, _, _, instanceConfig, cleanup := gittest.NewForTest(t)
	defer cleanup()
	g, err := perfgit.New(ctx, true, db, instanceConfig)
	require.NoError(t, err)

	instanceConfig.GitRepoConfig.FileChangeMarker = "bar.txt"
	config.Config = instanceConfig

	_, err = getSkps(ctx, []*ColumnHeader{
		{
			Offset: -3,
		},
		{
			Offset: -1,
		},
	}, g)
	require.Error(t, err)
}

func TestProcessFrameRequest(t *testing.T) {
	unittest.SmallTest(t)

	fr := &FrameRequest{
		Queries:  []string{"http://[::1]a"}, // A known query that will fail to parse.
		Progress: progress.New(),
	}
	err := ProcessFrameRequest(context.Background(), fr, nil, nil, nil)
	require.Error(t, err)
	var b bytes.Buffer
	err = fr.Progress.JSON(&b)
	require.NoError(t, err)
	assert.Equal(t, "{\"status\":\"Running\",\"messages\":[],\"url\":\"\"}\n", b.String())
}
