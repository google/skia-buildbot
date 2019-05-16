package digeststore

import (
	"os"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

const TEST_DATA_DIR = "testdata"

func TestDigestStore(t *testing.T) {
	unittest.MediumTest(t)
	assert.NoError(t, os.MkdirAll(TEST_DATA_DIR, 0755))
	defer testutils.RemoveAll(t, TEST_DATA_DIR)

	digestStore, err := New(TEST_DATA_DIR)
	assert.NoError(t, err)
	testDigestStore(t, digestStore)
}

func testDigestStore(t assert.TestingT, digestStore DigestStore) {
	testName_1, digest_1 := "smapleTest_1", "sampleDigest_1"
	timestamp_1 := time.Now().Unix() - 20

	// TODO(kjlubick): assert something with di
	di, ok, err := digestStore.Get(testName_1, digest_1)
	assert.NoError(t, err)
	assert.False(t, ok)

	digestInfos := []*DigestInfo{
		{TestName: testName_1, Digest: digest_1, First: timestamp_1, Last: timestamp_1},
	}
	assert.NoError(t, digestStore.Update(digestInfos))

	di, ok, err = digestStore.Get(testName_1, digest_1)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, timestamp_1, di.Last)
	assert.Equal(t, timestamp_1, di.First)

	// Update the digest with a commit 10 seconds later than the first one.
	timestamp_2 := timestamp_1 + 10
	digestInfos = []*DigestInfo{
		{TestName: testName_1, Digest: digest_1, First: timestamp_2, Last: timestamp_2},
	}

	assert.NoError(t, digestStore.Update(digestInfos))

	di, ok, err = digestStore.Get(testName_1, digest_1)
	assert.NoError(t, err)
	assert.True(t, ok)

	assert.Equal(t, timestamp_1, di.First)
	assert.Equal(t, timestamp_2, di.Last)
}
