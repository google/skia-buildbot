package repo_manager

import (
	"context"
	"io/ioutil"
	"path"
	"path/filepath"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/exec"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/recipe_cfg"
	"go.skia.org/infra/go/testutils"
)

const (
	fuchsiaSDKRevPrev = "000633ae6e904f7eaced443d6aa65fb3d24afe8c"
	fuchsiaSDKRevBase = "32a56ad54471732034ba802cbfc3c9ff277b9d1c"
	fuchsiaSDKRevNext = "37417b795289818723990da66dd7a7b38e50fc04"

	fuchsiaSDKTimePrev = "2009-11-10T23:00:01Z"
	fuchsiaSDKTimeBase = "2009-11-10T23:00:02Z"
	fuchsiaSDKTimeNext = "2009-11-10T23:00:03Z"

	fuchsiaSDKLatestArchiveUrlLinux = "https://storage.googleapis.com/fuchsia/sdk/linux-amd64/LATEST_ARCHIVE"
	fuchsiaSDKLatestArchiveUrlMac   = "https://storage.googleapis.com/fuchsia/sdk/mac-amd64/LATEST_ARCHIVE"
)

func fuchsiaCfg() *FuchsiaSDKRepoManagerConfig {
	return &FuchsiaSDKRepoManagerConfig{
		DepotToolsRepoManagerConfig: DepotToolsRepoManagerConfig{
			CommonRepoManagerConfig: CommonRepoManagerConfig{
				ChildBranch:  "master",
				ChildPath:    "unused/by/fuchsiaSDK/repomanager",
				ParentBranch: "master",
			},
		},
	}
}

func setupFuchsiaSDK(t *testing.T) (context.Context, string, *git_testutils.GitBuilder, *exec.CommandCollector, *mockhttpclient.URLMock, func()) {
	wd, err := ioutil.TempDir("", "")
	assert.NoError(t, err)

	// Create child and parent repos.
	parent := git_testutils.GitInit(t, context.Background())
	parent.Add(context.Background(), FUCHSIA_SDK_VERSION_FILE_PATH_LINUX, fuchsiaSDKRevBase)
	parent.Add(context.Background(), FUCHSIA_SDK_VERSION_FILE_PATH_MAC, fuchsiaSDKRevBase)
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
	urlmock := mockhttpclient.NewURLMock()

	cleanup := func() {
		testutils.RemoveAll(t, wd)
		parent.Cleanup()
	}

	return ctx, wd, parent, mockRun, urlmock, cleanup
}

func mockGetLatestSDK(urlmock *mockhttpclient.URLMock, revLinux, revMac string) {
	urlmock.MockOnce(fuchsiaSDKLatestArchiveUrlLinux, mockhttpclient.MockGetDialogue([]byte(revLinux)))
	urlmock.MockOnce(fuchsiaSDKLatestArchiveUrlMac, mockhttpclient.MockGetDialogue([]byte(revMac)))
}

func TestFuchsiaSDKRepoManager(t *testing.T) {
	testutils.LargeTest(t)

	ctx, wd, gb, _, urlmock, cleanup := setupFuchsiaSDK(t)
	defer cleanup()
	recipesCfg := filepath.Join(testutils.GetRepoRoot(t), recipe_cfg.RECIPE_CFG_PATH)
	g := setupFakeGerrit(t, wd)

	// Initial update, everything up-to-date.
	mockGSList(t, urlmock, FUCHSIA_SDK_GS_BUCKET, FUCHSIA_SDK_GS_PATH, map[string]string{
		fuchsiaSDKRevBase: fuchsiaSDKTimeBase,
		fuchsiaSDKRevPrev: fuchsiaSDKTimePrev,
	})
	mockGetLatestSDK(urlmock, fuchsiaSDKRevBase, "mac-base")
	cfg := fuchsiaCfg()
	cfg.ParentRepo = gb.RepoUrl()
	rm, err := NewFuchsiaSDKRepoManager(ctx, cfg, wd, g, recipesCfg, "fake.server.com", urlmock.Client())
	assert.NoError(t, err)
	assert.NoError(t, SetStrategy(ctx, rm, strategy.ROLL_STRATEGY_FUCHSIA_SDK))
	assert.NoError(t, rm.Update(ctx))
	assert.Equal(t, mockUser, rm.User())
	assert.Equal(t, fuchsiaSDKRevBase, rm.LastRollRev())
	assert.Equal(t, fuchsiaSDKRevBase, rm.NextRollRev())
	fch, err := rm.FullChildHash(ctx, rm.LastRollRev())
	assert.NoError(t, err)
	assert.Equal(t, fch, rm.LastRollRev())
	rolledPast, err := rm.RolledPast(ctx, fuchsiaSDKRevPrev)
	assert.NoError(t, err)
	assert.True(t, rolledPast)
	rolledPast, err = rm.RolledPast(ctx, fuchsiaSDKRevBase)
	assert.NoError(t, err)
	assert.True(t, rolledPast)
	assert.Empty(t, rm.PreUploadSteps())
	assert.Equal(t, 0, rm.CommitsNotRolled())

	// There's a new version.
	mockGSList(t, urlmock, FUCHSIA_SDK_GS_BUCKET, FUCHSIA_SDK_GS_PATH, map[string]string{
		fuchsiaSDKRevPrev: fuchsiaSDKTimePrev,
		fuchsiaSDKRevBase: fuchsiaSDKTimeBase,
		fuchsiaSDKRevNext: fuchsiaSDKTimeNext,
	})
	mockGetLatestSDK(urlmock, fuchsiaSDKRevNext, "mac-next")
	assert.NoError(t, rm.Update(ctx))
	assert.Equal(t, fuchsiaSDKRevBase, rm.LastRollRev())
	assert.Equal(t, fuchsiaSDKRevNext, rm.NextRollRev())
	rolledPast, err = rm.RolledPast(ctx, fuchsiaSDKRevPrev)
	assert.NoError(t, err)
	assert.True(t, rolledPast)
	rolledPast, err = rm.RolledPast(ctx, fuchsiaSDKRevBase)
	assert.NoError(t, err)
	assert.True(t, rolledPast)
	rolledPast, err = rm.RolledPast(ctx, fuchsiaSDKRevNext)
	assert.NoError(t, err)
	assert.False(t, rolledPast)
	assert.Equal(t, 1, rm.CommitsNotRolled())

	// Upload a CL.
	ran := false
	rm.(*fuchsiaSDKRepoManager).preUploadSteps = []PreUploadStep{
		func(context.Context, string) error {
			ran = true
			return nil
		},
	}
	cqExtraTrybots := "tryserver.chromium.linux:fuchsia_x64_cast_audio"
	issue, err := rm.CreateNewRoll(ctx, rm.LastRollRev(), rm.NextRollRev(), emails, cqExtraTrybots, false)
	assert.NoError(t, err)
	assert.Equal(t, issueNum, issue)
	msg, err := ioutil.ReadFile(path.Join(rm.(*fuchsiaSDKRepoManager).parentDir, ".git", "COMMIT_EDITMSG"))
	assert.NoError(t, err)
	from, to, err := autoroll.RollRev(strings.Split(string(msg), "\n")[0], func(h string) (string, error) {
		return rm.FullChildHash(ctx, h)
	})
	assert.NoError(t, err)
	assert.Equal(t, fuchsiaSDKRevBase, from)
	assert.Equal(t, fuchsiaSDKRevNext, to)
	assert.True(t, strings.Contains(string(msg), cqExtraTrybots))
	assert.True(t, ran)
}

func TestFuchsiaSDKConfigValidation(t *testing.T) {
	testutils.SmallTest(t)

	cfg := fuchsiaCfg()
	cfg.ParentRepo = "dummy" // Not supplied above.
	assert.NoError(t, cfg.Validate())

	// The only fields come from the nested Configs, so exclude them and
	// verify that we fail validation.
	cfg = &FuchsiaSDKRepoManagerConfig{}
	assert.Error(t, cfg.Validate())
}
