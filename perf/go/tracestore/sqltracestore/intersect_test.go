package sqltracestore

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/perf/go/types"
)

// asChan create a channel supplied by the given slice of strings 's'.
func asChan(s []types.TraceIDForSQL) <-chan types.TraceIDForSQL {
	ret := make(chan types.TraceIDForSQL)
	if len(s) == 0 {
		close(ret)
		return ret
	}
	go func() {
		for _, v := range s {
			ret <- v
		}
		close(ret)
	}()
	return ret
}

// fromChan returns a slice of all the strings produced by the channel 'ch'.
func fromChan(ch <-chan types.TraceIDForSQL) []types.TraceIDForSQL {
	ret := []types.TraceIDForSQL{}
	for v := range ch {
		ret = append(ret, v)
	}
	return ret
}

func TestIntersect2Cancel(t *testing.T) {
	a := make(chan types.TraceIDForSQL)
	b := make(chan types.TraceIDForSQL)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		s := fromChan(newIntersect2(ctx, a, b))
		assert.Equal(t, []types.TraceIDForSQL{}, s)
		wg.Done()
	}()
	cancel()
	wg.Wait()
	// The test passes by not timing out.
}

func TestIntersect2(t *testing.T) {
	testCases := []struct {
		a    []types.TraceIDForSQL
		b    []types.TraceIDForSQL
		exp  []types.TraceIDForSQL
		desc string
	}{
		{
			a:    []types.TraceIDForSQL{},
			b:    []types.TraceIDForSQL{"2", "4", "6", "8"},
			exp:  []types.TraceIDForSQL{},
			desc: "One empy",
		},
		{
			a:    []types.TraceIDForSQL{},
			b:    []types.TraceIDForSQL{},
			exp:  []types.TraceIDForSQL{},
			desc: "Both empy",
		},
		{
			a:    []types.TraceIDForSQL{"1", "2", "5", "7"},
			b:    []types.TraceIDForSQL{"2", "4", "6", "8"},
			exp:  []types.TraceIDForSQL{"2"},
			desc: "Same length",
		},
		{
			a:    []types.TraceIDForSQL{"1", "2", "3"},
			b:    []types.TraceIDForSQL{"1", "2", "3", "4"},
			exp:  []types.TraceIDForSQL{"1", "2", "3"},
			desc: "Most match",
		},
		{
			a:    []types.TraceIDForSQL{"1", "2", "3"},
			b:    []types.TraceIDForSQL{"1", "2", "3"},
			exp:  []types.TraceIDForSQL{"1", "2", "3"},
			desc: "All match",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			s := fromChan(newIntersect2(context.Background(), asChan(tc.a), asChan(tc.b)))
			assert.Equal(t, s, tc.exp)
		})
	}
}

func TestIntersect(t *testing.T) {
	testCases := []struct {
		inputs [][]types.TraceIDForSQL
		exp    []types.TraceIDForSQL
		desc   string
	}{
		{
			inputs: [][]types.TraceIDForSQL{
				{},
				{"2", "4", "6", "8"},
			},
			exp:  []types.TraceIDForSQL{},
			desc: "One empy",
		},
		{
			inputs: [][]types.TraceIDForSQL{
				{"2", "4", "6", "8"},
			},
			exp:  []types.TraceIDForSQL{"2", "4", "6", "8"},
			desc: "One",
		},
		{
			inputs: [][]types.TraceIDForSQL{
				{"1", "2", "5"},
				{"2", "5", "7"},
				{"4", "5", "8"},
			},
			exp:  []types.TraceIDForSQL{"5"},
			desc: "Three",
		},
		{
			inputs: [][]types.TraceIDForSQL{
				{"1", "2", "3", "4"},
				{"1", "2", "3", "5"},
				{"2", "3", "4", "5"},
				{"1", "2", "3", "6"},
			},
			exp:  []types.TraceIDForSQL{"2", "3"},
			desc: "Four",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			inputs := make([]<-chan types.TraceIDForSQL, len(tc.inputs))
			for i, arr := range tc.inputs {
				inputs[i] = asChan(arr)
			}
			s := fromChan(newIntersect(context.Background(), inputs))
			assert.Equal(t, s, tc.exp)
		})
	}
}
