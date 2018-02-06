package repo_manager

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/autoroll"
	depot_tools "go.skia.org/infra/go/depot_tools/testutils"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util"
)

const (
	childPath       = "path/to/child"
	cqExtraTrybots  = ""
	issueNum        = int64(12345)
	mockServer      = "https://skia-review.googlesource.com"
	mockUser        = "user@chromium.org"
	numChildCommits = 10
)

var (
	emails = []string{"reviewer@chromium.org"}
)

func setup(t *testing.T) (context.Context, string, *git_testutils.GitBuilder, []string, *git_testutils.GitBuilder, *exec.CommandCollector, func()) {
	wd, err := ioutil.TempDir("", "")
	assert.NoError(t, err)

	// Create child and parent repos.
	child := git_testutils.GitInit(t, context.Background())
	f := "somefile.txt"
	childCommits := make([]string, 0, 10)
	for i := 0; i < numChildCommits; i++ {
		childCommits = append(childCommits, child.CommitGen(context.Background(), f))
	}

	parent := git_testutils.GitInit(t, context.Background())
	parent.Add(context.Background(), "DEPS", fmt.Sprintf(`deps = {
  "%s": "%s@%s",
}`, childPath, child.RepoUrl(), childCommits[0]))
	parent.Commit(context.Background())

	mockRun := &exec.CommandCollector{}
	mockRun.SetDelegateRun(func(cmd *exec.Command) error {
		if cmd.Name == "git" && cmd.Args[0] == "cl" {
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
	ctx := exec.NewContext(context.Background(), mockRun.Run)

	cleanup := func() {
		testutils.RemoveAll(t, wd)
		child.Cleanup()
		parent.Cleanup()
	}

	return ctx, wd, child, childCommits, parent, mockRun, cleanup
}

func setupFakeGerrit(t *testing.T, wd string) *gerrit.Gerrit {
	gUrl := "https://fake-skia-review.googlesource.com"
	urlMock := mockhttpclient.NewURLMock()
	serialized, err := json.Marshal(&gerrit.AccountDetails{
		AccountId: 101,
		Name:      mockUser,
		Email:     mockUser,
		UserName:  mockUser,
	})
	assert.NoError(t, err)
	serialized = append([]byte("abcd\n"), serialized...)
	urlMock.MockOnce(gUrl+"/a/accounts/self/detail", mockhttpclient.MockGetDialogue(serialized))
	gitcookies := path.Join(wd, "gitcookies_fake")
	assert.NoError(t, ioutil.WriteFile(gitcookies, []byte(".googlesource.com\tTRUE\t/\tTRUE\t123\to\tgit-user.google.com=abc123"), os.ModePerm))
	g, err := gerrit.NewGerrit(gUrl, gitcookies, urlMock.Client())
	assert.NoError(t, err)
	return g
}

// TestRepoManager tests all aspects of the DEPSRepoManager except for CreateNewRoll.
func TestDEPSRepoManager(t *testing.T) {
	testutils.LargeTest(t)

	ctx, wd, child, childCommits, parent, _, cleanup := setup(t)
	defer cleanup()

	g := setupFakeGerrit(t, wd)
	s, err := GetNextRollStrategy(ROLL_STRATEGY_BATCH, "master", "")
	assert.NoError(t, err)
	rm, err := NewDEPSRepoManager(ctx, wd, parent.RepoUrl(), "master", childPath, "master", depot_tools.GetDepotTools(t, ctx), g, s, nil, true, "", "fake.server.com")
	assert.NoError(t, err)
	assert.Equal(t, childCommits[0], rm.LastRollRev())
	assert.Equal(t, childCommits[len(childCommits)-1], rm.NextRollRev())

	// Test FullChildHash.
	for _, c := range childCommits {
		h, err := rm.FullChildHash(ctx, c[:12])
		assert.NoError(t, err)
		assert.Equal(t, c, h)
	}

	// Test update.
	lastCommit := child.CommitGen(context.Background(), "abc.txt")
	assert.NoError(t, rm.Update(ctx))
	assert.Equal(t, lastCommit, rm.NextRollRev())

	// RolledPast.
	rp, err := rm.RolledPast(ctx, childCommits[0])
	assert.NoError(t, err)
	assert.True(t, rp)
	for _, c := range childCommits[1:] {
		rp, err := rm.RolledPast(ctx, c)
		assert.NoError(t, err)
		assert.False(t, rp)
	}

	// User, name only.
	assert.Equal(t, mockUser, rm.User())
}

func testCreateNewDEPSRoll(t *testing.T, strategy string, expectIdx int) {
	testutils.LargeTest(t)

	ctx, wd, child, childCommits, parent, _, cleanup := setup(t)
	defer cleanup()

	s, err := GetNextRollStrategy(strategy, "master", "")
	assert.NoError(t, err)
	g := setupFakeGerrit(t, wd)
	rm, err := NewDEPSRepoManager(ctx, wd, parent.RepoUrl(), "master", childPath, "master", depot_tools.GetDepotTools(t, ctx), g, s, nil, true, "", "fake.server.com")
	assert.NoError(t, err)

	// Create a roll, assert that it's at tip of tree.
	issue, err := rm.CreateNewRoll(ctx, rm.LastRollRev(), rm.NextRollRev(), emails, cqExtraTrybots, false)
	assert.NoError(t, err)
	assert.Equal(t, issueNum, issue)
	msg, err := ioutil.ReadFile(path.Join(rm.(*depsRepoManager).parentDir, ".git", "COMMIT_EDITMSG"))
	assert.NoError(t, err)
	from, to, err := autoroll.RollRev(strings.Split(string(msg), "\n")[0], func(h string) (string, error) {
		return git.GitDir(child.Dir()).RevParse(ctx, h)
	})
	assert.NoError(t, err)
	assert.Equal(t, childCommits[0], from)
	assert.Equal(t, childCommits[expectIdx], to)
}

// TestDEPSRepoManagerBatch tests the batch roll strategy.
func TestDEPSRepoManagerBatch(t *testing.T) {
	testCreateNewDEPSRoll(t, ROLL_STRATEGY_BATCH, numChildCommits-1)
}

// TestDEPSRepoManagerSingle tests the single-commit roll strategy.
func TestDEPSRepoManagerSingle(t *testing.T) {
	testCreateNewDEPSRoll(t, ROLL_STRATEGY_SINGLE, 1)
}

// Verify that we ran the PreUploadSteps.
func TestRanPreUploadStepsDeps(t *testing.T) {
	testutils.LargeTest(t)

	ctx, wd, _, _, parent, _, cleanup := setup(t)
	defer cleanup()

	s, err := GetNextRollStrategy(ROLL_STRATEGY_BATCH, "master", "")
	assert.NoError(t, err)
	g := setupFakeGerrit(t, wd)
	rm, err := NewDEPSRepoManager(ctx, wd, parent.RepoUrl(), "master", childPath, "master", depot_tools.GetDepotTools(t, ctx), g, s, nil, true, "", "fake.server.com")
	assert.NoError(t, err)

	ran := false
	rm.(*depsRepoManager).preUploadSteps = []PreUploadStep{
		func(context.Context, string) error {
			ran = true
			return nil
		},
	}

	// Create a roll, assert that we ran the PreUploadSteps.
	_, err = rm.CreateNewRoll(ctx, rm.LastRollRev(), rm.NextRollRev(), emails, cqExtraTrybots, false)
	assert.NoError(t, err)
	assert.True(t, ran)
}

// Verify that we respect the includeLog parameter.
func TestDEPSRepoManagerIncludeLog(t *testing.T) {
	testutils.LargeTest(t)

	test := func(includeLog bool) {
		ctx, wd, _, _, parent, mockRun, cleanup := setup(t)
		defer cleanup()

		s, err := GetNextRollStrategy(ROLL_STRATEGY_BATCH, "master", "")
		assert.NoError(t, err)
		g := setupFakeGerrit(t, wd)

		rm, err := NewDEPSRepoManager(ctx, wd, parent.RepoUrl(), "master", childPath, "master", depot_tools.GetDepotTools(t, ctx), g, s, nil, includeLog, "", "fake.server.com")
		assert.NoError(t, err)

		// Create a roll.
		_, err = rm.CreateNewRoll(ctx, rm.LastRollRev(), rm.NextRollRev(), emails, cqExtraTrybots, false)
		assert.NoError(t, err)

		// Ensure that --no-log is present or not, according to includeLog.
		found := false
		for _, c := range mockRun.Commands() {
			if strings.Contains(c.Name, "roll-dep") {
				found = true
				assert.Equal(t, !includeLog, util.In("--no-log", c.Args))
			}
		}
		assert.True(t, found)
	}

	test(true)
	test(false)
}

// Verify that we properly utilize a gclient spec.
func TestDEPSRepoManagerGclientSpec(t *testing.T) {
	testutils.LargeTest(t)

	ctx, wd, _, _, parent, mockRun, cleanup := setup(t)
	defer cleanup()

	s, err := GetNextRollStrategy(ROLL_STRATEGY_BATCH, "master", "")
	assert.NoError(t, err)
	g := setupFakeGerrit(t, wd)
	gclientSpec := fmt.Sprintf(`
solutions=[{
  "name": "%s",
  "url": "%s",
  "deps_file": "DEPS",
  "managed": True,
  "custom_deps": {},
  "custom_vars": {
    "a": "b",
    "c": "d",
  },
}];
cache_dir=None
`, path.Base(parent.RepoUrl()), parent.RepoUrl())
	// Remove newlines.
	gclientSpec = strings.Replace(gclientSpec, "\n", "", -1)

	rm, err := NewDEPSRepoManager(ctx, wd, parent.RepoUrl(), "master", childPath, "master", depot_tools.GetDepotTools(t, ctx), g, s, nil, true, gclientSpec, "fake.server.com")
	assert.NoError(t, err)

	// Create a roll.
	_, err = rm.CreateNewRoll(ctx, rm.LastRollRev(), rm.NextRollRev(), emails, cqExtraTrybots, false)
	assert.NoError(t, err)

	// Ensure that we pass the spec into "gclient config".
	found := false
	for _, c := range mockRun.Commands() {
		if c.Name == "python" && strings.Contains(c.Args[0], "gclient.py") && c.Args[1] == "config" {
			for _, arg := range c.Args {
				if arg == "--spec="+gclientSpec {
					found = true
				}
			}
		}
	}
	assert.True(t, found)
}
