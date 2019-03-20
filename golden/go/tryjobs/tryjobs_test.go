package tryjobs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/ds"
	ds_testutil "go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/golden/go/tryjobstore"
)

type MyGerritMock struct {
	*gerrit.MockedGerrit
}

func (m *MyGerritMock) AddComment(issue *gerrit.ChangeInfo, message string) error {
	m.Called(issue, message)
	return nil
}

func TestWriteGoldLinkToGerrit(t *testing.T) {
	testutils.LargeTest(t)

	// Initialize the cloud datastore
	initDS(t)
	client := ds.DS

	issueID := int64(12345)
	buildBucketID := int64(7654321)
	eventBus := eventbus.New()
	mockGerrit := &MyGerritMock{MockedGerrit: &gerrit.MockedGerrit{IssueID: issueID}}
	tjStore, err := tryjobstore.NewCloudTryjobStore(client, nil, eventBus)
	assert.NoError(t, err)

	siteURL := "https://gold.skia.org"
	tryjobMonitor := NewTryjobMonitor(tjStore, nil, nil, mockGerrit, siteURL, eventBus, true)
	_ = tryjobMonitor

	issue1 := &tryjobstore.Issue{
		ID:      issueID,
		Subject: "Test issue",
		Owner:   "jdoe@example.com",
		Updated: time.Now(),
		Status:  "",
	}
	assert.NoError(t, tjStore.UpdateIssue(issue1, nil))

	tryjob1 := &tryjobstore.Tryjob{
		BuildBucketID: buildBucketID,
		IssueID:       issueID,
		PatchsetID:    12345,
		Builder:       "Test-Builder",
		Status:        tryjobstore.TRYJOB_INGESTED,
	}

	waitCh := make(chan bool)
	gerritIssue, err := mockGerrit.GetIssueProperties(issueID)
	assert.NoError(t, err)
	gerritMsg := tryjobMonitor.getGerritMsg(issueID)
	mockGerrit.On("AddComment", gerritIssue, gerritMsg).Run(func(args mock.Arguments) {
		close(waitCh)
	})

	assert.NoError(t, tjStore.UpdateTryjob(1234567, tryjob1, nil))
	<-waitCh

	// The wait here is necessary to finish writing to the TryjobStore in WriteGoldLinkToGerrit
	time.Sleep(time.Second)
	mockGerrit.AssertCalled(t, "AddComment", gerritIssue, gerritMsg)

	foundIssue, err := tjStore.GetIssue(issueID, false)
	assert.NoError(t, err)
	assert.True(t, foundIssue.CommentAdded)

	// Call directly and make sure there is no error.
	assert.NoError(t, tryjobMonitor.WriteGoldLinkToGerrit(issueID))

	// Call with an invalid issue and make sure we get an error.
	assert.Error(t, tryjobMonitor.WriteGoldLinkToGerrit(999999))
}

// initDS initializes the datastore for testing.
func initDS(t *testing.T, kinds ...ds.Kind) func() {
	kinds = append([]ds.Kind{
		ds.MASTER_EXP_CHANGE,
		ds.TRYJOB_EXP_CHANGE,
		ds.TRYJOB_TEST_DIGEST_EXP,
		ds.HELPER_RECENT_KEYS,
		ds.EXPECTATIONS_BLOB_ROOT,
		ds.EXPECTATIONS_BLOB,
	}, kinds...)
	return ds_testutil.InitDatastore(t, kinds...)
}
