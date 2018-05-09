package tryjobstore

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"sort"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"google.golang.org/api/option"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/types"
)

func TestCloudTryjobStore(t *testing.T) {
	testutils.LargeTest(t)
	t.Skip()

	// If a service account file is in the environment then connect to the real datastore.
	serviceAccountFile := os.Getenv("DS_SERVICE_ACCOUNT_FILE")
	if serviceAccountFile != "" {
		// Construct a namespace based on the user.
		currUser, err := user.Current()
		assert.NoError(t, err)
		nameSpace := fmt.Sprintf("gold-localhost-%s-testing", currUser.Username)
		assert.NoError(t, ds.InitWithOpt(common.PROJECT_ID, nameSpace, option.WithServiceAccountFile(serviceAccountFile)))
	} else {
		// Otherwise try and connect to a locally running emulator.
		cleanup := testutil.InitDatastore(t,
			ds.ISSUE,
			ds.TRYJOB,
			ds.TRYJOB_RESULT,
			ds.TRYJOB_EXP_CHANGE,
			ds.TEST_DIGEST_EXP)
		defer cleanup()
	}

	eventBus := eventbus.New()
	store, err := NewCloudTryjobStore(ds.DS, eventBus)
	assert.NoError(t, err)

	testTryjobStore(t, store)
}

func testTryjobStore(t *testing.T, store TryjobStore) {
	// Add the issue and two tryjobs to the store.
	issueID := int64(99)
	patchsetID := int64(1099)
	buildBucketID := int64(30099)

	// Note: Cloud datastore only stores up to microseconds correctly, so if we
	// kept the time down to nanoseconds the test would fail. So we drop everything
	// smaller than a second.
	nowSec := time.Unix(time.Now().Unix(), 0)
	tryjob_1 := &Tryjob{
		IssueID:       issueID,
		PatchsetID:    patchsetID,
		Builder:       "Test-Builder-1",
		BuildBucketID: buildBucketID,
		Status:        TRYJOB_RUNNING,
		Updated:       nowSec,
	}

	patchsetID_2 := int64(1200)
	buildBucketID_2 := int64(30199)
	tryjob_2 := &Tryjob{
		IssueID:       issueID,
		PatchsetID:    patchsetID_2,
		Builder:       "Test-Builder-2",
		BuildBucketID: buildBucketID_2,
		Status:        TRYJOB_COMPLETE,
		Updated:       nowSec,
	}

	// Delete the tryjobs from the datastore.
	assert.NoError(t, store.DeleteIssue(issueID))
	fmt.Printf("Deleted tryjobs. Starting to insert.\n")
	defer func() {
		assert.NoError(t, store.DeleteIssue(issueID))
	}()
	time.Sleep(5 * time.Second)

	issue := &Issue{
		ID:      issueID,
		Subject: "Test issue",
		Owner:   "jdoe@example.com",
		Updated: time.Now(),
		Status:  "",
		PatchsetDetails: []*PatchsetDetail{
			{ID: patchsetID},
			{ID: patchsetID_2},
		},
	}
	assert.NoError(t, store.UpdateIssue(issue))

	expChangeKeys, _, err := store.(*cloudTryjobStore).getExpChangesForIssue(issueID, -1, -1, true)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(expChangeKeys))

	// Insert the tryjobs into the datastore.
	assert.NoError(t, store.UpdateTryjob(0, tryjob_1, nil))
	found, err := store.GetTryjob(issueID, buildBucketID)
	assert.NoError(t, err)
	assert.Equal(t, tryjob_1.Updated, found.Updated)
	assert.Equal(t, tryjob_1, found)
	assert.NoError(t, store.UpdateTryjob(0, tryjob_2, nil))

	time.Sleep(5 * time.Second)
	foundIssue, err := store.GetIssue(issueID, true, nil)
	assert.NoError(t, err)
	assert.NotNil(t, foundIssue)
	foundTryjobs := []*Tryjob{}
	for _, ps := range foundIssue.PatchsetDetails {
		foundTryjobs = append(foundTryjobs, ps.Tryjobs...)
	}
	assert.Equal(t, []*Tryjob{tryjob_1, tryjob_2}, foundTryjobs)

	listedIssues, total, err := store.ListIssues(0, 1000)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(listedIssues))
	assert.Equal(t, 1, total)
	checkEqualIssue(t, issue, listedIssues[0])

	// Generate instances of results
	allTryjobs := []*Tryjob{tryjob_1, tryjob_2}
	tryjobResults := make([][]*TryjobResult, len(allTryjobs), len(allTryjobs))
	for idx, tj := range allTryjobs {
		digestStart := int64((idx + 1) * 1000)
		results := []*TryjobResult{}

		for i := 0; i < 5; i++ {
			digestStr := fmt.Sprintf("%010d", digestStart+int64(i))
			testName := fmt.Sprintf("test-%d", i%5)
			results = append(results, &TryjobResult{
				Digest:   "digest-" + digestStr,
				TestName: testName,
				Params: map[string][]string{
					"name":    []string{testName},
					"param-1": []string{"value-1-1-" + digestStr, "value-1-2-" + digestStr},
					"param-2": []string{"value-2-1" + digestStr, "value-2-2-" + digestStr},
				},
			})
		}
		assert.NoError(t, store.UpdateTryjobResult(tj, results))
		tryjobResults[idx] = results
	}

	time.Sleep(5 * time.Second)

	foundTJs, foundTJResults, err := store.GetTryjobResults(issueID, []int64{patchsetID, patchsetID_2}, false)
	assert.NoError(t, err)
	assert.Equal(t, len(allTryjobs), len(foundTJs))
	for idx := range allTryjobs {
		assert.Equal(t, allTryjobs[idx], foundTJs[idx])
		tjr := foundTJResults[idx]
		sort.Slice(tjr, func(i, j int) bool { return tjr[i].Digest < tjr[j].Digest })
		assert.Equal(t, tryjobResults[idx], tjr)
	}

	// Add changes to the issue
	allChanges := expstorage.NewExpectations()
	expLogEntries := []*expstorage.TriageLogEntry{}
	userName := "jdoe@example.com"
	for i := 0; i < 5; i++ {
		triageDetails := []*expstorage.TriageDetail{}
		changes := expstorage.NewExpectations()
		for testCount := 0; testCount < 5; testCount++ {
			testName := fmt.Sprintf("test-%04d", testCount)
			for digestCount := 0; digestCount < 5; digestCount++ {
				digest := fmt.Sprintf("digest-%04d-%04d", testCount, digestCount)
				label := types.Label((i + testCount + digestCount) % 3)
				changes.SetTestExpectation(testName, digest, label)
				triageDetails = append(triageDetails, &expstorage.TriageDetail{
					TestName: testName, Digest: digest, Label: label.String(),
				})
			}
		}
		assert.NoError(t, store.AddChange(issueID, changes.Tests, userName))
		allChanges.AddDigests(changes.Tests)
		expLogEntries = append(expLogEntries, &expstorage.TriageLogEntry{
			Name: userName, ChangeCount: len(triageDetails), Details: triageDetails,
		})
		time.Sleep(2 * time.Second)
	}

	time.Sleep(30 * time.Second)
	foundExp, err := store.GetExpectations(issueID)
	assert.NoError(t, err)
	assert.Equal(t, allChanges, foundExp)

	logEntries, total, err := store.QueryLog(issueID, 0, -1, false)
	assert.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Equal(t, 5, len(logEntries))

	// Flip all expectations to untriaged.
	for _, digests := range foundExp.Tests {
		for digest := range digests {
			digests[digest] = types.UNTRIAGED
		}
	}

	assert.NoError(t, store.AddChange(issueID, foundExp.Tests, userName))
	time.Sleep(5 * time.Second)
	untriagedExp, err := store.GetExpectations(issueID)
	assert.NoError(t, err)
	assert.Equal(t, foundExp, untriagedExp)

	// Test commiting where the commit fails.
	assert.Error(t, store.CommitIssueExp(issueID, func() error {
		return errors.New("Write failed")
	}))
	foundIssue, err = store.GetIssue(issueID, false, nil)
	assert.NoError(t, err)
	assert.False(t, foundIssue.Commited)

	// Test commiting the changes.
	assert.NoError(t, store.CommitIssueExp(issueID, func() error {
		// Assume that writing the master baseline works.
		return nil
	}))

	foundIssue, err = store.GetIssue(issueID, false, nil)
	assert.NoError(t, err)
	assert.True(t, foundIssue.Commited)
}

func checkEqualIssue(t *testing.T, exp *Issue, actual *Issue) {
	expCp := *exp
	actCp := *actual

	expCp.Updated = normalizeTimeToMs(expCp.Updated)
	actCp.Updated = normalizeTimeToMs(actCp.Updated)
	assert.Equal(t, &expCp, &actCp)
}

func normalizeTimeToMs(t time.Time) time.Time {
	unixNano := t.UnixNano()
	secs := unixNano / int64(time.Second)
	newNanoRemainder := ((unixNano % int64(time.Second)) / int64(time.Millisecond)) * int64(time.Millisecond)
	return time.Unix(secs, newNanoRemainder)
}
