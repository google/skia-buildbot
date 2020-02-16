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
	a := make(chan int64)
	b := make(chan int64)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		s := fromChan(newIntersect2(ctx, a, b))
		assert.Equal(t, []int64{}, s)
		wg.Done()
	}()
	cancel()
	wg.Wait()
	// The test passes by not timing out.
}

func TestIntersect2(t *testing.T) {
	unittest.SmallTest(t)
	testCases := []struct {
		a    []int64
		b    []int64
		exp  []int64
		desc string
	}{
		{
			a:    []int64{},
			b:    []int64{2, 4, 6, 8},
			exp:  []int64{},
			desc: "One empy",
		},
		{
			a:    []int64{},
			b:    []int64{},
			exp:  []int64{},
			desc: "Both empy",
		},
		{
			a:    []int64{1, 2, 5, 7},
			b:    []int64{2, 4, 6, 8},
			exp:  []int64{2},
			desc: "Same length",
		},
		{
			a:    []int64{1, 2, 3},
			b:    []int64{1, 2, 3, 4},
			exp:  []int64{1, 2, 3},
			desc: "Most match",
		},
		{
			a:    []int64{1, 2, 3},
			b:    []int64{1, 2, 3},
			exp:  []int64{1, 2, 3},
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
		inputs [][]int64
		exp    []int64
		desc   string
	}{
		{
			inputs: [][]int64{
				{},
				{2, 4, 6, 8},
			},
			exp:  []int64{},
			desc: "One empy",
		},
		{
			inputs: [][]int64{
				{2, 4, 6, 8},
			},
			exp:  []int64{2, 4, 6, 8},
			desc: "One",
		},
		{
			inputs: [][]int64{
				{1, 2, 5},
				{2, 5, 7},
				{4, 5, 8},
			},
			exp:  []int64{5},
			desc: "Three",
		},
		{
			inputs: [][]int64{
				{1, 2, 3, 4},
				{1, 2, 3, 5},
				{2, 3, 4, 5},
				{1, 2, 3, 6},
			},
			exp:  []int64{2, 3},
			desc: "Four",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			inputs := make([]<-chan int64, len(tc.inputs))
			for i, arr := range tc.inputs {
				inputs[i] = asChan(arr)
			}
			s := fromChan(NewIntersect(context.Background(), inputs))
			assert.Equal(t, s, tc.exp)
		})
	}
}
