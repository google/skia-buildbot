package types

import (
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/tiling"
)

func TestGoldenTrace(t *testing.T) {
	unittest.SmallTest(t)
	N := 5
	// Test NewGoldenTrace.
	g := NewEmptyGoldenTrace(N, nil)
	assert.Equal(t, N, g.Len(), "wrong digests size")
	assert.Equal(t, 0, len(g.Keys), "wrong keys initial size")
	g.Digests[0] = "a digest"

	assert.True(t, g.IsMissing(1), "values start missing")
	assert.False(t, g.IsMissing(0), "set values shouldn't be missing")

	// Test Merge.
	M := 7
	gm := NewEmptyGoldenTrace(M, nil)
	gm.Digests[1] = "another digest"
	g2 := g.Merge(gm)
	assert.Equal(t, N+M, g2.Len(), "merge length wrong")
	assert.Equal(t, Digest("a digest"), g2.(*GoldenTrace).Digests[0])
	assert.Equal(t, Digest("another digest"), g2.(*GoldenTrace).Digests[6])

	// Test Grow.
	g = NewEmptyGoldenTrace(N, nil)
	g.Digests[0] = "foo"
	g.Grow(2*N, tiling.FILL_BEFORE)
	assert.Equal(t, Digest("foo"), g.Digests[N], "Grow didn't FILL_BEFORE correctly")

	g = NewEmptyGoldenTrace(N, nil)
	g.Digests[0] = "foo"
	g.Grow(2*N, tiling.FILL_AFTER)
	assert.Equal(t, Digest("foo"), g.Digests[0], "Grow didn't FILL_AFTER correctly")

	// Test Trim
	g = NewEmptyGoldenTrace(N, nil)
	g.Digests[1] = "foo"
	require.NoError(t, g.Trim(1, 3))
	assert.Equal(t, Digest("foo"), g.Digests[0], "Trim didn't copy correctly")
	assert.Equal(t, 2, g.Len(), "Trim wrong length")

	assert.Error(t, g.Trim(-1, 1))
	assert.Error(t, g.Trim(1, 3))
	assert.Error(t, g.Trim(2, 1))

	require.NoError(t, g.Trim(1, 1))
	assert.Equal(t, 0, g.Len(), "final size wrong")
}

func TestSetAt(t *testing.T) {
	unittest.SmallTest(t)
	testCases := []struct {
		want Digest
	}{
		{
			want: "",
		},
		{
			want: "abcd",
		},
		{
			want: MISSING_DIGEST,
		},
	}
	tr := NewEmptyGoldenTrace(len(testCases), nil)
	for i, tc := range testCases {
		require.NoError(t, tr.SetAt(i, []byte(tc.want)))
	}
	for i, tc := range testCases {
		assert.Equal(t, tc.want, tr.Digests[i], "Bad at test case %d", i)
	}
}

var _tn TestName

// BenchmarkTraceTestName shows that a map-lookup in go for this example param map is about
// 15 nanoseconds, whereas pre-caching that value makes it about 0.5 ns.
func BenchmarkTraceTestName(b *testing.B) {
	// This is a typical paramset for a Skia trace, grabbed at random from the live data.
	gt := NewEmptyGoldenTrace(10, map[string]string{
		"alpha_type":       "Premul",
		"arch":             "arm64",
		"color_depth":      "8888",
		"color_type":       "RGBA_8888",
		"compiler":         "Clang",
		"config":           "ddl-vk",
		"configuration":    "Debug",
		"cpu_or_gpu":       "GPU",
		"cpu_or_gpu_value": "Adreno540",
		"ext":              "png",
		"extra_config":     "Android_DDL1_Vulkan",
		"gamut":            "untagged",
		"model":            "Pixel2XL",
		"name":             "blurredclippedcircle",
		"os":               "Android",
		"source_type":      "gm",
		"style":            "DDL",
		"transfer_fn":      "untagged",
	})

	var r TestName
	for n := 0; n < b.N; n++ {
		// always record the result of TestName to prevent
		// the compiler eliminating the function call.
		r = gt.TestName()
	}
	// always store the result to a package level variable
	// so the compiler cannot eliminate the Benchmark itself.
	_tn = r
}

// BenchmarkTraceMapIteration shows that iterating through a map of 1.3 million traces
// takes about 21 milliseconds.
func BenchmarkTraceMapIteration(b *testing.B) {
	// This is a typical size of a Skia tile, 1.3 million traces.
	const numTraces = 1300000
	// When we make the traces in bt_tracestore, we don't know how big they can be, so
	// we just start from an empty map
	traces := map[tiling.TraceId]tiling.Trace{}
	for i := 0; i < numTraces; i++ {
		id := randomString()
		traces[tiling.TraceId(id)] = NewEmptyGoldenTrace(10, map[string]string{
			"alpha_type": "Premul",
			"arch":       "arm64",
			"name":       id,
		})
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		for id, tr := range traces {
			if id == "blerg" { // should never happen
				fmt.Println(tr)
			}
		}
	}
}

// BenchmarkTraceSliceIteration shows that iterating through a slice of 1.3 million traces
// takes about 5 milliseconds.
func BenchmarkTraceSliceIteration(b *testing.B) {
	// This is a typical size of a Skia tile, 1.3 million traces.
	const numTraces = 1300000
	// When we make the traces in bt_tracestore, we wouldn't know how big they can be, so
	// we just start from an empty slice
	traces := []TracePair{}
	for i := 0; i < numTraces; i++ {
		id := randomString()
		traces = append(traces, TracePair{
			ID: tiling.TraceId(id),
			Trace: NewEmptyGoldenTrace(10, map[string]string{
				"alpha_type": "Premul",
				"arch":       "arm64",
				"name":       id,
			}),
		})
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		for _, tp := range traces {
			id, tr := tp.ID, tp.Trace
			if id == "blerg" { // should never happen
				fmt.Println(tr)
			}
		}
	}
}

const stringSize = 64

func randomString() string {
	b := make([]byte, stringSize)
	_, _ = rand.Read(b)
	return string(b)
}
