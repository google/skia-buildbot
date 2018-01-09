package tryjobstore

import (
	"encoding/json"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils"
)

func TestIssueDetails(t *testing.T) {
	testutils.SmallTest(t)

	issue := &IssueDetails{
		Issue: &Issue{
			ID:      12345,
			Subject: "Test Subject",
			Owner:   "jdoe@example.com",
			Updated: time.Now(),
			URL:     "https://cr.example.com",
		},
	}

	firstPS := &PatchsetDetail{ID: 34567}
	seconPS := &PatchsetDetail{ID: 23456}
	thirdPS := &PatchsetDetail{ID: 44567}

	issue.UpdatePatchsets([]*PatchsetDetail{firstPS})
	assert.Equal(t, firstPS, issue.PatchsetDetails[0])

	// Add the first again and the others.
	issue.UpdatePatchsets([]*PatchsetDetail{firstPS})
	issue.UpdatePatchsets([]*PatchsetDetail{seconPS})
	issue.UpdatePatchsets([]*PatchsetDetail{thirdPS})
	issue.UpdatePatchsets([]*PatchsetDetail{firstPS})
	issue.UpdatePatchsets([]*PatchsetDetail{seconPS})
	issue.UpdatePatchsets([]*PatchsetDetail{thirdPS})
	issue.UpdatePatchsets([]*PatchsetDetail{firstPS})
	issue.UpdatePatchsets([]*PatchsetDetail{seconPS})
	issue.UpdatePatchsets([]*PatchsetDetail{thirdPS})

	// Make sure we are sorted.
	testutils.AssertDeepEqual(t, []*PatchsetDetail{seconPS, firstPS, thirdPS}, issue.PatchsetDetails)
	assert.NoError(t, nil)
}

func TestSerialize(t *testing.T) {
	testutils.SmallTest(t)
	status := TryjobStatus(TRYJOB_INGESTED)
	jsonStatus, err := json.Marshal(status)
	assert.NoError(t, err)
	assert.Equal(t, "\"ingested\"", string(jsonStatus))
}
