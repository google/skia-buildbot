package parent

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/revision"
	cipd_git "go.skia.org/infra/bazel/external/cipd/git"
	"go.skia.org/infra/go/gerrit"
	gerrit_testutils "go.skia.org/infra/go/gerrit/testutils"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
)

const (
	goModContents = `
module go.skia.org/autoroll-go-mod-test

go 1.18

require (
	go.skia.org/infra v0.0.0-20221018142618-5ea492a442f6
)
`
	goSumContents = `
go.skia.org/infra v0.0.0-20221018142618-5ea492a442f6 h1:tce0T72GBQJbKZh2xwrNlGGQ1elDPkzbtVeXi2OuXog=
go.skia.org/infra v0.0.0-20221018142618-5ea492a442f6/go.mod h1:PgN44PnlopspeC/ZsWi698vz14dYVV2P2fOVJXdkNqM=
`
	goModMainContents = `package main

import (
	"go.skia.org/infra/go/exec"
)

func main() {
	common.Init()
}
`

	goModRollingFrom = "5ea492a442f6"
	goModRollingTo   = "606b5a09ed2422df94a075ff6bbdc009d836c64c"
)

func goModCfg() *config.GoModParentConfig {
	return &config.GoModParentConfig{
		GitCheckout: &config.GitCheckoutConfig{
			Branch:  "main",
			RepoUrl: "fake.git",
		},
		ModulePath: "go.skia.org/infra",
	}
}

func setupGoModGerrit(t *testing.T) (context.Context, *goModParent, *gerrit_testutils.MockGerrit, func()) {
	// We do actually want to call git for some commands, so we need to use the git from CIPD.
	ctx := cipd_git.UseGitFinder(context.Background())
	tmp, err := ioutil.TempDir("", "")
	require.NoError(t, err)

	parent := git_testutils.GitInit(t, ctx)
	parent.Add(ctx, "go.mod", goModContents)
	parent.Add(ctx, "go.sum", goSumContents)
	parent.Add(ctx, "cmd/main/main.go", goModMainContents)
	parent.Commit(ctx)

	g := gerrit_testutils.NewGerrit(t)
	cfg := &config.GoModGerritParentConfig{
		GoMod: goModCfg(),
		Gerrit: &config.GerritConfig{
			Url:     "fake-review.googlesource.com",
			Project: "fake-project",
			Config:  config.GerritConfig_CHROMIUM_BOT_COMMIT,
		},
	}
	cfg.GoMod.GitCheckout.RepoUrl = parent.RepoUrl()
	urlmock := mockhttpclient.NewURLMock()
	g.MockDownloadCommitMsgHook()
	g.MockGetUserEmail(&gerrit.AccountDetails{
		AccountId: 12345,
		Name:      "Test User",
		Email:     "test-user@fake.com",
		UserName:  "test-user",
	})
	cr, err := codereview.NewGerrit(cfg.Gerrit, g.Gerrit, urlmock.Client())
	require.NoError(t, err)
	p, err := NewGoModGerritParent(ctx, cfg, &config_vars.Registry{}, urlmock.Client(), tmp, cr)
	require.NoError(t, err)
	return ctx, p, g, func() {
		g.AssertEmpty()
		testutils.RemoveAll(t, tmp)
	}
}

func TestGoModGerritUpdate(t *testing.T) {
	ctx, p, _, cleanup := setupGoModGerrit(t)
	defer cleanup()

	lastRollRev, err := p.Update(ctx)
	require.NoError(t, err)
	require.Equal(t, goModRollingFrom, lastRollRev)
}

func TestGoModGerritCreateNewRoll(t *testing.T) {
	ctx, p, g, cleanup := setupGoModGerrit(t)
	defer cleanup()

	from := &revision.Revision{
		Id: goModRollingFrom,
	}
	to := &revision.Revision{
		Id: goModRollingTo,
	}
	rolling := []*revision.Revision{to}
	emails := []string{"me@google.com"}

	ci := &gerrit.ChangeInfo{
		Id:       gerrit_testutils.FakeChangeId,
		Issue:    123,
		ChangeId: gerrit_testutils.FakeChangeId,
		Patchsets: []*gerrit.Revision{
			{
				ID:     "1",
				Number: 1,
			},
		},
		Project:   "fake-project",
		Branch:    "fake-branch",
		Revisions: map[string]*gerrit.Revision{},
	}
	for _, rev := range ci.Patchsets {
		ci.Revisions[rev.ID] = rev
	}
	g.MockGetIssueProperties(ci)
	g.MockSetCQ(ci, "", emails)

	issue, err := p.CreateNewRoll(ctx, from, to, rolling, emails, false, "roll")
	require.NoError(t, err)
	require.Equal(t, ci.Issue, issue)
	rolledRev, err := p.getPinnedRevision()
	require.NoError(t, err)
	require.Equal(t, goModRollingTo[:12], rolledRev)
}
