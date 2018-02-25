package dstilestore

import (
	"testing"
)

func TestTrace(t *testing.T) {
	/*
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
	*/
}

func TestAdd(t *testing.T) {
	/*
		testutils.LargeTest(t)

		cleanup := testutil.InitDatastore(t,
			ds.TRACE_GOLD,
			ds.PARAMS_GOLD,
			ds.DIGESTS_GOLD,
		)
		defer cleanup()

		ctx := context.Background()
		d := NewDSTileStore(ctx, ds.DS)

		entries := []*types.ParsedIngestionEntry{
			&types.ParsedIngestionEntry{
				TraceID: "arm:GCC:8888:shadermaskfilter_image",
				Keys:    paramtools.Params{"name": "shadermaskfilter_image"},
				Digest:  "123",
			},
			&types.ParsedIngestionEntry{
				TraceID: "x86:GCC:8888:shadermaskfilter_image",
				Keys:    paramtools.Params{"name": "shadermaskfilter_image"},
				Options: paramtools.Params{"gamma_correct": "no"},
				Digest:  "456",
			},
			&types.ParsedIngestionEntry{
				TraceID: "x86:GCC:8888:oval",
				Keys:    paramtools.Params{"name": "oval"},
				Digest:  "789",
			},
		}
		err := d.Add(102, entries)
		assert.NoError(t, err)

		time.Sleep(1 * time.Second)

		var dig Digests
		key := digestDSKey(102, "oval")
		err = ds.DS.Get(ctx, key, &dig)
		assert.NoError(t, err)
		assert.Equal(t, []string{"", "789"}, dig.Digests)

		key = digestDSKey(102, "shadermaskfilter_image")
		err = ds.DS.Get(ctx, key, &dig)
		assert.NoError(t, err)
		assert.Len(t, dig.Digests, 3)
		assert.True(t, util.In("123", dig.Digests))
		assert.True(t, util.In("456", dig.Digests))
		assert.True(t, util.In("", dig.Digests))

		var trace Trace
		var p Params

		// First test for a trace that doesn't exist.
		key = traceDSKey("some unknown key", 102)
		err = ds.DS.Get(ctx, key, &trace)
		assert.Error(t, err)

		// Then load the oval.
		key = traceDSKey("x86:GCC:8888:oval", 102)
		err = ds.DS.Get(ctx, key, &trace)
		assert.NoError(t, err)
		assert.True(t, strings.HasSuffix(trace.TileShard, "-2"))
		assert.Equal(t, []int{0, 0, 1, 0}, trace.Offsets[:4])

		paramsKey := ds.NewKey(ds.PARAMS_GOLD)
		paramsKey.Name = key.Name
		err = ds.DS.Get(ctx, paramsKey, &p)
		assert.NoError(t, err)
		assert.True(t, strings.HasSuffix(p.TileShard, "-2"))
		assert.Equal(t, "x86:GCC:8888:oval", p.TraceID)
		decodedParams := paramtools.Params{}
		err = json.Unmarshal([]byte(p.Keys), &decodedParams)
		assert.NoError(t, err)
		assert.Equal(t, "oval", decodedParams["name"])

		// Check the shadermaskfilter_image traces.
		key = traceDSKey("x86:GCC:8888:shadermaskfilter_image", 102)
		err = ds.DS.Get(ctx, key, &trace)
		assert.NoError(t, err)
		assert.True(t, strings.HasSuffix(trace.TileShard, "-2"))
		assert.True(t, trace.Offsets[2] == 1 || trace.Offsets[2] == 2)

		var otherTrace Trace
		key = traceDSKey("arm:GCC:8888:shadermaskfilter_image", 102)
		err = ds.DS.Get(ctx, key, &otherTrace)
		assert.NoError(t, err)
		assert.True(t, strings.HasSuffix(trace.TileShard, "-2"))
		assert.True(t, otherTrace.Offsets[2] == 1 || otherTrace.Offsets[2] == 2)
		assert.True(t, trace.Offsets[2] != otherTrace.Offsets[2])

		// Test overwrite hash with new value.
		entries = []*types.ParsedIngestionEntry{
			&types.ParsedIngestionEntry{
				TraceID: "x86:GCC:8888:oval",
				Keys:    paramtools.Params{"name": "oval"},
				Digest:  "abc",
			},
		}
		err = d.Add(102, entries)
		assert.NoError(t, err)

		time.Sleep(1 * time.Second)

		key = digestDSKey(102, "oval")
		err = ds.DS.Get(ctx, key, &dig)
		assert.NoError(t, err)
		assert.Equal(t, []string{"", "789", "abc"}, dig.Digests)

		key = traceDSKey("x86:GCC:8888:oval", 102)
		err = ds.DS.Get(ctx, key, &trace)
		assert.NoError(t, err)
		assert.True(t, strings.HasSuffix(trace.TileShard, "-2"))
		assert.Equal(t, []int{0, 0, 2, 0}, trace.Offsets[:4])

		// Test add second digest in a trace.
		entries = []*types.ParsedIngestionEntry{
			&types.ParsedIngestionEntry{
				TraceID: "x86:GCC:8888:oval",
				Keys:    paramtools.Params{"name": "oval"},
				Digest:  "789",
			},
		}
		err = d.Add(101, entries)
		assert.NoError(t, err)

		time.Sleep(1 * time.Second)

		key = digestDSKey(101, "oval")
		err = ds.DS.Get(ctx, key, &dig)
		assert.NoError(t, err)
		assert.Equal(t, []string{"", "789", "abc"}, dig.Digests)

		key = traceDSKey("x86:GCC:8888:oval", 101)
		err = ds.DS.Get(ctx, key, &trace)
		assert.NoError(t, err)
		assert.True(t, strings.HasSuffix(trace.TileShard, "-2"))
		assert.Equal(t, []int{0, 1, 2, 0}, trace.Offsets[:4])
	*/
}
