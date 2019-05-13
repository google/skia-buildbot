package ingestion

import (
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

const (
	// Config settings for BTIStore to run against the emulator.
	projectID  = "test-project"
	instanceID = "test-instance"
	nameSpace  = "testing"
)

var exampleHashes = []string{
	"45bf36e647291ff2d8b532422dae8dca",
	"ffe25ed9e89485273b64867fefc72560",
	"22d2e278850258dedd96760932305330",
	"fa32db2ee1390f98b49ab38349087699",
	"eca06e60234edafeeef5dd327d4412fe",
	"2354fe28b44a29b5fff1669896c45ad2",
	"73fb19a5881ec99aeb3230589c856aeb",
	"604869fd1c14e621c24b4d395a71b7bc",
}

func TestBTIStore(t *testing.T) {
	unittest.LargeTest(t)

	// Set up the table and column families.
	assert.NoError(t, InitBT(projectID, instanceID, TABLE_FILES_PROCESSED))

	store, err := NewBTIStore(projectID, instanceID, nameSpace)
	assert.NoError(t, err)
	assert.NotNil(t, store)

	for _, md5Hash := range exampleHashes {
		// Write it twice to make sure we correctly delete old timestamps in BT.
		assert.NoError(t, store.SetResultFileHash(md5Hash))
		assert.NoError(t, store.SetResultFileHash(md5Hash))
	}

	for _, md5Hash := range exampleHashes {
		ok, err := store.ContainsResultFileHash(md5Hash)
		assert.NoError(t, err)
		assert.True(t, ok)
	}

	// Delete the table again.
	assert.NoError(t, store.Clear())
	found, err := store.ContainsResultFileHash(exampleHashes[0])
	assert.NoError(t, err)
	assert.False(t, found)
}
