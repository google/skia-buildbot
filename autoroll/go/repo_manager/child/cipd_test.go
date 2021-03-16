package child

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.chromium.org/luci/cipd/client/cipd"
	"go.chromium.org/luci/cipd/common"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gitiles_common"
	"go.skia.org/infra/autoroll/go/revision"
	chrome_branch_mocks "go.skia.org/infra/go/chrome_branch/mocks"
	"go.skia.org/infra/go/cipd/mocks"
	"go.skia.org/infra/go/git"
	gitiles_mocks "go.skia.org/infra/go/gitiles/mocks"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/vcsinfo"
)

func TestCIPDInstanceToRevision(t *testing.T) {
	unittest.SmallTest(t)

	ts := time.Unix(1615384545, 0)
	pkg := &cipd.InstanceDescription{
		InstanceInfo: cipd.InstanceInfo{
			Pin: common.Pin{
				PackageName: "some/package",
				InstanceID:  "instanceID123",
			},
			RegisteredBy: "me@google.com",
			RegisteredTs: cipd.UnixTime(ts),
		},
		Tags: []cipd.TagInfo{
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
		Id:     "instanceID123",
		Author: "me@google.com",
		Bugs: map[string][]string{
			"skia": {"12345"},
		},
		Description: "some/package:instanceID123",
		Display:     "instanceI...",
		Timestamp:   ts,
		URL:         "https://chrome-infra-packages.appspot.com/p/some/package/+/instanceID123",
	}
	rev := CIPDInstanceToRevision("some/package", pkg)
	require.Equal(t, expect, rev)
}

func TestCIPDChild_GetRevision(t *testing.T) {
	unittest.SmallTest(t)

	mockCipdClient := &mocks.CIPDClient{}
	c := &CIPDChild{
		client: mockCipdClient,
		name:   "some/package",
		tag:    "latest",
	}
	ctx := context.Background()
	ts := time.Unix(1615384545, 0)
	instanceID := "instanceID123"
	mockCipdClient.On("Describe", testutils.AnyContext, c.name, instanceID).Return(&cipd.InstanceDescription{
		InstanceInfo: cipd.InstanceInfo{
			Pin: common.Pin{
				PackageName: c.name,
				InstanceID:  instanceID,
			},
			RegisteredBy: "me@google.com",
			RegisteredTs: cipd.UnixTime(ts),
		},
		Tags: []cipd.TagInfo{
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
		Id:     instanceID,
		Author: "me@google.com",
		Bugs: map[string][]string{
			"skia": {"12345"},
		},
		Description: "some/package:instanceID123",
		Display:     "instanceI...",
		Timestamp:   ts,
		URL:         "https://chrome-infra-packages.appspot.com/p/some/package/+/instanceID123",
	}, rev)
}

func TestCIPDChild_GetRevision_HasBackingRepo(t *testing.T) {
	unittest.SmallTest(t)

	mockCipdClient := &mocks.CIPDClient{}
	mockGitiles := &gitiles_mocks.GitilesRepo{}
	ctx := context.Background()
	cbc := &chrome_branch_mocks.Client{}
	configDummyVars := config_vars.DummyVars()
	cbc.On("Get", ctx).Return(configDummyVars.Branches.Chromium, nil)
	reg, err := config_vars.NewRegistry(ctx, cbc)
	require.NoError(t, err)
	gitilesConfig := &config.GitilesConfig{
		Branch:  git.DefaultBranch,
		RepoUrl: "fake.git",
	}
	gitilesRepo, err := gitiles_common.NewGitilesRepo(ctx, gitilesConfig, reg, nil)
	require.NoError(t, err)
	gitilesRepo.GitilesRepo = mockGitiles
	c := &CIPDChild{
		client:  mockCipdClient,
		name:    "some/package",
		tag:     "latest",
		gitRepo: gitilesRepo,
	}

	gitRevision := "abcde12345abcde12345abcde12345abcde12345"
	gitTs := time.Unix(1615384887, 0)
	cipdTs := time.Unix(1615384545, 0)
	instanceID := "instanceID123"
	instanceTag := fmt.Sprintf("%s:%s", gitRevisionTag, gitRevision)
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
	mockCipdClient.On("Describe", testutils.AnyContext, c.name, instanceTag).Return(nil, errors.New("No such instance"))
	mockCipdClient.On("SearchInstances", testutils.AnyContext, c.name, []string{instanceTag}).Return(common.PinSlice([]common.Pin{
		{
			PackageName: c.name,
			InstanceID:  instanceID,
		},
	}), nil)
	mockCipdClient.On("Describe", testutils.AnyContext, c.name, instanceID).Return(&cipd.InstanceDescription{
		InstanceInfo: cipd.InstanceInfo{
			Pin: common.Pin{
				PackageName: c.name,
				InstanceID:  instanceID,
			},
			RegisteredBy: "me@google.com",
			RegisteredTs: cipd.UnixTime(cipdTs),
		},
		Tags: []cipd.TagInfo{
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
		Author:      "you@google.com",
		Bugs:        map[string][]string{},
		Description: gitRev.Subject,
		Display:     gitRev.Hash[:12],
		Timestamp:   gitRev.Timestamp,
		URL:         "fake.git/+show/" + gitRev.Hash,
	}, rev)
}
