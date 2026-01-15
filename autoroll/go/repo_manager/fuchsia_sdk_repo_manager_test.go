package repo_manager

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	gerrit_mocks "go.skia.org/infra/go/gerrit/mocks"
	"go.skia.org/infra/go/git"
	gitiles_mocks "go.skia.org/infra/go/gitiles/mocks"
	"go.skia.org/infra/go/mockhttpclient"
)

const (
	fuchsiaSDKRevPrev = "000633ae6e904f7eaced443d6aa65fb3d24afe8c"
	fuchsiaSDKRevBase = "32a56ad54471732034ba802cbfc3c9ff277b9d1c"
	fuchsiaSDKRevNext = "37417b795289818723990da66dd7a7b38e50fc04"

	fuchsiaSDKTimePrev = "2009-11-10T23:00:01Z"
	fuchsiaSDKTimeBase = "2009-11-10T23:00:02Z"
	fuchsiaSDKTimeNext = "2009-11-10T23:00:03Z"

	fuchsiaSDKArchiveUrlTmpl = "https://storage.googleapis.com/fuchsia/development/%s"

	fuchsiaSDKVersionFilePathLinux = "build/fuchsia/linux.sdk.sha1"
	fuchsiaSDKVersionFilePathMac   = "build/fuchsia/mac.sdk.sha1"
)

var (
	fuchsiaSDKLatestArchiveUrlLinux = fmt.Sprintf(fuchsiaSDKArchiveUrlTmpl, "LATEST_LINUX")
	fuchsiaSDKLatestArchiveUrlMac   = fmt.Sprintf(fuchsiaSDKArchiveUrlTmpl, "LATEST_MAC")
)

func fuchsiaCfg() *config.ParentChildRepoManagerConfig {
	child := &config.ParentChildRepoManagerConfig_FuchsiaSdkChild{
		FuchsiaSdkChild: &config.FuchsiaSDKChildConfig{
			GcsBucket:            "fake-fuchsia-sdk",
			LatestLinuxPath:      "linux/LATEST",
			LatestMacPath:        "mac/LATEST",
			TarballLinuxPathTmpl: "%s.sdk",
		},
	}
	return &config.ParentChildRepoManagerConfig{
		Parent: &config.ParentChildRepoManagerConfig_GitilesParent{
			GitilesParent: &config.GitilesParentConfig{
				Gitiles: &config.GitilesConfig{
					Branch:  git.MainBranch,
					RepoUrl: "todo.git",
				},
				Dep: &config.DependencyConfig{
					Primary: &config.VersionFileConfig{
						Id: "FuchsiaSDK",
						File: []*config.VersionFileConfig_File{
							{Path: fuchsiaSDKVersionFilePathLinux},
						},
					},
					Transitive: []*config.TransitiveDepConfig{
						{
							Child: &config.VersionFileConfig{
								Id: child.FuchsiaSdkChild.LatestMacPath,
								File: []*config.VersionFileConfig_File{
									{Path: fuchsiaSDKVersionFilePathMac},
								},
							},
							Parent: &config.VersionFileConfig{
								Id: child.FuchsiaSdkChild.LatestLinuxPath,
								File: []*config.VersionFileConfig_File{
									{Path: fuchsiaSDKVersionFilePathMac},
								},
							},
						},
					},
				},
				Gerrit: &config.GerritConfig{
					Url:     "https://fake-skia-review.googlesource.com",
					Project: "fake-gerrit-project",
					Config:  config.GerritConfig_CHROMIUM,
				},
			},
		},
		Child: child,
	}
}

func setupFuchsiaSDK(t *testing.T) (*parentChildRepoManager, *gitiles_mocks.GitilesRepo, *gerrit_mocks.GerritInterface, *mockhttpclient.URLMock) {
	cfg := fuchsiaCfg()
	parentCfg := cfg.GetGitilesParent()
	p, parentGitiles, parentGerrit := parent.NewGitilesFileForTesting(t, parentCfg)

	urlmock := mockhttpclient.NewURLMock()
	c, err := child.NewFuchsiaSDK(t.Context(), cfg.GetFuchsiaSdkChild(), urlmock.Client())
	require.NoError(t, err)

	// Create the RepoManager.
	rm := &parentChildRepoManager{
		Parent: p,
		Child:  c,
	}

	// Mock requests for Update().
	fileContents := map[string]string{
		fuchsiaSDKVersionFilePathLinux: fuchsiaSDKRevBase + "\n",
		fuchsiaSDKVersionFilePathMac:   "mac-base\n",
	}
	parent.MockGitilesFileForUpdate(parentGitiles, cfg.GetGitilesParent(), noCheckoutParentHead, fileContents)
	mockGetLatestSDK(urlmock, fuchsiaSDKRevBase, "mac-base")

	// Update.
	_, _, _ = updateAndAssert(t, rm, parentGitiles, parentGerrit, urlmock)
	return rm, parentGitiles, parentGerrit, urlmock
}

func mockGetLatestSDK(urlmock *mockhttpclient.URLMock, revLinux, revMac string) {
	c := fuchsiaCfg().GetFuchsiaSdkChild()
	urlmock.MockOnce("https://storage.googleapis.com/"+c.GcsBucket+"/"+url.QueryEscape(c.LatestLinuxPath), mockhttpclient.MockGetDialogue([]byte(revLinux)))
	if c.LatestMacPath != "" {
		urlmock.MockOnce("https://storage.googleapis.com/"+c.GcsBucket+"/"+url.QueryEscape(c.LatestMacPath), mockhttpclient.MockGetDialogue([]byte(revMac)))
	}
}

func TestFuchsiaSDKRepoManager(t *testing.T) {
	rm, parentGitiles, parentGerrit, urlmock := setupFuchsiaSDK(t)

	// Mock requests for Update().
	cfg := fuchsiaCfg()
	oldContent := map[string]string{
		fuchsiaSDKVersionFilePathLinux: fuchsiaSDKRevBase + "\n",
		fuchsiaSDKVersionFilePathMac:   "mac-base\n",
	}
	parent.MockGitilesFileForUpdate(parentGitiles, cfg.GetGitilesParent(), noCheckoutParentHead, oldContent)
	mockGetLatestSDK(urlmock, fuchsiaSDKRevBase, "mac-base")

	// Update.
	lastRollRev, tipRev, notRolledRevs := updateAndAssert(t, rm, parentGitiles, parentGerrit, urlmock)
	require.Equal(t, fuchsiaSDKRevBase, lastRollRev.Id)
	require.Equal(t, fuchsiaSDKRevBase, tipRev.Id)
	prev, err := rm.GetRevision(t.Context(), fuchsiaSDKRevPrev)
	require.NoError(t, err)
	require.Equal(t, fuchsiaSDKRevPrev, prev.Id)
	base, err := rm.GetRevision(t.Context(), fuchsiaSDKRevBase)
	require.NoError(t, err)
	require.Equal(t, fuchsiaSDKRevBase, base.Id)
	next, err := rm.GetRevision(t.Context(), fuchsiaSDKRevNext)
	require.NoError(t, err)
	require.Equal(t, fuchsiaSDKRevNext, next.Id)
	require.Equal(t, 0, len(notRolledRevs))

	// // There's a new version.
	parent.MockGitilesFileForUpdate(parentGitiles, cfg.GetGitilesParent(), noCheckoutParentHead, oldContent)
	mockGetLatestSDK(urlmock, fuchsiaSDKRevNext, "mac-next")
	lastRollRev, tipRev, notRolledRevs = updateAndAssert(t, rm, parentGitiles, parentGerrit, urlmock)
	require.Equal(t, fuchsiaSDKRevBase, lastRollRev.Id)
	require.Equal(t, fuchsiaSDKRevNext, tipRev.Id)
	require.Equal(t, 1, len(notRolledRevs))
	require.Equal(t, fuchsiaSDKRevNext, notRolledRevs[0].Id)

	// Upload a CL.
	newContent := map[string]string{
		fuchsiaSDKVersionFilePathLinux: fuchsiaSDKRevNext + "\n",
		fuchsiaSDKVersionFilePathMac:   "mac-next\n",
	}
	parent.MockGitilesFileForCreateNewRoll(parentGitiles, parentGerrit, cfg.GetGitilesParent(), noCheckoutParentHead, fakeCommitMsgMock, oldContent, newContent, fakeReviewers)
	_, err = rm.CreateNewRoll(t.Context(), lastRollRev, tipRev, notRolledRevs, fakeReviewers, false, false, fakeCommitMsg)
	require.NoError(t, err)
}
