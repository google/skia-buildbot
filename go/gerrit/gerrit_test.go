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
	changeInfo, err := api.GetIssueProperties("Ibd49ce0e9f07bf5386f710df4621ff68f12e6ceb")
	assert.NoError(t, err)

	//p := PatchSet{RevisionId: "bd10a1250b3e5d42ac33b62bb904d2d3c3a1fa35"}
	//patchsets := []PatchSet{}
	//patchsets = append(patchsets, p)
	//c1 := ChangeInfo{
	//	ChangeId:  "Ibd49ce0e9f07bf5386f710df4621ff68f12e6ceb",
	//	Patchsets: patchsets,
	//}
	err = api.AddComment(changeInfo, "Testing API!")
	assert.NoError(t, err)

	api.GetCredentials("/usr/local/google/home/rmistry/.gitcookies")
	//assert.Equal(t, 1, len(tries))
}

func TestAbandon(t *testing.T) {
	api := NewGerrit(GERRIT_SKIA_URL, nil)
	c1 := ChangeInfo{
		ChangeId: "Ife82eaa96ae64edd4f075688884b416cb8fec500",
	}
	err := api.Abandon(&c1)
	assert.NoError(t, err)

	//api.GetCredentials("/usr/local/google/home/rmistry/.gitcookies")
	//assert.Equal(t, 1, len(tries))
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
