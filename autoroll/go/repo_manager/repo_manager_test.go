package repo_manager

import (
	"fmt"
	"io/ioutil"
	"path"
	"strings"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/testutils"
)

const (
	childPath       = "path/to/child"
	cqExtraTrybots  = ""
	depotTools      = ""
	issueNum        = int64(12345)
	mockServer      = "https://codereview.chromium.org"
	mockUser        = "user@chromium.org"
	numChildCommits = 10
)

var (
	emails = []string{"reviewer@chromium.org"}
)

func setup(t *testing.T) (string, *git_testutils.GitBuilder, []string, *git_testutils.GitBuilder, func()) {
	wd, err := ioutil.TempDir("", "")
	assert.NoError(t, err)

	// Create child and parent repos.
	child := git_testutils.GitInit(t)
	f := "somefile.txt"
	childCommits := make([]string, 0, 10)
	for i := 0; i < numChildCommits; i++ {
		childCommits = append(childCommits, child.CommitGen(f))
	}

	parent := git_testutils.GitInit(t)
	parent.Add("DEPS", fmt.Sprintf(`deps = {
  "%s": "%s@%s",
}`, childPath, child.RepoUrl(), childCommits[0]))
	parent.Commit()

	mockRun := exec.CommandCollector{}
	mockRun.SetDelegateRun(func(cmd *exec.Command) error {
		if strings.Contains(cmd.Name, "depot-tools-auth") {
			auth := fmt.Sprintf(`Logged in to %s as %s.

To login with a different email run:
  depot-tools-auth login https://codereview.chromium.org
To logout and purge the authentication token run:
  depot-tools-auth logout https://codereview.chromium.org
`, mockServer, mockUser)
			n, err := cmd.CombinedOutput.Write([]byte(auth))
			assert.NoError(t, err)
			assert.Equal(t, len(auth), n)
			return nil
		} else if cmd.Name == "git" && cmd.Args[0] == "cl" {
			if cmd.Args[1] == "upload" {
				return nil
			} else if cmd.Args[1] == "issue" {
				json := testutils.MarshalJSON(t, &issueJson{
					Issue:    issueNum,
					IssueUrl: "???",
				})
				f := strings.Split(cmd.Args[2], "=")[1]
				testutils.WriteFile(t, f, json)
				return nil
			}
		}
		return exec.DefaultRun(cmd)
	})
	exec.SetRunForTesting(mockRun.Run)

	cleanup := func() {
		exec.SetRunForTesting(exec.DefaultRun)
		testutils.RemoveAll(t, wd)
		child.Cleanup()
		parent.Cleanup()
	}

	return wd, child, childCommits, parent, cleanup
}

// TestRepoManager tests all aspects of the RepoManager except for CreateNewRoll.
func TestRepoManager(t *testing.T) {
	testutils.LargeTest(t)

	wd, child, childCommits, parent, cleanup := setup(t)
	defer cleanup()

	rm, err := NewDefaultRepoManager(wd, parent.RepoUrl(), childPath, 24*time.Hour, depotTools)
	assert.NoError(t, err)
	assert.Equal(t, childCommits[0], rm.LastRollRev())
	assert.Equal(t, childCommits[len(childCommits)-1], rm.ChildHead())

	// Test FullChildHash.
	for _, c := range childCommits {
		h, err := rm.FullChildHash(c[:12])
		assert.NoError(t, err)
		assert.Equal(t, c, h)
	}

	// Test update.
	lastCommit := child.CommitGen("abc.txt")
	assert.NoError(t, rm.ForceUpdate())
	assert.Equal(t, lastCommit, rm.ChildHead())

	// RolledPast.
	rp, err := rm.RolledPast(childCommits[0])
	assert.NoError(t, err)
	assert.True(t, rp)
	for _, c := range childCommits[1:] {
		rp, err := rm.RolledPast(c)
		assert.NoError(t, err)
		assert.False(t, rp)
	}

	// User, name only.
	assert.Equal(t, strings.Split(mockUser, "@")[0], rm.User())
}

func testCreateNewRoll(t *testing.T, strategy string, expectIdx int) {
	testutils.LargeTest(t)

	wd, child, childCommits, parent, cleanup := setup(t)
	defer cleanup()

	rm, err := NewDefaultRepoManager(wd, parent.RepoUrl(), childPath, 24*time.Hour, depotTools)
	assert.NoError(t, err)

	// Create a roll, assert that it's at tip of tree.
	issue, err := rm.CreateNewRoll(strategy, emails, cqExtraTrybots, false, true)
	assert.NoError(t, err)
	assert.Equal(t, issueNum, issue)
	msg, err := ioutil.ReadFile(path.Join(rm.(*repoManager).parentDir, ".git", "COMMIT_EDITMSG"))
	assert.NoError(t, err)
	from, to, err := autoroll.RollRev(strings.Split(string(msg), "\n")[0], func(h string) (string, error) {
		return git.GitDir(child.Dir()).RevParse(h)
	})
	assert.NoError(t, err)
	assert.Equal(t, childCommits[0], from)
	assert.Equal(t, childCommits[expectIdx], to)
}

// TestRepoManagerBatch tests the batch roll strategy.
func TestRepoManagerBatch(t *testing.T) {
	testCreateNewRoll(t, ROLL_STRATEGY_BATCH, numChildCommits-1)
}

// TestRepoManagerSingle tests the single-commit roll strategy.
func TestRepoManagerSingle(t *testing.T) {
	testCreateNewRoll(t, ROLL_STRATEGY_SINGLE, 1)
}
