package accum

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"go.skia.org/infra/go/testutils"
)

func TestAccum(t *testing.T) {
	testutils.SmallTest(t)

	lastValues := []int64{}

	r := func(metric string, tags map[string]string, value int64) {
		lastValues = append(lastValues, value)
	}

	a := New(r)
	tags := map[string]string{
		"bot":  "foo",
		"task": "bar",
	}

	a.Add("some.duration", tags, 12)
	assert.NotNil(t, a.values["some.duration"])
	assert.NotNil(t, a.values["some.duration"]["bot foo task bar"])
	assert.Equal(t, int64(12), a.values["some.duration"]["bot foo task bar"].total)
	assert.Equal(t, int64(1), a.values["some.duration"]["bot foo task bar"].num)
	assert.Equal(t, "foo", a.values["some.duration"]["bot foo task bar"].tags["bot"])
	a.Add("some.duration", tags, 30)
	assert.Equal(t, int64(42), a.values["some.duration"]["bot foo task bar"].total)
	assert.Equal(t, int64(2), a.values["some.duration"]["bot foo task bar"].num)
	a.Report()
	assert.Equal(t, 2, len(lastValues))
}
