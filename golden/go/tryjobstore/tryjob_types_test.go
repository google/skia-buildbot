package tryjobstore

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestIssueDetails(t *testing.T) {
	unittest.SmallTest(t)

	issue := &Issue{
		ID:      12345,
		Subject: "Test Subject",
		Owner:   "jdoe@example.com",
		Updated: time.Now(),
		URL:     "https://cr.example.com",
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
	deepequal.AssertDeepEqual(t, []*PatchsetDetail{seconPS, firstPS, thirdPS}, issue.PatchsetDetails)
	assert.NoError(t, nil)
}

func TestSerialize(t *testing.T) {
	unittest.SmallTest(t)
	status := TryjobStatus(TRYJOB_INGESTED)
	jsonStatus, err := json.Marshal(status)
	assert.NoError(t, err)
	assert.Equal(t, "\"ingested\"", string(jsonStatus))
}

func TestTimeJsonMs(t *testing.T) {
	unittest.SmallTest(t)

	now := TimeJsonMs(time.Now())
	expMs := fmt.Sprintf("%d", time.Time(now).UnixNano()/int64(time.Millisecond))

	jsonBytes, err := json.Marshal(now)
	assert.NoError(t, err)
	assert.Equal(t, expMs, string(jsonBytes))

	tjs := []*Issue{
		{Updated: time.Time(now)},
	}

	jsonBytes, err = json.Marshal(tjs)
	assert.NoError(t, err)
	expField := `"updated":` + expMs
	assert.True(t, strings.Contains(string(jsonBytes), expField))
}
