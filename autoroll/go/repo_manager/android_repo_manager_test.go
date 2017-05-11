package repo_manager

import (
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/skexec"
	"go.skia.org/infra/go/skexec/skexec_testutils"
	"go.skia.org/infra/go/testutils"
)

const (
	androidIssueNum        = int64(12345)
	mockAndroidServer      = "https://mock-server-review.googlesource.com"
	numAndroidChildCommits = 10
)

var (
	androidEmails = []string{"reviewer@chromium.org"}
	childCommits  = []string{
		"5678888888888888888888888888888888888888",
		"1234444444444444444444444444444444444444"}
)

func setupAndroid(t *testing.T) (string, func()) {
	wd, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	cleanup := func() {
		testutils.RemoveAll(t, wd)
	}
	return wd, cleanup
}

func mockExecAndroid(t *testing.T) *skexec.Exec {
	mock := skexec_testutils.Mock{}
	mock.AddRule("[^ ]*/repo ", "", nil)
	mock.AddRule("^git log --format=format:%H%x20%ci", fmt.Sprintf("%s 2017-03-29 18:29:22 +0000\n%s 2017-03-29 18:29:22 +0000", childCommits[0], childCommits[1]), nil)
	mock.AddRule("^git ls-remote", childCommits[0], nil)
	mock.AddRule("^git merge-base", childCommits[1], nil)
	exec := skexec.NewExec()
	exec.SetRun(mock.Run)
	return exec
}

// TestAndroidRepoManager tests all aspects of the RepoManager except for CreateNewRoll.
func TestAndroidRepoManager(t *testing.T) {
	testutils.LargeTest(t)
	wd, cleanup := setupAndroid(t)
	defer cleanup()
	g, err := gerrit.NewGerrit(mockAndroidServer, "", nil)
	assert.NoError(t, err)
	rm, err := NewAndroidRepoManager(wd, "master", childPath, "master", 24*time.Hour, g, mockExecAndroid(t))
	assert.NoError(t, err)

	assert.Equal(t, fmt.Sprintf("%s/android_repo/%s", wd, childPath), rm.(*androidRepoManager).childDir)
	assert.Equal(t, "https://mock-server.googlesource.com", rm.(*androidRepoManager).repoUrl)
	assert.Equal(t, childCommits[len(childCommits)-1], rm.LastRollRev())
	assert.Equal(t, childCommits[0], rm.ChildHead())
	assert.Equal(t, SERVICE_ACCOUNT, rm.User())
}

// TestCreateNewAndroidRoll tests creating a new roll.
func TestCreateNewAndroidRoll(t *testing.T) {
	testutils.LargeTest(t)
	wd, cleanup := setupAndroid(t)
	defer cleanup()

	g := &gerrit.MockedGerrit{IssueID: androidIssueNum}
	rm, err := NewAndroidRepoManager(wd, "master", childPath, "master", 24*time.Hour, g, mockExecAndroid(t))
	assert.NoError(t, err)

	issue, err := rm.CreateNewRoll(ROLL_STRATEGY_BATCH, androidEmails, "", false)
	assert.NoError(t, err)
	assert.Equal(t, issueNum, issue)
}

func TestExtractBugNumbers(t *testing.T) {
	testutils.SmallTest(t)

	bodyWithTwoBugs := `testing
Test: tested
BUG=skia:123
Bug: skia:456
BUG=b/123
Bug: b/234`
	bugNumbers := ExtractBugNumbers(bodyWithTwoBugs)
	assert.Equal(t, 2, len(bugNumbers))
	assert.True(t, bugNumbers["123"])
	assert.True(t, bugNumbers["234"])

	bodyWithNoBugs := `testing
Test: tested
BUG=skia:123
Bug: skia:456
BUG=ba/123
Bug: bb/234`
	bugNumbers = ExtractBugNumbers(bodyWithNoBugs)
	assert.Equal(t, 0, len(bugNumbers))
}

func TestExtractTestLines(t *testing.T) {
	testutils.SmallTest(t)

	bodyWithThreeTestLines := `testing
Test: tested with 0
testing
BUG=skia:123
Bug: skia:456
Test: tested with 1
BUG=b/123
Bug: b/234

Test: tested with 2
`
	testLines := ExtractTestLines(bodyWithThreeTestLines)
	assert.Equal(t, []string{"Test: tested with 0", "Test: tested with 1", "Test: tested with 2"}, testLines)

	bodyWithNoTestLines := `testing
no test
lines
included
here
`
	testLines = ExtractTestLines(bodyWithNoTestLines)
	assert.Equal(t, 0, len(testLines))
}
