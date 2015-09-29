package trybot

import (
	"io"
	"os"
	"testing"
	"time"

	"github.com/rcrowley/go-metrics"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/database/testutil"
	"go.skia.org/infra/go/ingester"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/db"
)

const (
	TEST_FILE       = "testdata/dm.json"
	TEST_FILE_2     = "testdata/dm-2.json"
	TEST_ISSUE      = "1279303002"
	MODIFIED_DIGEST = "c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0"
)

var TEST_DIGESTS map[string]bool = map[string]bool{
	"c0c1907f57bbd25c5e1e6e3853a0a48f": true,
	"859d7ec4a35cb329a7ccf97ff8715b55": true,
	"c6aaf68f7dd29ac57e898db90fb19e78": true,
	"fa3c371d201d6f88f7a47b41862e2e85": true,
}

func TestTrybotIngester(t *testing.T) {
	// Set up the test database.
	testDb := testutil.SetupMySQLTestDatabase(t, db.MigrationSteps())
	defer testDb.Close(t)

	conf := testutil.LocalTestDatabaseConfig(db.MigrationSteps())
	vdb, err := conf.NewVersionedDB()
	assert.Nil(t, err)

	Init(vdb)
	resultStore := NewTrybotResultStorage(vdb)

	// Get the constructor and create an instance of gold-trybot ingester.
	tbIngester := ingester.Constructor(config.CONSTRUCTOR_GOLD_TRYBOT)()
	_ = ingestFile(t, tbIngester, TEST_FILE)

	tries, err := resultStore.Get(TEST_ISSUE)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(tries.Bots))
	var targetBot *BotResults
	for _, v := range tries.Bots {
		assert.Equal(t, len(TEST_DIGESTS), len(v.TestResults))
		targetBot = v
	}

	for _, entry := range targetBot.TestResults {
		assert.True(t, TEST_DIGESTS[tries.Digests[entry.DigestIdx]])
	}

	allIssues, _, err := resultStore.List(0, 10)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(allIssues))
	assert.Equal(t, TEST_ISSUE, allIssues[0])

	time.Sleep(time.Second)

	_ = ingestFile(t, tbIngester, TEST_FILE_2)
	tries, err = resultStore.Get(TEST_ISSUE)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(tries.Bots))
	assert.Equal(t, 1, len(tries.Digests))
	assert.Equal(t, MODIFIED_DIGEST, tries.Digests[0])

	for _, bot := range tries.Bots {
		for _, result := range bot.TestResults {
			assert.Equal(t, MODIFIED_DIGEST, result.digest)
			assert.Equal(t, 0, result.DigestIdx)
		}
	}
}

func ingestFile(t *testing.T, tbIngester ingester.ResultIngester, fName string) int64 {
	opener := func() (io.ReadCloser, error) {
		return os.Open(fName)
	}

	now := time.Now().Unix()
	fileInfo := &ingester.ResultsFileLocation{
		Name:        fName,
		LastUpdated: now,
	}

	counter := metrics.NewCounter()
	assert.Nil(t, tbIngester.Ingest(nil, opener, fileInfo, counter))
	assert.Nil(t, tbIngester.BatchFinished(counter))
	return now
}
