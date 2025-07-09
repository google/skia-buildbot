package issuetracker

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/bugs-central/go/bugs"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/gcs/mem_gcsclient"
)

func TestIssueTrackerSearch(t *testing.T) {
	ctx := context.Background()

	storageClient := mem_gcsclient.New("fake-bucket")
	require.NoError(t, storageClient.SetFileContents(ctx, resultsFileName, gcs.FileWriteOptions{}, []byte(testResultsContents)))
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

const testResultsContents = `{
    "componentid:1346 status:open": [
        {
            "id": 170281898,
            "status": "ASSIGNED",
            "priority": "P4",
            "assignee": "skia-android-triage@google.com",
            "created_ts": 1602060687,
            "modified_ts": 1602518120
        },
        {
            "id": 169248132,
            "status": "ASSIGNED",
            "priority": "P4",
            "assignee": "scroggo@google.com",
            "created_ts": 1600882072,
            "modified_ts": 1600978467
        },
        {
            "id": 169135830,
            "status": "ASSIGNED",
            "priority": "P2",
            "assignee": "djsollen@google.com",
            "created_ts": 1600780807,
            "modified_ts": 1601398644
        },
        {
            "id": 168500121,
            "status": "ASSIGNED",
            "priority": "P4",
            "assignee": "egdaniel@google.com",
            "created_ts": 1600104005,
            "modified_ts": 1602769379
        },
        {
            "id": 167743764,
            "status": "ASSIGNED",
            "priority": "P4",
            "assignee": "skia-android-triage@google.com",
            "created_ts": 1599190439,
            "modified_ts": 1601401004
        },
        {
            "id": 167484937,
            "status": "ASSIGNED",
            "priority": "P4",
            "assignee": "scroggo@google.com",
            "created_ts": 1599019657,
            "modified_ts": 1601401113
        },
        {
            "id": 167423351,
            "status": "ACCEPTED",
            "priority": "P2",
            "assignee": "scroggo@google.com",
            "created_ts": 1598990092,
            "modified_ts": 1599686584
        },
        {
            "id": 167162080,
            "status": "NEW",
            "priority": "P2",
            "assignee": "",
            "created_ts": 1598864117,
            "modified_ts": 1598915429
        },
        {
            "id": 163494562,
            "status": "ASSIGNED",
            "priority": "P3",
            "assignee": "djsollen@google.com",
            "created_ts": 1597151142,
            "modified_ts": 1601398807
        },
        {
            "id": 162229548,
            "status": "ASSIGNED",
            "priority": "P4",
            "assignee": "skia-android-triage@google.com",
            "created_ts": 1595865547,
            "modified_ts": 1601401005
        },
        {
            "id": 161926977,
            "status": "ASSIGNED",
            "priority": "P2",
            "assignee": "csmartdalton@google.com",
            "created_ts": 1595529795,
            "modified_ts": 1597763846
        },
        {
            "id": 160012732,
            "status": "ASSIGNED",
            "priority": "P4",
            "assignee": "skia-android-triage@google.com",
            "created_ts": 1593204638,
            "modified_ts": 1601401005
        },
        {
            "id": 159246162,
            "status": "ASSIGNED",
            "priority": "P3",
            "assignee": "scroggo@google.com",
            "created_ts": 1592428515,
            "modified_ts": 1595755318
        },
        {
            "id": 157410317,
            "status": "ASSIGNED",
            "priority": "P4",
            "assignee": "jcgregorio@google.com",
            "created_ts": 1590415473,
            "modified_ts": 1590520044
        },
        {
            "id": 156373344,
            "status": "ASSIGNED",
            "priority": "P2",
            "assignee": "borenet@google.com",
            "created_ts": 1589304770,
            "modified_ts": 1589899925
        },
        {
            "id": 154245216,
            "status": "ASSIGNED",
            "priority": "P4",
            "assignee": "scroggo@google.com",
            "created_ts": 1587157519,
            "modified_ts": 1587157519
        },
        {
            "id": 148177699,
            "status": "ASSIGNED",
            "priority": "P2",
            "assignee": "scroggo@google.com",
            "created_ts": 1579739484,
            "modified_ts": 1581531363
        },
        {
            "id": 146149527,
            "status": "ASSIGNED",
            "priority": "P3",
            "assignee": "fmalita@google.com",
            "created_ts": 1576265009,
            "modified_ts": 1603121065
        },
        {
            "id": 145280862,
            "status": "ASSIGNED",
            "priority": "P2",
            "assignee": "skia-android-triage@google.com",
            "created_ts": 1574871965,
            "modified_ts": 1603206776
        },
        {
            "id": 129885946,
            "status": "ASSIGNED",
            "priority": "P2",
            "assignee": "ethannicholas@google.com",
            "created_ts": 1554328497,
            "modified_ts": 1593448678
        },
        {
            "id": 123699380,
            "status": "ASSIGNED",
            "priority": "P2",
            "assignee": "djsollen@google.com",
            "created_ts": 1548959512,
            "modified_ts": 1601398674
        },
        {
            "id": 122840906,
            "status": "ASSIGNED",
            "priority": "P2",
            "assignee": "scroggo@google.com",
            "created_ts": 1547497777,
            "modified_ts": 1580393320
        },
        {
            "id": 78883721,
            "status": "ASSIGNED",
            "priority": "P3",
            "assignee": "nifong@google.com",
            "created_ts": 1525107463,
            "modified_ts": 1602612509
        },
        {
            "id": 77985528,
            "status": "ASSIGNED",
            "priority": "P2",
            "assignee": "jvanverth@google.com",
            "created_ts": 1523603582,
            "modified_ts": 1585674756
        }
    ]
}`
