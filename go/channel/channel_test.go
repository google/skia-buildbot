package channel

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/sklog"
)

func TestBatch(t *testing.T) {
	inCh := make(chan interface{})

	expectTotal := 0
	mk := func(n int) []struct{} {
		expectTotal += n
		return make([]struct{}, n)
	}
	put := func(n int) {
		for _, elem := range mk(n) {
			inCh <- interface{}(elem)
		}
	}
	go func() {
		put(6)
		time.Sleep(time.Second)
		put(14)
		put(27)
		put(1)
		put(1)
		put(1)
		close(inCh)
	}()

	total := 0
	for elems := range Batch(10, chan interface{}(inCh)) {
		assert.True(t, len(elems) <= 10)
		assert.NotEqual(t, len(elems), 0)
		sklog.Errorf("Processing %d elems", len(elems))
		time.Sleep(time.Second)
		total += len(elems)
	}
	assert.Equal(t, expectTotal, total)
}
