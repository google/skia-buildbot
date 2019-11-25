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

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/git"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/issues"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/recipe_cfg"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
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
	require.NoError(t, err)

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
	mockRun.SetDelegateRun(func(ctx context.Context, cmd *exec.Command) error {
		if strings.Contains(cmd.Name, "git") && cmd.Args[0] == "cl" {
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
		if strings.Contains(cmd.Name, "gclient") && util.In("setdep", cmd.Args) {
			splitDep := strings.Split(cmd.Args[len(cmd.Args)-1], "@")
			require.Equal(t, 2, len(splitDep))
			require.Equal(t, 40, len(splitDep[1]))
		}
		return exec.DefaultRun(ctx, cmd)
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
	require.NoError(t, err)
	serialized = append([]byte("abcd\n"), serialized...)
	urlMock.MockOnce(gUrl+"/a/accounts/self/detail", mockhttpclient.MockGetDialogue(serialized))
	gitcookies := path.Join(wd, "gitcookies_fake")
	require.NoError(t, ioutil.WriteFile(gitcookies, []byte(".googlesource.com\tTRUE\t/\tTRUE\t123\to\tgit-user.google.com=abc123"), os.ModePerm))
	g, err := gerrit.NewGerrit(gUrl, gitcookies, urlMock.Client())
	require.NoError(t, err)
	return g
}

// TestRepoManager tests all aspects of the DEPSRepoManager except for CreateNewRoll.
func TestDEPSRepoManager(t *testing.T) {
	unittest.LargeTest(t)

	ctx, wd, _, childCommits, parent, _, _, cleanup := setup(t)
	defer cleanup()
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	g := setupFakeGerrit(t, wd)
	cfg := depsCfg()
	cfg.ParentRepo = parent.RepoUrl()
	rm, err := NewDEPSRepoManager(ctx, cfg, wd, g, recipesCfg, "fake.server.com", nil, gerritCR(t, g), false)
	require.NoError(t, err)

	// Test update.
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)
	require.Equal(t, childCommits[0], lastRollRev.Id)
	require.Equal(t, childCommits[len(childCommits)-1], tipRev.Id)
	require.Equal(t, len(childCommits)-1, len(notRolledRevs))
}

func TestCreateNewDEPSRoll(t *testing.T) {
	unittest.LargeTest(t)

	ctx, wd, _, _, parent, _, _, cleanup := setup(t)
	defer cleanup()
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	g := setupFakeGerrit(t, wd)
	cfg := depsCfg()
	cfg.ParentRepo = parent.RepoUrl()
	rm, err := NewDEPSRepoManager(ctx, cfg, wd, g, recipesCfg, "fake.server.com", nil, gerritCR(t, g), false)
	require.NoError(t, err)
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	// Create a roll, assert that it's at tip of tree.
	issue, err := rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, cqExtraTrybots, false)
	require.NoError(t, err)
	require.Equal(t, issueNum, issue)
}

// Verify that we ran the PreUploadSteps.
func TestRanPreUploadStepsDeps(t *testing.T) {
	unittest.LargeTest(t)

	ctx, wd, _, _, parent, _, _, cleanup := setup(t)
	defer cleanup()
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

	g := setupFakeGerrit(t, wd)
	cfg := depsCfg()
	cfg.ParentRepo = parent.RepoUrl()
	rm, err := NewDEPSRepoManager(ctx, cfg, wd, g, recipesCfg, "fake.server.com", nil, gerritCR(t, g), false)
	require.NoError(t, err)
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	ran := false
	rm.(*depsRepoManager).preUploadSteps = []PreUploadStep{
		func(context.Context, []string, *http.Client, string) error {
			ran = true
			return nil
		},
	}

	// Create a roll, assert that we ran the PreUploadSteps.
	_, err = rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, cqExtraTrybots, false)
	require.NoError(t, err)
	require.True(t, ran)
}

// Verify that we respect the includeLog parameter.
func TestDEPSRepoManagerIncludeLog(t *testing.T) {
	unittest.LargeTest(t)

	test := func(includeLog bool) {
		ctx, wd, _, _, parent, _, lastUpload, cleanup := setup(t)
		defer cleanup()
		recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

		g := setupFakeGerrit(t, wd)
		cfg := depsCfg()
		cfg.ParentRepo = parent.RepoUrl()
		cfg.IncludeLog = includeLog
		rm, err := NewDEPSRepoManager(ctx, cfg, wd, g, recipesCfg, "fake.server.com", nil, gerritCR(t, g), false)
		require.NoError(t, err)
		lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
		require.NoError(t, err)

		// Create a roll.
		_, err = rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, cqExtraTrybots, false)
		require.NoError(t, err)

		// Ensure that we included the log, or not, as appropriate.
		require.NoError(t, err)
		require.Equal(t, includeLog, strings.Contains(lastUpload.Body, "git log"))
	}

	test(true)
	test(false)
}

// Verify that we properly utilize a gclient spec.
func TestDEPSRepoManagerGclientSpec(t *testing.T) {
	unittest.LargeTest(t)

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
	rm, err := NewDEPSRepoManager(ctx, cfg, wd, g, recipesCfg, "fake.server.com", nil, gerritCR(t, g), false)
	require.NoError(t, err)
	lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	// Create a roll.
	_, err = rm.CreateNewRoll(ctx, lastRollRev, tipRev, notRolledRevs, emails, cqExtraTrybots, false)
	require.NoError(t, err)

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
	require.True(t, found)
}

// Verify that we include the correct bug lings.
func TestDEPSRepoManagerBugs(t *testing.T) {
	unittest.LargeTest(t)

	project := "skiatestproject"

	test := func(bugLine, expect string) {
		// Setup.
		ctx, wd, child, _, parent, _, lastUpload, cleanup := setup(t)
		defer cleanup()
		recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)

		g := setupFakeGerrit(t, wd)
		cfg := depsCfg()
		cfg.IncludeBugs = true
		cfg.MonorailProject = project
		cfg.ParentRepo = parent.RepoUrl()
		rm, err := NewDEPSRepoManager(ctx, cfg, wd, g, recipesCfg, "fake.server.com", nil, gerritCR(t, g), false)
		require.NoError(t, err)

		// Insert a fake entry into the repo mapping.
		issues.REPO_PROJECT_MAPPING[parent.RepoUrl()] = project

		// Make a commit with the bug entry.
		child.AddGen(ctx, "myfile")
		hash := child.CommitMsg(ctx, fmt.Sprintf(`Some dummy commit

%s
`, bugLine))
		details, err := git.GitDir(child.Dir()).Details(ctx, hash)
		require.NoError(t, err)
		rev := revision.FromLongCommit(rm.(*depsRepoManager).childRevLinkTmpl, details)

		// Create a roll.
		lastRollRev, tipRev, notRolledRevs, err := rm.Update(ctx)
		require.NoError(t, err)
		require.Equal(t, hash, tipRev.Id)
		_, err = rm.CreateNewRoll(ctx, lastRollRev, rev, notRolledRevs, emails, cqExtraTrybots, false)
		require.NoError(t, err)

		// Verify that we passed the correct --bug argument to roll-dep.
		found := false
		for _, line := range strings.Split(lastUpload.Body, "\n") {
			if strings.HasPrefix(line, "BUG=") {
				found = true
				require.Equal(t, line[4:], expect)
			} else if strings.HasPrefix(line, "Bug: ") {
				found = true
				require.Equal(t, line[5:], expect)
			}
		}
		if expect == "" {
			require.False(t, found)
		} else {
			require.True(t, found)
		}
	}

	// Test cases.
	test("", "None")
	test("BUG=skiatestproject:23", "skiatestproject:23")
	test("BUG=skiatestproject:18,skiatestproject:58", "skiatestproject:18,skiatestproject:58")
	// No prefix defaults to "chromium", which we don't include for rolls into "skiatestproject".
	test("BUG=skiatestproject:18,58", "skiatestproject:18")
	test("BUG=456", "None")
	test("BUG=skia:123,chromium:4532,skiatestproject:21", "skiatestproject:21")
	test("Bug: skiatestproject:33", "skiatestproject:33")
}

func TestDEPSConfigValidation(t *testing.T) {
	unittest.SmallTest(t)

	cfg := depsCfg()
	cfg.ParentRepo = "dummy" // Not supplied above.
	require.NoError(t, cfg.Validate())

	// The only fields come from the nested Configs, so exclude them and
	// verify that we fail validation.
	cfg = &DEPSRepoManagerConfig{}
	require.Error(t, cfg.Validate())
}
