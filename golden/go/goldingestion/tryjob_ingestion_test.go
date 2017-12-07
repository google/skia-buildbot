package goldingestion

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/ingestion"

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

	testIssue := &tryjobstore.IssueDetails{
		Issue: &tryjobstore.Issue{
			ID:      81300,
			Subject: "[infra] Move commands from isolates to gen_tasks.go",
			Owner:   "someone@example.com",
			Status:  "MERGED",
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
	}

	mockedIBF := &mockIBF{issue: testIssue, tryjob: testTryjob}
	memTryjobStore := tryjobstore.NewMemTryjobStore()

	processor := &goldTryjobProcessor{
		issueBuildFetcher: mockedIBF,
		tryjobStore:       memTryjobStore,
	}

	// Call process for the input file.
	fsResult, err := ingestion.FileSystemResult(TRYJOB_INGESTION_FILE, TEST_DATA_DIR)
	assert.NoError(t, err)
	assert.NoError(t, processor.Process(context.Background(), fsResult))

	foundIssue, err := memTryjobStore.GetIssue(testIssue.ID, false, nil)
	assert.NoError(t, err)
	assert.Equal(t, testIssue, foundIssue)

	foundTryjob, err := memTryjobStore.GetTryjob(testIssue.ID, testTryjob.BuildBucketID)
	assert.NoError(t, err)
	assert.Equal(t, testTryjob, foundTryjob)
}

type mockIBF struct {
	issue  *tryjobstore.IssueDetails
	tryjob *tryjobstore.Tryjob
}

func (m *mockIBF) FetchIssueAndTryjob(issueID, buildBucketID int64) (*tryjobstore.IssueDetails, *tryjobstore.Tryjob, error) {
	if issueID != m.issue.ID {
		return nil, nil, fmt.Errorf("Unknown issued.")
	}

	if buildBucketID != m.tryjob.BuildBucketID {
		return nil, nil, fmt.Errorf("Unknown buildbucket id.")
	}

	return m.issue, m.tryjob, nil
}
