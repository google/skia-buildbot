package child

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	cipd_api "go.chromium.org/luci/cipd/client/cipd"
	"go.chromium.org/luci/cipd/common"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gitiles_common"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/cipd"
	"go.skia.org/infra/go/cipd/mocks"
	"go.skia.org/infra/go/git"
	gitiles_mocks "go.skia.org/infra/go/gitiles/mocks"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vcsinfo"
)

const (
	cipdInstanceChecksum = "f0409b2fc2b61d5bb51862d132d9f3757af9206fa4cb442703e814e3805588f6"
)

var (
	// Arbitrary timestamp.
	ts = time.Unix(1615384545, 0)
)

func TestCIPDInstanceToRevision(t *testing.T) {
	pkg := &cipd_api.InstanceDescription{
		InstanceInfo: cipd_api.InstanceInfo{
			Pin: common.Pin{
				PackageName: "some/package",
				InstanceID:  "8ECbL8K2HVu1GGLRMtnzdXr5IG-ky0QnA-gU44BViPYC",
			},
			RegisteredBy: "me@google.com",
			RegisteredTs: cipd_api.UnixTime(ts),
		},
		Tags: []cipd_api.TagInfo{
			{
				Tag: "version:5",
			},
			{
				Tag: "otherTag:blahblah",
			},
			{
				Tag: "bug:skia:12345",
			},
		},
	}
	expect := &revision.Revision{
		Id:       "8ECbL8K2HVu1GGLRMtnzdXr5IG-ky0QnA-gU44BViPYC",
		Checksum: cipdInstanceChecksum,
		Author:   "me@google.com",
		Bugs: map[string][]string{
			"skia": {"12345"},
		},
		Description: "some/package:8ECbL8K2HVu1GGLRMtnzdXr5IG-ky0QnA-gU44BViPYC",
		Display:     "8ECbL8K2HVu1GGLRM...",
		Timestamp:   ts,
		URL:         "https://chrome-infra-packages.appspot.com/p/some/package/+/8ECbL8K2HVu1GGLRMtnzdXr5IG-ky0QnA-gU44BViPYC",
	}
	rev, err := CIPDInstanceToRevision("some/package", pkg, "", false)
	require.NoError(t, err)
	require.Equal(t, expect, rev)
}

func TestCIPDInstanceToRevision_MissingRevisionIdTag(t *testing.T) {
	pkg := &cipd_api.InstanceDescription{
		InstanceInfo: cipd_api.InstanceInfo{
			Pin: common.Pin{
				PackageName: "some/package",
				InstanceID:  "8ECbL8K2HVu1GGLRMtnzdXr5IG-ky0QnA-gU44BViPYC",
			},
			RegisteredBy: "me@google.com",
			RegisteredTs: cipd_api.UnixTime(ts),
		},
		Tags: []cipd_api.TagInfo{
			{
				Tag: "version:5",
			},
			{
				Tag: "otherTag:blahblah",
			},
			{
				Tag: "bug:skia:12345",
			},
		},
	}
	expect := &revision.Revision{
		Id:       "8ECbL8K2HVu1GGLRMtnzdXr5IG-ky0QnA-gU44BViPYC",
		Checksum: cipdInstanceChecksum,
		Author:   "me@google.com",
		Bugs: map[string][]string{
			"skia": {"12345"},
		},
		Description:   "some/package:8ECbL8K2HVu1GGLRMtnzdXr5IG-ky0QnA-gU44BViPYC",
		Display:       "8ECbL8K2HVu1GGLRM...",
		Timestamp:     ts,
		URL:           "https://chrome-infra-packages.appspot.com/p/some/package/+/8ECbL8K2HVu1GGLRMtnzdXr5IG-ky0QnA-gU44BViPYC",
		InvalidReason: "Package instance has no tag \"missing\"",
	}
	rev, err := CIPDInstanceToRevision("some/package", pkg, "missing", false)
	require.NoError(t, err)
	require.Equal(t, expect, rev)
}

func TestCIPDInstanceToRevision_RevisionIdTag(t *testing.T) {
	pkg := &cipd_api.InstanceDescription{
		InstanceInfo: cipd_api.InstanceInfo{
			Pin: common.Pin{
				PackageName: "some/package",
				InstanceID:  "8ECbL8K2HVu1GGLRMtnzdXr5IG-ky0QnA-gU44BViPYC",
			},
			RegisteredBy: "me@google.com",
			RegisteredTs: cipd_api.UnixTime(ts),
		},
		Tags: []cipd_api.TagInfo{
			{
				Tag: "version:5",
			},
			{
				Tag: "otherTag:blahblah",
			},
			{
				Tag: "bug:skia:12345",
			},
		},
	}
	expect := &revision.Revision{
		Id:       "version:5",
		Checksum: cipdInstanceChecksum,
		Author:   "me@google.com",
		Bugs: map[string][]string{
			"skia": {"12345"},
		},
		Description: "some/package:8ECbL8K2HVu1GGLRMtnzdXr5IG-ky0QnA-gU44BViPYC",
		Display:     "5",
		Timestamp:   ts,
		URL:         "https://chrome-infra-packages.appspot.com/p/some/package/+/8ECbL8K2HVu1GGLRMtnzdXr5IG-ky0QnA-gU44BViPYC",
	}
	rev, err := CIPDInstanceToRevision("some/package", pkg, "version", false)
	require.NoError(t, err)
	require.Equal(t, expect, rev)
}

func TestCIPDInstanceToRevision_RevisionIdTagStripKey(t *testing.T) {
	pkg := &cipd_api.InstanceDescription{
		InstanceInfo: cipd_api.InstanceInfo{
			Pin: common.Pin{
				PackageName: "some/package",
				InstanceID:  "8ECbL8K2HVu1GGLRMtnzdXr5IG-ky0QnA-gU44BViPYC",
			},
			RegisteredBy: "me@google.com",
			RegisteredTs: cipd_api.UnixTime(ts),
		},
		Tags: []cipd_api.TagInfo{
			{
				Tag: "version:5",
			},
			{
				Tag: "otherTag:blahblah",
			},
			{
				Tag: "bug:skia:12345",
			},
		},
	}
	expect := &revision.Revision{
		Id:       "5",
		Checksum: cipdInstanceChecksum,
		Author:   "me@google.com",
		Bugs: map[string][]string{
			"skia": {"12345"},
		},
		Description: "some/package:8ECbL8K2HVu1GGLRMtnzdXr5IG-ky0QnA-gU44BViPYC",
		Display:     "5",
		Timestamp:   ts,
		URL:         "https://chrome-infra-packages.appspot.com/p/some/package/+/8ECbL8K2HVu1GGLRMtnzdXr5IG-ky0QnA-gU44BViPYC",
	}
	rev, err := CIPDInstanceToRevision("some/package", pkg, "version", true)
	require.NoError(t, err)
	require.Equal(t, expect, rev)
}

func TestCIPDChild_GetRevision(t *testing.T) {
	mockCipdClient := &mocks.CIPDClient{}
	c := &CIPDChild{
		client: mockCipdClient,
		name:   "some/package",
		ref:    "latest",
	}
	ctx := context.Background()
	instanceID := "8ECbL8K2HVu1GGLRMtnzdXr5IG-ky0QnA-gU44BViPYC"
	mockCipdClient.On("Describe", testutils.AnyContext, c.name, instanceID, false).Return(&cipd_api.InstanceDescription{
		InstanceInfo: cipd_api.InstanceInfo{
			Pin: common.Pin{
				PackageName: c.name,
				InstanceID:  instanceID,
			},
			RegisteredBy: "me@google.com",
			RegisteredTs: cipd_api.UnixTime(ts),
		},
		Tags: []cipd_api.TagInfo{
			{
				Tag: "version:5",
			},
			{
				Tag: "otherTag:blahblah",
			},
			{
				Tag: "bug:skia:12345",
			},
		},
	}, nil)
	rev, err := c.GetRevision(ctx, instanceID)
	require.NoError(t, err)
	require.Equal(t, &revision.Revision{
		Id:       instanceID,
		Checksum: cipdInstanceChecksum,
		Author:   "me@google.com",
		Bugs: map[string][]string{
			"skia": {"12345"},
		},
		Description: "some/package:8ECbL8K2HVu1GGLRMtnzdXr5IG-ky0QnA-gU44BViPYC",
		Display:     "8ECbL8K2HVu1GGLRM...",
		Timestamp:   ts,
		URL:         "https://chrome-infra-packages.appspot.com/p/some/package/+/8ECbL8K2HVu1GGLRMtnzdXr5IG-ky0QnA-gU44BViPYC",
	}, rev)
}

func TestCIPDChild_GetRevision_HasBackingRepo(t *testing.T) {
	mockCipdClient := &mocks.CIPDClient{}
	mockGitiles := &gitiles_mocks.GitilesRepo{}
	ctx := context.Background()
	gitilesConfig := &config.GitilesConfig{
		Branch:  git.MainBranch,
		RepoUrl: "fake.git",
	}
	gitilesRepo, err := gitiles_common.NewGitilesRepo(ctx, gitilesConfig, nil)
	require.NoError(t, err)
	gitilesRepo.GitilesRepo = mockGitiles
	c := &CIPDChild{
		client:  mockCipdClient,
		name:    "some/package",
		ref:     "latest",
		gitRepo: gitilesRepo,
	}

	gitRevision := "abcde12345abcde12345abcde12345abcde12345"
	gitTs := time.Unix(1615384887, 0)
	cipdTs := time.Unix(1615384545, 0)
	instanceID := "8ECbL8K2HVu1GGLRMtnzdXr5IG-ky0QnA-gU44BViPYC"
	instanceTag := CIPDGitRevisionTag(gitRevision)
	gitRev := &vcsinfo.LongCommit{
		ShortCommit: &vcsinfo.ShortCommit{
			Hash:    gitRevision,
			Author:  "you@google.com",
			Subject: "fake commit",
		},
		Timestamp: gitTs,
	}
	mockGitiles.On("Details", testutils.AnyContext, gitRevision).Return(gitRev, nil)
	mockGitiles.On("URL").Return(gitilesConfig.RepoUrl)
	mockCipdClient.On("Describe", testutils.AnyContext, c.name, instanceTag, false).Return(nil, errors.New("No such instance"))
	mockCipdClient.On("SearchInstances", testutils.AnyContext, c.name, []string{instanceTag}).Return(common.PinSlice([]common.Pin{
		{
			PackageName: c.name,
			InstanceID:  instanceID,
		},
	}), nil)
	mockCipdClient.On("Describe", testutils.AnyContext, c.name, instanceID, false).Return(&cipd_api.InstanceDescription{
		InstanceInfo: cipd_api.InstanceInfo{
			Pin: common.Pin{
				PackageName: c.name,
				InstanceID:  instanceID,
			},
			RegisteredBy: "me@google.com",
			RegisteredTs: cipd_api.UnixTime(cipdTs),
		},
		Tags: []cipd_api.TagInfo{
			{
				Tag: instanceTag,
			},
			{
				Tag: "otherTag:blahblah",
			},
			{
				Tag: "bug:skia:12345",
			},
		},
	}, nil)

	rev, err := c.GetRevision(ctx, instanceID)
	require.NoError(t, err)
	require.Equal(t, &revision.Revision{
		Id:          instanceTag,
		Checksum:    cipdInstanceChecksum,
		Author:      "you@google.com",
		Bugs:        map[string][]string{},
		Description: gitRev.Subject,
		Display:     gitRev.Hash[:12],
		Timestamp:   gitRev.Timestamp,
		URL:         "fake.git/+show/" + gitRev.Hash,
	}, rev)
}

func TestCIPDChild_GetRevision_HasRevisionIDTag(t *testing.T) {
	mockCipdClient := &mocks.CIPDClient{}
	c := &CIPDChild{
		client:        mockCipdClient,
		name:          "some/package",
		ref:           "latest",
		revisionIdTag: "version",
	}

	instanceID := "8ECbL8K2HVu1GGLRMtnzdXr5IG-ky0QnA-gU44BViPYC"
	instanceTag := "version:5"

	mockCipdClient.On("Describe", testutils.AnyContext, c.name, c.revisionIdTag, false).Return(nil, errors.New("No such instance"))
	mockCipdClient.On("SearchInstances", testutils.AnyContext, c.name, []string{instanceTag}).Return(common.PinSlice([]common.Pin{
		{
			PackageName: c.name,
			InstanceID:  instanceID,
		},
	}), nil)
	mockCipdClient.On("Describe", testutils.AnyContext, c.name, instanceID, false).Return(&cipd_api.InstanceDescription{
		InstanceInfo: cipd_api.InstanceInfo{
			Pin: common.Pin{
				PackageName: c.name,
				InstanceID:  instanceID,
			},
			RegisteredBy: "me@google.com",
			RegisteredTs: cipd_api.UnixTime(ts),
		},
		Tags: []cipd_api.TagInfo{
			{
				Tag: instanceTag,
			},
			{
				Tag: "otherTag:blahblah",
			},
			{
				Tag: "bug:skia:12345",
			},
		},
	}, nil)

	rev, err := c.GetRevision(t.Context(), instanceID)
	require.NoError(t, err)
	require.Equal(t, &revision.Revision{
		Id:       "version:5",
		Checksum: cipdInstanceChecksum,
		Author:   "me@google.com",
		Bugs: map[string][]string{
			"skia": {"12345"},
		},
		Description: "some/package:8ECbL8K2HVu1GGLRMtnzdXr5IG-ky0QnA-gU44BViPYC",
		Display:     "5",
		Timestamp:   ts,
		URL:         "https://chrome-infra-packages.appspot.com/p/some/package/+/8ECbL8K2HVu1GGLRMtnzdXr5IG-ky0QnA-gU44BViPYC",
	}, rev)
}

func TestCIPDChild_GetRevision_HasRevisionIDTag_StripKey(t *testing.T) {
	mockCipdClient := &mocks.CIPDClient{}
	c := &CIPDChild{
		client:                mockCipdClient,
		name:                  "some/package",
		ref:                   "latest",
		revisionIdTag:         "version",
		revisionIdTagStripKey: true,
	}

	instanceID := "8ECbL8K2HVu1GGLRMtnzdXr5IG-ky0QnA-gU44BViPYC"
	instanceTag := "version:5"

	mockCipdClient.On("Describe", testutils.AnyContext, c.name, c.revisionIdTag, false).Return(nil, errors.New("No such instance"))
	mockCipdClient.On("SearchInstances", testutils.AnyContext, c.name, []string{instanceTag}).Return(common.PinSlice([]common.Pin{
		{
			PackageName: c.name,
			InstanceID:  instanceID,
		},
	}), nil)
	mockCipdClient.On("Describe", testutils.AnyContext, c.name, instanceID, false).Return(&cipd_api.InstanceDescription{
		InstanceInfo: cipd_api.InstanceInfo{
			Pin: common.Pin{
				PackageName: c.name,
				InstanceID:  instanceID,
			},
			RegisteredBy: "me@google.com",
			RegisteredTs: cipd_api.UnixTime(ts),
		},
		Tags: []cipd_api.TagInfo{
			{
				Tag: instanceTag,
			},
			{
				Tag: "otherTag:blahblah",
			},
			{
				Tag: "bug:skia:12345",
			},
		},
	}, nil)

	rev, err := c.GetRevision(t.Context(), instanceID)
	require.NoError(t, err)
	require.Equal(t, &revision.Revision{
		Id:       "5",
		Checksum: cipdInstanceChecksum,
		Author:   "me@google.com",
		Bugs: map[string][]string{
			"skia": {"12345"},
		},
		Description: "some/package:8ECbL8K2HVu1GGLRMtnzdXr5IG-ky0QnA-gU44BViPYC",
		Display:     "5",
		Timestamp:   ts,
		URL:         "https://chrome-infra-packages.appspot.com/p/some/package/+/8ECbL8K2HVu1GGLRMtnzdXr5IG-ky0QnA-gU44BViPYC",
	}, rev)
}

func TestCIPDChild_GetRevision_Platforms(t *testing.T) {
	mockCipdClient := &mocks.CIPDClient{}
	c := &CIPDChild{
		client:        mockCipdClient,
		name:          "some/package/${platform}",
		ref:           "latest",
		revisionIdTag: "version",
		platforms: []string{
			"linux-amd64",
			"linux-arm64",
			"mac-amd64",
			"mac-arm64",
			"windows-amd64",
		},
	}
	ctx := context.Background()
	const instanceTag = "version:5"
	fullPackageToID := map[string]string{
		"some/package/linux-amd64":   "Vxy3fSLmQ7k5NqOhQUvsgu_GSH387Gp05UB0qd5tl6MC",
		"some/package/linux-arm64":   "XhM6rCCaRV3DJ4DkjsSTEahG-w3Er_ulss6w9-GwgkwC",
		"some/package/mac-amd64":     "1az5xiBG-ds55R4yd7fCkODv0xrApC5gC6iLb2SCig8C",
		"some/package/mac-arm64":     "bfOh3y10stM2Fj7HG-dDsJpJfm-J8yELSuoY94ec9UQC",
		"some/package/windows-amd64": "MeJ2G6pEJ4Vz3CvzoEf1QhrbEZqSzSh2uujaq7KwJtYC",
	}
	var primarySha256 string
	for pkg, instanceID := range fullPackageToID {
		sha256, err := cipd.InstanceIDToSha256(instanceID)
		require.NoError(t, err)
		if strings.HasSuffix(pkg, c.platforms[0]) {
			primarySha256 = sha256
		}

		mockCipdClient.On("Describe", testutils.AnyContext, pkg, instanceTag, false).Return(nil, errors.New("No such instance"))
		mockCipdClient.On("SearchInstances", testutils.AnyContext, pkg, []string{instanceTag}).Return(common.PinSlice([]common.Pin{
			{
				PackageName: pkg,
				InstanceID:  instanceID,
			},
		}), nil)
		mockCipdClient.On("Describe", testutils.AnyContext, pkg, instanceID, false).Return(&cipd_api.InstanceDescription{
			InstanceInfo: cipd_api.InstanceInfo{
				Pin: common.Pin{
					PackageName: pkg,
					InstanceID:  instanceID,
				},
				RegisteredBy: "me@google.com",
				RegisteredTs: cipd_api.UnixTime(ts),
			},
			Tags: []cipd_api.TagInfo{
				{
					Tag: instanceTag,
				},
			},
		}, nil)
	}
	rev, err := c.GetRevision(ctx, instanceTag)
	require.NoError(t, err)
	require.Equal(t, &revision.Revision{
		Id:          instanceTag,
		Checksum:    primarySha256,
		Author:      "me@google.com",
		Description: "some/package/linux-amd64:Vxy3fSLmQ7k5NqOhQUvsgu_GSH387Gp05UB0qd5tl6MC",
		Display:     "5",
		Timestamp:   ts,
		URL:         "https://chrome-infra-packages.appspot.com/p/some/package/linux-amd64/+/Vxy3fSLmQ7k5NqOhQUvsgu_GSH387Gp05UB0qd5tl6MC",
		Meta: map[string]string{
			"linux-amd64":   "571cb77d22e643b93936a3a1414bec82efc6487dfcec6a74e54074a9de6d97a3",
			"linux-arm64":   "5e133aac209a455dc32780e48ec49311a846fb0dc4affba5b2ceb0f7e1b0824c",
			"mac-amd64":     "d5acf9c62046f9db39e51e3277b7c290e0efd31ac0a42e600ba88b6f64828a0f",
			"mac-arm64":     "6df3a1df2d74b2d336163ec71be743b09a497e6f89f3210b4aea18f7879cf544",
			"windows-amd64": "31e2761baa44278573dc2bf3a047f5421adb119a92cd2876bae8daabb2b026d6",
		},
	}, rev)
}

func TestCIPDChild_GetRevision_Platforms_OneMissing(t *testing.T) {
	mockCipdClient := &mocks.CIPDClient{}
	c := &CIPDChild{
		client:        mockCipdClient,
		name:          "some/package/${platform}",
		ref:           "latest",
		revisionIdTag: "version",
		platforms: []string{
			"linux-amd64",
			"linux-arm64",
			"mac-amd64",
			"mac-arm64",
			"windows-amd64",
		},
	}
	ctx := context.Background()
	const instanceTag = "version:5"
	fullPackageToID := map[string]string{
		"some/package/linux-amd64":   "Vxy3fSLmQ7k5NqOhQUvsgu_GSH387Gp05UB0qd5tl6MC",
		"some/package/linux-arm64":   "XhM6rCCaRV3DJ4DkjsSTEahG-w3Er_ulss6w9-GwgkwC",
		"some/package/mac-amd64":     "1az5xiBG-ds55R4yd7fCkODv0xrApC5gC6iLb2SCig8C",
		"some/package/mac-arm64":     "bfOh3y10stM2Fj7HG-dDsJpJfm-J8yELSuoY94ec9UQC",
		"some/package/windows-amd64": "MeJ2G6pEJ4Vz3CvzoEf1QhrbEZqSzSh2uujaq7KwJtYC",
	}
	dependencies := make(map[string]string, len(fullPackageToID)-1)
	var primarySha256 string
	for pkg, instanceID := range fullPackageToID {
		sha256, err := cipd.InstanceIDToSha256(instanceID)
		require.NoError(t, err)
		if strings.HasSuffix(pkg, c.platforms[0]) {
			primarySha256 = sha256
		} else if !strings.Contains(pkg, "mac-amd64") {
			dependencies[pkg] = sha256
		}

		mockCipdClient.On("Describe", testutils.AnyContext, pkg, instanceTag, false).Return(nil, errors.New("No such instance"))
		// One is missing...
		if strings.Contains(pkg, "mac-amd64") {
			mockCipdClient.On("SearchInstances", testutils.AnyContext, pkg, []string{instanceTag}).Return(common.PinSlice(nil), nil)
		} else {
			mockCipdClient.On("SearchInstances", testutils.AnyContext, pkg, []string{instanceTag}).Return(common.PinSlice([]common.Pin{
				{
					PackageName: pkg,
					InstanceID:  instanceID,
				},
			}), nil)
		}
		mockCipdClient.On("Describe", testutils.AnyContext, pkg, instanceID, false).Return(&cipd_api.InstanceDescription{
			InstanceInfo: cipd_api.InstanceInfo{
				Pin: common.Pin{
					PackageName: pkg,
					InstanceID:  instanceID,
				},
				RegisteredBy: "me@google.com",
				RegisteredTs: cipd_api.UnixTime(ts),
			},
			Tags: []cipd_api.TagInfo{
				{
					Tag: instanceTag,
				},
			},
		}, nil)
	}
	rev, err := c.GetRevision(ctx, instanceTag)
	require.NoError(t, err)
	require.Equal(t, &revision.Revision{
		Id:            instanceTag,
		Checksum:      primarySha256,
		Author:        "me@google.com",
		Description:   "some/package/linux-amd64:Vxy3fSLmQ7k5NqOhQUvsgu_GSH387Gp05UB0qd5tl6MC",
		Display:       "5",
		Timestamp:     ts,
		URL:           "https://chrome-infra-packages.appspot.com/p/some/package/linux-amd64/+/Vxy3fSLmQ7k5NqOhQUvsgu_GSH387Gp05UB0qd5tl6MC",
		InvalidReason: "no package instance exists for \"some/package/mac-amd64\" with version \"version:5\"",
		Meta: map[string]string{
			"linux-amd64":   "571cb77d22e643b93936a3a1414bec82efc6487dfcec6a74e54074a9de6d97a3",
			"linux-arm64":   "5e133aac209a455dc32780e48ec49311a846fb0dc4affba5b2ceb0f7e1b0824c",
			"mac-arm64":     "6df3a1df2d74b2d336163ec71be743b09a497e6f89f3210b4aea18f7879cf544",
			"windows-amd64": "31e2761baa44278573dc2bf3a047f5421adb119a92cd2876bae8daabb2b026d6",
		},
	}, rev)
}

func TestCIPDChild_GetRevision_PlatformsAndBackingRepo(t *testing.T) {
	mockCipdClient := &mocks.CIPDClient{}
	mockGitiles := &gitiles_mocks.GitilesRepo{}
	ctx := context.Background()
	gitilesConfig := &config.GitilesConfig{
		Branch:  git.MainBranch,
		RepoUrl: "fake.git",
	}
	gitilesRepo, err := gitiles_common.NewGitilesRepo(ctx, gitilesConfig, nil)
	require.NoError(t, err)
	gitilesRepo.GitilesRepo = mockGitiles
	c := &CIPDChild{
		client:        mockCipdClient,
		name:          "some/package/${platform}",
		ref:           "latest",
		revisionIdTag: "version",
		gitRepo:       gitilesRepo,
		platforms: []string{
			"linux-amd64",
			"linux-arm64",
			"mac-amd64",
			"mac-arm64",
			"windows-amd64",
		},
	}

	gitRevision := "abcde12345abcde12345abcde12345abcde12345"
	gitTs := time.Unix(1615384887, 0)
	instanceTag := CIPDGitRevisionTag(gitRevision)
	gitRev := &vcsinfo.LongCommit{
		ShortCommit: &vcsinfo.ShortCommit{
			Hash:    gitRevision,
			Author:  "you@google.com",
			Subject: "fake commit",
		},
		Timestamp: gitTs,
	}
	mockGitiles.On("Details", testutils.AnyContext, gitRevision).Return(gitRev, nil)
	mockGitiles.On("URL").Return(gitilesConfig.RepoUrl)

	fullPackageToID := map[string]string{
		"some/package/linux-amd64":   "Vxy3fSLmQ7k5NqOhQUvsgu_GSH387Gp05UB0qd5tl6MC",
		"some/package/linux-arm64":   "XhM6rCCaRV3DJ4DkjsSTEahG-w3Er_ulss6w9-GwgkwC",
		"some/package/mac-amd64":     "1az5xiBG-ds55R4yd7fCkODv0xrApC5gC6iLb2SCig8C",
		"some/package/mac-arm64":     "bfOh3y10stM2Fj7HG-dDsJpJfm-J8yELSuoY94ec9UQC",
		"some/package/windows-amd64": "MeJ2G6pEJ4Vz3CvzoEf1QhrbEZqSzSh2uujaq7KwJtYC",
	}
	var primarySha256 string
	for pkg, instanceID := range fullPackageToID {
		sha256, err := cipd.InstanceIDToSha256(instanceID)
		require.NoError(t, err)
		if strings.HasSuffix(pkg, c.platforms[0]) {
			primarySha256 = sha256
		}

		mockCipdClient.On("Describe", testutils.AnyContext, pkg, instanceTag, false).Return(nil, errors.New("No such instance"))
		mockCipdClient.On("SearchInstances", testutils.AnyContext, pkg, []string{instanceTag}).Return(common.PinSlice([]common.Pin{
			{
				PackageName: pkg,
				InstanceID:  instanceID,
			},
		}), nil)
		mockCipdClient.On("Describe", testutils.AnyContext, pkg, instanceID, false).Return(&cipd_api.InstanceDescription{
			InstanceInfo: cipd_api.InstanceInfo{
				Pin: common.Pin{
					PackageName: pkg,
					InstanceID:  instanceID,
				},
				RegisteredBy: "me@google.com",
				RegisteredTs: cipd_api.UnixTime(ts),
			},
			Tags: []cipd_api.TagInfo{
				{
					Tag: instanceTag,
				},
			},
		}, nil)
	}
	rev, err := c.GetRevision(ctx, instanceTag)
	require.NoError(t, err)
	require.Equal(t, &revision.Revision{
		Id:          instanceTag,
		Checksum:    primarySha256,
		Author:      gitRev.Author,
		Bugs:        map[string][]string{},
		Description: gitRev.Subject,
		Display:     gitRevision[:12],
		Timestamp:   gitTs,
		URL:         "fake.git/+show/" + gitRev.Hash,
		Meta: map[string]string{
			"linux-amd64":   "571cb77d22e643b93936a3a1414bec82efc6487dfcec6a74e54074a9de6d97a3",
			"linux-arm64":   "5e133aac209a455dc32780e48ec49311a846fb0dc4affba5b2ceb0f7e1b0824c",
			"mac-amd64":     "d5acf9c62046f9db39e51e3277b7c290e0efd31ac0a42e600ba88b6f64828a0f",
			"mac-arm64":     "6df3a1df2d74b2d336163ec71be743b09a497e6f89f3210b4aea18f7879cf544",
			"windows-amd64": "31e2761baa44278573dc2bf3a047f5421adb119a92cd2876bae8daabb2b026d6",
		},
	}, rev)
}

func TestCIPDChild_Update(t *testing.T) {
	mockCipdClient := &mocks.CIPDClient{}
	c := &CIPDChild{
		client: mockCipdClient,
		name:   "some/package",
		ref:    "latest",
	}
	ctx := context.Background()
	instanceID := "8ECbL8K2HVu1GGLRMtnzdXr5IG-ky0QnA-gU44BViPYC"
	mockCipdClient.On("ResolveVersion", testutils.AnyContext, c.name, c.ref).Return(common.Pin{
		PackageName: c.name,
		InstanceID:  instanceID,
	}, nil)
	mockCipdClient.On("Describe", testutils.AnyContext, c.name, c.ref, false).Return(nil, fmt.Errorf("no such instance"))
	mockCipdClient.On("SearchInstances", testutils.AnyContext, c.name, []string{c.ref}).Return(common.PinSlice{
		{
			PackageName: c.name,
			InstanceID:  instanceID,
		},
	}, nil)
	mockCipdClient.On("Describe", testutils.AnyContext, c.name, instanceID, false).Return(&cipd_api.InstanceDescription{
		InstanceInfo: cipd_api.InstanceInfo{
			Pin: common.Pin{
				PackageName: c.name,
				InstanceID:  instanceID,
			},
			RegisteredBy: "me@google.com",
			RegisteredTs: cipd_api.UnixTime(ts),
		},
		Tags: []cipd_api.TagInfo{
			{
				Tag: "version:5",
			},
			{
				Tag: "otherTag:blahblah",
			},
			{
				Tag: "bug:skia:12345",
			},
		},
	}, nil)
	lastRollRev := &revision.Revision{
		Id:     "instanceID_lastRollRev",
		Author: "me@google.com",
		Bugs: map[string][]string{
			"skia": {"12345"},
		},
		Description: "some/package:instanceID_lastRollRev",
		Display:     "8ECbL8K2HVu1GGLRM...",
		Timestamp:   ts,
		URL:         "https://chrome-infra-packages.appspot.com/p/some/package/+/instanceID_lastRollRev",
	}
	nextRollRev, notRolledRevs, err := c.Update(ctx, lastRollRev)
	require.NoError(t, err)
	expectRev := &revision.Revision{
		Id:       instanceID,
		Checksum: cipdInstanceChecksum,
		Author:   "me@google.com",
		Bugs: map[string][]string{
			"skia": {"12345"},
		},
		Description: "some/package:8ECbL8K2HVu1GGLRMtnzdXr5IG-ky0QnA-gU44BViPYC",
		Display:     "8ECbL8K2HVu1GGLRM...",
		Timestamp:   ts,
		URL:         "https://chrome-infra-packages.appspot.com/p/some/package/+/8ECbL8K2HVu1GGLRMtnzdXr5IG-ky0QnA-gU44BViPYC",
	}
	require.Equal(t, expectRev, nextRollRev)
	require.Equal(t, []*revision.Revision{expectRev}, notRolledRevs)
}

func TestCIPDChild_Update_HasBackingRepo(t *testing.T) {
	mockCipdClient := &mocks.CIPDClient{}
	mockGitiles := &gitiles_mocks.GitilesRepo{}
	gitilesConfig := &config.GitilesConfig{
		Branch:  git.MainBranch,
		RepoUrl: "fake.git",
	}
	gitilesRepo, err := gitiles_common.NewGitilesRepo(t.Context(), gitilesConfig, nil)
	require.NoError(t, err)
	gitilesRepo.GitilesRepo = mockGitiles
	c := &CIPDChild{
		client:  mockCipdClient,
		name:    "some/package",
		ref:     "latest",
		gitRepo: gitilesRepo,
	}

	tipRevHash := "abcde12345abcde12345abcde12345abcde12345"
	gitTs := time.Unix(1615384887, 0)
	cipdTs := time.Unix(1615384545, 0)
	instanceID := "8ECbL8K2HVu1GGLRMtnzdXr5IG-ky0QnA-gU44BViPYC"
	instanceTag := CIPDGitRevisionTag(tipRevHash)

	mockCipdClient.On("ResolveVersion", testutils.AnyContext, c.name, c.ref).Return(common.Pin{
		PackageName: c.name,
		InstanceID:  instanceID,
	}, nil)
	mockCipdClient.On("Describe", testutils.AnyContext, c.name, c.ref, false).Return(nil, fmt.Errorf("no such instance"))
	mockCipdClient.On("SearchInstances", testutils.AnyContext, c.name, []string{c.ref}).Return(common.PinSlice{
		{
			PackageName: c.name,
			InstanceID:  instanceID,
		},
	}, nil)
	mockCipdClient.On("Describe", testutils.AnyContext, c.name, instanceTag, false).Return(nil, errors.New("No such instance"))
	mockCipdClient.On("SearchInstances", testutils.AnyContext, c.name, []string{instanceTag}).Return(common.PinSlice([]common.Pin{
		{
			PackageName: c.name,
			InstanceID:  instanceID,
		},
	}), nil)
	mockCipdClient.On("Describe", testutils.AnyContext, c.name, instanceID, false).Return(&cipd_api.InstanceDescription{
		InstanceInfo: cipd_api.InstanceInfo{
			Pin: common.Pin{
				PackageName: c.name,
				InstanceID:  instanceID,
			},
			RegisteredBy: "me@google.com",
			RegisteredTs: cipd_api.UnixTime(cipdTs),
		},
		Tags: []cipd_api.TagInfo{
			{
				Tag: instanceTag,
			},
			{
				Tag: "otherTag:blahblah",
			},
			{
				Tag: "bug:skia:12345",
			},
		},
	}, nil)

	tipCommit := &vcsinfo.LongCommit{
		ShortCommit: &vcsinfo.ShortCommit{
			Hash:    tipRevHash,
			Author:  "you@google.com",
			Subject: "fake commit",
		},
		Timestamp: gitTs,
	}
	middleCommitA := &vcsinfo.LongCommit{
		ShortCommit: &vcsinfo.ShortCommit{
			Hash:    "cccccccccccccccccccccccccccccccccccccccc",
			Author:  "you@google.com",
			Subject: "middle commit A",
		},
		Timestamp: gitTs.Add(-4 * time.Minute),
	}
	middleCommitB := &vcsinfo.LongCommit{
		ShortCommit: &vcsinfo.ShortCommit{
			Hash:    "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			Author:  "you@google.com",
			Subject: "middle commit B",
		},
		Timestamp: gitTs.Add(-2 * time.Minute),
	}
	lastRollRevHash := "bcdef67890bcdef67890bcdef67890bcdef67890"
	lastRollRev := &revision.Revision{
		Id:          CIPDGitRevisionTag(lastRollRevHash),
		Author:      "me@google.com",
		Bugs:        map[string][]string{},
		Description: "fake last roll rev",
		Display:     "bcdef67890bc",
		Timestamp:   gitTs.Add(-10 * time.Minute),
		URL:         "fake.git/+show/" + lastRollRevHash,
	}
	hashes := []string{tipCommit.Hash, middleCommitB.Hash, middleCommitA.Hash, lastRollRevHash}
	commits := []*vcsinfo.LongCommit{tipCommit, middleCommitB, middleCommitA}

	mockGitiles.On("Details", testutils.AnyContext, tipRevHash).Return(tipCommit, nil).Once()
	mockGitiles_getRevisionHelper(mockGitiles, tipRevHash, tipRevHash)
	mockGitiles.On("LogFirstParent", testutils.AnyContext, lastRollRevHash, tipRevHash).Return(commits, nil).Once()
	MockGitiles_ConvertRevisions(mockGitiles, hashes, tipRevHash)
	mockGitiles.On("URL").Return("fake.git")

	nextRollRev, notRolledRevs, err := c.Update(t.Context(), lastRollRev)
	require.NoError(t, err)
	expectNextRollRev := &revision.Revision{
		Id:          instanceTag,
		Checksum:    cipdInstanceChecksum,
		Author:      "you@google.com",
		Bugs:        map[string][]string{},
		Description: tipCommit.Subject,
		Display:     tipCommit.Hash[:12],
		Timestamp:   tipCommit.Timestamp,
		URL:         "fake.git/+show/" + tipCommit.Hash,
	}
	expectMiddleRevA := &revision.Revision{
		Id:            CIPDGitRevisionTag(middleCommitA.Hash),
		Checksum:      "",
		Author:        "you@google.com",
		Bugs:          map[string][]string{},
		Description:   middleCommitA.Subject,
		Display:       middleCommitA.Hash[:12],
		InvalidReason: "No associated CIPD package.",
		Timestamp:     middleCommitA.Timestamp,
		URL:           "fake.git/+show/" + middleCommitA.Hash,
	}
	expectMiddleRevB := &revision.Revision{
		Id:            CIPDGitRevisionTag(middleCommitB.Hash),
		Checksum:      "",
		Author:        "you@google.com",
		Bugs:          map[string][]string{},
		Description:   middleCommitB.Subject,
		Display:       middleCommitB.Hash[:12],
		InvalidReason: "No associated CIPD package.",
		Timestamp:     middleCommitB.Timestamp,
		URL:           "fake.git/+show/" + middleCommitB.Hash,
	}
	require.Equal(t, expectNextRollRev, nextRollRev)
	require.Equal(t, []*revision.Revision{
		expectNextRollRev,
		expectMiddleRevB,
		expectMiddleRevA,
	}, notRolledRevs)
}
