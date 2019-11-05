package keysubsetmap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestAddGet(t *testing.T) {
	unittest.SmallTest(t)
	// config: "8888", "sRGB"
	// gpu: "Adreno", "nVidia", "Radeon"
	// os: "Android", "Windows", "iOS"

	m := New()
	m.Add([]Key{
		{keyIdx: 0, valueIdx: 0},
		{keyIdx: 1, valueIdx: 0},
		{keyIdx: 2, valueIdx: 0},
	}, ",config=8888,gpu=Adreno,os=Android,")
	m.Add([]Key{
		{keyIdx: 0, valueIdx: 1},
		{keyIdx: 1, valueIdx: 0},
		{keyIdx: 2, valueIdx: 0},
	}, ",config=sRGB,gpu=Adreno,os=Android,")
	m.Add([]Key{
		{keyIdx: 0, valueIdx: 1},
		{keyIdx: 1, valueIdx: 1},
		{keyIdx: 2, valueIdx: 0},
	}, ",config=sRGB,gpu=nVidia,os=Android,")
	m.Add([]Key{
		{keyIdx: 0, valueIdx: 1},
		{keyIdx: 1, valueIdx: 1},
		{keyIdx: 2, valueIdx: 1},
	}, ",config=sRGB,gpu=nVidia,os=Windows,")

	v := m.Get(nil)
	require.Len(t, v, 4)

	v = m.Get([]Key{
		{keyIdx: 1, valueIdx: 1},
	})
	require.Len(t, v, 2)
	assert.Contains(t, v, Value(",config=sRGB,gpu=nVidia,os=Android,"))
	assert.Contains(t, v, Value(",config=sRGB,gpu=nVidia,os=Windows,"))

	v = m.Get([]Key{
		{keyIdx: -8, valueIdx: 1},
	})
	require.Empty(t, v)

	v = m.Get([]Key{
		{keyIdx: 0, valueIdx: 1},
		{keyIdx: 1, valueIdx: 1},
	})
	require.Len(t, v, 2)
	assert.Contains(t, v, Value(",config=sRGB,gpu=nVidia,os=Android,"))
	assert.Contains(t, v, Value(",config=sRGB,gpu=nVidia,os=Windows,"))

	v = m.Get([]Key{
		{keyIdx: 0, valueIdx: 1},
		{keyIdx: 2, valueIdx: 1},
	})
	require.Len(t, v, 1)
	assert.Contains(t, v, Value(",config=sRGB,gpu=nVidia,os=Windows,"))
}

var _benchm *Map

func BenchmarkAdd_N4000_M_6(b *testing.B) {
	var m *Map
	for n := 0; n < b.N; n++ {
		// always record the result of bench to prevent
		// the compiler eliminating the function call.
		m = bench(4000, 6)
	}
	// always store the result to a package level variable
	// so the compiler cannot eliminate the Benchmark itself.
	_benchm = m
}

func BenchmarkAdd_N4000_M_7(b *testing.B) {
	var m *Map
	for n := 0; n < b.N; n++ {
		// always record the result of bench to prevent
		// the compiler eliminating the function call.
		m = bench(4000, 7)
	}
	// always store the result to a package level variable
	// so the compiler cannot eliminate the Benchmark itself.
	_benchm = m
}

func BenchmarkAdd_N4000_M_8(b *testing.B) {
	var m *Map
	for n := 0; n < b.N; n++ {
		// always record the result of bench to prevent
		// the compiler eliminating the function call.
		m = bench(4000, 8)
	}
	// always store the result to a package level variable
	// so the compiler cannot eliminate the Benchmark itself.
	_benchm = m
}

const maxParams = 10
const fakeData = "this doesn't matter"

func bench(numTraces, numParams int) *Map {
	m := New()
	for i := 0; i < numTraces; i++ {
		var keys []Key
		for j := 0; j < numParams; j++ {
			keys = append(keys, Key{
				keyIdx:   j,
				valueIdx: (i + j) % maxParams,
			})
		}
		m.Add(keys, fakeData)
	}
	return m
}
