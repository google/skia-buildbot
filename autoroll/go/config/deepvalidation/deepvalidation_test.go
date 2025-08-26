package deepvalidation

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/go/chrome_branch"
	"go.skia.org/infra/go/chrome_branch/mocks"
	"go.skia.org/infra/go/gerrit"
	gerrit_testutils "go.skia.org/infra/go/gerrit/testutils"
	"go.skia.org/infra/go/mockhttpclient"
)

const (
	fakeGitHubRepoOwner = "fake"
	fakeGitHubRepoName  = "skia"
)

// newTestDeepValidator creates a deepvalidator instance for testing.
func newTestDeepValidator(t *testing.T) (*deepvalidator, *mockhttpclient.URLMock) {
	urlMock := mockhttpclient.NewURLMock()
	cbc := &mocks.Client{}
	cbc.On("Get", mock.Anything).Return(&chrome_branch.Branches{}, []*chrome_branch.Branch{{Milestone: 123}}, nil)
	reg, err := config_vars.NewRegistry(context.Background(), cbc)
	require.NoError(t, err)
	return &deepvalidator{
			client:           urlMock.Client(),
			reg:              reg,
			githubHttpClient: urlMock.Client(),
		},
		urlMock
}

func TestDeepValidator_deepValidate(t *testing.T) {
	ctx := context.Background()

	t.Run("gerrit", func(t *testing.T) {
		dv, urlMock := newTestDeepValidator(t)
		defer urlMock.AssertExpectations(t)
		cfg := &config.Config{
			CodeReview: &config.Config_Gerrit{
				Gerrit: makeGerritConfig(),
			},
		}
		mocksForGerritConfig(t, urlMock, cfg.GetGerrit())
		require.NoError(t, dv.deepValidate(ctx, cfg))
	})

	t.Run("github", func(t *testing.T) {
		dv, urlMock := newTestDeepValidator(t)
		defer urlMock.AssertExpectations(t)
		cfg := &config.Config{
			CodeReview: &config.Config_Github{
				Github: makeGitHubConfig(),
			},
		}
		mocksForGitHubConfig(t, urlMock, cfg.GetGithub())
		require.NoError(t, dv.deepValidate(ctx, cfg))
	})
}

func TestDeepValidator_gerritConfig(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	gerritConfig := makeGerritConfig()

	t.Run("Success", func(t *testing.T) {
		mocksForGerritConfig(t, urlMock, gerritConfig)
		require.NoError(t, dv.gerritConfig(t.Context(), gerritConfig))
		urlMock.AssertExpectations(t)
	})

	t.Run("No Changes Found", func(t *testing.T) {
		mg := gerrit_testutils.NewGerrit(t)
		mg.Mock = urlMock
		mg.MockSearch([]*gerrit.ChangeInfo{}, 1, gerrit.SearchProject(gerritConfig.Project))
		err := dv.gerritConfig(t.Context(), gerritConfig)
		require.Error(t, err)
		require.Contains(t, err.Error(), "no changes found")
		urlMock.AssertExpectations(t)
	})

	t.Run("Unknown Config", func(t *testing.T) {
		gerritConfig.Config = config.GerritConfig_Config(999) // Invalid enum value.
		err := dv.gerritConfig(t.Context(), gerritConfig)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown config")
		urlMock.AssertExpectations(t)
	})
}

func TestDeepValidator_gitHubConfig(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := makeGitHubConfig()

	t.Run("Success", func(t *testing.T) {
		mocksForGitHubConfig(t, urlMock, cfg)

		err := dv.gitHubConfig(t.Context(), cfg)
		require.NoError(t, err)
		urlMock.AssertExpectations(t)
	})

	t.Run("Not Found", func(t *testing.T) {
		url := githHubPullRequestsURL(cfg)
		urlMock.MockOnce(url, mockhttpclient.MockGetError("not found", http.StatusNotFound))
		err := dv.gitHubConfig(t.Context(), cfg)
		require.Error(t, err)
		urlMock.AssertExpectations(t)
	})
}

func makeGerritConfig() *config.GerritConfig {
	return &config.GerritConfig{
		Url:     gerrit_testutils.FakeGerritURL,
		Project: "fake",
		Config:  config.GerritConfig_CHROMIUM,
	}
}

func makeGitHubConfig() *config.GitHubConfig {
	return &config.GitHubConfig{
		RepoOwner: fakeGitHubRepoOwner,
		RepoName:  fakeGitHubRepoName,
	}
}

func mocksForGerritConfig(t *testing.T, urlMock *mockhttpclient.URLMock, cfg *config.GerritConfig) {
	mg := gerrit_testutils.NewGerrit(t)
	mg.Mock = urlMock
	mg.MockSearch([]*gerrit.ChangeInfo{
		{
			Subject: "test change",
		},
	}, 1, gerrit.SearchProject(cfg.Project))
}

func githHubPullRequestsURL(cfg *config.GitHubConfig) string {
	return fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls?state=open", cfg.RepoOwner, cfg.RepoName)
}

func mocksForGitHubConfig(t *testing.T, urlMock *mockhttpclient.URLMock, cfg *config.GitHubConfig) {
	urlMock.MockOnce(githHubPullRequestsURL(cfg), mockhttpclient.MockGetDialogue([]byte(`[{"number": 1}]`)))
}
