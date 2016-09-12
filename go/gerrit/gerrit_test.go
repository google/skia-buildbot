package gerrit

import (
	"fmt"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
)

// Basic test to make sure we can retrieve issues from Gerrit.
func TestGerritSearch(t *testing.T) {
	testutils.SkipIfShort(t)

	api := NewGerrit(GERRIT_SKIA_URL, nil)
	t_delta := time.Now().Add(-10 * 24 * time.Hour)
	issues, err := api.Search(1, api.SearchModifiedAfter(t_delta))
	assert.NoError(t, err)
	assert.True(t, len(issues) > 0)

	for _, issue := range issues {
		assert.True(t, issue.Updated.After(t_delta))
		details, err := api.GetIssueProperties(issue.ChangeId)
		assert.NoError(t, err)
		assert.True(t, details.Updated.After(t_delta))
		assert.True(t, len(details.Patchsets) > 0)
	}

	issues, err = api.Search(2, api.SearchModifiedAfter(time.Now().Add(-time.Hour)))
	assert.NoError(t, err)
	assert.Equal(t, 2, len(issues))
}

// Basic poller test.
func GerritPollerTest(t *testing.T) {
	testutils.SkipIfShort(t)

	api := NewGerrit(GERRIT_SKIA_URL, nil)
	cache := NewCodeReviewCache(api, 10*time.Second, 3)
	fmt.Println(cache.Get("I50682ad635c352308c5689514c520f6e159f23be"))
	c1 := ChangeInfo{
		ChangeId: "I50682ad635c352308c5689514c520f6e159f23be",
	}
	cache.Add("I50682ad635c352308c5689514c520f6e159f23be", &c1)
	fmt.Println(cache.Get("I50682ad635c352308c5689514c520f6e159f23be"))
	time.Sleep(time.Hour)
}

// Basic test to make sure we can retrieve issues from Rietveld.
func TestRietveld(t *testing.T) {
	testutils.SkipIfShort(t)

	api := New("https://codereview.chromium.org", nil)
	t_delta := time.Now().Add(-10 * 24 * time.Hour)
	issues, err := api.Search(1, SearchModifiedAfter(t_delta))
	assert.NoError(t, err)
	assert.True(t, len(issues) > 0)

	for _, issue := range issues {
		assert.True(t, issue.Modified.After(t_delta))
		details, err := api.GetIssueProperties(issue.Issue, false)
		assert.NoError(t, err)
		assert.True(t, details.Modified.After(t_delta))
		assert.True(t, len(details.Patchsets) > 0)
	}

	keys, err := api.SearchKeys(5, SearchModifiedAfter(time.Now().Add(-time.Hour)))
	assert.NoError(t, err)
	assert.Equal(t, 5, len(keys))
}
