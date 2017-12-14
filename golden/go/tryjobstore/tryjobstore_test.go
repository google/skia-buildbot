package tryjobstore

import (
	"fmt"
	"sort"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/types"
)

func TestCloudTryjobStore(t *testing.T) {
	testutils.MediumTest(t)

	// TODO(stephana): This test should be tested shomehow, probably by running
	// the simulator in the bot.
	t.Skip()

	serviceAccountFile := "./service-account.json"

	store, err := NewCloudTryjobStore(common.PROJECT_ID, "gold-localhost-stephana", serviceAccountFile)
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

	expChangeKeys, err := store.(*cloudTryjobStore).getExpChangesForIssue(issueID)
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

		for i := 0; i < 100; i++ {
			digestStr := fmt.Sprintf("%010d", digestStart+int64(i))
			results = append(results, &TryjobResult{
				Digest: "digest-" + digestStr,
				Params: map[string][]string{
					"param-1": []string{"value-1-1-" + digestStr, "value-1-2-" + digestStr},
					"param-2": []string{"value-2-1" + digestStr, "value-2-2-" + digestStr},
				},
			})
		}
		assert.NoError(t, store.UpdateTryjobResult(tj, results))
		tryjobResults[idx] = results
	}

	foundTJs, foundTJResults, err := store.GetTryjobResults(issueID, []int64{patchsetID, patchsetID_2})
	assert.NoError(t, err)
	for idx := range allTryjobs {
		assert.Equal(t, allTryjobs[idx], foundTJs[idx])
		tjr := foundTJResults[idx]
		sort.Slice(tjr, func(i, j int) bool { return tjr[i].Digest < tjr[j].Digest })
		assert.Equal(t, tryjobResults[idx], tjr)
	}

	// Add changes to the issue
	allChanges := expstorage.NewExpectations()
	for i := 0; i < 10; i++ {
		changes := expstorage.NewExpectations()
		for j := 0; j < 5; j++ {
			testName := fmt.Sprintf("test-%04d", j)
			for k := 0; k < 5; k++ {
				digest := fmt.Sprintf("digest-%04d-%04d", j, k)
				label := (i + j + k) % 3
				changes.SetTestExpectation(testName, digest, types.Label(label))
			}
		}
		assert.NoError(t, store.AddChange(issueID, changes.Tests, "jdoe@example.com"))
		allChanges.AddDigests(changes.Tests)
		time.Sleep(5 * time.Millisecond)
	}

	foundExp, err := store.GetExpectations(issueID)
	assert.NoError(t, err)
	assert.Equal(t, allChanges, foundExp)
}
