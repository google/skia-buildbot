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
		details, err := api.GetIssueProperties(issue.Issue)
		assert.NoError(t, err)
		assert.True(t, details.Updated.After(t_delta))
		assert.True(t, len(details.Patchsets) > 0)
	}

	issues, err = api.Search(2, api.SearchModifiedAfter(time.Now().Add(-time.Hour)))
	assert.NoError(t, err)
	//assert.Equal(t, 2, len(issues))
}

// Basic poller test.
func GerritPollerTest(t *testing.T) {
	testutils.SkipIfShort(t)

	api := NewGerrit(GERRIT_SKIA_URL, nil)
	cache := NewCodeReviewCache(api, 10*time.Second, 3)
	fmt.Println(cache.Get(2386))
	c1 := ChangeInfo{
		Issue: 2386,
	}
	cache.Add(2386, &c1)
	fmt.Println(cache.Get(2386))
	time.Sleep(time.Hour)
}

func TestGetTrybotResults(t *testing.T) {
	api := NewGerrit(GERRIT_SKIA_URL, nil)
	tries, err := api.GetTrybotResults(2347, 7)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(tries))
}

func TestAddComment(t *testing.T) {
	api := NewGerrit(GERRIT_SKIA_URL, nil)

	// Find the last patchset revision id
	changeInfo, err := api.GetIssueProperties(2370)
	assert.NoError(t, err)
	err = api.AddComment(changeInfo, "Testing API!!")
	assert.NoError(t, err)
}

func TestSendToDryRun(t *testing.T) {
	api := NewGerrit(GERRIT_SKIA_URL, nil)

	// Send to dry run.
	changeInfo, err := api.GetIssueProperties(2370)
	assert.NoError(t, err)
	defaultLabelValue := changeInfo.Labels[COMMITQUEUE_LABEL].DefaultValue
	err = api.SendToDryRun(changeInfo, "Sending to dry run")
	assert.NoError(t, err)

	// Wait for a second for the above to take place.
	time.Sleep(time.Second)

	// Verify that the change was sent to dry run.
	changeInfo, err = api.GetIssueProperties(2370)
	assert.NoError(t, err)
	assert.Equal(t, 1, changeInfo.Labels[COMMITQUEUE_LABEL].All[0].Value)

	// Remove from dry run.
	err = api.RemoveFromCQ(changeInfo, "")
	assert.NoError(t, err)

	// Verify that the change was removed from dry run.
	changeInfo, err = api.GetIssueProperties(2370)
	assert.NoError(t, err)
	assert.Equal(t, defaultLabelValue, changeInfo.Labels[COMMITQUEUE_LABEL].All[0].Value)
}

func TestSendToCQ(t *testing.T) {
	api := NewGerrit(GERRIT_SKIA_URL, nil)

	// Send to CQ.
	changeInfo, err := api.GetIssueProperties(2370)
	assert.NoError(t, err)
	defaultLabelValue := changeInfo.Labels[COMMITQUEUE_LABEL].DefaultValue
	err = api.SendToCQ(changeInfo, "Sending to CQ")
	assert.NoError(t, err)

	// Wait for a second for the above to take place.
	time.Sleep(time.Second)

	// Verify that the change was sent to CQ.
	changeInfo, err = api.GetIssueProperties(2370)
	assert.NoError(t, err)
	assert.Equal(t, 2, changeInfo.Labels[COMMITQUEUE_LABEL].All[0].Value)

	// Remove from CQ.
	err = api.RemoveFromCQ(changeInfo, "")
	assert.NoError(t, err)

	// Verify that the change was removed from CQ.
	changeInfo, err = api.GetIssueProperties(2370)
	assert.NoError(t, err)
	assert.Equal(t, defaultLabelValue, changeInfo.Labels[COMMITQUEUE_LABEL].All[0].Value)
}

func TestApprove(t *testing.T) {
	api := NewGerrit(GERRIT_SKIA_URL, nil)

	// Approve.
	changeInfo, err := api.GetIssueProperties(2370)
	assert.NoError(t, err)
	defaultLabelValue := changeInfo.Labels[CODEREVIEW_LABEL].DefaultValue
	err = api.Approve(changeInfo, "LGTM")
	assert.NoError(t, err)

	// Wait for a second for the above to take place.
	time.Sleep(time.Second)

	// Verify that the change was approved.
	changeInfo, err = api.GetIssueProperties(2370)
	assert.NoError(t, err)
	assert.Equal(t, 1, changeInfo.Labels[CODEREVIEW_LABEL].All[0].Value)

	// Remove approval.
	err = api.NoScore(changeInfo, "")
	assert.NoError(t, err)

	// Verify that the change has no score.
	changeInfo, err = api.GetIssueProperties(2370)
	assert.NoError(t, err)
	assert.Equal(t, defaultLabelValue, changeInfo.Labels[CODEREVIEW_LABEL].All[0].Value)
}

func TestDisApprove(t *testing.T) {
	api := NewGerrit(GERRIT_SKIA_URL, nil)

	// DisApprove.
	changeInfo, err := api.GetIssueProperties(2370)
	assert.NoError(t, err)
	defaultLabelValue := changeInfo.Labels[CODEREVIEW_LABEL].DefaultValue
	err = api.DisApprove(changeInfo, "not LGTM")
	assert.NoError(t, err)

	// Wait for a second for the above to take place.
	time.Sleep(time.Second)

	// Verify that the change was disapproved.
	changeInfo, err = api.GetIssueProperties(2370)
	assert.NoError(t, err)
	assert.Equal(t, -1, changeInfo.Labels[CODEREVIEW_LABEL].All[0].Value)

	// Remove disapproval.
	err = api.NoScore(changeInfo, "")
	assert.NoError(t, err)

	// Verify that the change has no score.
	changeInfo, err = api.GetIssueProperties(2370)
	assert.NoError(t, err)
	assert.Equal(t, defaultLabelValue, changeInfo.Labels[CODEREVIEW_LABEL].All[0].Value)
}

func TestAbandon(t *testing.T) {
	api := NewGerrit(GERRIT_SKIA_URL, nil)
	c1 := ChangeInfo{
		ChangeId: "Ife82eaa96ae64edd4f075688884b416cb8fec500",
	}
	api.Abandon(&c1)
	//err := api.Abandon(&c1)
	//assert.NoError(t, err)
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
		fmt.Println("mmmmmmmmm")
		fmt.Println(api.GetTrybotResults(issue.Issue, issue.Patchsets[len(issue.Patchsets)-1]))
	}

	keys, err := api.SearchKeys(5, SearchModifiedAfter(time.Now().Add(-time.Hour)))
	assert.NoError(t, err)
	assert.Equal(t, 5, len(keys))
}
