package deepequal

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTime(t *testing.T) {

	t1 := time.Now()
	t2 := t1.Round(0)

	require.True(t, DeepEqual(t1, t2))
}

type customEqualValue struct {
	a string
}

func (b customEqualValue) Equal(o customEqualValue) bool {
	return b.a == "foo" && o.a == "bar"
}

func TestCustomEqualValue(t *testing.T) {

	a := customEqualValue{a: "foo"}
	b := customEqualValue{a: "bar"}

	require.True(t, DeepEqual(a, b))
}

type customEqualPointer struct {
	a string
}

func (b *customEqualPointer) Equal(o customEqualPointer) bool {
	return true
}

func TestCustomEqualPointer(t *testing.T) {

	a := customEqualPointer{a: "foo"}
	b := customEqualPointer{a: "bar"}

	require.True(t, DeepEqual(a, b))
}

type equalNoArgs struct {
	a string
}

func (b equalNoArgs) Equal() bool {
	return true
}

func TestEqualWithNoArgs(t *testing.T) {

	a := &equalNoArgs{a: "foo"}
	b := &equalNoArgs{a: "bar"}

	require.False(t, DeepEqual(a, b))
}

type equalWrongArgs struct {
	a string
}

func (b equalWrongArgs) Equal(foo time.Time) bool {
	return true
}

func TestEqualWithWrongArgs(t *testing.T) {

	a := &equalWrongArgs{a: "foo"}
	b := &equalWrongArgs{a: "bar"}

	require.False(t, DeepEqual(a, b))
}

type infiniteNesting struct {
	alpha interface{}
}

func TestInfiniteNesting(t *testing.T) {

	a := &infiniteNesting{}
	a.alpha = a
	b := &infiniteNesting{}
	b.alpha = b

	require.True(t, reflect.DeepEqual(a, b))
	require.True(t, DeepEqual(a, b))
}
