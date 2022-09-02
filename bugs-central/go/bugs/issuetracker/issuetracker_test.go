package issuetracker

import (
	"context"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"

	"go.skia.org/infra/bugs-central/go/bugs"
	"go.skia.org/infra/go/httputils"
)

const (
	gcsTestBucket       = "skia-infra-testdata"
	testResultsFileName = "issuetracker-results.json"
)

func TestIssueTrackerSearch(t *testing.T) {
	ctx := context.Background()

	// The test bucket is a public bucket, so we don't need to worry about authentication.
	unauthedClient := httputils.DefaultClientConfig().Client()

	storageClient, err := storage.NewClient(ctx, option.WithHTTPClient(unauthedClient))
	require.NoError(t, err)
	issueTrackerBucket = gcsTestBucket
	resultsFileName = testResultsFileName

	it, err := New(storageClient, bugs.InitOpenIssues(), &IssueTrackerQueryConfig{
		Query:  "componentid:1346 status:open",
		Client: "Android",
	})
	require.NoError(t, err)
	issues, countsData, err := it.Search(ctx)
	require.NoError(t, err)
	require.Equal(t, 24, len(issues))
	require.Equal(t, 0, countsData.P0Count)
	require.Equal(t, 0, countsData.P1Count)
	require.Equal(t, 11, countsData.P2Count)
	require.Equal(t, 4, countsData.P3Count)
	require.Equal(t, 9, countsData.P4Count)

	// Use a query that does not match. Should throw an error.
	it, err = New(storageClient, bugs.InitOpenIssues(), &IssueTrackerQueryConfig{
		Query:  "does not match",
		Client: "Android",
	})
	require.NoError(t, err)
	_, _, err = it.Search(ctx)
	require.Error(t, err)
}
