package ring

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestStringRing(t *testing.T) {
	unittest.SmallTest(t)

	// No capacity.
	r, err := NewStringRing(0)
	require.Nil(t, r)
	require.NotNil(t, err)
	r, err = NewStringRing(-1)
	require.Nil(t, r)
	require.NotNil(t, err)

	// Cap of 1.
	r, err = NewStringRing(1)
	require.Nil(t, err)
	assertdeep.Equal(t, []string{}, r.GetAll())
	r.Put("a")
	assertdeep.Equal(t, []string{"a"}, r.GetAll())
	r.Put("b")
	assertdeep.Equal(t, []string{"b"}, r.GetAll())
	r.Put("c")
	assertdeep.Equal(t, []string{"c"}, r.GetAll())

	// Cap of 2.
	r, err = NewStringRing(2)
	require.Nil(t, err)
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
	r, err = NewStringRing(3)
	require.Nil(t, err)
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
