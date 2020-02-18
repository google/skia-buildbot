package branch

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/mockhttpclient"
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

func TestConfigCreateStatic(t *testing.T) {
	unittest.SmallTest(t)

	b, err := staticCfg().Create(context.Background(), http.DefaultClient)
	require.NoError(t, err)
	require.NotNil(t, b)
	require.Equal(t, "master", b.Ref())
}

func TestConfigCreateChrome(t *testing.T) {
	unittest.SmallTest(t)

	// TODO(borenet): Is there a better way to mock this?
	urlmock := mockhttpclient.NewURLMock()
	urlmock.MockOnce(jsonUrl, mockhttpclient.MockGetDialogue([]byte(`[
  {
    "os": "linux",
    "versions": [
      {
        "channel": "beta",
        "true_branch": "4044"
      },
      {
        "channel": "stable",
        "true_branch": "3987"
      }
    ]
  }
]`)))
	b, err := chromeCfg().Create(context.Background(), urlmock.Client())
	require.NoError(t, err)
	require.NotNil(t, b)
	require.Equal(t, "refs/branch-heads/4044", b.Ref())
}
