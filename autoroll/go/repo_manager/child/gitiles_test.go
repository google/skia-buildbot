package child

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gitiles_common"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/chrome_branch/mocks"
	git_testutils "go.skia.org/infra/go/git/testutils"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

// TODO(borenet): Split up the tests in no_checkout_deps_repo_manager_test.go
// and move the relevant parts here.

// TODO(borenet): This was copied from no_checkout_deps_repo_manager_test.go.
func masterBranchTmpl(t *testing.T) *config_vars.Template {
	master, err := config_vars.NewTemplate("master")
	require.NoError(t, err)
	return master
}

// TODO(borenet): This was copied from repo_manager_test.go.
func setupRegistry(t *testing.T) *config_vars.Registry {
	cbc := &mocks.Client{}
	cbc.On("Get", mock.Anything).Return(config_vars.DummyVars().Branches.Chromium, nil)
	reg, err := config_vars.NewRegistry(context.Background(), cbc)
	require.NoError(t, err)
	return reg
}

func TestGitilesChild_Download(t *testing.T) {
	unittest.LargeTest(t)

	// Setup. Copied from no_checkout_deps_repo_manager_test.go.
	child := git_testutils.GitInit(t, context.Background())
	child.Add(context.Background(), "DEPS", `deps = {
  "child/dep": "grandchild@def4560000def4560000def4560000def4560000",
}`)
	hash := child.Commit(context.Background())
	cfg := GitilesConfig{
		gitiles_common.GitilesConfig{
			Branch:  masterBranchTmpl(t),
			RepoURL: child.RepoUrl(),
		},
	}
	ctx := context.Background()
	c, err := NewGitiles(ctx, cfg, setupRegistry(t), nil)
	require.NoError(t, err)

	// Download.
	wd, cleanup := testutils.TempDir(t)
	defer cleanup()
	require.NoError(t, c.Download(ctx, &revision.Revision{Id: hash}, wd))

	// Verify that we have the correct contents.
	contents, err := ioutil.ReadDir(wd)
	require.NoError(t, err)
	require.Len(t, contents, 2)
	require.Equal(t, ".git", contents[0].Name())
	require.Equal(t, "DEPS", contents[1].Name())
}
