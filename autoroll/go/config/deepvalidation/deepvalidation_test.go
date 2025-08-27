package deepvalidation

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v29/github"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/chrome_branch"
	"go.skia.org/infra/go/chrome_branch/mocks"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/gerrit"
	gerrit_testutils "go.skia.org/infra/go/gerrit/testutils"
	"go.skia.org/infra/go/git"
	skgithub "go.skia.org/infra/go/github"
	"go.skia.org/infra/go/gitiles"
	gitiles_testutils "go.skia.org/infra/go/gitiles/testutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/vfs"
)

const (
	fakeGitRepo    = "https://fake.googlesource.com/fake.git"
	fakeParentRepo = "https://fake.googlesource.com/parent.git"
	fakeChildRepo  = "https://fake.googlesource.com/child.git"

	fakeGitHubRepoOwner = "fake"
	fakeGitHubRepoName  = "skia"
	fakeGitHubRepo      = "https://github.com/fake/skia"
	fakeGitHubRef       = "refs/heads%2Fmain"

	fakeCommitHash = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	fakeDepsContent = `deps = { 'path/to/my-dep': 'my-dep@123' }`
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

	t.Run("android", func(t *testing.T) {
		dv, urlMock := newTestDeepValidator(t)
		defer urlMock.AssertExpectations(t)
		androidCfg := &config.AndroidRepoManagerConfig{
			ParentRepoUrl: fakeParentRepo,
			ParentBranch:  git.MainBranch,
			ChildRepoUrl:  fakeChildRepo,
			ChildBranch:   git.MainBranch,
		}
		cfg := &config.Config{
			RepoManager: &config.Config_AndroidRepoManager{
				AndroidRepoManager: androidCfg,
			},
		}
		gitiles_testutils.MockGetCommit(t, urlMock, androidCfg.ParentRepoUrl, androidCfg.ParentBranch, makeFakeCommit())
		gitiles_testutils.MockGetCommit(t, urlMock, androidCfg.ChildRepoUrl, androidCfg.ChildBranch, makeFakeCommit())
		require.NoError(t, dv.deepValidate(ctx, cfg))
	})

	t.Run("command", func(t *testing.T) {
		dv, urlMock := newTestDeepValidator(t)
		defer urlMock.AssertExpectations(t)
		cfg := &config.Config{
			RepoManager: &config.Config_CommandRepoManager{
				CommandRepoManager: makeCommandRepoManagerConfig(),
			},
		}
		mocksForCommandRepoManagerConfig(t, urlMock, cfg.GetCommandRepoManager())
		require.NoError(t, dv.deepValidate(ctx, cfg))
	})

	t.Run("freetype", func(t *testing.T) {
		dv, urlMock := newTestDeepValidator(t)
		defer urlMock.AssertExpectations(t)
		cfg := &config.Config{
			RepoManager: &config.Config_FreetypeRepoManager{
				FreetypeRepoManager: &config.FreeTypeRepoManagerConfig{
					Child:  makeGitilesChildConfig(),
					Parent: makeFreeTypeParentConfig(),
				},
			},
		}
		mocksForGitilesChildConfig(t, urlMock, cfg.GetFreetypeRepoManager().Child)
		mocksForFreeTypeParentConfig(t, urlMock, cfg.GetFreetypeRepoManager().Parent)
		require.NoError(t, dv.deepValidate(ctx, cfg))
	})

	t.Run("google3", func(t *testing.T) {
		dv, urlMock := newTestDeepValidator(t)
		defer urlMock.AssertExpectations(t)
		g3Cfg := &config.Google3RepoManagerConfig{
			ChildRepo:   fakeGitRepo,
			ChildBranch: git.MainBranch,
		}
		cfg := &config.Config{
			RepoManager: &config.Config_Google3RepoManager{
				Google3RepoManager: g3Cfg,
			},
		}
		gitiles_testutils.MockGetCommit(t, urlMock, g3Cfg.ChildRepo, g3Cfg.ChildBranch, makeFakeCommit())
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

func TestDeepValidator_versionFileConfig(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := makeVersionFileConfig()
	getFile := func(ctx context.Context, path string) (string, error) {
		return fakeDepsContent, nil
	}

	t.Run("Success", func(t *testing.T) {
		err := dv.versionFileConfig(t.Context(), cfg, getFile)
		require.NoError(t, err)
		urlMock.AssertExpectations(t)
	})

	t.Run("Non-Existent Dep", func(t *testing.T) {
		cfg := makeVersionFileConfig()
		cfg.Id = "non-existent-dep"
		err := dv.versionFileConfig(t.Context(), cfg, getFile)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Unable to find")
		urlMock.AssertExpectations(t)
	})

	t.Run("getFile fails", func(t *testing.T) {
		getFile = func(ctx context.Context, path string) (string, error) {
			return "", fmt.Errorf("failed to read file")
		}
		err := dv.versionFileConfig(t.Context(), cfg, getFile)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to read file")
		urlMock.AssertExpectations(t)
	})
}

func TestDeepValidator_deepValidateGitilesRepo(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	repoURL := fakeGitRepo
	branchTmpl, err := config_vars.NewTemplate(git.MainBranch)
	require.NoError(t, err)
	require.NoError(t, dv.reg.Register(branchTmpl))

	t.Run("Success", func(t *testing.T) {
		gitiles_testutils.MockGetCommit(t, urlMock, repoURL, git.MainBranch, makeFakeCommit())
		_, _, err := dv.deepValidateGitilesRepo(t.Context(), repoURL, branchTmpl)
		require.NoError(t, err)
		urlMock.AssertExpectations(t)
	})

	t.Run("Not Found", func(t *testing.T) {
		urlMock.MockOnce(repoURL+"/+show/main?format=JSON", mockhttpclient.MockGetError("not found", http.StatusNotFound))
		_, _, err := dv.deepValidateGitilesRepo(t.Context(), repoURL, branchTmpl)
		require.Error(t, err)
		urlMock.AssertExpectations(t)
	})
}

func TestDeepValidator_deepValidateGitHubRepo(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	repoURL := fakeGitHubRepo
	branchTmpl, err := config_vars.NewTemplate(git.MainBranch)
	require.NoError(t, err)
	require.NoError(t, dv.reg.Register(branchTmpl))

	t.Run("Success", func(t *testing.T) {
		mockGitHubAPICalls(t, urlMock, repoURL)
		_, _, err := dv.deepValidateGitHubRepo(t.Context(), repoURL, branchTmpl)
		require.NoError(t, err)
		urlMock.AssertExpectations(t)
	})

	t.Run("Not Found", func(t *testing.T) {
		refURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/%s", fakeGitHubRepoOwner, fakeGitHubRepoName, fakeGitHubRef)
		urlMock.MockOnce(refURL, mockhttpclient.MockGetError("not found", http.StatusNotFound))
		_, _, err = dv.deepValidateGitHubRepo(t.Context(), repoURL, branchTmpl)
		require.Error(t, err)
		urlMock.AssertExpectations(t)
	})
}

func TestDeepValidator_deepValidateGitRepo(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	t.Run("Gitiles", func(t *testing.T) {
		gitilesRepoURL := fakeGitRepo
		gitiles_testutils.MockGetCommit(t, urlMock, gitilesRepoURL, git.MainBranch, makeFakeCommit())
		_, _, err := dv.deepValidateGitRepo(t.Context(), gitilesRepoURL, git.MainBranch)
		require.NoError(t, err)
		urlMock.AssertExpectations(t)
	})

	t.Run("GitHub", func(t *testing.T) {
		mockGitHubAPICalls(t, urlMock, fakeGitHubRepo)
		_, _, err := dv.deepValidateGitRepo(t.Context(), fakeGitHubRepo, git.MainBranch)
		require.NoError(t, err)
		urlMock.AssertExpectations(t)
	})

	t.Run("Unknown", func(t *testing.T) {
		_, _, err := dv.deepValidateGitRepo(t.Context(), "https://unknown.com/repo", git.MainBranch)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown git repo source")
		urlMock.AssertExpectations(t)
	})
}

func TestDeepValidator_google3RepoManagerConfig(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := &config.Google3RepoManagerConfig{
		ChildRepo:   fakeGitRepo,
		ChildBranch: git.MainBranch,
	}

	t.Run("Success", func(t *testing.T) {
		gitiles_testutils.MockGetCommit(t, urlMock, cfg.ChildRepo, git.MainBranch, makeFakeCommit())
		err := dv.google3RepoManagerConfig(t.Context(), cfg)
		require.NoError(t, err)
		urlMock.AssertExpectations(t)
	})

	t.Run("Not Found", func(t *testing.T) {
		urlMock.MockOnce(cfg.ChildRepo+"/+show/main?format=JSON", mockhttpclient.MockGetError("not found", http.StatusNotFound))
		err := dv.google3RepoManagerConfig(t.Context(), cfg)
		require.Error(t, err)
		urlMock.AssertExpectations(t)
	})
}

func TestDeepValidator_androidRepoManagerConfig(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := &config.AndroidRepoManagerConfig{
		ParentRepoUrl: fakeParentRepo,
		ParentBranch:  git.MainBranch,
		ChildRepoUrl:  fakeChildRepo,
		ChildBranch:   git.MainBranch,
	}

	t.Run("Success", func(t *testing.T) {
		gitiles_testutils.MockGetCommit(t, urlMock, cfg.ParentRepoUrl, git.MainBranch, makeFakeCommit())
		gitiles_testutils.MockGetCommit(t, urlMock, cfg.ChildRepoUrl, git.MainBranch, makeFakeCommit())
		_, _, err := dv.androidRepoManagerConfig(t.Context(), cfg)
		require.NoError(t, err)
		urlMock.AssertExpectations(t)
	})

	t.Run("Parent Error", func(t *testing.T) {
		urlMock.MockOnce(cfg.ParentRepoUrl+"/+show/main?format=JSON", mockhttpclient.MockGetError("not found", http.StatusNotFound))
		_, _, err := dv.androidRepoManagerConfig(t.Context(), cfg)
		require.Error(t, err)
		urlMock.AssertExpectations(t)
	})

	t.Run("Child Error", func(t *testing.T) {
		gitiles_testutils.MockGetCommit(t, urlMock, cfg.ParentRepoUrl, git.MainBranch, makeFakeCommit())
		urlMock.MockOnce(cfg.ChildRepoUrl+"/+show/main?format=JSON", mockhttpclient.MockGetError("not found", http.StatusNotFound))
		_, _, err := dv.androidRepoManagerConfig(t.Context(), cfg)
		require.Error(t, err)
		urlMock.AssertExpectations(t)
	})
}

func TestDeepValidator_commandRepoManagerConfig(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := makeCommandRepoManagerConfig()

	t.Run("Success", func(t *testing.T) {
		mocksForCommandRepoManagerConfig(t, urlMock, cfg)
		err := dv.commandRepoManagerConfig(t.Context(), cfg)
		require.NoError(t, err)
		urlMock.AssertExpectations(t)
	})

	t.Run("Not Found", func(t *testing.T) {
		urlMock.MockOnce(cfg.GitCheckout.RepoUrl+"/+show/main?format=JSON", mockhttpclient.MockGetError("not found", http.StatusNotFound))
		err := dv.commandRepoManagerConfig(t.Context(), cfg)
		require.Error(t, err)
		urlMock.AssertExpectations(t)
	})
}

func TestDeepValidator_commandRepoManagerConfig_CommandConfig(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := &config.CommandRepoManagerConfig_CommandConfig{
		Command: []string{"./test.sh"},
	}

	t.Run("Success", func(t *testing.T) {
		err := dv.commandRepoManagerConfig_CommandConfig(t.Context(), cfg, &revision.Revision{}, func(ctx context.Context, path string) (string, error) {
			return "", nil
		})
		require.NoError(t, err)
		urlMock.AssertExpectations(t)
	})

	t.Run("Not Found", func(t *testing.T) {
		cfg.Command[0] = "./missing.sh" // This is just for clarity in the logs.
		err := dv.commandRepoManagerConfig_CommandConfig(t.Context(), cfg, &revision.Revision{}, func(ctx context.Context, path string) (string, error) {
			return "", fmt.Errorf("file not found")
		})
		require.Error(t, err)
		urlMock.AssertExpectations(t)
	})
}

func TestDeepValidator_freeTypeRepoManagerConfig(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := &config.FreeTypeRepoManagerConfig{
		Child:  makeGitilesChildConfig(),
		Parent: makeFreeTypeParentConfig(),
	}
	cfg.Child.Gitiles.RepoUrl = fakeChildRepo
	cfg.Parent.Gitiles.Gitiles.RepoUrl = fakeParentRepo

	mocksForGitilesChildConfig(t, urlMock, cfg.Child)
	mocksForFreeTypeParentConfig(t, urlMock, cfg.Parent)

	_, _, err := dv.freeTypeRepoManagerConfig(t.Context(), cfg)
	require.NoError(t, err)
}

func TestDeepValidator_gitCheckoutConfig(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := makeGitCheckoutConfig()

	t.Run("Success", func(t *testing.T) {
		gitiles_testutils.MockGetCommit(t, urlMock, cfg.RepoUrl, git.MainBranch, makeFakeCommit())
		_, _, err := dv.gitCheckoutConfig(t.Context(), cfg)
		require.NoError(t, err)
		urlMock.AssertExpectations(t)
	})

	t.Run("Not Found", func(t *testing.T) {
		urlMock.MockOnce(cfg.RepoUrl+"/+show/main?format=JSON", mockhttpclient.MockGetError("not found", http.StatusNotFound))
		_, _, err := dv.gitCheckoutConfig(t.Context(), cfg)
		require.Error(t, err)
		urlMock.AssertExpectations(t)
	})
}

func TestDeepValidator_gitilesChildConfig(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := makeGitilesChildConfig()

	mocksForGitilesChildConfig(t, urlMock, cfg)

	_, _, err := dv.gitilesChildConfig(t.Context(), cfg)
	require.NoError(t, err)
}

func TestDeepValidator_freeTypeParentConfig(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := makeFreeTypeParentConfig()

	mocksForFreeTypeParentConfig(t, urlMock, cfg)

	getFileChild := func(ctx context.Context, file string) (string, error) {
		return "", nil
	}
	_, err := dv.freeTypeParentConfig(t.Context(), cfg, getFileChild)
	require.NoError(t, err)
}

func TestDeepValidator_gitilesParentConfig(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := makeGitilesParentConfig()

	mocksForGitilesParentConfig(t, urlMock, cfg)

	getFileChild := func(ctx context.Context, file string) (string, error) {
		return "", nil
	}
	_, err := dv.gitilesParentConfig(t.Context(), cfg, getFileChild)
	require.NoError(t, err)
}

func TestDeepValidator_dependencyConfig(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := makeDependencyConfig()

	t.Run("Success", func(t *testing.T) {
		loadedFiles := map[string]bool{}
		getFile := func(ctx context.Context, path string) (string, error) {
			loadedFiles[path] = true
			return fakeDepsContent, nil
		}
		err := dv.dependencyConfig(t.Context(), cfg, getFile, getFile)
		require.NoError(t, err)
		require.True(t, loadedFiles[deps_parser.DepsFileName])
		require.True(t, loadedFiles[cfg.FindAndReplace[0]])
		urlMock.AssertExpectations(t)
	})

	t.Run("Not Found", func(t *testing.T) {
		getFile := func(ctx context.Context, path string) (string, error) {
			return fakeDepsContent, nil
		}
		getFileFail := func(ctx context.Context, path string) (string, error) {
			return "", fmt.Errorf("failed to read file")
		}
		err := dv.dependencyConfig(t.Context(), cfg, getFileFail, getFile)
		require.Error(t, err)
		urlMock.AssertExpectations(t)
	})
}

func makeCommandRepoManagerConfig() *config.CommandRepoManagerConfig {
	return &config.CommandRepoManagerConfig{
		GitCheckout: &config.GitCheckoutConfig{
			RepoUrl: fakeGitRepo,
			Branch:  git.MainBranch,
		},
		GetTipRev: &config.CommandRepoManagerConfig_CommandConfig{
			Command: []string{"./get_tip_rev.sh"},
		},
		GetPinnedRev: &config.CommandRepoManagerConfig_CommandConfig{
			Command: []string{"./get_pinned_rev.sh"},
		},
		SetPinnedRev: &config.CommandRepoManagerConfig_CommandConfig{
			Command: []string{"./set_pinned_rev.sh"},
		},
	}
}

func makeGitCheckoutConfig() *config.GitCheckoutConfig {
	return &config.GitCheckoutConfig{
		Branch:  git.MainBranch,
		RepoUrl: fakeGitRepo,
	}
}

func makeDependencyConfig() *config.DependencyConfig {
	return &config.DependencyConfig{
		Primary: makeVersionFileConfig(),
		FindAndReplace: []string{
			"some/submodule",
		},
	}
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

func makeGitilesConfig() *config.GitilesConfig {
	return &config.GitilesConfig{
		Branch:  git.MainBranch,
		RepoUrl: fakeGitRepo,
	}
}

func makeGitilesParentConfig() *config.GitilesParentConfig {
	return &config.GitilesParentConfig{
		Gerrit:  makeGerritConfig(),
		Gitiles: makeGitilesConfig(),
		Dep:     makeDependencyConfig(),
	}
}

func makeFreeTypeParentConfig() *config.FreeTypeParentConfig {
	return &config.FreeTypeParentConfig{
		Gitiles: makeGitilesParentConfig(),
	}
}

func makeGitilesChildConfig() *config.GitilesChildConfig {
	return &config.GitilesChildConfig{
		Gitiles: makeGitilesConfig(),
	}
}

func makeVersionFileConfig() *config.VersionFileConfig {
	return &config.VersionFileConfig{
		Id: "my-dep",
		File: []*config.VersionFileConfig_File{
			{
				Path: deps_parser.DepsFileName,
			},
		},
	}
}

func makeFakeCommit() *gitiles.Commit {
	return &gitiles.Commit{
		Commit: fakeCommitHash,
		Author: &gitiles.Author{
			Name:  "Author",
			Email: "author@google.com",
			Time:  time.Now().Format(gitiles.DateFormatNoTZ),
		},
		Committer: &gitiles.Author{
			Name:  "Committer",
			Email: "committer@google.com",
			Time:  time.Now().Format(gitiles.DateFormatNoTZ),
		},
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

func mockGitHubAPICalls(t *testing.T, urlMock *mockhttpclient.URLMock, repoURL string) string {
	// Mock GitHub API.
	repoOwner, repoName, err := skgithub.ParseRepoOwnerAndName(repoURL)
	require.NoError(t, err)
	sha := fakeCommitHash

	// Mock GetReference.
	refURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/%s", repoOwner, repoName, fakeGitHubRef)
	refBody, err := json.Marshal(&github.Reference{
		Object: &github.GitObject{
			SHA: &sha,
		},
	})
	require.NoError(t, err)
	urlMock.MockOnce(refURL, mockhttpclient.MockGetDialogue(refBody))

	// Mock GetCommit.
	commitURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits/%s", repoOwner, repoName, sha)
	now := time.Now()
	msg := "test commit"
	authorName := "test"
	authorEmail := "test@google.com"
	commitBody, err := json.Marshal(&github.RepositoryCommit{
		SHA: &sha,
		Commit: &github.Commit{
			Message: &msg,
			Author: &github.CommitAuthor{
				Name:  &authorName,
				Email: &authorEmail,
				Date:  &now,
			},
			Committer: &github.CommitAuthor{
				Date: &now,
			},
		},
	})
	require.NoError(t, err)
	urlMock.MockOnce(commitURL, mockhttpclient.MockGetDialogue(commitBody))
	return sha
}

func mocksForCommandRepoManagerConfig(t *testing.T, urlMock *mockhttpclient.URLMock, cfg *config.CommandRepoManagerConfig) {
	commit := makeFakeCommit()
	gitiles_testutils.MockGetCommit(t, urlMock, cfg.GitCheckout.RepoUrl, git.MainBranch, commit)
	mockCmd := func(cmd *config.CommandRepoManagerConfig_CommandConfig) {
		script := path.Base(cmd.Command[0])
		gitiles_testutils.MockReadFile(t, urlMock, cfg.GitCheckout.RepoUrl, script, commit.Commit, []byte(""), vfs.FileInfo{
			Name:  script,
			Size:  int64(0),
			Mode:  0755,
			IsDir: false,
		}.Get())
	}
	mockCmd(cfg.GetPinnedRev)
	mockCmd(cfg.GetTipRev)
	mockCmd(cfg.SetPinnedRev)
}

func mocksForDependencyConfig(t *testing.T, urlMock *mockhttpclient.URLMock, cfg *config.DependencyConfig, parentRepoURL, ref string) {
	var mockFile func(string, string)
	if strings.Contains(parentRepoURL, "googlesource") {
		mockFile = func(file, content string) {
			gitiles_testutils.MockReadFile(t, urlMock, parentRepoURL, file, ref, []byte(content), vfs.FileInfoImpl{}.Get())
		}
	} else if strings.Contains(parentRepoURL, "github") {
		repoOwner, repoName, err := skgithub.ParseRepoOwnerAndName(parentRepoURL)
		require.NoError(t, err)
		mockFile = func(file, content string) {
			url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", repoOwner, repoName, ref, file)
			urlMock.MockOnce(url, mockhttpclient.MockGetDialogue([]byte(content)))
		}
	} else {
		require.FailNow(t, "Unknown repo URL %s", parentRepoURL)
	}

	mockFile(deps_parser.DepsFileName, fakeDepsContent)
	for _, findAndReplaceFile := range cfg.FindAndReplace {
		mockFile(findAndReplaceFile, "")
	}
}

func mocksForGitilesChildConfig(t *testing.T, urlMock *mockhttpclient.URLMock, cfg *config.GitilesChildConfig) {
	commit := makeFakeCommit()
	gitiles_testutils.MockGetCommit(t, urlMock, cfg.Gitiles.RepoUrl, git.MainBranch, commit)
	gitiles_testutils.MockLog(t, urlMock, cfg.Gitiles.RepoUrl, git.LogFromTo(commit.Commit, commit.Commit), &gitiles.Log{
		Log: []*gitiles.Commit{},
	})
	// TODO(borenet): Where is this second call coming from?
	gitiles_testutils.MockGetCommit(t, urlMock, cfg.Gitiles.RepoUrl, git.MainBranch, commit)
}

func mocksForFreeTypeParentConfig(t *testing.T, urlMock *mockhttpclient.URLMock, cfg *config.FreeTypeParentConfig) {
	mocksForGitilesParentConfig(t, urlMock, cfg.Gitiles)
}

func sharedMocksForGitilesRepo(t *testing.T, urlMock *mockhttpclient.URLMock, repoURL, branch string) string {
	commit := makeFakeCommit()
	gitiles_testutils.MockGetCommit(t, urlMock, repoURL, branch, commit)
	return commit.Commit
}

func sharedMocksForGitilesParent(t *testing.T, urlMock *mockhttpclient.URLMock, repoURL, branch string) string {
	commit := sharedMocksForGitilesRepo(t, urlMock, repoURL, branch)
	mockDEPSContent(t, urlMock, repoURL, commit)
	return commit
}

func mocksForGitilesParentConfig(t *testing.T, urlMock *mockhttpclient.URLMock, cfg *config.GitilesParentConfig) {
	sharedMocksForGitilesParent(t, urlMock, cfg.Gitiles.RepoUrl, cfg.Gitiles.Branch)
	mocksForGerritConfig(t, urlMock, cfg.Gerrit)
	mocksForDependencyConfig(t, urlMock, cfg.Dep, cfg.Gitiles.RepoUrl, cfg.Gitiles.Branch)
}

func mockDEPSContent(t *testing.T, urlMock *mockhttpclient.URLMock, repoURL, commit string) {
	gitiles_testutils.MockReadFile(t, urlMock, repoURL, deps_parser.DepsFileName, commit, []byte(fakeDepsContent), vfs.FileInfo{
		Name:  deps_parser.DepsFileName,
		Size:  int64(len(fakeDepsContent)),
		Mode:  0644,
		IsDir: false,
	}.Get())
}
