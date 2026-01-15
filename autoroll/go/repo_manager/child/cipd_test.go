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
	"go.skia.org/infra/autoroll/go/repo_manager/common/gitiles_common"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/cipd/mocks"
	"go.skia.org/infra/go/git"
	gitiles_mocks "go.skia.org/infra/go/gitiles/mocks"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vcsinfo"
)

const (
	cipdInstanceChecksum = "f0409b2fc2b61d5bb51862d132d9f3757af9206fa4cb442703e814e3805588f6"
)

func TestCIPDInstanceToRevision(t *testing.T) {
	ts := time.Unix(1615384545, 0)
	pkg := &cipd.InstanceDescription{
		InstanceInfo: cipd.InstanceInfo{
			Pin: common.Pin{
				PackageName: "some/package",
				InstanceID:  "8ECbL8K2HVu1GGLRMtnzdXr5IG-ky0QnA-gU44BViPYC",
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
	ts := time.Unix(1615384545, 0)
	pkg := &cipd.InstanceDescription{
		InstanceInfo: cipd.InstanceInfo{
			Pin: common.Pin{
				PackageName: "some/package",
				InstanceID:  "8ECbL8K2HVu1GGLRMtnzdXr5IG-ky0QnA-gU44BViPYC",
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
	ts := time.Unix(1615384545, 0)
	pkg := &cipd.InstanceDescription{
		InstanceInfo: cipd.InstanceInfo{
			Pin: common.Pin{
				PackageName: "some/package",
				InstanceID:  "8ECbL8K2HVu1GGLRMtnzdXr5IG-ky0QnA-gU44BViPYC",
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
	ts := time.Unix(1615384545, 0)
	pkg := &cipd.InstanceDescription{
		InstanceInfo: cipd.InstanceInfo{
			Pin: common.Pin{
				PackageName: "some/package",
				InstanceID:  "8ECbL8K2HVu1GGLRMtnzdXr5IG-ky0QnA-gU44BViPYC",
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
		tag:    "latest",
	}
	ctx := context.Background()
	ts := time.Unix(1615384545, 0)
	instanceID := "8ECbL8K2HVu1GGLRMtnzdXr5IG-ky0QnA-gU44BViPYC"
	mockCipdClient.On("Describe", testutils.AnyContext, c.name, instanceID, false).Return(&cipd.InstanceDescription{
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
		tag:     "latest",
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
	mockCipdClient.On("Describe", testutils.AnyContext, c.name, instanceID, false).Return(&cipd.InstanceDescription{
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
		tag:           "latest",
		revisionIdTag: "version",
	}

	ts := time.Unix(1615384545, 0)
	instanceID := "8ECbL8K2HVu1GGLRMtnzdXr5IG-ky0QnA-gU44BViPYC"
	instanceTag := "version:5"

	mockCipdClient.On("Describe", testutils.AnyContext, c.name, c.revisionIdTag, false).Return(nil, errors.New("No such instance"))
	mockCipdClient.On("SearchInstances", testutils.AnyContext, c.name, []string{instanceTag}).Return(common.PinSlice([]common.Pin{
		{
			PackageName: c.name,
			InstanceID:  instanceID,
		},
	}), nil)
	mockCipdClient.On("Describe", testutils.AnyContext, c.name, instanceID, false).Return(&cipd.InstanceDescription{
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
		tag:                   "latest",
		revisionIdTag:         "version",
		revisionIdTagStripKey: true,
	}

	ts := time.Unix(1615384545, 0)
	instanceID := "8ECbL8K2HVu1GGLRMtnzdXr5IG-ky0QnA-gU44BViPYC"
	instanceTag := "version:5"

	mockCipdClient.On("Describe", testutils.AnyContext, c.name, c.revisionIdTag, false).Return(nil, errors.New("No such instance"))
	mockCipdClient.On("SearchInstances", testutils.AnyContext, c.name, []string{instanceTag}).Return(common.PinSlice([]common.Pin{
		{
			PackageName: c.name,
			InstanceID:  instanceID,
		},
	}), nil)
	mockCipdClient.On("Describe", testutils.AnyContext, c.name, instanceID, false).Return(&cipd.InstanceDescription{
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

func TestCIPDChild_Update(t *testing.T) {
	mockCipdClient := &mocks.CIPDClient{}
	c := &CIPDChild{
		client: mockCipdClient,
		name:   "some/package",
		tag:    "latest",
	}
	ctx := context.Background()
	ts := time.Unix(1615384545, 0)
	instanceID := "8ECbL8K2HVu1GGLRMtnzdXr5IG-ky0QnA-gU44BViPYC"
	mockCipdClient.On("ResolveVersion", testutils.AnyContext, c.name, c.tag).Return(common.Pin{
		PackageName: c.name,
		InstanceID:  instanceID,
	}, nil)
	mockCipdClient.On("Describe", testutils.AnyContext, c.name, instanceID, false).Return(&cipd.InstanceDescription{
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
		tag:     "latest",
		gitRepo: gitilesRepo,
	}

	tipRevHash := "abcde12345abcde12345abcde12345abcde12345"
	gitTs := time.Unix(1615384887, 0)
	cipdTs := time.Unix(1615384545, 0)
	instanceID := "8ECbL8K2HVu1GGLRMtnzdXr5IG-ky0QnA-gU44BViPYC"
	instanceTag := CIPDGitRevisionTag(tipRevHash)

	mockCipdClient.On("ResolveVersion", testutils.AnyContext, c.name, c.tag).Return(common.Pin{
		PackageName: c.name,
		InstanceID:  instanceID,
	}, nil)
	mockCipdClient.On("Describe", testutils.AnyContext, c.name, instanceTag, false).Return(nil, errors.New("No such instance"))
	mockCipdClient.On("SearchInstances", testutils.AnyContext, c.name, []string{instanceTag}).Return(common.PinSlice([]common.Pin{
		{
			PackageName: c.name,
			InstanceID:  instanceID,
		},
	}), nil)
	mockCipdClient.On("Describe", testutils.AnyContext, c.name, instanceID, false).Return(&cipd.InstanceDescription{
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
