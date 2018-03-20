package repo_manager

import (
	"context"
	"io/ioutil"
	"path"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/autoroll"
	depot_tools "go.skia.org/infra/go/depot_tools/testutils"
	"go.skia.org/infra/go/exec"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
)

const (
	fuchsiaSDKRevPrev = "def456"
	fuchsiaSDKRevBase = "abc123"
	fuchsiaSDKRevNext = "ace999"

	fuchsiaSDKTimePrev = "2009-11-10T23:00:01Z"
	fuchsiaSDKTimeBase = "2009-11-10T23:00:02Z"
	fuchsiaSDKTimeNext = "2009-11-10T23:00:03Z"
)

func setupFuchsiaSDK(t *testing.T) (context.Context, string, *git_testutils.GitBuilder, *exec.CommandCollector, *mockhttpclient.URLMock, func()) {
	wd, err := ioutil.TempDir("", "")
	assert.NoError(t, err)

	// Create child and parent repos.
	parent := git_testutils.GitInit(t, context.Background())
	parent.Add(context.Background(), FUCHSIA_SDK_VERSION_FILE_PATH, fuchsiaSDKRevBase)
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

func TestFuchsiaSDKRepoManager(t *testing.T) {
	testutils.LargeTest(t)

	ctx, wd, gb, _, urlmock, cleanup := setupFuchsiaSDK(t)
	defer cleanup()
	g := setupFakeGerrit(t, wd)

	// Initial update, everything up-to-date.
	mockGSList(t, urlmock, FUCHSIA_SDK_GS_BUCKET, FUCHSIA_SDK_GS_PATH, map[string]string{
		fuchsiaSDKRevBase: fuchsiaSDKTimeBase,
		fuchsiaSDKRevPrev: fuchsiaSDKTimePrev,
	})
	rm, err := NewFuchsiaSDKRepoManager(ctx, wd, gb.RepoUrl(), "master", depot_tools.GetDepotTools(t, ctx), g, "fake.server.com", urlmock.Client())
	assert.NoError(t, err)
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
	assert.Nil(t, rm.PreUploadSteps())
	assert.Equal(t, 0, rm.CommitsNotRolled())

	// There's a new version.
	mockGSList(t, urlmock, FUCHSIA_SDK_GS_BUCKET, FUCHSIA_SDK_GS_PATH, map[string]string{
		fuchsiaSDKRevPrev: fuchsiaSDKTimePrev,
		fuchsiaSDKRevBase: fuchsiaSDKTimeBase,
		fuchsiaSDKRevNext: fuchsiaSDKTimeNext,
	})
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
}
