package testutils

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTime(t *testing.T) {
	SmallTest(t)

	t1 := time.Now()
	t2 := t1.Round(0)

	AssertDeepEqual(t, t1, t2)
}

type customEqualValue struct {
	a string
}

func (b customEqualValue) Equal(o customEqualValue) bool {
	return b.a == "foo" && o.a == "bar"
}

func TestCustomEqualValue(t *testing.T) {
	SmallTest(t)

	a := customEqualValue{a: "foo"}
	b := customEqualValue{a: "bar"}

	AssertDeepEqual(t, a, b)
}

type customEqualPointer struct {
	a string
}

func (b *customEqualPointer) Equal(o customEqualPointer) bool {
	return true
}

func TestCustomEqualPointer(t *testing.T) {
	SmallTest(t)

	a := customEqualPointer{a: "foo"}
	b := customEqualPointer{a: "bar"}

	AssertDeepEqual(t, a, b)
}

type equalNoArgs struct {
	a string
}

func (b equalNoArgs) Equal() bool {
	return true
}

func TestEqualWithNoArgs(t *testing.T) {
	SmallTest(t)

	a := &equalNoArgs{a: "foo"}
	b := &equalNoArgs{a: "bar"}

	assert.False(t, DeepEqual(a, b))
}

type equalWrongArgs struct {
	a string
}

func (b equalWrongArgs) Equal(foo time.Time) bool {
	return true
}

func TestEqualWithWrongArgs(t *testing.T) {
	SmallTest(t)

	a := &equalWrongArgs{a: "foo"}
	b := &equalWrongArgs{a: "bar"}

	assert.False(t, DeepEqual(a, b))
}

type infiniteNesting struct {
	alpha interface{}
}

func TestInfiniteNesting(t *testing.T) {
	SmallTest(t)

	a := &infiniteNesting{}
	a.alpha = a
	b := &infiniteNesting{}
	b.alpha = b

	assert.True(t, reflect.DeepEqual(a, b))

	AssertDeepEqual(t, a, b)
}
