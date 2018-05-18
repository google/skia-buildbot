package deepequal

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
)

type equalNoArgs struct {
	a string
}

func (b equalNoArgs) Equal() bool {
	return true
}

func TestEqualWithNoArgs(t *testing.T) {
	testutils.SmallTest(t)

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
	testutils.SmallTest(t)

	a := &equalWrongArgs{a: "foo"}
	b := &equalWrongArgs{a: "bar"}

	assert.False(t, DeepEqual(a, b))
}

type infiniteNesting struct {
	alpha interface{}
}

func TestInfiniteNesting(t *testing.T) {
	testutils.SmallTest(t)

	a := &infiniteNesting{}
	a.alpha = a
	b := &infiniteNesting{}
	b.alpha = b

	assert.True(t, reflect.DeepEqual(a, b))
}
