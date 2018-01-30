package dstilestore

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils"
)

func TestTrace(t *testing.T) {
	testutils.SmallTest(t)

	d := NewDSTileStore(context.Background(), ds.DS)

	keys := paramtools.Params{
		"arch":          "arm",
		"compiler":      "Clang",
		"config":        "vk",
		"configuration": "Debug",
		"cpu_or_gpu":    "GPU",
	}
	options := paramtools.Params{
		"gamma_correct": "no",
	}
	traceID := "arm:Clang:vk:Debug:GPU:Adreno418:Android_Vulkan:Nexus5x:circular_arcs_weird:Android:gm"

	trace, params, err := d.newTrace(101, keys, options, traceID)
	assert.NoError(t, err)
	assert.Equal(t, make([]int, TILE_SIZE), trace.Offsets)
	assert.True(t, strings.HasSuffix(trace.TileShard, "-2"), trace.TileShard)
	assert.Equal(t, traceID, params.TraceID)
	assert.NotEqual(t, "", params.Keys)
	assert.NotEqual(t, "", params.Options)

	assert.Equal(t, trace.TileShard, params.TileShard)
	assert.Equal(t, []int{0, 0, 0, 0}, trace.Offsets[:4])

	dig := NewDigests()
	digestIndex := dig.Add("123")
	trace.Add(1, digestIndex)
	assert.Equal(t, []string{"", "123"}, dig.Digests)
	assert.Equal(t, []int{0, 1, 0, 0}, trace.Offsets[:4])

	digestIndex = dig.Add("123")
	trace.Add(3, digestIndex)
	assert.Equal(t, []string{"", "123"}, dig.Digests)
	assert.Equal(t, []int{0, 1, 0, 1}, trace.Offsets[:4])

	digestIndex = dig.Add("456")
	trace.Add(2, digestIndex)
	assert.Equal(t, []string{"", "123", "456"}, dig.Digests)
	assert.Equal(t, []int{0, 1, 2, 1}, trace.Offsets[:4])

	digestIndex = dig.Add("789")
	trace.Add(0, digestIndex)
	assert.Equal(t, []string{"", "123", "456", "789"}, dig.Digests)
	assert.Equal(t, []int{3, 1, 2, 1}, trace.Offsets[:4])
}

func TestAdd(t *testing.T) {
	testutils.LargeTest(t)

	cleanup := testutil.InitDatastore(t,
		ds.TRACE_GOLD,
		ds.PARAMS_GOLD,
		ds.DIGESTS_GOLD,
	)
	defer cleanup()
	/*

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
	*/
}
