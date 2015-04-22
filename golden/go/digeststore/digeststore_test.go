package digeststore

import (
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/database"
	"go.skia.org/infra/go/database/testutil"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/db"
	ptypes "go.skia.org/infra/perf/go/types"
)

func TestMemDigestStore(t *testing.T) {
	testDigestStore(t, NewMemDigestStore())
}

func TestSQLDigestStore(t *testing.T) {
	// Set up the test database.
	testDb := testutil.SetupMySQLTestDatabase(t, db.MigrationSteps())
	defer testDb.Close(t)

	conf := testutil.LocalTestDatabaseConfig(db.MigrationSteps())
	vdb := database.NewVersionedDB(conf)
	defer util.Close(vdb)

	testDigestStore(t, NewSQLDigestStore(vdb))
}

func testDigestStore(t assert.TestingT, digestStore DigestStore) {
	testName_1, digest_1 := "smapleTest_1", "sampleDigest_1"
	commit_1 := &ptypes.Commit{CommitTime: time.Now().Unix() - 20}

	di, ok, err := digestStore.GetDigestInfo(testName_1, digest_1)
	assert.Nil(t, err)
	assert.False(t, ok)

	_, err = digestStore.UpdateDigestTimeStamps(testName_1, digest_1, commit_1)
	assert.Nil(t, err)
	di, ok, err = digestStore.GetDigestInfo(testName_1, digest_1)
	assert.Nil(t, err)
	assert.True(t, ok)
	assert.Equal(t, commit_1.CommitTime, di.Last)
	assert.Equal(t, commit_1.CommitTime, di.First)

	// Update the digest with a commit 10 seconds later than the first one.
	commit_2 := &ptypes.Commit{CommitTime: commit_1.CommitTime + 10}
	_, err = digestStore.UpdateDigestTimeStamps(testName_1, digest_1, commit_2)
	assert.Nil(t, err)
	di, ok, err = digestStore.GetDigestInfo(testName_1, digest_1)
	assert.Nil(t, err)
	assert.True(t, ok)

	assert.Equal(t, commit_1.CommitTime, di.First)
	assert.Equal(t, commit_2.CommitTime, di.Last)
}
