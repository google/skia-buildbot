package engine

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestIntersect2_CanCancel(t *testing.T) {
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

func TestIntersect2_BothChannelsEmpty(t *testing.T) {
	unittest.SmallTest(t)
	s := fromChan(newIntersect2(context.Background(), asChan([]int64{}), asChan([]int64{})))
	assert.Equal(t, []int64{}, s)
}

func TestIntersect2_OneChannelEmpty(t *testing.T) {
	unittest.SmallTest(t)
	s := fromChan(newIntersect2(context.Background(), asChan([]int64{2, 4, 6, 8}), asChan([]int64{})))
	assert.Equal(t, []int64{}, s)
}

func TestIntersect2_BothChannelsMatchAll(t *testing.T) {
	unittest.SmallTest(t)
	s := fromChan(newIntersect2(context.Background(), asChan([]int64{1, 2, 3}), asChan([]int64{1, 2, 3})))
	assert.Equal(t, []int64{1, 2, 3}, s)
}

func TestIntersect2_MostChannelValuesMatchAll(t *testing.T) {
	unittest.SmallTest(t)
	s := fromChan(newIntersect2(context.Background(), asChan([]int64{1, 2, 3, 4}), asChan([]int64{1, 2, 3})))
	assert.Equal(t, []int64{1, 2, 3}, s)
}

func TestIntersect2_BothChannelsSameLength(t *testing.T) {
	unittest.SmallTest(t)
	s := fromChan(newIntersect2(context.Background(), asChan([]int64{1, 2, 3, 4}), asChan([]int64{2, 5, 7, 9})))
	assert.Equal(t, []int64{2}, s)
}

func TestNewIntersect_OneChannel(t *testing.T) {
	unittest.SmallTest(t)
	s := fromChan(NewIntersect(context.Background(), []<-chan int64{asChan([]int64{1, 2, 3, 4})}))
	assert.Equal(t, []int64{1, 2, 3, 4}, s)
}

func TestNewIntersect_TwoChannelsOneEmpty(t *testing.T) {
	unittest.SmallTest(t)
	s := fromChan(NewIntersect(context.Background(), []<-chan int64{asChan([]int64{1, 2, 3, 4}), asChan([]int64{})}))
	assert.Equal(t, []int64{}, s)
}

func TestNewIntersect_ThreeChannels(t *testing.T) {
	unittest.SmallTest(t)
	s := fromChan(NewIntersect(context.Background(), []<-chan int64{
		asChan([]int64{1, 2, 5}),
		asChan([]int64{2, 5, 7}),
		asChan([]int64{4, 5, 8}),
	}))
	assert.Equal(t, []int64{5}, s)
}

func TestNewIntersect_FourChannels(t *testing.T) {
	unittest.SmallTest(t)
	s := fromChan(NewIntersect(context.Background(), []<-chan int64{
		asChan([]int64{1, 2, 3, 4}),
		asChan([]int64{1, 2, 3, 5}),
		asChan([]int64{2, 3, 4, 5}),
		asChan([]int64{1, 2, 3, 6}),
	}))
	assert.Equal(t, []int64{2, 3}, s)
}
