package util

import (
	"io/ioutil"
	"path"
	"testing"
	"time"

	"go.skia.org/infra/go/testutils"

	assert "github.com/stretchr/testify/require"
)

func TestPersistentAutoDecrementCounter(t *testing.T) {
	testutils.MediumTest(t)

	w, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	defer testutils.RemoveAll(t, w)

	f := path.Join(w, "counter")
	d := 200 * time.Millisecond
	c, err := NewPersistentAutoDecrementCounter(f, d)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), c.Get())
	assert.NoError(t, c.Inc())
	assert.Equal(t, int64(1), c.Get())

	c2, err := NewPersistentAutoDecrementCounter(f, d)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), c2.Get())

	time.Sleep(time.Duration(1.5 * float64(d)))

	assert.Equal(t, int64(0), c.Get())
	assert.Equal(t, int64(0), c2.Get())

	c3, err := NewPersistentAutoDecrementCounter(f, d)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), c3.Get())

	i := 0
	for range time.Tick(time.Duration(float64(d) / float64(4))) {
		assert.Equal(t, int64(i), c.Get())
		assert.NoError(t, c.Inc())
		if i == 2 {
			break
		}
		i++
	}
	time.Sleep(time.Duration(float64(d) / float64(8)))
	expect := int64(3)
	for range time.Tick(time.Duration(float64(d) / float64(4))) {
		assert.Equal(t, expect, c.Get())
		c4, err := NewPersistentAutoDecrementCounter(f, d)
		assert.NoError(t, err)
		assert.Equal(t, expect, c4.Get())
		if expect == 0 {
			break
		}
		expect--
	}
}
