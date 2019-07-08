package ds_tryjobstore

import (
	"errors"
	"fmt"
	"sort"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/tryjobstore"
	"go.skia.org/infra/golden/go/types"
)

func TestCloudTryjobStore(t *testing.T) {
	unittest.LargeTest(t)

	// Otherwise try and connect to a locally running emulator.
	cleanup := testutil.InitDatastore(t,
		ds.ISSUE,
		ds.TRYJOB,
		ds.TRYJOB_RESULT)
	defer cleanup()

	eventBus := eventbus.New()
	store, err := New(ds.DS, eventBus)
	assert.NoError(t, err)

	// Add the issue and two tryjobs to the store.
	issueID := int64(99)
	patchsetID := int64(1099)
	buildBucketID := int64(30099)

	// Note: Cloud datastore only stores up to microseconds correctly, so if we
	// kept the time down to nanoseconds the test would fail. So we drop everything
	// smaller than a second.
	nowSec := time.Unix(time.Now().Unix(), 0)
	tryjob_1 := &tryjobstore.Tryjob{
		IssueID:       issueID,
		PatchsetID:    patchsetID,
		Builder:       "Test-Builder-1",
		BuildBucketID: buildBucketID,
		Status:        tryjobstore.TRYJOB_RUNNING,
		Updated:       nowSec,
	}

	patchsetID_2 := int64(1200)
	buildBucketID_2 := int64(30199)
	tryjob_2 := &tryjobstore.Tryjob{
		IssueID:       issueID,
		PatchsetID:    patchsetID_2,
		Builder:       "Test-Builder-2",
		BuildBucketID: buildBucketID_2,
		Status:        tryjobstore.TRYJOB_COMPLETE,
		Updated:       nowSec,
	}

	buildBucketID_3 := int64(40199)
	tryjob_3 := &tryjobstore.Tryjob{
		IssueID:       issueID,
		PatchsetID:    patchsetID_2,
		Builder:       "Test-Builder-2",
		BuildBucketID: buildBucketID_3,
		Status:        tryjobstore.TRYJOB_COMPLETE,
		Updated:       nowSec.Add(-time.Hour),
	}

	// Delete the tryjobs from the datastore.
	issue := &tryjobstore.Issue{
		ID:      issueID,
		Subject: "Test issue",
		Owner:   "jdoe@example.com",
		Updated: time.Now(),
		Status:  "",
		PatchsetDetails: []*tryjobstore.PatchsetDetail{
			{ID: patchsetID},
			{ID: patchsetID_2},
		},
	}
	assert.NoError(t, store.UpdateIssue(issue, nil))

	// Insert the tryjobs into the datastore.
	assert.NoError(t, store.UpdateTryjob(0, tryjob_1, nil))
	found, err := store.GetTryjob(issueID, buildBucketID)
	assert.NoError(t, err)
	found.Key = nil
	assert.Equal(t, tryjob_1.Updated, found.Updated)
	assert.Equal(t, tryjob_1, found)
	assert.NoError(t, store.UpdateTryjob(0, tryjob_2, nil))

	expTryjobs := []*tryjobstore.Tryjob{tryjob_1, tryjob_2}
	foundTryjobs := []*tryjobstore.Tryjob{}
	assert.NoError(t, testutils.EventuallyConsistent(5*time.Second, func() error {
		foundIssue, err := store.GetIssue(issueID, true)
		assert.NoError(t, err)
		assert.NotNil(t, foundIssue)
		for _, ps := range foundIssue.PatchsetDetails {
			for _, tj := range ps.Tryjobs {
				tj.Key = nil
			}
			foundTryjobs = append(foundTryjobs, ps.Tryjobs...)
		}
		if len(foundTryjobs) != len(expTryjobs) {
			return testutils.TryAgainErr
		}
		return nil
	}))
	deepequal.AssertDeepEqual(t, expTryjobs, foundTryjobs)

	listedIssues, total, err := store.ListIssues(0, 1000)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(listedIssues))
	assert.Equal(t, 1, total)
	checkEqualIssue(t, issue, listedIssues[0])

	// Generate instances of results
	allTryjobs := []*tryjobstore.Tryjob{tryjob_1, tryjob_2}
	tryjobResults := make([][]*tryjobstore.TryjobResult, len(allTryjobs))
	for idx, tj := range allTryjobs {
		digestStart := int64((idx + 1) * 1000)
		results := []*tryjobstore.TryjobResult{}

		for i := 0; i < 5; i++ {
			digestStr := fmt.Sprintf("%010d", digestStart+int64(i))
			testName := fmt.Sprintf("test-%d", i%5)
			results = append(results, &tryjobstore.TryjobResult{
				BuildBucketID: tj.BuildBucketID,
				Digest:        types.Digest("digest-" + digestStr),
				TestName:      types.TestName(testName),
				Params: map[string][]string{
					"name":    {testName},
					"param-1": {"value-1-1-" + digestStr, "value-1-2-" + digestStr},
					"param-2": {"value-2-1" + digestStr, "value-2-2-" + digestStr},
				},
			})
		}
		assert.NoError(t, store.UpdateTryjobResult(results))
		tryjobResults[idx] = results
	}

	var foundTJs []*tryjobstore.Tryjob
	var foundTJResults [][]*tryjobstore.TryjobResult
	assert.NoError(t, testutils.EventuallyConsistent(3*time.Second, func() error {
		foundTJs, foundTJResults, err = store.GetTryjobs(issueID, []int64{patchsetID, patchsetID_2}, false, true)
		if err != nil {
			return err
		}
		if len(allTryjobs) != len(foundTJs) {
			return testutils.TryAgainErr
		}
		return nil
	}))
	for idx := range allTryjobs {
		foundTJs[idx].Key = nil
		assert.Equal(t, allTryjobs[idx], foundTJs[idx])
		tjr := foundTJResults[idx]
		sort.Slice(tjr, func(i, j int) bool { return tjr[i].Digest < tjr[j].Digest })
		assert.Equal(t, tryjobResults[idx], tjr)
	}

	// Add a redundant Tryjob make sure it's not filtered out.
	assert.NoError(t, store.UpdateTryjob(0, tryjob_3, nil))
	assert.NoError(t, testutils.EventuallyConsistent(10*time.Second, func() error {
		foundTJs, _, err := store.GetTryjobs(issueID, []int64{patchsetID, patchsetID_2}, false, false)
		assert.NoError(t, err)
		if len(foundTJs) == len(allTryjobs)+1 {
			return nil
		}
		return testutils.TryAgainErr
	}))

	foundTJs, _, err = store.GetTryjobs(issueID, []int64{patchsetID, patchsetID_2}, false, false)
	assert.NoError(t, err)
	assert.Equal(t, len(allTryjobs)+1, len(foundTJs))

	// Filter out duplicates
	foundTJs, _, err = store.GetTryjobs(issueID, []int64{patchsetID, patchsetID_2}, true, false)
	assert.NoError(t, err)
	assert.Equal(t, len(allTryjobs), len(foundTJs))
	for idx := range allTryjobs {
		foundTJs[idx].Key = nil
		assert.Equal(t, allTryjobs[idx], foundTJs[idx])
	}

	// Test committing where the commit fails.
	assert.Error(t, store.CommitIssueExp(issueID, func() error {
		return errors.New("Write failed")
	}))
	foundIssue, err := store.GetIssue(issueID, false)
	assert.NoError(t, err)
	assert.False(t, foundIssue.Committed)

	// Test committing the changes.
	assert.NoError(t, store.CommitIssueExp(issueID, func() error {
		// Assume that writing the master baseline works.
		return nil
	}))

	foundIssue, err = store.GetIssue(issueID, false)
	assert.NoError(t, err)
	assert.True(t, foundIssue.Committed)
}

func checkEqualIssue(t *testing.T, exp *tryjobstore.Issue, actual *tryjobstore.Issue) {
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
