package ring

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
)

func TestStringRing(t *testing.T) {

	// Cap of 1.
	r := NewStringRing(1)
	assertdeep.Equal(t, []string{}, r.GetAll())
	r.Put("a")
	assertdeep.Equal(t, []string{"a"}, r.GetAll())
	r.Put("b")
	assertdeep.Equal(t, []string{"b"}, r.GetAll())
	r.Put("c")
	assertdeep.Equal(t, []string{"c"}, r.GetAll())

	// Cap of 2.
	r = NewStringRing(2)
	assertdeep.Equal(t, []string{}, r.GetAll())
	r.Put("a")
	assertdeep.Equal(t, []string{"a"}, r.GetAll())
	r.Put("b")
	assertdeep.Equal(t, []string{"a", "b"}, r.GetAll())
	r.Put("c")
	assertdeep.Equal(t, []string{"b", "c"}, r.GetAll())
	r.Put("d")
	assertdeep.Equal(t, []string{"c", "d"}, r.GetAll())

	// Cap of 3.
	r = NewStringRing(3)
	assertdeep.Equal(t, []string{}, r.GetAll())
	r.Put("a")
	assertdeep.Equal(t, []string{"a"}, r.GetAll())
	r.Put("b")
	assertdeep.Equal(t, []string{"a", "b"}, r.GetAll())
	r.Put("c")
	assertdeep.Equal(t, []string{"a", "b", "c"}, r.GetAll())
	r.Put("d")
	assertdeep.Equal(t, []string{"b", "c", "d"}, r.GetAll())
}

func TestStringRingConcurrent(t *testing.T) {

	// Check for racy behavior by spinning up a bunch of goroutines to write
	// to the ring and verifying that it ends up with the correct set of
	// entries.
	r := NewStringRing(1000)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				r.Put(fmt.Sprintf("%d:%d", i, j))
			}
		}(i)
	}
	wg.Wait()
	require.Equal(t, 1000, len(r.GetAll()))
}
