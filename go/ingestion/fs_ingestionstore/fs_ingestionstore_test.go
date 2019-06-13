package fs_ingestionstore

import (
	"context"
	"testing"

	"github.com/google/uuid"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/testutils/unittest"
)

// TODO(kjlubick): These tests are marked as manual because the
// Firestore Emulator is not yet on the bots, due to some more complicated
// setup (e.g. chmod)

// TestGetExpectations writes some changes and then reads back the
// aggregated results.
func TestSetContains(t *testing.T) {
	unittest.ManualTest(t)
	unittest.RequiresFirestoreEmulator(t)

	c := getTestFirestoreInstance(t)

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

// Creates an empty firestore instance. The emulator keeps the tables in ram, but
// by appending a random nonce, we can be assured the collection we get is empty.
func getTestFirestoreInstance(t *testing.T) *firestore.Client {
	randInstance := uuid.New().String()
	c, err := firestore.NewClient(context.Background(), "emulated-project", "gold", "test-"+randInstance, nil)
	assert.NoError(t, err)
	return c
}
