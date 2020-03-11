package ring

import (
	"testing"

	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestStringRing(t *testing.T) {
	unittest.SmallTest(t)

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
