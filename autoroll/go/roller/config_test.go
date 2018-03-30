package roller

import (
	"encoding/json"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	arb_notifier "go.skia.org/infra/autoroll/go/notifier"
	"go.skia.org/infra/autoroll/go/repo_manager"
	"go.skia.org/infra/go/testutils"
)

// validBaseConfig returns a minimal valid AutoRollerConfig.
func validBaseConfig() *AutoRollerConfig {
	return &AutoRollerConfig{
		ChildName:       "childName",
		GerritURL:       "https://gerrit",
		ParentName:      "parentName",
		ParentWaterfall: "parentWaterfall",
		Sheriff:         []string{"sheriff@gmail.com"},

		// Use the fake Google3 repo manager config, so that we don't
		// have to bother with correctly filling in real configs.
		Google3RepoManager: &google3FakeRepoManagerConfig{},
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
	}, "GerritURL is required.")

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
		c.Notifiers = []arb_notifier.Config{
			arb_notifier.Config{
				"type":    "email",
				"filter":  "debug",
				"subject": "Override Subject",
				"emails":  []interface{}{"me@example.com"},
				"markup":  "dummy",
			},
			arb_notifier.Config{
				"type":   "chat",
				"filter": "warning",
				"room":   "dummy-room",
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
		assert.NoError(t, json.Unmarshal(bytes, &b))
		testutils.AssertDeepEqual(t, a, &b)
	}

	test()

	a.CqExtraTrybots = []string{"extra-bot"}
	a.MaxRollFrequency = "1h"
	a.Notifiers = []arb_notifier.Config{
		arb_notifier.Config{
			"type":    "email",
			"filter":  "debug",
			"subject": "Override Subject",
			"emails":  []interface{}{"me@example.com"},
			"markup":  "dummy",
		},
		arb_notifier.Config{
			"type":   "chat",
			"filter": "warning",
			"room":   "dummy-room",
		},
	}
	a.SafetyThrottle = &ThrottleConfig{
		AttemptCount: 5,
		TimeWindow:   time.Hour,
	}

	test()
}
