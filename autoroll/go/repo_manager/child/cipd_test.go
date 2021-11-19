package child

import (
	"context"
	"errors"
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
		Branch:  git.MainBranch,
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
	instanceTag := gitRevTag(gitRevision)
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

func TestCIPDChild_Update(t *testing.T) {
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
	mockCipdClient.On("ResolveVersion", testutils.AnyContext, c.name, c.tag).Return(common.Pin{
		PackageName: c.name,
		InstanceID:  instanceID,
	}, nil)
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
	lastRollRev := &revision.Revision{
		Id:     "instanceID_lastRollRev",
		Author: "me@google.com",
		Bugs: map[string][]string{
			"skia": {"12345"},
		},
		Description: "some/package:instanceID_lastRollRev",
		Display:     "instanceI...",
		Timestamp:   ts,
		URL:         "https://chrome-infra-packages.appspot.com/p/some/package/+/instanceID_lastRollRev",
	}
	nextRollRev, notRolledRevs, err := c.Update(ctx, lastRollRev)
	require.NoError(t, err)
	expectRev := &revision.Revision{
		Id:     instanceID,
		Author: "me@google.com",
		Bugs: map[string][]string{
			"skia": {"12345"},
		},
		Description: "some/package:instanceID123",
		Display:     "instanceI...",
		Timestamp:   ts,
		URL:         "https://chrome-infra-packages.appspot.com/p/some/package/+/instanceID123",
	}
	require.Equal(t, expectRev, nextRollRev)
	require.Equal(t, []*revision.Revision{expectRev}, notRolledRevs)
}

func TestCIPDChild_Update_HasBackingRepo(t *testing.T) {
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
		Branch:  git.MainBranch,
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
	instanceTag := gitRevTag(gitRevision)
	tipCommit := &vcsinfo.LongCommit{
		ShortCommit: &vcsinfo.ShortCommit{
			Hash:    gitRevision,
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
	mockGitiles.On("Details", testutils.AnyContext, gitRevision).Return(tipCommit, nil)
	mockGitiles.On("Details", testutils.AnyContext, middleCommitA.Hash).Return(middleCommitA, nil)
	mockGitiles.On("Details", testutils.AnyContext, middleCommitB.Hash).Return(middleCommitB, nil)
	mockGitiles.On("URL").Return(gitilesConfig.RepoUrl)
	mockCipdClient.On("ResolveVersion", testutils.AnyContext, c.name, c.tag).Return(common.Pin{
		PackageName: c.name,
		InstanceID:  instanceID,
	}, nil)
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
	lastRollRevHash := "bcdef67890bcdef67890bcdef67890bcdef67890"
	lastRollRev := &revision.Revision{
		Id:          gitRevTag(lastRollRevHash),
		Author:      "me@google.com",
		Bugs:        map[string][]string{},
		Description: "fake last roll rev",
		Display:     "bcdef67890bc",
		Timestamp:   gitTs.Add(-10 * time.Minute),
		URL:         "fake.git/+show/" + lastRollRevHash,
	}

	mockGitiles.On("LogFirstParent", testutils.AnyContext, lastRollRevHash, tipCommit.Hash).Return([]*vcsinfo.LongCommit{
		tipCommit,
		middleCommitB,
		middleCommitA,
	}, nil)
	nextRollRev, notRolledRevs, err := c.Update(ctx, lastRollRev)
	require.NoError(t, err)
	expectNextRollRev := &revision.Revision{
		Id:          instanceTag,
		Author:      "you@google.com",
		Bugs:        map[string][]string{},
		Description: tipCommit.Subject,
		Display:     tipCommit.Hash[:12],
		Timestamp:   tipCommit.Timestamp,
		URL:         "fake.git/+show/" + tipCommit.Hash,
	}
	expectMiddleRevA := &revision.Revision{
		Id:            gitRevTag(middleCommitA.Hash),
		Author:        "you@google.com",
		Bugs:          map[string][]string{},
		Description:   middleCommitA.Subject,
		Display:       middleCommitA.Hash[:12],
		InvalidReason: "No associated CIPD package.",
		Timestamp:     middleCommitA.Timestamp,
		URL:           "fake.git/+show/" + middleCommitA.Hash,
	}
	expectMiddleRevB := &revision.Revision{
		Id:            gitRevTag(middleCommitB.Hash),
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
