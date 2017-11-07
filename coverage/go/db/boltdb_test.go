package db

import (
	"path"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/coverage/go/common"
	"go.skia.org/infra/go/testutils"
)

func TestStorage(t *testing.T) {
	// MediumTest because we are actually testing boltdb on disk.
	testutils.MediumTest(t)

	tpath, cleanup := testutils.TempDir(t)
	defer cleanup()

	b, err := NewBoltDB(path.Join(tpath, "boltdb"))
	assert.NoError(t, err)
	_, ok := b.CheckCache("key1")
	assert.False(t, ok, "nothing should exist at first")
	expected1 := common.CoverageSummary{TotalLines: 2000000, MissedLines: 7000}
	expected2 := common.CoverageSummary{TotalLines: -2, MissedLines: -7}
	assert.NoError(t, b.StoreToCache("key1", expected1))
	assert.NoError(t, b.StoreToCache("key2", expected2))
	actual, ok := b.CheckCache("key2")
	assert.True(t, ok, "key2 should have a value")
	testutils.AssertDeepEqual(t, actual, expected2)
	actual, ok = b.CheckCache("key1")
	assert.True(t, ok, "key1 should now have a value")
	testutils.AssertDeepEqual(t, actual, expected1)
	_, ok = b.CheckCache("key:doesnotexist")
	assert.False(t, ok, "nonexistant key should not exist still")
	assert.NoError(t, b.Close())

	// Wait for disk writing to complete
	time.Sleep(2 * time.Second)
	// Open boltdb again
	c, err := NewBoltDB(path.Join(tpath, "boltdb"))
	assert.NoError(t, err)

	actual, ok = c.CheckCache("key2")
	assert.True(t, ok, "key2 should have a value after opening")
	testutils.AssertDeepEqual(t, actual, expected2)
	actual, ok = c.CheckCache("key1")
	assert.True(t, ok, "key1 should now have a value after opening")
	testutils.AssertDeepEqual(t, actual, expected1)
	_, ok = c.CheckCache("key:doesnotexist")
	assert.False(t, ok, "nonexistant key should not exist still after opening")
	assert.NoError(t, c.Close())
}
