package repo_manager

import (
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/testutils/unittest"
)

func validCommonBaseConfig() *CommonRepoManagerConfig {
	return &CommonRepoManagerConfig{
		ChildBranch:  "childBranch",
		ChildPath:    "childPath",
		ParentBranch: "parentBranch",
	}
}

func TestCommonConfigValidation(t *testing.T) {
	unittest.SmallTest(t)

	assert.NoError(t, validCommonBaseConfig().Validate())
	cfg := validCommonBaseConfig()
	cfg.PreUploadSteps = []string{"TrainInfra"}
	assert.NoError(t, cfg.Validate())

	// Helper function: create a valid base config, allow the caller to
	// mutate it, then assert that validation fails with the given message.
	testErr := func(fn func(c *CommonRepoManagerConfig), err string) {
		c := validCommonBaseConfig()
		fn(c)
		assert.EqualError(t, c.Validate(), err)
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
			ParentRepo:              "parentRepo",
		}
	}

	assert.NoError(t, validBaseConfig().Validate())
	cfg := validBaseConfig()
	cfg.GClientSpec = "dummy"
	assert.NoError(t, cfg.Validate())

	cfg.ParentRepo = ""
	assert.EqualError(t, cfg.Validate(), "ParentRepo is required.")

	// Verify that the CommonRepoManagerConfig gets validated.
	cfg = &DepotToolsRepoManagerConfig{
		ParentRepo: "parentRepo",
	}
	assert.Error(t, cfg.Validate())
}

func TestCopyRevision(t *testing.T) {
	unittest.SmallTest(t)

	v := &Revision{
		Id:          "abc123",
		Display:     "abc",
		Description: "This is a great commit.",
		Timestamp:   time.Now(),
		URL:         "www.best-commit.com",
	}
	deepequal.AssertCopy(t, v, v.Copy())
}
