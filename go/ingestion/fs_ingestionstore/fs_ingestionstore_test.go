package fs_ingestionstore

import (
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/testutils/unittest"
)

// TestGetExpectations writes some changes and then reads back the
// aggregated results.
func TestSetContains(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := firestore.NewClientForTesting(t)
	defer cleanup()

	f := New(c)

	b, err := f.ContainsResultFileHash("nope", "not here")
	assert.NoError(t, err)
	assert.False(t, b)

	err = f.SetResultFileHash("skia-gold-flutter/dm-json-v1/2019/foo.json", "version1")
	assert.NoError(t, err)
	err = f.SetResultFileHash("skia-gold-flutter/dm-json-v1/2019/foo.json", "version2")
	assert.NoError(t, err)
	err = f.SetResultFileHash("skia-gold-flutter/dm-json-v1/2020/bar.json", "versionA")
	assert.NoError(t, err)

	b, err = f.ContainsResultFileHash("skia-gold-flutter/dm-json-v1/2019/foo.json", "version2")
	assert.NoError(t, err)
	assert.True(t, b)

	b, err = f.ContainsResultFileHash("skia-gold-flutter/dm-json-v1/2019/foo.json", "version1")
	assert.NoError(t, err)
	assert.True(t, b)

	b, err = f.ContainsResultFileHash("nope", "version1")
	assert.NoError(t, err)
	assert.False(t, b)

	b, err = f.ContainsResultFileHash("skia-gold-flutter/dm-json-v1/2019/foo.json", "versionA")
	assert.NoError(t, err)
	assert.False(t, b)
}
