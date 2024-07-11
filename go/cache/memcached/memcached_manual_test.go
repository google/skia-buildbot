package memcached

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// To run these tests manually run a local memcached instance at the given port
// and interface:
//
// $ memcached -p 11211 -l 127.0.0.1
var localServerAddress = []string{"127.0.0.1:11211"}

func TestCache_New_Failure(t *testing.T) {
	_, err := New([]string{""}, "")
	require.Error(t, err)
}

func TestCache_Exists_Success(t *testing.T) {
	c, err := New(localServerAddress, "test-namespace")
	require.NoError(t, err)

	c.Add("foo")
	ok := c.Exists("foo")
	assert.True(t, ok)
}

func TestCache_ExistsOnlyInOneNamespace_Success(t *testing.T) {
	c, err := New(localServerAddress, "test-namespace")
	require.NoError(t, err)

	c.Add("foo")
	ok := c.Exists("foo")
	assert.True(t, ok)

	c, err = New(localServerAddress, "test-namespace-2")
	require.NoError(t, err)

	ok = c.Exists("foo")
	assert.False(t, ok)

}

func TestCache_Exists_FalseOnMiss(t *testing.T) {
	c, err := New(localServerAddress, "test-namespace")
	require.NoError(t, err)

	ok := c.Exists("qux")
	assert.False(t, ok)
}
