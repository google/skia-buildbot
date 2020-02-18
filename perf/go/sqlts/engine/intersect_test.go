package engine

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestIntersect2_can_cancel(t *testing.T) {
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

func TestIntersect2_both_channels_empty(t *testing.T) {
	unittest.SmallTest(t)
	s := fromChan(newIntersect2(context.Background(), asChan([]int64{}), asChan([]int64{})))
	assert.Equal(t, []int64{}, s)
}

func TestIntersect2_one_channel_empty(t *testing.T) {
	unittest.SmallTest(t)
	s := fromChan(newIntersect2(context.Background(), asChan([]int64{2, 4, 6, 8}), asChan([]int64{})))
	assert.Equal(t, []int64{}, s)
}

func TestIntersect2_both_channels_match_all(t *testing.T) {
	unittest.SmallTest(t)
	s := fromChan(newIntersect2(context.Background(), asChan([]int64{1, 2, 3}), asChan([]int64{1, 2, 3})))
	assert.Equal(t, []int64{1, 2, 3}, s)
}

func TestIntersect2_most_channel_values_match_all(t *testing.T) {
	unittest.SmallTest(t)
	s := fromChan(newIntersect2(context.Background(), asChan([]int64{1, 2, 3, 4}), asChan([]int64{1, 2, 3})))
	assert.Equal(t, []int64{1, 2, 3}, s)
}

func TestIntersect2_both_channels_same_length(t *testing.T) {
	unittest.SmallTest(t)
	s := fromChan(newIntersect2(context.Background(), asChan([]int64{1, 2, 3, 4}), asChan([]int64{2, 5, 7, 9})))
	assert.Equal(t, []int64{2}, s)
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
