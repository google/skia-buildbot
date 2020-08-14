package repo_manager

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/go/chrome_branch/mocks"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/testutils/unittest"
)

func setupRegistry(t *testing.T) *config_vars.Registry {
	cbc := &mocks.Client{}
	cbc.On("Get", mock.Anything).Return(config_vars.DummyVars().Branches.Chromium, nil)
	reg, err := config_vars.NewRegistry(context.Background(), cbc)
	require.NoError(t, err)
	return reg
}

func defaultBranchTmpl(t *testing.T) *config_vars.Template {
	tmpl, err := config_vars.NewTemplate(git.DefaultBranch)
	require.NoError(t, err)
	return tmpl
}

func validCommonBaseConfig(t *testing.T) *CommonRepoManagerConfig {
	return &CommonRepoManagerConfig{
		ChildBranch:  defaultBranchTmpl(t),
		ChildPath:    "childPath",
		ParentBranch: defaultBranchTmpl(t),
		ParentRepo:   "https://my-repo.com",
	}
}

func TestCommonConfigValidation(t *testing.T) {
	unittest.SmallTest(t)

	require.NoError(t, validCommonBaseConfig(t).Validate())
	cfg := validCommonBaseConfig(t)
	cfg.PreUploadSteps = []string{"TrainInfra"}
	require.NoError(t, cfg.Validate())

	// Helper function: create a valid base config, allow the caller to
	// mutate it, then assert that validation fails with the given message.
	testErr := func(fn func(c *CommonRepoManagerConfig), err string) {
		c := validCommonBaseConfig(t)
		fn(c)
		require.EqualError(t, c.Validate(), err)
	}

	// Test cases.

	testErr(func(c *CommonRepoManagerConfig) {
		c.ChildBranch = nil
	}, "ChildBranch is required.")

	testErr(func(c *CommonRepoManagerConfig) {
		c.ChildPath = ""
	}, "ChildPath is required.")

	testErr(func(c *CommonRepoManagerConfig) {
		c.ParentBranch = nil
	}, "ParentBranch is required.")

	testErr(func(c *CommonRepoManagerConfig) {
		c.ParentRepo = ""
	}, "ParentRepo is required.")

	testErr(func(c *CommonRepoManagerConfig) {
		c.PreUploadSteps = []string{
			"bogus",
		}
	}, "No such pre-upload step: bogus")
}

func TestDepotToolsConfigValidation(t *testing.T) {
	unittest.SmallTest(t)

	validBaseConfig := func() *DepotToolsRepoManagerConfig {
		return &DepotToolsRepoManagerConfig{
			CommonRepoManagerConfig: *validCommonBaseConfig(t),
		}
	}

	require.NoError(t, validBaseConfig().Validate())
	cfg := validBaseConfig()
	cfg.GClientSpec = "dummy"
	require.NoError(t, cfg.Validate())

	cfg.ParentRepo = ""
	require.EqualError(t, cfg.Validate(), "ParentRepo is required.")
}
