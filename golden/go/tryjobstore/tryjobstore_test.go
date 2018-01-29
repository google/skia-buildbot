package tryjobstore

import (
	"fmt"
	"sort"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/types"
)

func TestCloudTryjobStore(t *testing.T) {
	testutils.LargeTest(t)

	cleanup := testutil.InitDatastore(t,
		ds.ISSUE,
		ds.TRYJOB,
		ds.TRYJOB_RESULT,
		ds.TRYJOB_EXP_CHANGE,
		ds.TEST_DIGEST_EXP)
	defer cleanup()

	store, err := NewCloudTryjobStore(ds.DS, nil)
	assert.NoError(t, err)

	testTryjobStore(t, store)
}

func testTryjobStore(t *testing.T, store TryjobStore) {
	// Add a two tryjobs and add them to the store.
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
	time.Sleep(10 * time.Second)

	expChangeKeys, _, err := store.(*cloudTryjobStore).getExpChangesForIssue(issueID, -1, -1, true)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(expChangeKeys))

	// Insert the tryjobs into the datastore.
	assert.NoError(t, store.UpdateTryjob(issueID, tryjob_1))
	found, err := store.GetTryjob(issueID, buildBucketID)
	assert.NoError(t, err)
	assert.Equal(t, tryjob_1.Updated, found.Updated)
	assert.Equal(t, tryjob_1, found)
	assert.NoError(t, store.UpdateTryjob(issueID, tryjob_2))

	// Generate instances of results
	allTryjobs := []*Tryjob{tryjob_1, tryjob_2}
	tryjobResults := make([][]*TryjobResult, len(allTryjobs), len(allTryjobs))
	for idx, tj := range allTryjobs {
		digestStart := int64((idx + 1) * 1000)
		results := []*TryjobResult{}

		for i := 0; i < 5; i++ {
			digestStr := fmt.Sprintf("%010d", digestStart+int64(i))
			testName := fmt.Sprintf("%d", i%5)
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

	time.Sleep(10 * time.Second)
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
	time.Sleep(10 * time.Second)
	untriagedExp, err := store.GetExpectations(issueID)
	assert.NoError(t, err)
	assert.Equal(t, foundExp, untriagedExp)
}
