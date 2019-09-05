package engine

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestUnion2Cancel(t *testing.T) {
	unittest.SmallTest(t)
	a := make(chan string)
	b := make(chan string)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		s := fromChan(newUnion2(ctx, a, b))
		assert.Equal(t, []string{}, s)
		wg.Done()
	}()
	cancel()
	wg.Wait()
	// The test passes by not timing out.
}

func TestUnion2(t *testing.T) {
	unittest.SmallTest(t)
	testCases := []struct {
		a    []string
		b    []string
		exp  []string
		desc string
	}{
		{
			a:    []string{},
			b:    []string{"2", "4", "6", "8"},
			exp:  []string{"2", "4", "6", "8"},
			desc: "One empy",
		},
		{
			a:    []string{},
			b:    []string{},
			exp:  []string{},
			desc: "Both empy",
		},
		{
			a:    []string{"1", "3", "5", "7"},
			b:    []string{"2", "4", "6", "8"},
			exp:  []string{"1", "2", "3", "4", "5", "6", "7", "8"},
			desc: "Same length",
		},
		{
			a:    []string{"1", "2", "3"},
			b:    []string{"4", "5", "6", "7"},
			exp:  []string{"1", "2", "3", "4", "5", "6", "7"},
			desc: "Not interleaved",
		},
		{
			a:    []string{"1", "3"},
			b:    []string{"2", "4", "6", "8"},
			exp:  []string{"1", "2", "3", "4", "6", "8"},
			desc: "Different lengths",
		},
		{
			a:    []string{"1", "2", "3", "5", "7"},
			b:    []string{"2", "4", "6", "7", "8"},
			exp:  []string{"1", "2", "3", "4", "5", "6", "7", "8"},
			desc: "De-dup",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			s := fromChan(newUnion2(context.Background(), asChan(tc.a), asChan(tc.b)))
			assert.Equal(t, s, tc.exp)
		})
	}
}

func TestUnion(t *testing.T) {
	unittest.SmallTest(t)
	testCases := []struct {
		inputs [][]string
		exp    []string
		desc   string
	}{
		{
			inputs: [][]string{
				{},
				{"2", "4", "6", "8"},
			},
			exp:  []string{"2", "4", "6", "8"},
			desc: "One empy",
		},
		{
			inputs: [][]string{
				{"2", "4", "6", "8"},
			},
			exp:  []string{"2", "4", "6", "8"},
			desc: "One",
		},
		{
			inputs: [][]string{
				{"1", "2"},
				{"3", "5", "7"},
				{"4", "6", "8"},
			},
			exp:  []string{"1", "2", "3", "4", "5", "6", "7", "8"},
			desc: "Three",
		},
		{
			inputs: [][]string{
				{"1", "2"},
				{"3", "7"},
				{"4", "5"},
				{"6", "8"},
			},
			exp:  []string{"1", "2", "3", "4", "5", "6", "7", "8"},
			desc: "Four",
		},
		{
			inputs: [][]string{
				{"1", "2"},
				{"2", "3"},
				{"1", "3"},
				{"1"},
			},
			exp:  []string{"1", "2", "3"},
			desc: "De-dup",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			inputs := make([]<-chan string, len(tc.inputs))
			for i, arr := range tc.inputs {
				inputs[i] = asChan(arr)
			}
			s := fromChan(NewUnion(context.Background(), inputs))
			assert.Equal(t, s, tc.exp)
		})
	}
}
