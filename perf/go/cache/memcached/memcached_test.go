package memcached

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

var localServerAddress = []string{"127.0.0.1:11211"}

func TestCache_New_Failure(t *testing.T) {
	unittest.ManualTest(t)
	_, err := New([]string{""}, "")
	require.Error(t, err)
}

func TestCache_Get_Success(t *testing.T) {
	unittest.ManualTest(t)
	c, err := New(localServerAddress, "test-namespace")
	require.NoError(t, err)

	c.Add("foo", "bar")
	got, ok := c.Get("foo")
	assert.True(t, ok)
	assert.Equal(t, "bar", got)

	_, ok = c.Get("quux")
	assert.False(t, ok)
}

func TestCache_Get_FalseOnMiss(t *testing.T) {
	unittest.ManualTest(t)
	c, err := New(localServerAddress, "test-namespace")
	require.NoError(t, err)

	_, ok := c.Get("quux")
	assert.False(t, ok)
}

func TestCache_Exists_Success(t *testing.T) {
	unittest.ManualTest(t)
	c, err := New(localServerAddress, "test-namespace")
	require.NoError(t, err)

	c.Add("foo", "baz")
	ok := c.Exists("foo")
	assert.True(t, ok)
}

func TestCache_ExistsOnlyInOneNamespace_Success(t *testing.T) {
	unittest.ManualTest(t)
	c, err := New(localServerAddress, "test-namespace")
	require.NoError(t, err)

	c.Add("foo", "quux")
	ok := c.Exists("foo")
	assert.True(t, ok)

	c, err = New(localServerAddress, "test-namespace-2")
	require.NoError(t, err)

	ok = c.Exists("foo")
	assert.False(t, ok)

}

func TestCache_Exists_FalseOnMiss(t *testing.T) {
	unittest.ManualTest(t)
	c, err := New(localServerAddress, "test-namespace")
	require.NoError(t, err)

	ok := c.Exists("qux")
	assert.False(t, ok)
}
