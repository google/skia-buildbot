package dstilestore

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/golden/go/types"
)

func TestTrace(t *testing.T) {
	testutils.SmallTest(t)

	trace, err := NewTrace(0, nil)
	assert.NoError(t, err)
	assert.Equal(t, []string{}, trace.Digests)
	assert.Equal(t, []int{0, 0, 0, 0}, trace.Trace[:4])

	trace.Add("123", 0)
	assert.Equal(t, []string{"123"}, trace.Digests)
	assert.Equal(t, []int{1, 0, 0, 0}, trace.Trace[:4])

	trace.Add("123", 2)
	assert.Equal(t, []string{"123"}, trace.Digests)
	assert.Equal(t, []int{1, 0, 1, 0}, trace.Trace[:4])

	trace.Add("456", 1)
	assert.Equal(t, []string{"123", "456"}, trace.Digests)
	assert.Equal(t, []int{1, 2, 1, 0}, trace.Trace[:4])

	trace.Add("789", 53)
	assert.Equal(t, []string{"123", "456", "789"}, trace.Digests)
	assert.Equal(t, []int{1, 2, 1, 3}, trace.Trace[:4])

	gt := trace.AsGoldenTrace(",arch=x86,config=8888,")
	assert.Equal(t, map[string]string{"arch": "x86", "config": "8888"}, gt.Params_)
	assert.Equal(t, []string{"123", "456", "123", "789", ""}, gt.Values[:5])
}

func TestAdd(t *testing.T) {
	testutils.LargeTest(t)

	cleanup := testutil.InitDatastore(t,
		ds.TRACE)
	defer cleanup()

	d := NewDSTileStore(context.Background(), ds.DS)

	ctx := context.Background()
	entries := map[string]*types.ParsedIngestionEntry{
		"arm:GCC:8888:shadermaskfilter_image": &types.ParsedIngestionEntry{
			Name:   "shadermaskfilter_image",
			Digest: "123",
		},
		"x86:GCC:8888:shadermaskfilter_image": &types.ParsedIngestionEntry{
			Name:   "shadermaskfilter_image",
			Digest: "456",
		},
		"x86:GCC:8888:oval": &types.ParsedIngestionEntry{
			Name:   "oval",
			Digest: "789",
		},
	}
	err := d.Add(102, entries)
	assert.NoError(t, err)

	time.Sleep(1 * time.Second)

	var trace Trace
	key := traceKey(102, "arm:GCC:8888:shadermaskfilter_image")
	err = ds.DS.Get(ctx, key, &trace)
	assert.NoError(t, err)

	key = traceKey(102, "x86:GCC:8888:shadermaskfilter_image")
	err = ds.DS.Get(ctx, key, &trace)
	assert.NoError(t, err)

	key = traceKey(102, "some unknown key")
	err = ds.DS.Get(ctx, key, &trace)
	assert.Error(t, err)

	q := ds.NewQuery(ds.TRACE)
	traces := []*Trace{}
	_, err = ds.DS.GetAll(ctx, q, &traces)
	assert.NoError(t, err)
	assert.Len(t, traces, 3)

	entries = map[string]*types.ParsedIngestionEntry{
		"x86:GCC:8888:oval": &types.ParsedIngestionEntry{
			Name:   "oval",
			Digest: "789",
		},
	}
	err = d.Add(102, entries)
	assert.NoError(t, err)

	time.Sleep(1 * time.Second)

	traces = []*Trace{}
	_, err = ds.DS.GetAll(ctx, q, &traces)
	assert.NoError(t, err)
	assert.Len(t, traces, 3)
}
