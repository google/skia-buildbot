package tryjobs

import (
	"sync"
	"testing"
	"time"

	"go.skia.org/infra/go/sklog"

	"github.com/stretchr/testify/mock"

	"go.skia.org/infra/golden/go/tryjobstore"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/ds"
	ds_testutil "go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/testutils"
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
	tryjobStore, err := tryjobstore.NewCloudTryjobStore(client, eventBus)
	assert.NoError(t, err)

	siteURL := "https://gold.skia.org"
	tryjobMonitor := NewTryjobMonitor(tryjobStore, mockGerrit, siteURL, eventBus)
	_ = tryjobMonitor

	issue1 := &tryjobstore.Issue{
		ID:      issueID,
		Subject: "Test issue",
		Owner:   "jdoe@example.com",
		Updated: time.Now(),
		Status:  "",
	}
	assert.NoError(t, tryjobStore.UpdateIssue(issue1, nil))

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

	assert.NoError(t, tryjobStore.UpdateTryjob(1234567, tryjob1, nil))
	<-waitCh

	// The wait here is necessary to finis the write the tryjobStore in WriteGoldLinkToGerrit
	time.Sleep(time.Second)
	mockGerrit.AssertCalled(t, "AddComment", gerritIssue, gerritMsg)

	foundIssue, err := tryjobStore.GetIssue(issueID, false)
	assert.NoError(t, err)
	assert.True(t, foundIssue.CommentAdded)

	// Call directly and make sure there is no error.
	assert.NoError(t, tryjobMonitor.WriteGoldLinkToGerrit(issueID))

	// Call with an invalid issue and make sure we get an error.
	assert.Error(t, tryjobMonitor.WriteGoldLinkToGerrit(999999))
}

func TestCondInt64Monitor(t *testing.T) {
	testutils.MediumTest(t)

	// Define the id range and the number of concurrent calls for each id.
	nFnCalls := 50
	nIDs := 50
	mon := NewCondInt64Monitor(1)
	concurMap := sync.Map{}
	errCh := make(chan error, nFnCalls*nIDs)
	var wg sync.WaitGroup
	fn := func(id, callID int64) {
		defer wg.Done()
		defer mon.Enter(id).Release()

		val, _ := concurMap.LoadOrStore(id, 0)
		concurMap.Store(id, val.(int)+1)
		time.Sleep(10 * time.Millisecond)
		val, _ = concurMap.Load(id)
		if val.(int) > 1 {
			errCh <- sklog.FmtErrorf("More than one thread with the same ID entered the critical section")
		}
		val, _ = concurMap.Load(id)
		concurMap.Store(id, val.(int)-1)
	}

	// Make lots of function calls
	for id := 1; id < nIDs+1; id++ {
		for callIdx := 0; callIdx < nFnCalls; callIdx++ {
			wg.Add(1)
			go fn(int64(id), int64(callIdx))
		}
	}
	wg.Wait()
	close(errCh)

	// Note: This will fail for the first error we encountered. That's ok.
	for err := range errCh {
		assert.NoError(t, err)
	}
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
