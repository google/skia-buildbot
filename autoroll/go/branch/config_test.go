package branch

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func staticCfg() *StaticBranchConfig {
	c := StaticBranchConfig("master")
	return &c
}

func chromeCfg() *ChromeBranchConfig {
	c := ChromeBranchConfig("refs/branch-heads/{{.Beta}}")
	return &c
}

func cfg() *Config {
	return &Config{
		Static: staticCfg(),
	}
}

func TestConfigValidate(t *testing.T) {
	unittest.SmallTest(t)

	test := func(fn func(*Config) string) {
		c := cfg()
		expect := fn(c)
		err := c.Validate()
		if expect == "" {
			require.Nil(t, err, err)
		} else {
			require.NotNil(t, err)
			require.True(t, strings.Contains(err.Error(), expect))
		}
	}

	// Static config, no error.
	test(func(c *Config) string {
		return ""
	})

	// Chrome config, no error.
	test(func(c *Config) string {
		c.Static = nil
		c.Chrome = chromeCfg()
		return ""
	})

	// No config.
	test(func(c *Config) string {
		c.Static = nil
		return "Exactly one branch configuration is required."
	})

	// Static config can't be the empty string.
	test(func(c *Config) string {
		s := StaticBranchConfig("")
		c.Static = &s
		return "Ref is required."
	})

	// Chrome config can't be the empty string.
	test(func(c *Config) string {
		c.Static = nil
		chrome := ChromeBranchConfig("")
		c.Chrome = &chrome
		return "Template is required."
	})

	// Chrome config must be a valid template.
	test(func(c *Config) string {
		c.Static = nil
		chrome := ChromeBranchConfig("{{{")
		c.Chrome = &chrome
		return "unexpected \"{\" in command"
	})

	// Chrome config must actually use the branch.
	test(func(c *Config) string {
		c.Static = nil
		chrome := ChromeBranchConfig("master")
		c.Chrome = &chrome
		return "Template does not make use of Chrome branch; use static branch instead."
	})
}
