package rietveld

import (
	"fmt"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
)

// Basic test to make sure we can retrieve issues from Rietveld.
// Note: Below test is disabled because it expects 5 issues to be modified in
//       the last hour. This is not always true.
func SKIP_TestRietveld(t *testing.T) {
	testutils.LargeTest(t)
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

func TestUrlAndExtractIssue(t *testing.T) {
	testutils.SmallTest(t)
	api := New(RIETVELD_SKIA_URL, nil)
	assert.Equal(t, RIETVELD_SKIA_URL, api.Url(0))
	url1 := api.Url(1234)
	assert.Equal(t, fmt.Sprintf("%s/%d", RIETVELD_SKIA_URL, 1234), url1)
	found, ok := api.ExtractIssue(url1)
	assert.True(t, ok)
	assert.Equal(t, "1234", found)
	found, ok = api.ExtractIssue(fmt.Sprintf("%s/c/%d", RIETVELD_SKIA_URL, 1234))
	assert.Equal(t, "", found)
	assert.False(t, ok)
	found, ok = api.ExtractIssue("random string")
	assert.Equal(t, "", found)
	assert.False(t, ok)
}
