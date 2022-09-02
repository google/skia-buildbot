package sqltracestore

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// asChan create a channel supplied by the given slice of strings 's'.
func asChan(s []traceIDForSQL) <-chan traceIDForSQL {
	ret := make(chan traceIDForSQL)
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
func fromChan(ch <-chan traceIDForSQL) []traceIDForSQL {
	ret := []traceIDForSQL{}
	for v := range ch {
		ret = append(ret, v)
	}
	return ret
}

func TestIntersect2Cancel(t *testing.T) {
	a := make(chan traceIDForSQL)
	b := make(chan traceIDForSQL)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		s := fromChan(newIntersect2(ctx, a, b))
		assert.Equal(t, []traceIDForSQL{}, s)
		wg.Done()
	}()
	cancel()
	wg.Wait()
	// The test passes by not timing out.
}

func TestIntersect2(t *testing.T) {
	testCases := []struct {
		a    []traceIDForSQL
		b    []traceIDForSQL
		exp  []traceIDForSQL
		desc string
	}{
		{
			a:    []traceIDForSQL{},
			b:    []traceIDForSQL{"2", "4", "6", "8"},
			exp:  []traceIDForSQL{},
			desc: "One empy",
		},
		{
			a:    []traceIDForSQL{},
			b:    []traceIDForSQL{},
			exp:  []traceIDForSQL{},
			desc: "Both empy",
		},
		{
			a:    []traceIDForSQL{"1", "2", "5", "7"},
			b:    []traceIDForSQL{"2", "4", "6", "8"},
			exp:  []traceIDForSQL{"2"},
			desc: "Same length",
		},
		{
			a:    []traceIDForSQL{"1", "2", "3"},
			b:    []traceIDForSQL{"1", "2", "3", "4"},
			exp:  []traceIDForSQL{"1", "2", "3"},
			desc: "Most match",
		},
		{
			a:    []traceIDForSQL{"1", "2", "3"},
			b:    []traceIDForSQL{"1", "2", "3"},
			exp:  []traceIDForSQL{"1", "2", "3"},
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
		inputs [][]traceIDForSQL
		exp    []traceIDForSQL
		desc   string
	}{
		{
			inputs: [][]traceIDForSQL{
				{},
				{"2", "4", "6", "8"},
			},
			exp:  []traceIDForSQL{},
			desc: "One empy",
		},
		{
			inputs: [][]traceIDForSQL{
				{"2", "4", "6", "8"},
			},
			exp:  []traceIDForSQL{"2", "4", "6", "8"},
			desc: "One",
		},
		{
			inputs: [][]traceIDForSQL{
				{"1", "2", "5"},
				{"2", "5", "7"},
				{"4", "5", "8"},
			},
			exp:  []traceIDForSQL{"5"},
			desc: "Three",
		},
		{
			inputs: [][]traceIDForSQL{
				{"1", "2", "3", "4"},
				{"1", "2", "3", "5"},
				{"2", "3", "4", "5"},
				{"1", "2", "3", "6"},
			},
			exp:  []traceIDForSQL{"2", "3"},
			desc: "Four",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			inputs := make([]<-chan traceIDForSQL, len(tc.inputs))
			for i, arr := range tc.inputs {
				inputs[i] = asChan(arr)
			}
			s := fromChan(newIntersect(context.Background(), inputs))
			assert.Equal(t, s, tc.exp)
		})
	}
}
