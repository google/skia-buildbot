package repo_manager

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/issues"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/recipe_cfg"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vcsinfo"
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

func depsCfg() *DEPSRepoManagerConfig {
	return &DEPSRepoManagerConfig{
		DepotToolsRepoManagerConfig: DepotToolsRepoManagerConfig{
			CommonRepoManagerConfig: CommonRepoManagerConfig{
				ChildBranch:  "master",
				ChildPath:    childPath,
				ParentBranch: "master",
			},
		},
	}
}

func setup(t *testing.T) (context.Context, string, *git_testutils.GitBuilder, []string, *git_testutils.GitBuilder, *exec.CommandCollector, *vcsinfo.LongCommit, func()) {
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

	lastUpload := new(vcsinfo.LongCommit)
	mockRun := &exec.CommandCollector{}
	ctx := exec.NewContext(context.Background(), mockRun.Run)
	mockRun.SetDelegateRun(func(cmd *exec.Command) error {
		if cmd.Name == "git" && cmd.Args[0] == "cl" {
			if cmd.Args[1] == "upload" {
				d, err := git.GitDir(cmd.Dir).Details(ctx, "HEAD")
				if err != nil {
					return err
				}
				*lastUpload = *d
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

	cleanup := func() {
		testutils.RemoveAll(t, wd)
		child.Cleanup()
		parent.Cleanup()
	}

	return ctx, wd, child, childCommits, parent, mockRun, lastUpload, cleanup
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

	ctx, wd, child, childCommits, parent, _, _, cleanup := setup(t)
	defer cleanup()
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	g := setupFakeGerrit(t, wd)
	cfg := depsCfg()
	cfg.ParentRepo = parent.RepoUrl()
	rm, err := NewDEPSRepoManager(ctx, cfg, wd, g, recipesCfg, "fake.server.com", nil, false)
	assert.NoError(t, err)
	assert.NoError(t, SetStrategy(ctx, rm, strategy.ROLL_STRATEGY_BATCH))
	assert.NoError(t, rm.Update(ctx))
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

	// Switch next-roll-rev strategies.
	assert.NoError(t, SetStrategy(ctx, rm, strategy.ROLL_STRATEGY_SINGLE))
	assert.NoError(t, rm.Update(ctx))
	assert.Equal(t, childCommits[1], rm.NextRollRev())
	// And back again.
	assert.NoError(t, SetStrategy(ctx, rm, strategy.ROLL_STRATEGY_BATCH))
	assert.NoError(t, rm.Update(ctx))
	assert.Equal(t, lastCommit, rm.NextRollRev())
}

func testCreateNewDEPSRoll(t *testing.T, strategy string, expectIdx int) {
	testutils.LargeTest(t)

	ctx, wd, child, childCommits, parent, _, _, cleanup := setup(t)
	defer cleanup()
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	g := setupFakeGerrit(t, wd)
	cfg := depsCfg()
	cfg.ParentRepo = parent.RepoUrl()
	rm, err := NewDEPSRepoManager(ctx, cfg, wd, g, recipesCfg, "fake.server.com", nil, false)
	assert.NoError(t, err)
	assert.NoError(t, SetStrategy(ctx, rm, strategy))
	assert.NoError(t, rm.Update(ctx))

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
	testCreateNewDEPSRoll(t, strategy.ROLL_STRATEGY_BATCH, numChildCommits-1)
}

// TestDEPSRepoManagerSingle tests the single-commit roll strategy.
func TestDEPSRepoManagerSingle(t *testing.T) {
	testCreateNewDEPSRoll(t, strategy.ROLL_STRATEGY_SINGLE, 1)
}

// Verify that we ran the PreUploadSteps.
func TestRanPreUploadStepsDeps(t *testing.T) {
	testutils.LargeTest(t)

	ctx, wd, _, _, parent, _, _, cleanup := setup(t)
	defer cleanup()
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	g := setupFakeGerrit(t, wd)
	cfg := depsCfg()
	cfg.ParentRepo = parent.RepoUrl()
	rm, err := NewDEPSRepoManager(ctx, cfg, wd, g, recipesCfg, "fake.server.com", nil, false)
	assert.NoError(t, err)
	assert.NoError(t, SetStrategy(ctx, rm, strategy.ROLL_STRATEGY_BATCH))
	assert.NoError(t, rm.Update(ctx))

	ran := false
	rm.(*depsRepoManager).preUploadSteps = []PreUploadStep{
		func(context.Context, *http.Client, string) error {
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
		ctx, wd, _, _, parent, _, lastUpload, cleanup := setup(t)
		defer cleanup()
		recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

		g := setupFakeGerrit(t, wd)
		cfg := depsCfg()
		cfg.ParentRepo = parent.RepoUrl()
		cfg.IncludeLog = includeLog
		rm, err := NewDEPSRepoManager(ctx, cfg, wd, g, recipesCfg, "fake.server.com", nil, false)
		assert.NoError(t, err)
		assert.NoError(t, SetStrategy(ctx, rm, strategy.ROLL_STRATEGY_BATCH))
		assert.NoError(t, rm.Update(ctx))

		// Create a roll.
		_, err = rm.CreateNewRoll(ctx, rm.LastRollRev(), rm.NextRollRev(), emails, cqExtraTrybots, false)
		assert.NoError(t, err)

		// Ensure that we included the log, or not, as appropriate.
		assert.NoError(t, err)
		assert.Equal(t, includeLog, strings.Contains(lastUpload.Body, "git log"))
	}

	test(true)
	test(false)
}

// Verify that we properly utilize a gclient spec.
func TestDEPSRepoManagerGclientSpec(t *testing.T) {
	testutils.LargeTest(t)

	ctx, wd, _, _, parent, mockRun, _, cleanup := setup(t)
	defer cleanup()
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

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
	cfg := depsCfg()
	cfg.GClientSpec = gclientSpec
	cfg.ParentRepo = parent.RepoUrl()
	rm, err := NewDEPSRepoManager(ctx, cfg, wd, g, recipesCfg, "fake.server.com", nil, false)
	assert.NoError(t, err)
	assert.NoError(t, SetStrategy(ctx, rm, strategy.ROLL_STRATEGY_BATCH))
	assert.NoError(t, rm.Update(ctx))

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

// Verify that we include the correct bug lings.
func TestDEPSRepoManagerBugs(t *testing.T) {
	testutils.LargeTest(t)

	project := "skiatestproject"

	test := func(bugLine, expect string) {
		// Setup.
		ctx, wd, child, _, parent, _, lastUpload, cleanup := setup(t)
		defer cleanup()
		recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

		g := setupFakeGerrit(t, wd)
		cfg := depsCfg()
		cfg.IncludeBugs = true
		cfg.ParentRepo = parent.RepoUrl()
		rm, err := NewDEPSRepoManager(ctx, cfg, wd, g, recipesCfg, "fake.server.com", nil, false)
		assert.NoError(t, err)
		assert.NoError(t, SetStrategy(ctx, rm, strategy.ROLL_STRATEGY_BATCH))
		assert.NoError(t, rm.Update(ctx))

		// Insert a fake entry into the repo mapping.
		issues.REPO_PROJECT_MAPPING[parent.RepoUrl()] = project

		// Make a commit with the bug entry.
		child.AddGen(ctx, "myfile")
		hash := child.CommitMsg(ctx, fmt.Sprintf(`Some dummy commit

%s
`, bugLine))

		// Create a roll.
		assert.NoError(t, rm.Update(ctx))
		_, err = rm.CreateNewRoll(ctx, rm.LastRollRev(), hash, emails, cqExtraTrybots, false)
		assert.NoError(t, err)

		// Verify that we passed the correct --bug argument to roll-dep.
		found := false
		for _, line := range strings.Split(lastUpload.Body, "\n") {
			if strings.HasPrefix(line, "BUG=") {
				found = true
				assert.Equal(t, line[4:], expect)
			}
		}
		if expect == "" {
			assert.False(t, found)
		} else {
			assert.True(t, found)
		}
	}

	// Test cases.
	test("", "")
	test("BUG=skiatestproject:23", "skiatestproject:23")
	test("BUG=skiatestproject:18,skiatestproject:58", "skiatestproject:18,skiatestproject:58")
	// No prefix defaults to "chromium", which we don't include for rolls into "skiatestproject".
	test("BUG=skiatestproject:18,58", "skiatestproject:18")
	test("BUG=456", "")
	test("BUG=skia:123,chromium:4532,skiatestproject:21", "skiatestproject:21")
	test("Bug: skiatestproject:33", "skiatestproject:33")
}

func TestDEPSConfigValidation(t *testing.T) {
	testutils.SmallTest(t)

	cfg := depsCfg()
	cfg.ParentRepo = "dummy" // Not supplied above.
	assert.NoError(t, cfg.Validate())

	// The only fields come from the nested Configs, so exclude them and
	// verify that we fail validation.
	cfg = &DEPSRepoManagerConfig{}
	assert.Error(t, cfg.Validate())
}
