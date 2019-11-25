package repo_manager

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func validCommonBaseConfig() *CommonRepoManagerConfig {
	return &CommonRepoManagerConfig{
		ChildBranch:  "childBranch",
		ChildPath:    "childPath",
		ParentBranch: "parentBranch",
		ParentRepo:   "https://my-repo.com",
	}
}

func TestCommonConfigValidation(t *testing.T) {
	unittest.SmallTest(t)

	require.NoError(t, validCommonBaseConfig().Validate())
	cfg := validCommonBaseConfig()
	cfg.PreUploadSteps = []string{"TrainInfra"}
	require.NoError(t, cfg.Validate())

	// Helper function: create a valid base config, allow the caller to
	// mutate it, then assert that validation fails with the given message.
	testErr := func(fn func(c *CommonRepoManagerConfig), err string) {
		c := validCommonBaseConfig()
		fn(c)
		require.EqualError(t, c.Validate(), err)
	}

	// Test cases.

	testErr(func(c *CommonRepoManagerConfig) {
		c.ChildBranch = ""
	}, "ChildBranch is required.")

	testErr(func(c *CommonRepoManagerConfig) {
		c.ChildPath = ""
	}, "ChildPath is required.")

	testErr(func(c *CommonRepoManagerConfig) {
		c.ParentBranch = ""
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
			CommonRepoManagerConfig: *validCommonBaseConfig(),
		}
	}

	require.NoError(t, validBaseConfig().Validate())
	cfg := validBaseConfig()
	cfg.GClientSpec = "dummy"
	require.NoError(t, cfg.Validate())

	cfg.ParentRepo = ""
	require.EqualError(t, cfg.Validate(), "ParentRepo is required.")
}
