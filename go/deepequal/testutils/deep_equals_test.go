package testutils

import (
	"testing"
	"time"

	"go.skia.org/infra/go/testutils"
)

func TestTime(t *testing.T) {
	testutils.SmallTest(t)

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
	testutils.SmallTest(t)

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
	testutils.SmallTest(t)

	a := customEqualPointer{a: "foo"}
	b := customEqualPointer{a: "bar"}

	AssertDeepEqual(t, a, b)
}
