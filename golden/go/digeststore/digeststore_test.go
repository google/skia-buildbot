package digeststore

import (
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	ptypes "go.skia.org/infra/perf/go/types"
)

func TestMemDigestStore(t *testing.T) {
	testDigestStore(t, NewMemDigestStore())
}

func testDigestStore(t assert.TestingT, digestStore DigestStore) {
	testName_1, digest_1 := "smapleTest_1", "sampleDigest_1"
	commit_1 := &ptypes.Commit{CommitTime: time.Now().Unix() - 20}

	di, ok := digestStore.GetDigestInfo(testName_1, digest_1)
	assert.False(t, ok)

	_, err := digestStore.UpdateDigestTimeStamps(testName_1, digest_1, commit_1)
	assert.Nil(t, err)
	di, ok = digestStore.GetDigestInfo(testName_1, digest_1)
	assert.True(t, ok)
	assert.Equal(t, commit_1.CommitTime, di.Last)
	assert.Equal(t, commit_1.CommitTime, di.First)

	// Update the digest with a commit 10 seconds later than the first one.
	commit_2 := &ptypes.Commit{CommitTime: commit_1.CommitTime + 10}
	_, err = digestStore.UpdateDigestTimeStamps(testName_1, digest_1, commit_2)
	assert.Nil(t, err)
	di, ok = digestStore.GetDigestInfo(testName_1, digest_1)
	assert.True(t, ok)

	assert.Equal(t, commit_1.CommitTime, di.First)
	assert.Equal(t, commit_2.CommitTime, di.Last)
}
