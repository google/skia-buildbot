package deepvalidation

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v29/github"
	"github.com/stretchr/testify/require"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	luci_cipd "go.chromium.org/luci/cipd/client/cipd"
	cipd_common "go.chromium.org/luci/cipd/common"

	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/revision"
	buildbucket_mocks "go.skia.org/infra/go/buildbucket/mocks"
	"go.skia.org/infra/go/cipd"
	cipd_mocks "go.skia.org/infra/go/cipd/mocks"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/docker"
	docker_mocks "go.skia.org/infra/go/docker/mocks"
	"go.skia.org/infra/go/gerrit"
	gerrit_testutils "go.skia.org/infra/go/gerrit/testutils"
	"go.skia.org/infra/go/git"
	skgithub "go.skia.org/infra/go/github"
	"go.skia.org/infra/go/gitiles"
	gitiles_testutils "go.skia.org/infra/go/gitiles/testutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vfs"
)

const (
	fakeGitRepo    = "https://fake.googlesource.com/fake.git"
	fakeParentRepo = "https://fake.googlesource.com/parent.git"
	fakeChildRepo  = "https://fake.googlesource.com/child.git"

	fakeGitHubRepoOwner    = "fake"
	fakeGitHubRepoName     = "skia"
	fakeGitHubRepo         = "https://github.com/fake/skia"
	fakeGitHubForkRepoName = "fork"
	fakeGitHubForkRepo     = "https://github.com/fake/fork"
	fakeGitHubRef          = "refs/heads%2Fmain"

	fakeCommitHash = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	fakeDepsContent             = `deps = { 'path/to/my-dep': 'my-dep@123' }`
	fakeParentDepsForTransitive = `deps = { 'path/to/parent-dep': 'parent-dep@123' }`
	fakeChildDepsForTransitive  = `deps = { 'path/to/child-dep': 'child-dep@456' }`

	fakeCIPDDigest   = "wIAwHNjMc5BjlUmQFUuvLCMMXyDEmyjxMldtRXoWJVIC"
	fakeDockerDigest = "sha256:d509c16b3df2b81393476f1b6aa32b61c3aeca20238d52c39e17667f278c49a3"
)

// newTestDeepValidator creates a deepvalidator instance for testing.
func newTestDeepValidator(t *testing.T) (*deepvalidator, *mockhttpclient.URLMock) {
	urlMock := mockhttpclient.NewURLMock()
	return &deepvalidator{
		bbClient:         &buildbucket_mocks.BuildBucketInterface{},
		client:           urlMock.Client(),
		cipdClient:       &cipd_mocks.CIPDClient{},
		dockerClient:     &docker_mocks.Client{},
		githubHttpClient: urlMock.Client(),
	}, urlMock
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

	t.Run("reviewer", func(t *testing.T) {
		dv, _ := newTestDeepValidator(t)
		cfg := &config.Config{
			Reviewer: []string{
				"me@google.com",
			},
		}
		require.NoError(t, dv.deepValidate(ctx, cfg))
	})

	t.Run("commitMsg", func(t *testing.T) {
		dv, urlMock := newTestDeepValidator(t)
		defer urlMock.AssertExpectations(t)
		cfg := &config.Config{
			CommitMsg: &config.CommitMsgConfig{},
		}
		require.NoError(t, dv.deepValidate(ctx, cfg))
	})

	t.Run("parentChild", func(t *testing.T) {
		dv, urlMock := newTestDeepValidator(t)
		defer urlMock.AssertExpectations(t)
		childCfg := makeGitilesChildConfig()
		parentCfg := makeGitilesParentConfig()
		cfg := &config.Config{
			RepoManager: &config.Config_ParentChildRepoManager{
				ParentChildRepoManager: &config.ParentChildRepoManagerConfig{
					Child: &config.ParentChildRepoManagerConfig_GitilesChild{
						GitilesChild: childCfg,
					},
					Parent: &config.ParentChildRepoManagerConfig_GitilesParent{
						GitilesParent: parentCfg,
					},
				},
			},
		}
		mocksForGitilesChildConfig(t, urlMock, childCfg)
		mocksForGitilesParentConfig(t, urlMock, parentCfg)
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

	t.Run("transitive", func(t *testing.T) {
		dv, urlMock := newTestDeepValidator(t)
		defer urlMock.AssertExpectations(t)
		cfg := &config.Config{
			RepoManager: &config.Config_AndroidRepoManager{
				AndroidRepoManager: &config.AndroidRepoManagerConfig{
					ParentRepoUrl: fakeParentRepo,
					ParentBranch:  git.MainBranch,
					ChildRepoUrl:  fakeChildRepo,
					ChildBranch:   git.MainBranch,
				},
			},
			TransitiveDeps: []*config.TransitiveDepConfig{
				makeTransitiveDepConfig(),
			},
		}
		commit := makeFakeCommit()
		gitiles_testutils.MockGetCommit(t, urlMock, cfg.GetAndroidRepoManager().ParentRepoUrl, cfg.GetAndroidRepoManager().ParentBranch, commit)
		gitiles_testutils.MockGetCommit(t, urlMock, cfg.GetAndroidRepoManager().ChildRepoUrl, cfg.GetAndroidRepoManager().ChildBranch, commit)
		gitiles_testutils.MockReadFile(t, urlMock, cfg.GetAndroidRepoManager().ParentRepoUrl, deps_parser.DepsFileName, commit.Commit, []byte(fakeParentDepsForTransitive), &vfs.FileInfoImpl{})
		gitiles_testutils.MockReadFile(t, urlMock, cfg.GetAndroidRepoManager().ChildRepoUrl, deps_parser.DepsFileName, commit.Commit, []byte(fakeChildDepsForTransitive), &vfs.FileInfoImpl{})

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

func TestDeepValidator_transitiveDepConfig(t *testing.T) {
	dv, _ := newTestDeepValidator(t)

	cfg := makeTransitiveDepConfig()

	getFileParentSuccess := makeGetFileFunc(fakeParentDepsForTransitive, nil)
	getFileChildSuccess := makeGetFileFunc(fakeChildDepsForTransitive, nil)

	getFileParentFail := makeGetFileFunc("", fmt.Errorf("failed to read parent file"))
	getFileChildFail := makeGetFileFunc("", fmt.Errorf("failed to read child file"))

	t.Run("Success", func(t *testing.T) {
		err := dv.transitiveDepConfig(t.Context(), cfg, getFileParentSuccess, getFileChildSuccess)
		require.NoError(t, err)
	})

	t.Run("Parent Fails", func(t *testing.T) {
		err := dv.transitiveDepConfig(t.Context(), cfg, getFileParentFail, getFileChildSuccess)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to read parent file")
	})

	t.Run("Child Fails", func(t *testing.T) {
		err := dv.transitiveDepConfig(t.Context(), cfg, getFileParentSuccess, getFileChildFail)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to read child file")
	})
}

func TestDeepValidator_versionFileConfig(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := makeVersionFileConfig()
	getFile := makeGetFileFunc(fakeDepsContent, nil)

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
		getFile := makeGetFileFunc("", fmt.Errorf("failed to read file"))
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
	t.Run("Success", func(t *testing.T) {
		gitiles_testutils.MockGetCommit(t, urlMock, repoURL, git.MainBranch, makeFakeCommit())
		_, _, err := dv.deepValidateGitilesRepo(t.Context(), repoURL, git.MainBranch)
		require.NoError(t, err)
		urlMock.AssertExpectations(t)
	})

	t.Run("Not Found", func(t *testing.T) {
		urlMock.MockOnce(repoURL+"/+show/main?format=JSON", mockhttpclient.MockGetError("not found", http.StatusNotFound))
		_, _, err := dv.deepValidateGitilesRepo(t.Context(), repoURL, git.MainBranch)
		require.Error(t, err)
		urlMock.AssertExpectations(t)
	})
}

func TestDeepValidator_deepValidateGitHubRepo(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	repoURL := fmt.Sprintf("https://github.com/%s/%s", fakeGitHubRepoOwner, fakeGitHubRepoName)
	t.Run("Success", func(t *testing.T) {
		mockGitHubAPICalls(t, urlMock, repoURL)
		_, _, err := dv.deepValidateGitHubRepo(t.Context(), repoURL, git.MainBranch)
		require.NoError(t, err)
		urlMock.AssertExpectations(t)
	})

	t.Run("Not Found", func(t *testing.T) {
		refURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/%s", fakeGitHubRepoOwner, fakeGitHubRepoName, fakeGitHubRef)
		urlMock.MockOnce(refURL, mockhttpclient.MockGetError("not found", http.StatusNotFound))
		_, _, err := dv.deepValidateGitHubRepo(t.Context(), repoURL, git.MainBranch)
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
		err := dv.commandRepoManagerConfig_CommandConfig(t.Context(), cfg, &revision.Revision{}, makeGetFileFunc("", nil))
		require.NoError(t, err)
		urlMock.AssertExpectations(t)
	})

	t.Run("Not Found", func(t *testing.T) {
		cfg.Command[0] = "./missing.sh" // This is just for clarity in the logs.
		err := dv.commandRepoManagerConfig_CommandConfig(t.Context(), cfg, &revision.Revision{}, makeGetFileFunc("", fmt.Errorf("file not found")))
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

func TestDeepValidator_gitCheckoutGitHubChildConfig(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := makeGitCheckoutGitHubChildConfig()

	t.Run("Success", func(t *testing.T) {
		mocksForGitCheckoutGitHubChildConfig(t, urlMock, cfg)
		_, _, err := dv.gitCheckoutGitHubChildConfig(t.Context(), cfg)
		require.NoError(t, err)
		urlMock.AssertExpectations(t)
	})

	t.Run("Not Found", func(t *testing.T) {
		repoOwner, repoName, err := skgithub.ParseRepoOwnerAndName(cfg.GitCheckout.GitCheckout.RepoUrl)
		require.NoError(t, err)
		ref := "refs/heads%2Fmain"
		refURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/%s", repoOwner, repoName, ref)
		urlMock.MockOnce(refURL, mockhttpclient.MockGetError("not found", http.StatusNotFound))
		_, _, err = dv.gitCheckoutGitHubChildConfig(t.Context(), cfg)
		require.Error(t, err)
		urlMock.AssertExpectations(t)
	})
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

	getFileChild := makeGetFileFunc("", nil)
	_, err := dv.freeTypeParentConfig(t.Context(), cfg, getFileChild)
	require.NoError(t, err)
}

func TestDeepValidator_gitilesParentConfig(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := makeGitilesParentConfig()

	mocksForGitilesParentConfig(t, urlMock, cfg)

	getFileChild := makeGetFileFunc("", nil)
	_, err := dv.gitilesParentConfig(t.Context(), cfg, getFileChild)
	require.NoError(t, err)
}

func TestDeepValidator_goModGerritParentConfig(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := makeGoModGerritParentConfig()

	mocksForGoModGerritParentConfig(t, urlMock, cfg)

	_, err := dv.goModGerritParentConfig(t.Context(), cfg, &revision.Revision{})
	require.NoError(t, err)
}

func TestDeepValidator_goModParentConfig(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := makeGoModParentConfig()

	mocksForGoModParentConfig(t, urlMock, cfg)

	_, err := dv.goModParentConfig(t.Context(), cfg, &revision.Revision{})
	require.NoError(t, err)
}

func TestDeepValidator_dependencyConfig(t *testing.T) {
	dv, _ := newTestDeepValidator(t)

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
	})

	t.Run("Not Found", func(t *testing.T) {
		getFile := makeGetFileFunc(fakeDepsContent, nil)
		getFileFail := makeGetFileFunc("", fmt.Errorf("failed to read file"))
		err := dv.dependencyConfig(t.Context(), cfg, getFileFail, getFile)
		require.Error(t, err)
	})
}

func TestDeepValidator_fuchsiaSDKChildConfig(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := makeFuchsiaSDKChildConfig()

	mocksForFuchsiaSDKChildConfig(t, urlMock, cfg)

	_, err := dv.fuchsiaSDKChildConfig(t.Context(), cfg)
	require.NoError(t, err)
}

func TestDeepValidator_cipdChildConfig(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := makeCIPDChildConfig()
	mocksForCIPDChildConfig(t, dv, cfg, nil)

	_, err := dv.cipdChildConfig(t.Context(), cfg)
	require.NoError(t, err)
}

func TestDeepValidator_cipdChildConfig_GitilesRepo(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := makeCIPDChildConfig()
	cfg.GitilesRepo = fakeGitRepo

	// Mock Gitiles calls.
	parent := makeFakeCommit()
	parent.Commit = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	head := makeFakeCommit()
	head.Parents = []string{parent.Commit}
	gitiles_testutils.MockGetCommit(t, urlMock, cfg.GitilesRepo, git.MainBranch, head)
	gitiles_testutils.MockGetCommit(t, urlMock, cfg.GitilesRepo, head.Commit, head)
	gitiles_testutils.MockGetCommit(t, urlMock, cfg.GitilesRepo, parent.Commit, parent)
	gitiles_testutils.MockLog(t, urlMock, cfg.GitilesRepo, git.LogFromTo(parent.Commit, head.Commit), &gitiles.Log{
		Log: []*gitiles.Commit{head},
	})

	// Mock CIPD calls.
	mocksForCIPDChildConfig(t, dv, cfg, []string{child.CIPDGitRevisionTag(head.Commit)})

	_, err := dv.cipdChildConfig(t.Context(), cfg)
	require.NoError(t, err)
}

func TestDeepValidator_cipdChildConfig_SourceRepo(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := makeCIPDChildConfig()
	cfg.SourceRepo = makeGitilesConfig()

	// Mock Gitiles calls.
	parent := makeFakeCommit()
	parent.Commit = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	head := makeFakeCommit()
	head.Parents = []string{parent.Commit}
	gitiles_testutils.MockGetCommit(t, urlMock, cfg.SourceRepo.RepoUrl, git.MainBranch, head)
	gitiles_testutils.MockGetCommit(t, urlMock, cfg.SourceRepo.RepoUrl, head.Commit, head)
	gitiles_testutils.MockGetCommit(t, urlMock, cfg.SourceRepo.RepoUrl, parent.Commit, parent)
	gitiles_testutils.MockLog(t, urlMock, cfg.SourceRepo.RepoUrl, git.LogFromTo(parent.Commit, head.Commit), &gitiles.Log{
		Log: []*gitiles.Commit{head},
	})

	// Mock CIPD calls.
	mocksForCIPDChildConfig(t, dv, cfg, []string{child.CIPDGitRevisionTag(head.Commit)})

	_, err := dv.cipdChildConfig(t.Context(), cfg)
	require.NoError(t, err)
}

func TestDeepValidator_cipdChildConfig_RevisionIdTag(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := makeCIPDChildConfig()
	cfg.RevisionIdTag = "custom-revision-id"
	const revisionID = "some-revision-id"

	// Mock CIPD calls.
	mocksForCIPDChildConfig(t, dv, cfg, []string{
		cipd.JoinTag(cfg.RevisionIdTag, revisionID),
	})

	rev, err := dv.cipdChildConfig(t.Context(), cfg)
	require.NoError(t, err)
	require.Equal(t, cfg.RevisionIdTag+":"+revisionID, rev.Id)
}

func TestDeepValidator_cipdChildConfig_RevisionIdTag_StripKey(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := makeCIPDChildConfig()
	cfg.RevisionIdTag = "custom-revision-id"
	cfg.RevisionIdTagStripKey = true
	const revisionID = "some-revision-id"

	// Mock CIPD calls.
	mocksForCIPDChildConfig(t, dv, cfg, []string{
		cipd.JoinTag(cfg.RevisionIdTag, revisionID),
	})

	rev, err := dv.cipdChildConfig(t.Context(), cfg)
	require.NoError(t, err)
	require.Equal(t, revisionID, rev.Id)
}

func TestDeepValidator_gitCheckoutChildConfig(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := makeGitCheckoutChildConfig()

	t.Run("Success", func(t *testing.T) {
		mocksForGitCheckoutChildConfig(t, urlMock, cfg)
		_, _, err := dv.gitCheckoutChildConfig(t.Context(), cfg)
		require.NoError(t, err)
		urlMock.AssertExpectations(t)
	})

	t.Run("Not Found", func(t *testing.T) {
		urlMock.MockOnce(cfg.GitCheckout.RepoUrl+"/+show/main?format=JSON", mockhttpclient.MockGetError("not found", http.StatusNotFound))
		_, _, err := dv.gitCheckoutChildConfig(t.Context(), cfg)
		require.Error(t, err)
		urlMock.AssertExpectations(t)
	})
}

func TestDeepValidator_semVerGCSChildConfig(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := makeSemVerGCSChildConfig()

	mocksForSemVerGCSChildConfig(t, urlMock, cfg)

	_, err := dv.semVerGCSChildConfig(t.Context(), cfg)
	require.NoError(t, err)
}

func TestDeepValidator_copyParentConfig(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := makeCopyParentConfig()

	mocksForCopyParentConfig(t, urlMock, cfg)

	getFileChild := func(ctx context.Context, path string) (string, error) {
		require.Equal(t, cfg.Copies[0].SrcRelPath, path)
		return "child content", nil
	}

	_, err := dv.copyParentConfig(t.Context(), cfg, getFileChild)
	require.NoError(t, err)
}

func TestDeepValidator_copyParentConfig_CopyEntry(t *testing.T) {
	dv, _ := newTestDeepValidator(t)

	cfg := makeCopyParentConfig_CopyEntry()

	getFileSuccess := makeGetFileFunc("some content", nil)
	getFileFail := makeGetFileFunc("", fmt.Errorf("file not found"))

	t.Run("Success", func(t *testing.T) {
		err := dv.copyParentConfig_CopyEntry(t.Context(), cfg, getFileSuccess, getFileSuccess)
		require.NoError(t, err)
	})

	t.Run("Src Fails", func(t *testing.T) {
		err := dv.copyParentConfig_CopyEntry(t.Context(), cfg, getFileFail, getFileSuccess)
		require.Error(t, err)
		require.Contains(t, err.Error(), "file not found")
	})

	t.Run("Dst Fails", func(t *testing.T) {
		err := dv.copyParentConfig_CopyEntry(t.Context(), cfg, getFileSuccess, getFileFail)
		require.Error(t, err)
		require.Contains(t, err.Error(), "file not found")
	})
}

func TestDeepValidator_depsLocalGitHubParentConfig(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := makeDEPSLocalGitHubParentConfig()

	mocksForDepsLocalGitHubParentConfig(t, dv, urlMock, cfg)

	getFileChild := makeGetFileFunc("", nil)
	_, err := dv.depsLocalGitHubParentConfig(t.Context(), cfg, &revision.Revision{}, getFileChild)
	require.NoError(t, err)
}

func TestDeepValidator_depsLocalParentConfig(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := makeDEPSLocalParentConfig()
	mocksForDepsLocalParentConfig(t, dv, urlMock, cfg)

	getFileChild := makeGetFileFunc("", nil)
	tipRev := &revision.Revision{
		Id: "test-rev",
	}
	_, err := dv.depsLocalParentConfig(t.Context(), cfg, tipRev, getFileChild)
	require.NoError(t, err)

	// Test failure case.
	repoURL := cfg.GitCheckout.GitCheckout.RepoUrl
	urlMock.MockOnce(repoURL+"/+show/main?format=JSON", mockhttpclient.MockGetError("not found", http.StatusNotFound))
	_, err = dv.depsLocalParentConfig(t.Context(), cfg, tipRev, getFileChild)
	require.Error(t, err)
}

func TestDeepValidator_gitCheckoutParentConfig(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := makeGitCheckoutParentConfig()

	mocksForGitCheckoutParentConfig(t, urlMock, cfg)

	getFileChild := makeGetFileFunc("child content", nil)
	_, err := dv.gitCheckoutParentConfig(t.Context(), cfg, getFileChild)
	require.NoError(t, err)

	// Test failure case.
	urlMock.MockOnce(cfg.GitCheckout.RepoUrl+"/+show/main?format=JSON", mockhttpclient.MockGetError("not found", http.StatusNotFound))
	_, err = dv.gitCheckoutParentConfig(t.Context(), cfg, getFileChild)
	require.Error(t, err)
}

func TestDeepValidator_depsLocalGerritParentConfig(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := makeDEPSLocalGerritParentConfig()

	mocksForDepsLocalGerritParentConfig(t, dv, urlMock, cfg)

	getFileChild := makeGetFileFunc("child content", nil)
	_, err := dv.depsLocalGerritParentConfig(t.Context(), cfg, &revision.Revision{}, getFileChild)
	require.NoError(t, err)
}

func TestDeepValidator_gitCheckoutGitHubFileParentConfig(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := makeGitCheckoutGitHubFileParentConfig()

	mocksForGitCheckoutGitHubFileParentConfig(t, urlMock, cfg)

	getFileChild := makeGetFileFunc("child content", nil)
	_, err := dv.gitCheckoutGitHubFileParentConfig(t.Context(), cfg, &revision.Revision{}, getFileChild)
	require.NoError(t, err)
}

func TestDeepValidator_gitCheckoutGitHubParentConfig(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := makeGitCheckoutGitHubParentConfig()

	mocksForGitCheckoutGitHubParentConfig(t, urlMock, cfg)

	getFileChild := makeGetFileFunc("child content", nil)
	_, err := dv.gitCheckoutGitHubParentConfig(t.Context(), cfg, getFileChild)
	require.NoError(t, err)
}

func TestDeepValidator_gitCheckoutGerritParentConfig(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := makeGitCheckoutGerritParentConfig()

	mocksForGitCheckoutGerritParentConfig(t, urlMock, cfg)

	getFileChild := makeGetFileFunc("child content", nil)
	_, err := dv.gitCheckoutGerritParentConfig(t.Context(), cfg, &revision.Revision{}, getFileChild)
	require.NoError(t, err)
}

func TestDeepValidator_buildbucketRevisionFilterConfig(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := &config.BuildbucketRevisionFilterConfig{
		Project:            "my-project",
		Bucket:             "my-bucket",
		Builder:            []string{"my-builder"},
		BuildsetCommitTmpl: "commit/gitiles/skia.googlesource.com/skia/+/%s",
	}

	rev := &revision.Revision{Id: "test"}
	pred := &buildbucketpb.BuildPredicate{
		Builder: &buildbucketpb.BuilderID{
			Project: cfg.Project,
			Bucket:  cfg.Bucket,
		},
		Tags: []*buildbucketpb.StringPair{
			{Key: "buildset", Value: fmt.Sprintf(cfg.BuildsetCommitTmpl, rev.Id)},
		},
	}
	resp := []*buildbucketpb.Build{}
	dv.bbClient.(*buildbucket_mocks.BuildBucketInterface).On("Search", testutils.AnyContext, pred).Return(resp, nil)

	require.NoError(t, dv.buildbucketRevisionFilterConfig(t.Context(), cfg, rev))
}

func TestDeepValidator_cipdRevisionFilterConfig(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := &config.CIPDRevisionFilterConfig{
		Package:  []string{"my-package"},
		Platform: []string{"linux-amd64"},
		TagKey:   "my-tag",
	}

	rev := &revision.Revision{Id: "tag-value"}

	// Mock CIPD calls.
	pkg := cfg.Package[0] + "/" + cfg.Platform[0]
	tags := []string{cipd.JoinTag(cfg.TagKey, rev.Id)}
	pin := cipd_common.Pin{
		PackageName: pkg,
		InstanceID:  fakeCIPDDigest,
	}
	dv.cipdClient.(*cipd_mocks.CIPDClient).On("SearchInstances", testutils.AnyContext, pkg, tags).Return(cipd_common.PinSlice([]cipd_common.Pin{pin}), nil)

	require.NoError(t, dv.cipdRevisionFilterConfig(t.Context(), cfg, rev))
}

func TestDeepValidator_dockerChildConfig(t *testing.T) {
	dv, _ := newTestDeepValidator(t)
	defer dv.dockerClient.(*docker_mocks.Client).AssertExpectations(t)

	cfg := makeDockerChildConfig()

	mocksForDockerChildConfig(t, dv.dockerClient.(*docker_mocks.Client), cfg)

	rev, err := dv.dockerChildConfig(t.Context(), cfg)
	require.NoError(t, err)
	require.Equal(t, fakeDockerDigest, rev.Id)
}

func TestDeepValidator_preUploadConfig(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := makePreUploadConfig()

	mocksForPreUploadConfig(t, dv, cfg)

	getFile := makeGetFileFunc("", nil)
	tipRev := &revision.Revision{
		Id: "test-rev",
	}
	require.NoError(t, dv.preUploadConfig(t.Context(), cfg, tipRev, getFile))
}

func TestDeepValidator_preUploadConfig_executableInCIPD_PATH(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := makePreUploadConfig()
	cfg.Command[0].Command = "some-executable-from-cipd --some-flag"
	cfg.Command[0].Env = []string{"PATH=${cipd_root}/bin:${PATH}"}

	mocksForPreUploadConfig(t, dv, cfg)

	getFile := makeGetFileFunc("", nil)
	tipRev := &revision.Revision{
		Id: "test-rev",
	}
	require.NoError(t, dv.preUploadConfig(t.Context(), cfg, tipRev, getFile))
}

func TestDeepValidator_preUploadConfig_executableInCIPD_AbsolutePath(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := makePreUploadConfig()
	cfg.Command[0].Command = "${cipd_root}/bin/some-executable-from-cipd --some-flag"

	mocksForPreUploadConfig(t, dv, cfg)

	getFile := makeGetFileFunc("", nil)
	tipRev := &revision.Revision{
		Id: "test-rev",
	}
	require.NoError(t, dv.preUploadConfig(t.Context(), cfg, tipRev, getFile))
}

func TestDeepValidator_parentChildRepoManagerConfig(t *testing.T) {
	// Shorthand for functions to set up mocks for parent and child,
	// respectively.
	type mockParentFunc func(*testing.T, *deepvalidator, *mockhttpclient.URLMock)
	type mockChildFunc func(*testing.T, *deepvalidator, *mockhttpclient.URLMock, string)

	// test is a helper function that actually tests a combination of parent
	// and child types.
	test := func(name string, parentCfg *config.ParentChildRepoManagerConfig, parentMocks mockParentFunc, childCfg *config.ParentChildRepoManagerConfig, childMocks mockChildFunc) {
		t.Run(name, func(t *testing.T) {
			dv, urlMock := newTestDeepValidator(t)

			// Merge the configs.
			cfg := &config.ParentChildRepoManagerConfig{
				Parent: parentCfg.Parent,
				Child:  childCfg.Child,
			}

			// Need to ensure that the child mocks any requests to read a
			// copied file.
			copiedFile := ""
			if parentCfg.GetCopyParent() != nil {
				copiedFile = parentCfg.GetCopyParent().Copies[0].SrcRelPath
			}

			// Mock the Child first, since deepvalidator.parentChildRepoManager
			// validates the Child first.
			childMocks(t, dv, urlMock, copiedFile)
			parentMocks(t, dv, urlMock)

			// Merge the configs and run the test.

			_, _, err := dv.parentChildRepoManagerConfig(t.Context(), cfg)
			require.NoError(t, err)
			urlMock.AssertExpectations(t)
		})
	}

	// Create all combinations of parent and child configs.
	//
	// We're using a full ParentChildRepoManagerConfig for both the parent and
	// child because the config.isParentChildRepoManagerConfig_Parent and
	// config.isParentChildRepoManagerConfig_Child interfaces are private, which
	// makes passing around any other type difficult.
	copyParentCfg := &config.ParentChildRepoManagerConfig{
		Parent: &config.ParentChildRepoManagerConfig_CopyParent{
			CopyParent: makeCopyParentConfig(),
		},
	}
	copyParentMocks := func(t *testing.T, dv *deepvalidator, urlMock *mockhttpclient.URLMock) {
		mocksForCopyParentConfig(t, urlMock, copyParentCfg.GetCopyParent())
	}
	depsLocalGithubParentCfg := &config.ParentChildRepoManagerConfig{
		Parent: &config.ParentChildRepoManagerConfig_DepsLocalGithubParent{DepsLocalGithubParent: makeDEPSLocalGitHubParentConfig()},
	}
	depsLocalGithubParentMocks := func(t *testing.T, dv *deepvalidator, urlMock *mockhttpclient.URLMock) {
		mocksForDepsLocalGitHubParentConfig(t, dv, urlMock, depsLocalGithubParentCfg.GetDepsLocalGithubParent())
	}
	depsLocalGerritParentCfg := &config.ParentChildRepoManagerConfig{
		Parent: &config.ParentChildRepoManagerConfig_DepsLocalGerritParent{DepsLocalGerritParent: makeDEPSLocalGerritParentConfig()},
	}
	depsLocalGerritParentMocks := func(t *testing.T, dv *deepvalidator, urlMock *mockhttpclient.URLMock) {
		mocksForDepsLocalGerritParentConfig(t, dv, urlMock, depsLocalGerritParentCfg.GetDepsLocalGerritParent())
	}
	gitCheckoutGithubFileParentCfg := &config.ParentChildRepoManagerConfig{
		Parent: &config.ParentChildRepoManagerConfig_GitCheckoutGithubFileParent{GitCheckoutGithubFileParent: makeGitCheckoutGitHubFileParentConfig()},
	}
	gitCheckoutGithubFileParentMocks := func(t *testing.T, dv *deepvalidator, urlMock *mockhttpclient.URLMock) {
		mocksForGitCheckoutGitHubFileParentConfig(t, urlMock, gitCheckoutGithubFileParentCfg.GetGitCheckoutGithubFileParent())
	}
	gitilesParentCfg := &config.ParentChildRepoManagerConfig{
		Parent: &config.ParentChildRepoManagerConfig_GitilesParent{GitilesParent: makeGitilesParentConfig()},
	}
	gitilesParentMocks := func(t *testing.T, dv *deepvalidator, urlMock *mockhttpclient.URLMock) {
		mocksForGitilesParentConfig(t, urlMock, gitilesParentCfg.GetGitilesParent())
	}
	goModGerritParentCfg := &config.ParentChildRepoManagerConfig{
		Parent: &config.ParentChildRepoManagerConfig_GoModGerritParent{GoModGerritParent: makeGoModGerritParentConfig()},
	}
	goModGerritParentMocks := func(t *testing.T, dv *deepvalidator, urlMock *mockhttpclient.URLMock) {
		mocksForGoModGerritParentConfig(t, urlMock, goModGerritParentCfg.GetGoModGerritParent())
	}
	gitCheckoutGerritParentCfg := &config.ParentChildRepoManagerConfig{
		Parent: &config.ParentChildRepoManagerConfig_GitCheckoutGerritParent{GitCheckoutGerritParent: makeGitCheckoutGerritParentConfig()},
	}
	gitCheckoutGerritParentMocks := func(t *testing.T, dv *deepvalidator, urlMock *mockhttpclient.URLMock) {
		mocksForGitCheckoutGerritParentConfig(t, urlMock, gitCheckoutGerritParentCfg.GetGitCheckoutGerritParent())
	}

	cipdChildCfg := &config.ParentChildRepoManagerConfig{
		Child: &config.ParentChildRepoManagerConfig_CipdChild{
			CipdChild: makeCIPDChildConfig(),
		},
	}
	cipdChildMocks := func(t *testing.T, dv *deepvalidator, urlMock *mockhttpclient.URLMock, copiedFile string) {
		mocksForCIPDChildConfig(t, dv, cipdChildCfg.GetCipdChild(), nil)
	}
	fuchsiaSdkChildCfg := &config.ParentChildRepoManagerConfig{
		Child: &config.ParentChildRepoManagerConfig_FuchsiaSdkChild{
			FuchsiaSdkChild: makeFuchsiaSDKChildConfig(),
		},
	}
	fuchsiaSdkChildMocks := func(t *testing.T, dv *deepvalidator, urlMock *mockhttpclient.URLMock, copiedFile string) {
		mocksForFuchsiaSDKChildConfig(t, urlMock, fuchsiaSdkChildCfg.GetFuchsiaSdkChild())
	}
	gitCheckoutChildCfg := &config.ParentChildRepoManagerConfig{
		Child: &config.ParentChildRepoManagerConfig_GitCheckoutChild{
			GitCheckoutChild: makeGitCheckoutChildConfig(),
		},
	}
	gitCheckoutChildMocks := func(t *testing.T, dv *deepvalidator, urlMock *mockhttpclient.URLMock, copiedFile string) {
		cfg := gitCheckoutChildCfg.GetGitCheckoutChild()
		sha := mocksForGitCheckoutChildConfig(t, urlMock, cfg)
		if copiedFile != "" {
			gitiles_testutils.MockReadFile(t, urlMock, cfg.GitCheckout.RepoUrl, copiedFile, sha, []byte(""), vfs.FileInfoImpl{}.Get())
		}
	}
	gitCheckoutGithubChildCfg := &config.ParentChildRepoManagerConfig{
		Child: &config.ParentChildRepoManagerConfig_GitCheckoutGithubChild{
			GitCheckoutGithubChild: makeGitCheckoutGitHubChildConfig(),
		},
	}
	gitCheckoutGithubChildMocks := func(t *testing.T, dv *deepvalidator, urlMock *mockhttpclient.URLMock, copiedFile string) {
		cfg := gitCheckoutGithubChildCfg.GetGitCheckoutGithubChild()
		sha := mocksForGitCheckoutGitHubChildConfig(t, urlMock, cfg)
		if copiedFile != "" {
			contentsURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", cfg.RepoOwner, cfg.RepoName, sha, copiedFile)
			urlMock.MockOnce(contentsURL, mockhttpclient.MockGetDialogue([]byte("")))
		}
	}
	gitilesChildCfg := &config.ParentChildRepoManagerConfig{
		Child: &config.ParentChildRepoManagerConfig_GitilesChild{
			GitilesChild: makeGitilesChildConfig(),
		},
	}
	gitilesChildMocks := func(t *testing.T, dv *deepvalidator, urlMock *mockhttpclient.URLMock, copiedFile string) {
		cfg := gitilesChildCfg.GetGitilesChild()
		mocksForGitilesChildConfig(t, urlMock, cfg)
		if copiedFile != "" {
			gitiles_testutils.MockReadFile(t, urlMock, cfg.Gitiles.RepoUrl, copiedFile, cfg.Gitiles.Branch, []byte(""), vfs.FileInfoImpl{}.Get())
		}
	}
	semverGcsChildCfg := &config.ParentChildRepoManagerConfig{
		Child: &config.ParentChildRepoManagerConfig_SemverGcsChild{
			SemverGcsChild: makeSemVerGCSChildConfig(),
		},
	}
	semverGcsChildMocks := func(t *testing.T, dv *deepvalidator, urlMock *mockhttpclient.URLMock, copiedFile string) {
		mocksForSemVerGCSChildConfig(t, urlMock, semverGcsChildCfg.GetSemverGcsChild())
	}
	dockerChildCfg := &config.ParentChildRepoManagerConfig{
		Child: &config.ParentChildRepoManagerConfig_DockerChild{
			DockerChild: makeDockerChildConfig(),
		},
	}
	dockerChildMocks := func(t *testing.T, dv *deepvalidator, urlMock *mockhttpclient.URLMock, copiedFile string) {
		mocksForDockerChildConfig(t, dv.dockerClient.(*docker_mocks.Client), dockerChildCfg.GetDockerChild())
	}

	test("CopyParent_GitCheckoutChild", copyParentCfg, copyParentMocks, gitCheckoutChildCfg, gitCheckoutChildMocks)
	test("CopyParent_GitCheckoutGithubChild", copyParentCfg, copyParentMocks, gitCheckoutGithubChildCfg, gitCheckoutGithubChildMocks)
	test("CopyParent_GitilesChild", copyParentCfg, copyParentMocks, gitilesChildCfg, gitilesChildMocks)

	test("DepsLocalGithubParent_CipdChild", depsLocalGithubParentCfg, depsLocalGithubParentMocks, cipdChildCfg, cipdChildMocks)
	test("DepsLocalGithubParent_FuchsiaSdkChild", depsLocalGithubParentCfg, depsLocalGithubParentMocks, fuchsiaSdkChildCfg, fuchsiaSdkChildMocks)
	test("DepsLocalGithubParent_GitCheckoutChild", depsLocalGithubParentCfg, depsLocalGithubParentMocks, gitCheckoutChildCfg, gitCheckoutChildMocks)
	test("DepsLocalGithubParent_GitCheckoutGithubChild", depsLocalGithubParentCfg, depsLocalGithubParentMocks, gitCheckoutGithubChildCfg, gitCheckoutGithubChildMocks)
	test("DepsLocalGithubParent_GitilesChild", depsLocalGithubParentCfg, depsLocalGithubParentMocks, gitilesChildCfg, gitilesChildMocks)
	test("DepsLocalGithubParent_SemverGcsChild", depsLocalGithubParentCfg, depsLocalGithubParentMocks, semverGcsChildCfg, semverGcsChildMocks)
	test("DepsLocalGithubParent_DockerChild", depsLocalGithubParentCfg, depsLocalGithubParentMocks, dockerChildCfg, dockerChildMocks)

	test("DepsLocalGerritParent_CipdChild", depsLocalGerritParentCfg, depsLocalGerritParentMocks, cipdChildCfg, cipdChildMocks)
	test("DepsLocalGerritParent_FuchsiaSdkChild", depsLocalGerritParentCfg, depsLocalGerritParentMocks, fuchsiaSdkChildCfg, fuchsiaSdkChildMocks)
	test("DepsLocalGerritParent_GitCheckoutChild", depsLocalGerritParentCfg, depsLocalGerritParentMocks, gitCheckoutChildCfg, gitCheckoutChildMocks)
	test("DepsLocalGerritParent_GitCheckoutGithubChild", depsLocalGerritParentCfg, depsLocalGerritParentMocks, gitCheckoutGithubChildCfg, gitCheckoutGithubChildMocks)
	test("DepsLocalGerritParent_GitilesChild", depsLocalGerritParentCfg, depsLocalGerritParentMocks, gitilesChildCfg, gitilesChildMocks)
	test("DepsLocalGerritParent_SemverGcsChild", depsLocalGerritParentCfg, depsLocalGerritParentMocks, semverGcsChildCfg, semverGcsChildMocks)
	test("DepsLocalGerritParent_DockerChild", depsLocalGerritParentCfg, depsLocalGerritParentMocks, dockerChildCfg, dockerChildMocks)

	test("GitCheckoutGithubFileParent_CipdChild", gitCheckoutGithubFileParentCfg, gitCheckoutGithubFileParentMocks, cipdChildCfg, cipdChildMocks)
	test("GitCheckoutGithubFileParent_FuchsiaSdkChild", gitCheckoutGithubFileParentCfg, gitCheckoutGithubFileParentMocks, fuchsiaSdkChildCfg, fuchsiaSdkChildMocks)
	test("GitCheckoutGithubFileParent_GitCheckoutChild", gitCheckoutGithubFileParentCfg, gitCheckoutGithubFileParentMocks, gitCheckoutChildCfg, gitCheckoutChildMocks)
	test("GitCheckoutGithubFileParent_GitCheckoutGithubChild", gitCheckoutGithubFileParentCfg, gitCheckoutGithubFileParentMocks, gitCheckoutGithubChildCfg, gitCheckoutGithubChildMocks)
	test("GitCheckoutGithubFileParent_GitilesChild", gitCheckoutGithubFileParentCfg, gitCheckoutGithubFileParentMocks, gitilesChildCfg, gitilesChildMocks)
	test("GitCheckoutGithubFileParent_SemverGcsChild", gitCheckoutGithubFileParentCfg, gitCheckoutGithubFileParentMocks, semverGcsChildCfg, semverGcsChildMocks)
	test("GitCheckoutGithubFileParent_DockerChild", gitCheckoutGithubFileParentCfg, gitCheckoutGithubFileParentMocks, dockerChildCfg, dockerChildMocks)

	test("GitilesParent_CipdChild", gitilesParentCfg, gitilesParentMocks, cipdChildCfg, cipdChildMocks)
	test("GitilesParent_FuchsiaSdkChild", gitilesParentCfg, gitilesParentMocks, fuchsiaSdkChildCfg, fuchsiaSdkChildMocks)
	test("GitilesParent_GitCheckoutChild", gitilesParentCfg, gitilesParentMocks, gitCheckoutChildCfg, gitCheckoutChildMocks)
	test("GitilesParent_GitCheckoutGithubChild", gitilesParentCfg, gitilesParentMocks, gitCheckoutGithubChildCfg, gitCheckoutGithubChildMocks)
	test("GitilesParent_GitilesChild", gitilesParentCfg, gitilesParentMocks, gitilesChildCfg, gitilesChildMocks)
	test("GitilesParent_SemverGcsChild", gitilesParentCfg, gitilesParentMocks, semverGcsChildCfg, semverGcsChildMocks)
	test("GitilesParent_DockerChild", gitilesParentCfg, gitilesParentMocks, dockerChildCfg, dockerChildMocks)

	test("GoModGerritParent_CipdChild", goModGerritParentCfg, goModGerritParentMocks, cipdChildCfg, cipdChildMocks)
	test("GoModGerritParent_FuchsiaSdkChild", goModGerritParentCfg, goModGerritParentMocks, fuchsiaSdkChildCfg, fuchsiaSdkChildMocks)
	test("GoModGerritParent_GitCheckoutChild", goModGerritParentCfg, goModGerritParentMocks, gitCheckoutChildCfg, gitCheckoutChildMocks)
	test("GoModGerritParent_GitCheckoutGithubChild", goModGerritParentCfg, goModGerritParentMocks, gitCheckoutGithubChildCfg, gitCheckoutGithubChildMocks)
	test("GoModGerritParent_GitilesChild", goModGerritParentCfg, goModGerritParentMocks, gitilesChildCfg, gitilesChildMocks)
	test("GoModGerritParent_SemverGcsChild", goModGerritParentCfg, goModGerritParentMocks, semverGcsChildCfg, semverGcsChildMocks)
	test("GoModGerritParent_DockerChild", goModGerritParentCfg, goModGerritParentMocks, dockerChildCfg, dockerChildMocks)

	test("GitCheckoutGerritParent_CipdChild", gitCheckoutGerritParentCfg, gitCheckoutGerritParentMocks, cipdChildCfg, cipdChildMocks)
	test("GitCheckoutGerritParent_FuchsiaSdkChild", gitCheckoutGerritParentCfg, gitCheckoutGerritParentMocks, fuchsiaSdkChildCfg, fuchsiaSdkChildMocks)
	test("GitCheckoutGerritParent_GitCheckoutChild", gitCheckoutGerritParentCfg, gitCheckoutGerritParentMocks, gitCheckoutChildCfg, gitCheckoutChildMocks)
	test("GitCheckoutGerritParent_GitCheckoutGithubChild", gitCheckoutGerritParentCfg, gitCheckoutGerritParentMocks, gitCheckoutGithubChildCfg, gitCheckoutGithubChildMocks)
	test("GitCheckoutGerritParent_GitilesChild", gitCheckoutGerritParentCfg, gitCheckoutGerritParentMocks, gitilesChildCfg, gitilesChildMocks)
	test("GitCheckoutGerritParent_SemverGcsChild", gitCheckoutGerritParentCfg, gitCheckoutGerritParentMocks, semverGcsChildCfg, semverGcsChildMocks)
	test("GitCheckoutGerritParent_DockerChild", gitCheckoutGerritParentCfg, gitCheckoutGerritParentMocks, dockerChildCfg, dockerChildMocks)

}

func TestDeepValidator_reviewer(t *testing.T) {
	t.Run("Email", func(t *testing.T) {
		dv, _ := newTestDeepValidator(t)
		require.NoError(t, dv.reviewer(t.Context(), "me@google.com"))
	})

	const rotationURL = "https://my-rotation.google.com"

	t.Run("Rotation", func(t *testing.T) {
		dv, urlMock := newTestDeepValidator(t)
		defer urlMock.AssertExpectations(t)
		urlMock.MockOnce(rotationURL, mockhttpclient.MockGetDialogue([]byte(`{"emails":["reviewer@google.com"]}`)))
		require.NoError(t, dv.reviewer(t.Context(), rotationURL))
	})
	t.Run("Rotation Failed Fetch", func(t *testing.T) {
		dv, urlMock := newTestDeepValidator(t)
		defer urlMock.AssertExpectations(t)
		urlMock.MockOnce(rotationURL, mockhttpclient.MockGetError(http.StatusText(http.StatusNotFound), http.StatusNotFound))
		require.Error(t, dv.reviewer(t.Context(), rotationURL))
	})
}

func TestDeepValidator_commitMsg(t *testing.T) {
	dv, urlMock := newTestDeepValidator(t)
	defer urlMock.AssertExpectations(t)

	cfg := &config.CommitMsgConfig{
		CqExtraTrybots: []string{
			"luci.fake.try:some-fake-builder",
		},
	}
	bbClient := dv.bbClient.(*buildbucket_mocks.BuildBucketInterface)
	bbClient.On("GetBuilder", testutils.AnyContext, &buildbucketpb.GetBuilderRequest{
		Id: &buildbucketpb.BuilderID{
			Project: "fake",
			Bucket:  "try",
			Builder: "some-fake-builder",
		},
	}).Return(&buildbucketpb.BuilderItem{}, nil).Once()

	require.NoError(t, dv.commitMsg(t.Context(), cfg))
	bbClient.AssertExpectations(t)
}

func makeTransitiveDepConfig() *config.TransitiveDepConfig {
	parentCfg := makeVersionFileConfig()
	parentCfg.Id = "parent-dep"
	childCfg := makeVersionFileConfig()
	childCfg.Id = "child-dep"
	return &config.TransitiveDepConfig{
		Parent: parentCfg,
		Child:  childCfg,
	}
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

func makeSemVerGCSChildConfig() *config.SemVerGCSChildConfig {
	return &config.SemVerGCSChildConfig{
		Gcs: &config.GCSChildConfig{
			GcsBucket: "my-bucket",
			GcsPath:   "my-path",
		},
		VersionRegex: `v(\d+)\.(\d+)\.(\d+)\.object`,
	}
}

func makeCopyParentConfig() *config.CopyParentConfig {
	cfg := &config.CopyParentConfig{
		Gitiles: makeGitilesParentConfig(),
		Copies: []*config.CopyParentConfig_CopyEntry{
			makeCopyParentConfig_CopyEntry(),
		},
	}
	return cfg
}

func makeCopyParentConfig_CopyEntry() *config.CopyParentConfig_CopyEntry {
	return &config.CopyParentConfig_CopyEntry{
		SrcRelPath: "src/file.txt",
		DstRelPath: "dst/file.txt",
	}
}

func makeDEPSLocalGitHubParentConfig() *config.DEPSLocalGitHubParentConfig {
	cfg := makeDEPSLocalParentConfig()
	cfg.GitCheckout.GitCheckout.RepoUrl = fakeGitHubRepo
	return &config.DEPSLocalGitHubParentConfig{
		DepsLocal:   cfg,
		Github:      makeGitHubConfig(),
		ForkRepoUrl: fakeGitHubForkRepo,
	}
}

func makeDockerChildConfig() *config.DockerChildConfig {
	return &config.DockerChildConfig{
		Registry:   "gcr.io",
		Repository: "skia-public/autoroll-be",
		Tag:        "latest",
	}
}

func makeGitCheckoutGerritParentConfig() *config.GitCheckoutGerritParentConfig {
	return &config.GitCheckoutGerritParentConfig{
		GitCheckout: makeGitCheckoutParentConfig(),
	}
}

func makeGitCheckoutGitHubParentConfig() *config.GitCheckoutGitHubParentConfig {
	cfg := makeGitCheckoutParentConfig()
	cfg.GitCheckout.RepoUrl = fakeGitHubRepo
	return &config.GitCheckoutGitHubParentConfig{
		GitCheckout: cfg,
		ForkRepoUrl: fakeGitHubForkRepo,
	}
}

func makeGitCheckoutGitHubFileParentConfig() *config.GitCheckoutGitHubFileParentConfig {
	return &config.GitCheckoutGitHubFileParentConfig{
		GitCheckout: makeGitCheckoutGitHubParentConfig(),
	}
}

func makeDEPSLocalGerritParentConfig() *config.DEPSLocalGerritParentConfig {
	return &config.DEPSLocalGerritParentConfig{
		DepsLocal: makeDEPSLocalParentConfig(),
		Gerrit:    makeGerritConfig(),
	}
}

func makeGitCheckoutParentConfig() *config.GitCheckoutParentConfig {
	return &config.GitCheckoutParentConfig{
		GitCheckout: makeGitCheckoutConfig(),
		Dep:         makeDependencyConfig(),
	}
}

func makeGitCheckoutConfig() *config.GitCheckoutConfig {
	return &config.GitCheckoutConfig{
		Branch:  git.MainBranch,
		RepoUrl: fakeGitRepo,
	}
}

func makeDEPSLocalParentConfig() *config.DEPSLocalParentConfig {
	return &config.DEPSLocalParentConfig{
		GitCheckout:       makeGitCheckoutParentConfig(),
		PreUploadCommands: makePreUploadConfig(),
	}
}

func makeGitCheckoutChildConfig() *config.GitCheckoutChildConfig {
	return &config.GitCheckoutChildConfig{
		GitCheckout: makeGitCheckoutConfig(),
	}
}

func makeCIPDChildConfig() *config.CIPDChildConfig {
	return &config.CIPDChildConfig{
		Name: "my-package",
		Tag:  "latest",
	}
}

func makeFuchsiaSDKChildConfig() *config.FuchsiaSDKChildConfig {
	return &config.FuchsiaSDKChildConfig{
		GcsBucket:            "fuchsia",
		LatestLinuxPath:      "development/LATEST_LINUX",
		TarballLinuxPathTmpl: "development/%s/sdk/linux-amd64/gn.tar.gz",
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

func makePreUploadConfig() *config.PreUploadConfig {
	return &config.PreUploadConfig{
		CipdPackage: []*config.PreUploadCIPDPackageConfig{
			{
				Name:    "pre-upload-package",
				Version: "my-version",
			},
		},
		Command: []*config.PreUploadCommandConfig{
			{
				Command: "./do_stuff.sh",
			},
		},
	}
}

func makeGoModParentConfig() *config.GoModParentConfig {
	return &config.GoModParentConfig{
		GitCheckout: makeGitCheckoutConfig(),
	}
}

func makeGoModGerritParentConfig() *config.GoModGerritParentConfig {
	return &config.GoModGerritParentConfig{
		Gerrit: makeGerritConfig(),
		GoMod:  makeGoModParentConfig(),
	}
}

func makeGitCheckoutGitHubChildConfig() *config.GitCheckoutGitHubChildConfig {
	cfg := makeGitCheckoutChildConfig()
	cfg.GitCheckout.RepoUrl = fakeGitHubRepo
	return &config.GitCheckoutGitHubChildConfig{
		GitCheckout: cfg,
		RepoOwner:   fakeGitHubRepoOwner,
		RepoName:    fakeGitHubRepoName,
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

func mocksForGitCheckoutGitHubChildConfig(t *testing.T, urlMock *mockhttpclient.URLMock, cfg *config.GitCheckoutGitHubChildConfig) string {
	return mockGitHubAPICalls(t, urlMock, cfg.GitCheckout.GitCheckout.RepoUrl)
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

func mocksForGoModGerritParentConfig(t *testing.T, urlMock *mockhttpclient.URLMock, cfg *config.GoModGerritParentConfig) {
	commit := makeFakeCommit()
	gitiles_testutils.MockGetCommit(t, urlMock, cfg.GoMod.GitCheckout.RepoUrl, git.MainBranch, commit)
	mocksForGerritConfig(t, urlMock, cfg.Gerrit)
}

func mocksForGoModParentConfig(t *testing.T, urlMock *mockhttpclient.URLMock, cfg *config.GoModParentConfig) {
	commit := makeFakeCommit()
	gitiles_testutils.MockGetCommit(t, urlMock, cfg.GitCheckout.RepoUrl, git.MainBranch, commit)
}

func mocksForFuchsiaSDKChildConfig(t *testing.T, urlMock *mockhttpclient.URLMock, cfg *config.FuchsiaSDKChildConfig) {
	urlMock.MockOnce("https://storage.googleapis.com/fuchsia/development%2FLATEST_LINUX", mockhttpclient.MockGetDialogue([]byte("12345")))
}

func mocksForCIPDChildConfig(t *testing.T, dv *deepvalidator, cfg *config.CIPDChildConfig, tags []string) {
	pin := cipd_common.Pin{
		PackageName: cfg.Name,
		InstanceID:  fakeCIPDDigest,
	}
	tagInfos := make([]luci_cipd.TagInfo, len(tags))
	for idx, tag := range tags {
		tagInfos[idx].Tag = tag
	}
	client := dv.cipdClient.(*cipd_mocks.CIPDClient)
	client.On("ResolveVersion", testutils.AnyContext, cfg.Name, cfg.Tag).Return(pin, nil)
	client.On("Describe", testutils.AnyContext, cfg.Name, pin.InstanceID, false).Return(&luci_cipd.InstanceDescription{
		InstanceInfo: luci_cipd.InstanceInfo{
			Pin: pin,
		},
		Tags: tagInfos,
	}, nil)
}

func mocksForGitCheckoutChildConfig(t *testing.T, urlMock *mockhttpclient.URLMock, cfg *config.GitCheckoutChildConfig) string {
	commit := makeFakeCommit()
	gitiles_testutils.MockGetCommit(t, urlMock, cfg.GitCheckout.RepoUrl, git.MainBranch, commit)
	return commit.Commit
}

func mocksForSemVerGCSChildConfig(t *testing.T, urlMock *mockhttpclient.URLMock, cfg *config.SemVerGCSChildConfig) {
	prefix := cfg.Gcs.GcsPath
	if cfg.VersionRegex != "" {
		regex, err := regexp.Compile(strings.TrimPrefix(cfg.VersionRegex, "^"))
		require.NoError(t, err)
		regexPrefix, _ := regex.LiteralPrefix()
		prefix = path.Join(prefix, regexPrefix)
	}
	prefix = url.PathEscape(prefix)
	body := `{"items": [{"name": "v1.2.3.object"}]}`
	url := fmt.Sprintf("https://storage.googleapis.com/storage/v1/b/%s/o?alt=json&delimiter=&endOffset=&includeFoldersAsPrefixes=false&includeTrailingDelimiter=false&matchGlob=&pageToken=&prefix=%s&prettyPrint=false&projection=full&startOffset=&versions=false", cfg.Gcs.GcsBucket, prefix)
	urlMock.Mock(url, mockhttpclient.MockGetDialogue([]byte(body)))
}

func mocksForCopyParentConfig(t *testing.T, urlMock *mockhttpclient.URLMock, cfg *config.CopyParentConfig) {
	repoURL := cfg.Gitiles.Gitiles.RepoUrl

	// Mock gitiles for parent.
	sharedMocksForGitilesParent(t, urlMock, cfg.Gitiles.Gitiles.RepoUrl, cfg.Gitiles.Gitiles.Branch)

	// Mock getFileParent for the copy validation.
	gitiles_testutils.MockReadFile(t, urlMock, repoURL, cfg.Copies[0].DstRelPath, cfg.Gitiles.Gitiles.Branch, []byte("parent content"), vfs.FileInfo{
		Name: "file.txt",
	}.Get())
}

func mocksForDepsLocalGitHubParentConfig(t *testing.T, dv *deepvalidator, urlMock *mockhttpclient.URLMock, cfg *config.DEPSLocalGitHubParentConfig) {
	sha := sharedMocksForGitCheckoutGitHubParentConfig(t, urlMock, cfg.DepsLocal.GitCheckout.GitCheckout.RepoUrl, cfg.ForkRepoUrl, cfg.DepsLocal.GitCheckout.Dep)

	// Mock GitHub API.
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls?state=open", cfg.Github.RepoOwner, cfg.Github.RepoName)
	urlMock.MockOnce(url, mockhttpclient.MockGetDialogue([]byte(`[{"number": 1}]`)))

	// Mock CIPD for pre-upload config.
	mocksForPreUploadConfig(t, dv, cfg.DepsLocal.PreUploadCommands)

	repoOwner, repoName, err := skgithub.ParseRepoOwnerAndName(cfg.DepsLocal.GitCheckout.GitCheckout.RepoUrl)
	require.NoError(t, err)
	preUploadContentsURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/do_stuff.sh", repoOwner, repoName, sha)
	urlMock.MockOnce(preUploadContentsURL, mockhttpclient.MockGetDialogue([]byte("")))
}

func mocksForDepsLocalParentConfig(t *testing.T, dv *deepvalidator, urlMock *mockhttpclient.URLMock, cfg *config.DEPSLocalParentConfig) {
	repoURL := cfg.GitCheckout.GitCheckout.RepoUrl

	// Mock gitiles for parent.
	commit := makeFakeCommit()
	gitiles_testutils.MockGetCommit(t, urlMock, repoURL, git.MainBranch, commit)
	gitiles_testutils.MockReadFile(t, urlMock, repoURL, "do_stuff.sh", commit.Commit, []byte("#!/bin/bash\necho hello"), vfs.FileInfo{
		Name: "do_stuff.sh",
	}.Get())

	mocksForDependencyConfig(t, urlMock, cfg.GitCheckout.Dep, repoURL, commit.Commit)

	// Mock CIPD for pre-upload config.
	mocksForPreUploadConfig(t, dv, cfg.PreUploadCommands)
}

func mocksForGitCheckoutParentConfig(t *testing.T, urlMock *mockhttpclient.URLMock, cfg *config.GitCheckoutParentConfig) {
	repoURL := cfg.GitCheckout.RepoUrl

	// Mock gitiles for parent.
	commit := makeFakeCommit()
	gitiles_testutils.MockGetCommit(t, urlMock, repoURL, git.MainBranch, commit)
	mocksForDependencyConfig(t, urlMock, cfg.Dep, repoURL, commit.Commit)
}

func mocksForDepsLocalGerritParentConfig(t *testing.T, dv *deepvalidator, urlMock *mockhttpclient.URLMock, cfg *config.DEPSLocalGerritParentConfig) {
	mocksForGerritConfig(t, urlMock, cfg.Gerrit)
	mocksForDepsLocalParentConfig(t, dv, urlMock, cfg.DepsLocal)
}

func mocksForGitCheckoutGitHubFileParentConfig(t *testing.T, urlMock *mockhttpclient.URLMock, cfg *config.GitCheckoutGitHubFileParentConfig) {
	mocksForGitCheckoutGitHubParentConfig(t, urlMock, cfg.GitCheckout)
}

func sharedMocksForGitCheckoutGitHubParentConfig(t *testing.T, urlMock *mockhttpclient.URLMock, repoURL, forkRepoURL string, dep *config.DependencyConfig) string {
	// Mocks for the fork.
	mockGitHubAPICalls(t, urlMock, forkRepoURL)

	// Mocks for the parent.
	sha := mockGitHubAPICalls(t, urlMock, repoURL)
	mocksForDependencyConfig(t, urlMock, dep, repoURL, sha)
	return sha
}

func mocksForGitCheckoutGitHubParentConfig(t *testing.T, urlMock *mockhttpclient.URLMock, cfg *config.GitCheckoutGitHubParentConfig) string {
	return sharedMocksForGitCheckoutGitHubParentConfig(t, urlMock, cfg.GitCheckout.GitCheckout.RepoUrl, cfg.ForkRepoUrl, cfg.GitCheckout.Dep)
}

func mocksForGitCheckoutGerritParentConfig(t *testing.T, urlMock *mockhttpclient.URLMock, cfg *config.GitCheckoutGerritParentConfig) {
	mocksForGitCheckoutParentConfig(t, urlMock, cfg.GitCheckout)
}

func mocksForDockerChildConfig(t *testing.T, dockerClient *docker_mocks.Client, cfg *config.DockerChildConfig) {
	now := time.Now()
	dockerClient.On("GetManifest", testutils.AnyContext, cfg.Registry, cfg.Repository, cfg.Tag).Return(&docker.Manifest{
		Digest: fakeDockerDigest,
		Config: docker.MediaConfig{
			Digest: fakeDockerDigest,
		},
	}, nil)
	dockerClient.On("GetConfig", testutils.AnyContext, cfg.Registry, cfg.Repository, fakeDockerDigest).Return(&docker.ImageConfig{
		Author:  "test-author",
		Created: now,
	}, nil)
}

func mocksForPreUploadConfig(t *testing.T, dv *deepvalidator, cfg *config.PreUploadConfig) {
	client := dv.cipdClient.(*cipd_mocks.CIPDClient)
	for _, pkg := range cfg.CipdPackage {
		client.On("ResolveVersion", testutils.AnyContext, pkg.Name, pkg.Version).Return(cipd_common.Pin{
			PackageName: pkg.Name,
			InstanceID:  fakeCIPDDigest,
		}, nil)
	}
}

func mockDEPSContent(t *testing.T, urlMock *mockhttpclient.URLMock, repoURL, commit string) {
	gitiles_testutils.MockReadFile(t, urlMock, repoURL, deps_parser.DepsFileName, commit, []byte(fakeDepsContent), vfs.FileInfo{
		Name:  deps_parser.DepsFileName,
		Size:  int64(len(fakeDepsContent)),
		Mode:  0644,
		IsDir: false,
	}.Get())
}

func makeGetFileFunc(returnedContents string, returnedErr error) func(context.Context, string) (string, error) {
	return func(_ context.Context, _ string) (string, error) {
		return returnedContents, returnedErr
	}
}
