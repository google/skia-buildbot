package trybotstore

import (
	"fmt"
	"sort"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/common"
)

func TestCloudTrybotStore(t *testing.T) {
	serviceAccountFile := "./service-account.json"
	// client, err := auth.NewJWTServiceAccountClient("", "./service-account.json", nil, gstorage.CloudPlatformScope)
	// assert.NoError(t, err)

	store, err := NewCloudTrybotStore(common.PROJECT_ID, "gold-testing-tarock", serviceAccountFile)
	assert.NoError(t, err)

	testTrybotStore(t, store)
}

func testTrybotStore(t *testing.T, store TrybotStore) {
	// Add a two tryjobs and add them to the store.
	issueID := int64(99)
	patchsetID := int64(1099)
	buildBucketID := int64(30099)
	tryjob_1 := &Tryjob{
		IssueID:       issueID,
		PatchsetID:    patchsetID,
		Builder:       "Test-Builder-1",
		BuildBucketID: buildBucketID,
		Status:        TRYJOB_RUNNING,
	}

	patchsetID_2 := int64(1200)
	buildBucketID_2 := int64(30199)
	tryjob_2 := &Tryjob{
		IssueID:       issueID,
		PatchsetID:    patchsetID_2,
		Builder:       "Test-Builder-2",
		BuildBucketID: buildBucketID_2,
		Status:        TRYJOB_COMPLETE,
	}

	// Delete the tryjobs from the datastore.
	assert.NoError(t, store.(*cloudTrybotStore).deleteTryjobsForIssue(issueID))
	fmt.Printf("Deleted tryjobs. Waiting.")
	time.Sleep(30 * time.Second)

	// Insert the tryjobs into the datastore.
	assert.NoError(t, store.UpdateTryjob(issueID, tryjob_1))
	found, err := store.GetTryjob(issueID, buildBucketID)
	assert.NoError(t, err)
	assert.Equal(t, tryjob_1, found)
	assert.NoError(t, store.UpdateTryjob(issueID, tryjob_2))

	// Generate instances of results
	allTryjobs := []*Tryjob{tryjob_1, tryjob_2}
	tryjobResults := make([][]*TryjobResult, len(allTryjobs), len(allTryjobs))
	for idx, tj := range allTryjobs {
		digestStart := int64((idx + 1) * 1000)
		results := []*TryjobResult{}

		for i := 0; i < 10000; i++ {
			digestStr := fmt.Sprintf("%010d", digestStart+int64(i))
			results = append(results, &TryjobResult{
				Digest: "digest-" + digestStr,
				Params: map[string][]string{
					"param-1-" + digestStr: []string{"value-1-" + digestStr, "value-2-" + digestStr},
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
}
