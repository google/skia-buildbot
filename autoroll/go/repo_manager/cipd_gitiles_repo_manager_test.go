package repo_manager

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.chromium.org/luci/cipd/client/cipd"
	"go.chromium.org/luci/cipd/common"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/repo_manager/child"
	"go.skia.org/infra/autoroll/go/repo_manager/parent"
	cipd_mocks "go.skia.org/infra/go/cipd/mocks"
	"go.skia.org/infra/go/gerrit"
	gerrit_mocks "go.skia.org/infra/go/gerrit/mocks"
	"go.skia.org/infra/go/git"
	gitiles_mocks "go.skia.org/infra/go/gitiles/mocks"
	"go.skia.org/infra/go/testutils"
)

func TestCIPDGitilesRepoManager_CreateRoll_MODULEbazel(t *testing.T) {
	// Setup.
	parentCfg := &config.GitilesParentConfig{
		Gitiles: &config.GitilesConfig{
			Branch:  git.MainBranch,
			RepoUrl: "fake.googlesource.com/fake.git",
		},
		Dep: &config.DependencyConfig{
			Primary: &config.VersionFileConfig{
				Id: "some/package/${platform}",
				File: []*config.VersionFileConfig_File{
					{
						Path: "MODULE.bazel",
					},
				},
			},
		},
		Gerrit: &config.GerritConfig{
			Url:     "fake-review.googlesource.com",
			Project: "fake",
			Config:  config.GerritConfig_CHROMIUM_BOT_COMMIT,
		},
	}
	childCfg := &config.CIPDChildConfig{
		Name:          "some/package/${platform}",
		Tag:           "latest",
		RevisionIdTag: "version",
		Platform: []string{
			"linux-amd64",
			"linux-arm64",
			"mac-amd64",
			"mac-arm64",
			"windows-amd64",
		},
	}
	ctx := t.Context()
	mockCIPD := &cipd_mocks.CIPDClient{}
	c, err := child.NewCIPD(ctx, childCfg, mockCIPD, nil, "")
	require.NoError(t, err)
	parentRepo := &gitiles_mocks.GitilesRepo{}
	mockGerrit := &gerrit_mocks.GerritInterface{}
	p, err := parent.NewGitilesFile(ctx, parentCfg, parentRepo, mockGerrit, "")
	require.NoError(t, err)
	rm := &parentChildRepoManager{
		Parent: p,
		Child:  c,
	}

	// Mocks for Update().
	const baseParentCommit = "abc123"

	const oldVersionTag = "version:3@3.11.9.chromium.35"
	const oldModuleBazelContents = `
cipd.download_http(
    name = "some_package",
    cipd_package = "some/package/${platform}",
    platform_to_sha256 = {
        "linux-amd64":   "11393ea425858fd31b3d6c3695367f40c53fd1d6fb087d22e20cbc362fb9c466",
        "linux-arm64":   "5195d86e545e62ea441112c6cde82d85f01366a7aeea38ee9be53f6a88ca7a11",
        "mac-amd64":     "124240f1c0a040610a450ad659cc68d5b7454688a4a96a065814ef04f5bd985f",
        "mac-arm64":     "a41b28cf7ffb369941d0b209d3bce700803763b4dc3e5370b89794830362cb15",
        "windows-amd64": "a6737d5c10ff0286ffc067c18614cb01a258b199acdfaadb13436b16cb1b3225",
    },
    tag = "` + oldVersionTag + `",
)
use_repo(cipd, "some_package_linux-amd64")
`
	oldInstanceIDs := map[string]string{
		"some/package/linux-amd64":   "ETk-pCWFj9MbPWw2lTZ_QMU_0db7CH0i4gy8Ni-5xGYC",
		"some/package/linux-arm64":   "UZXYblReYupEERLGzegthfATZqeu6jjum-U_aojKehEC",
		"some/package/mac-amd64":     "EkJA8cCgQGEKRQrWWcxo1bdFRoikqWoGWBTvBPW9mF8C",
		"some/package/mac-arm64":     "pBsoz3_7NplB0LIJ07znAIA3Y7TcPlNwuJeUgwNiyxUC",
		"some/package/windows-amd64": "pnN9XBD_Aob_wGfBhhTLAaJYsZms36rbE0NrFssbMiUC",
	}

	const newVersionTag = "version:3@3.11.9.chromium.36"
	const newModuleBazelContents = `
cipd.download_http(
    name = "some_package",
    cipd_package = "some/package/${platform}",
    platform_to_sha256 = {
        "linux-amd64":   "a06bad297267859aad818d2d6c9cc2865f4b0087c71dbf020f6b8e0cb92746c2",
        "linux-arm64":   "2885f51f1997b20460577c6e0b34c63910e0f3474ce17e266c3c17eab9b73337",
        "mac-amd64":     "787e1e98b6220b4bf887ae04a8a80fb4939ad3c24ca6e2e7753541048c97224a",
        "mac-arm64":     "6c7fb45e6ac6363c00b9820b259132554190a197d65437a0912025fa9c27e161",
        "windows-amd64": "cdc51dd7d25bc6a249b076e117cdfe29456a93cddd48d4193ca3742a20ea7699",
    },
    tag = "` + newVersionTag + `",
)
use_repo(cipd, "some_package_linux-amd64")
`
	newInstanceIDs := map[string]string{
		"some/package/linux-amd64":   "oGutKXJnhZqtgY0tbJzChl9LAIfHHb8CD2uODLknRsIC",
		"some/package/linux-arm64":   "KIX1HxmXsgRgV3xuCzTGORDg80dM4X4mbDwX6rm3MzcC",
		"some/package/mac-amd64":     "eH4emLYiC0v4h64EqKgPtJOa08JMpuLndTVBBIyXIkoC",
		"some/package/mac-arm64":     "bH-0XmrGNjwAuYILJZEyVUGQoZfWVDegkSAl-pwn4WEC",
		"some/package/windows-amd64": "zcUd19JbxqJJsHbhF83-KUVqk83dSNQZPKN0KiDqdpkC",
	}

	parentRepo.On("ResolveRef", testutils.AnyContext, "main").Return(baseParentCommit, nil)
	parentRepo.On("ReadFileAtRef", testutils.AnyContext, "MODULE.bazel", baseParentCommit).Return([]byte(oldModuleBazelContents), nil)
	for pkgName, instanceID := range oldInstanceIDs {
		mockCIPD.On("Describe", testutils.AnyContext, pkgName, oldVersionTag, false).Return(&cipd.InstanceDescription{
			InstanceInfo: cipd.InstanceInfo{
				Pin: common.Pin{
					PackageName: pkgName,
					InstanceID:  instanceID,
				},
			},
			Tags: []cipd.TagInfo{
				{Tag: oldVersionTag},
			},
		}, nil)
	}

	for pkgName, instanceID := range newInstanceIDs {
		mockCIPD.On("ResolveVersion", testutils.AnyContext, pkgName, childCfg.Tag).Return(common.Pin{
			PackageName: pkgName,
			InstanceID:  instanceID,
		}, nil)
		mockCIPD.On("Describe", testutils.AnyContext, pkgName, instanceID, false).Return(&cipd.InstanceDescription{
			InstanceInfo: cipd.InstanceInfo{
				Pin: common.Pin{
					PackageName: pkgName,
					InstanceID:  instanceID,
				},
			},
			Tags: []cipd.TagInfo{
				{Tag: newVersionTag},
			},
		}, nil)
	}

	// Run Update() to obtain the revision to roll.
	lastRollRev, nextRollRev, notRolledRevs, err := rm.Update(ctx)
	require.NoError(t, err)

	// Mocks for CreateNewRoll().
	commitMsg := "Here's a roll!"
	ci := &gerrit.ChangeInfo{
		Issue: 54321,
		Revisions: map[string]*gerrit.Revision{
			"1": {
				Number: 1,
			},
		},
	}
	mockGerrit.On("CreateChange", testutils.AnyContext, parentCfg.Gerrit.Project, parentCfg.Gitiles.Branch, commitMsg, baseParentCommit, "").Return(ci, nil)
	mockGerrit.On("PublishChangeEdit", testutils.AnyContext, ci).Return(nil)
	mockGerrit.On("EditFile", testutils.AnyContext, ci, "MODULE.bazel", newModuleBazelContents).Return(nil)
	ci.Revisions["2"] = &gerrit.Revision{Number: 2}
	mockGerrit.On("GetIssueProperties", testutils.AnyContext, ci.Issue).Return(ci, nil)
	mockGerrit.On("Config").Return(gerrit.ConfigChromiumBotCommit)
	mockGerrit.On("SetReview", testutils.AnyContext, ci, "", map[string]int{
		"Bot-Commit":   1,
		"Commit-Queue": 2,
	}, []string{"me@google.com"}, gerrit.NotifyOption(""), gerrit.NotifyDetails(nil), "", 0, []*gerrit.AttentionSetInput(nil)).Return(nil)

	_, err = rm.CreateNewRoll(ctx, lastRollRev, nextRollRev, notRolledRevs, []string{"me@google.com"}, false, false, commitMsg)
	require.NoError(t, err)
}
