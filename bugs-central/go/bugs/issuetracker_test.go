package bugs

import (
	"context"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/testutils/unittest"
)

const (
	gcsTestBucket   = "skia-infra-testdata"
	resultsFileName = "issuetracker-results.json"
)

func TestIssueTrackerSearch(t *testing.T) {
	unittest.LargeTest(t)
	ctx := context.Background()

	// The test bucket is a public bucket, so we don't need to worry about authentication.
	unauthedClient := httputils.DefaultClientConfig().Client()

	storageClient, err := storage.NewClient(ctx, option.WithHTTPClient(unauthedClient))
	require.NoError(t, err)
	IssueTrackerBucket = gcsTestBucket
	ResultsFileName = resultsFileName

	qc := IssueTrackerQueryConfig{
		Query:  "componentid:1346 status:open",
		Client: "Android",
	}
	it, err := InitIssueTracker(storageClient)
	require.NoError(t, err)
	issues, countsData, err := it.Search(ctx, qc)
	require.NoError(t, err)
	require.Equal(t, 24, len(issues))
	require.Equal(t, 0, countsData.P0Count)
	require.Equal(t, 0, countsData.P1Count)
	require.Equal(t, 11, countsData.P2Count)
	require.Equal(t, 4, countsData.P3Count)
	require.Equal(t, 9, countsData.P4Count)

	// Use a query that does not match. Should throw an error.
	qc.Query = "does not match"
	_, _, err = it.Search(ctx, qc)
	require.Error(t, err)
}
