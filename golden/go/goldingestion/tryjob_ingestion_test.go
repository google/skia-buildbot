package goldingestion

import (
	"context"
	"fmt"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ingestion"
	"google.golang.org/api/option"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/golden/go/tryjobstore"
)

const (
	// directory with the test data.
	TEST_DATA_DIR = "./testdata"

	// name of the input file containing test data.
	TRYJOB_INGESTION_FILE = TEST_DATA_DIR + "/tryjob-dm.json"
)

// Tests the processor in conjunction with the vcs.
func TestTryjobGoldProcessor(t *testing.T) {
	testutils.LargeTest(t)

	// TODO(stephana): This test should be tested shomehow, probably by running
	// the simulator in the bot.
	t.Skip()

	issueUpdated, err := time.Parse("2006-01-02 15:04:05 MST", "2017-12-07 14:54:05 EST")
	assert.NoError(t, err)

	testIssue := &tryjobstore.IssueDetails{
		Issue: &tryjobstore.Issue{
			ID:      81300,
			Subject: "[infra] Move commands from isolates to gen_tasks.go",
			Owner:   "someone@example.com",
			Status:  "MERGED",
			Updated: issueUpdated,
		},

		PatchsetDetails: []*tryjobstore.PatchsetDetail{
			&tryjobstore.PatchsetDetail{
				ID: 9,
			},
		},
	}
	testTryjob := &tryjobstore.Tryjob{
		BuildBucketID: 8960860541739306896,
		IssueID:       81300,
		PatchsetID:    9,
		Builder:       "Test-iOS-Clang-iPhone7-GPU-GT7600-arm64-Debug-All",
		Status:        tryjobstore.TRYJOB_COMPLETE,
		Updated:       time.Unix(1512655545, 180550*int64(time.Microsecond)),
	}

	opt := option.WithServiceAccountFile("service-account.json")
	tryjobStore, err := tryjobstore.NewCloudTryjobStore(common.PROJECT_ID, "gold-localhost-stephana", opt)
	assert.NoError(t, err)

	// Make sure the issue is removed.
	assert.NoError(t, tryjobStore.DeleteIssue(testIssue.ID))
	mockedIBF := &mockIBF{
		issue:       testIssue,
		tryjob:      testTryjob,
		tryjobStore: tryjobStore,
	}

	processor := &goldTryjobProcessor{
		issueBuildFetcher: mockedIBF,
		tryjobStore:       tryjobStore,
	}

	// Call process for the input file.
	fsResult, err := ingestion.FileSystemResult(TRYJOB_INGESTION_FILE, TEST_DATA_DIR)
	assert.NoError(t, err)
	assert.NoError(t, processor.Process(context.Background(), fsResult))

	foundIssue, err := tryjobStore.GetIssue(testIssue.ID, false, nil)
	assert.NoError(t, err)
	assert.Equal(t, testIssue, foundIssue)

	foundTryjob, err := tryjobStore.GetTryjob(testIssue.ID, testTryjob.BuildBucketID)
	assert.NoError(t, err)

	// At this point the tryjob should be marked ingested.
	testTryjob.Status = tryjobstore.TRYJOB_INGESTED
	assert.Equal(t, testTryjob, foundTryjob)
}

type mockIBF struct {
	issue       *tryjobstore.IssueDetails
	tryjob      *tryjobstore.Tryjob
	tryjobStore tryjobstore.TryjobStore
}

func (m *mockIBF) FetchIssueAndTryjob(issueID, buildBucketID int64) (*tryjobstore.IssueDetails, *tryjobstore.Tryjob, error) {
	if issueID != m.issue.ID {
		return nil, nil, fmt.Errorf("Unknown issued.")
	}

	if buildBucketID != m.tryjob.BuildBucketID {
		return nil, nil, fmt.Errorf("Unknown buildbucket id.")
	}

	// Make sure the issue tryjob are in the store.
	if err := m.tryjobStore.UpdateIssue(m.issue); err != nil {
		return nil, nil, err
	}

	if err := m.tryjobStore.UpdateTryjob(m.issue.ID, m.tryjob); err != nil {
		return nil, nil, err
	}

	return m.issue, m.tryjob, nil
}
