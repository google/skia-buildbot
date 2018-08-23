package roller

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/flynn/json5"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/repo_manager"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/notifier"
	"go.skia.org/infra/go/testutils"
)

// validBaseConfig returns a minimal valid AutoRollerConfig.
func validBaseConfig() *AutoRollerConfig {
	return &AutoRollerConfig{
		ChildName:       "childName",
		GerritURL:       "https://gerrit",
		ParentName:      "parentName",
		ParentWaterfall: "parentWaterfall",
		RollerName:      "test-roller",
		Sheriff:         []string{"sheriff@gmail.com"},

		// Use the fake Google3 repo manager config, so that we don't
		// have to bother with correctly filling in real configs.
		Google3RepoManager: &Google3FakeRepoManagerConfig{
			ChildBranch: "master",
			ChildRepo:   "my-repo",
		},
	}
}

func TestConfigs(t *testing.T) {
	testutils.SmallTest(t)

	// Sanity check: ensure that the base config is valid.
	assert.NoError(t, validBaseConfig().Validate())

	// Helper function: create a valid base config, allow the caller to
	// mutate it, then assert that validation fails with the given message.
	testErr := func(fn func(c *AutoRollerConfig), err string) {
		c := validBaseConfig()
		fn(c)
		assert.EqualError(t, c.Validate(), err)
	}

	// Test cases.

	testErr(func(c *AutoRollerConfig) {
		c.ChildName = ""
	}, "ChildName is required.")

	testErr(func(c *AutoRollerConfig) {
		c.GerritURL = ""
	}, "Either GerritURL OR both GithubRepoOwner/GithubRepoName is required.")

	testErr(func(c *AutoRollerConfig) {
		c.GerritURL = ""
		c.GithubRepoOwner = "superman"
	}, "Either GerritURL OR both GithubRepoOwner/GithubRepoName is required.")

	testErr(func(c *AutoRollerConfig) {
		c.ParentName = ""
	}, "ParentName is required.")

	testErr(func(c *AutoRollerConfig) {
		c.ParentWaterfall = ""
	}, "ParentWaterfall is required.")

	testErr(func(c *AutoRollerConfig) {
		c.Sheriff = nil
	}, "Sheriff is required.")

	testErr(func(c *AutoRollerConfig) {
		c.Google3RepoManager = nil
	}, "Exactly one repo manager must be supplied, but got 0")

	testErr(func(c *AutoRollerConfig) {
		c.AndroidRepoManager = &repo_manager.AndroidRepoManagerConfig{}
	}, "Exactly one repo manager must be supplied, but got 2")

	testErr(func(c *AutoRollerConfig) {
		c.Notifiers = []*notifier.Config{
			&notifier.Config{
				Filter: "debug",
			},
		}
	}, "Exactly one notification config must be supplied, but got 0")

	// Helper function: create a valid base config, allow the caller to
	// mutate it, then assert that validation succeeds.
	testNoErr := func(fn func(c *AutoRollerConfig)) {
		c := validBaseConfig()
		fn(c)
		assert.NoError(t, c.Validate())
	}

	// Test cases.

	testNoErr(func(c *AutoRollerConfig) {
		c.CqExtraTrybots = []string{"extra-bot"}
	})

	testNoErr(func(c *AutoRollerConfig) {
		c.MaxRollFrequency = "1h"
	})

	testNoErr(func(c *AutoRollerConfig) {
		c.Notifiers = []*notifier.Config{
			&notifier.Config{
				Filter:  "debug",
				Subject: "Override Subject",
				Email: &notifier.EmailNotifierConfig{
					Emails: []string{"me@example.com"},
				},
			},
			&notifier.Config{
				Filter: "warning",
				Chat: &notifier.ChatNotifierConfig{
					RoomID: "dummy-room",
				},
			},
		}
	})

	testNoErr(func(c *AutoRollerConfig) {
		c.SafetyThrottle = &ThrottleConfig{
			AttemptCount: 5,
			TimeWindow:   time.Hour,
		}
	})
}

func TestConfigSerialization(t *testing.T) {
	testutils.SmallTest(t)

	a := validBaseConfig()

	test := func() {
		var b AutoRollerConfig
		bytes, err := json.Marshal(a)
		assert.NoError(t, err)
		assert.NoError(t, json5.Unmarshal(bytes, &b))
		deepequal.AssertDeepEqual(t, a, &b)
	}

	test()

	a.CqExtraTrybots = []string{"extra-bot"}
	a.MaxRollFrequency = "1h"
	a.Notifiers = []*notifier.Config{
		&notifier.Config{
			Filter:  "debug",
			Subject: "Override Subject",
			Email: &notifier.EmailNotifierConfig{
				Emails: []string{"me@example.com"},
			},
		},
		&notifier.Config{
			Filter: "warning",
			Chat: &notifier.ChatNotifierConfig{
				RoomID: "dummy-room",
			},
		},
	}
	a.SafetyThrottle = &ThrottleConfig{
		AttemptCount: 5,
		TimeWindow:   time.Hour,
	}

	test()
}
