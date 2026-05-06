package chrome

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/gitiles/testutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/vfs"
	"go.skia.org/infra/perf/go/config"
)

func TestChromeProvider_ValidConfig_ReturnsExpectedRules(t *testing.T) {
	mockClient := mockhttpclient.NewURLMock()

	repoUrl := "https://chromium.googlesource.com/chromium/src"
	path := "testing/buildbot/perf.json"
	contents := []byte(`{"public_perf_builders": ["Linux Builder", "Mac Builder"]}`)

	testutils.MockReadFile(
		t,
		mockClient,
		repoUrl,
		path,
		git.MainBranch,
		contents,
		vfs.FileInfo{Name: "perf.json", Size: int64(len(contents)), Mode: 0644, IsDir: false}.Get(),
	)

	cfg := config.VisibilityConfig{
		Sources: map[string]config.VisibilitySourceConfig{
			"chrome_bots": {
				GitRepo:    repoUrl,
				Path:       path,
				RulePrefix: "bot=",
			},
		},
	}

	provider, err := ChromeProvider(cfg, mockClient.Client())
	require.NoError(t, err)

	ctx := context.Background()
	rules, err := provider.GetExpectedRules(ctx)
	require.NoError(t, err)

	require.Len(t, rules, 2)
	require.True(t, rules["bot=Linux Builder"])
	require.True(t, rules["bot=Mac Builder"])
}

func TestChromeProvider_MissingRequiredFields_ReturnsError(t *testing.T) {
	mockClient := mockhttpclient.NewURLMock()

	// Test missing git repo
	cfg := config.VisibilityConfig{
		Sources: map[string]config.VisibilitySourceConfig{
			"chrome_bots": {
				Path:       "some/path",
				RulePrefix: "bot=",
			},
		},
	}
	_, err := ChromeProvider(cfg, mockClient.Client())
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing required fields")

	// Test missing path
	cfg = config.VisibilityConfig{
		Sources: map[string]config.VisibilitySourceConfig{
			"chrome_bots": {
				GitRepo:    "https://example.com/repo",
				RulePrefix: "bot=",
			},
		},
	}
	_, err = ChromeProvider(cfg, mockClient.Client())
	require.Error(t, err)

	// Test missing rule prefix
	cfg = config.VisibilityConfig{
		Sources: map[string]config.VisibilitySourceConfig{
			"chrome_bots": {
				GitRepo: "https://example.com/repo",
				Path:    "some/path",
			},
		},
	}
	_, err = ChromeProvider(cfg, mockClient.Client())
	require.Error(t, err)

	// Test empty config
	cfgEmpty := config.VisibilityConfig{}
	_, err = ChromeProvider(cfgEmpty, mockClient.Client())
	require.Error(t, err)
	require.Contains(t, err.Error(), "at least one source is required")
}
