package engine

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestIntersect2Cancel(t *testing.T) {
	unittest.SmallTest(t)
	a := make(chan string)
	b := make(chan string)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	go func() {
		wg.Add(1)
		s := fromChan(newIntersect2(ctx, a, b))
		assert.Equal(t, []string{}, s)
		wg.Done()
	}()
	cancel()
	wg.Wait()
	// The test passes by not timing out.
}

func TestIntersect2(t *testing.T) {
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
			exp:  []string{},
			desc: "One empy",
		},
		{
			a:    []string{},
			b:    []string{},
			exp:  []string{},
			desc: "Both empy",
		},
		{
			a:    []string{"1", "2", "5", "7"},
			b:    []string{"2", "4", "6", "8"},
			exp:  []string{"2"},
			desc: "Same length",
		},
		{
			a:    []string{"1", "2", "3"},
			b:    []string{"1", "2", "3", "4"},
			exp:  []string{"1", "2", "3"},
			desc: "Most match",
		},
		{
			a:    []string{"1", "2", "3"},
			b:    []string{"1", "2", "3"},
			exp:  []string{"1", "2", "3"},
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
			exp:  []string{},
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
				{"1", "2", "5"},
				{"2", "5", "7"},
				{"4", "5", "8"},
			},
			exp:  []string{"5"},
			desc: "Three",
		},
		{
			inputs: [][]string{
				{"1", "2", "3", "4"},
				{"1", "2", "3", "5"},
				{"2", "3", "4", "5"},
				{"1", "2", "3", "6"},
			},
			exp:  []string{"2", "3"},
			desc: "Four",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			inputs := make([]<-chan string, len(tc.inputs))
			for i, arr := range tc.inputs {
				inputs[i] = asChan(arr)
			}
			s := fromChan(NewIntersect(context.Background(), inputs))
			assert.Equal(t, s, tc.exp)
		})
	}
}
