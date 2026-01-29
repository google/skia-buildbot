package child

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/repo_manager/common/gitiles_common"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/gitiles/mocks"
	"go.skia.org/infra/go/semver"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/vcsinfo"
)

func setupGitSemVerChild(t *testing.T, regex string) (*gitSemVerChild, *mocks.GitilesRepo) {
	parser, err := semver.NewParser(regex)
	require.NoError(t, err)
	mockRepo := &mocks.GitilesRepo{}
	return &gitSemVerChild{
		repo: &gitiles_common.GitilesRepo{
			GitilesRepo: mockRepo,
		},
		semVerParser: parser,
	}, mockRepo
}

func TestGitSemVerChild_getVersions(t *testing.T) {
	c, mockRepo := setupGitSemVerChild(t, `v(\d+)\.(\d+)\.(\d+)`)

	ver100, err := c.semVerParser.Parse("v1.0.0")
	require.NoError(t, err)
	ver120, err := c.semVerParser.Parse("v1.2.0")
	require.NoError(t, err)
	ver1150, err := c.semVerParser.Parse("v1.15.0")
	require.NoError(t, err)

	const hashA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const hashB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	const hashC = "0000000000000000000000000000000000000000"

	expectVersions := []*semver.Version{ver1150, ver120, ver100}
	expectHashToTags := map[string][]*semver.Version{
		hashA: {ver100},
		hashB: {ver1150, ver120},
	}
	expectTagToHash := map[string]string{
		ver100.String():  hashA,
		ver120.String():  hashB,
		ver1150.String(): hashB,
		"bogus":          hashC,
	}

	mockRepo.On("Tags", testutils.AnyContext).Return(expectTagToHash, nil)

	versions, tagToHash, hashToTags, err := c.getVersions(t.Context())
	require.NoError(t, err)
	require.Equal(t, expectVersions, versions)
	require.Equal(t, expectTagToHash, tagToHash)
	require.Equal(t, expectHashToTags, hashToTags)
}

func TestGitSemVerChild_Update(t *testing.T) {
	c, mockRepo := setupGitSemVerChild(t, `v(\d+)\.(\d+)\.(\d+)`)

	ver100, err := c.semVerParser.Parse("v1.0.0")
	require.NoError(t, err)
	ver110, err := c.semVerParser.Parse("v1.1.0")
	require.NoError(t, err)

	const hashA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const hashB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	lastRollRev := &revision.Revision{
		Id: hashA,
	}

	mockRepo.On("Tags", testutils.AnyContext).Return(map[string]string{
		ver100.String(): hashA,
		ver110.String(): hashB,
	}, nil)
	mockRepo.On("URL").Return("https://github.com/google/skia")
	tipCommit := &vcsinfo.LongCommit{
		ShortCommit: &vcsinfo.ShortCommit{
			Hash: hashB,
		},
		Timestamp: time.Now(),
	}
	mockRepo.On("Details", testutils.AnyContext, hashB).Return(tipCommit, nil)
	mockRepo.On("LogFirstParent", testutils.AnyContext, hashA, hashB).Return([]*vcsinfo.LongCommit{tipCommit}, nil)

	tip, notRolled, err := c.Update(t.Context(), lastRollRev)
	require.NoError(t, err)
	require.Equal(t, hashB, tip.Id)
	require.Equal(t, hashB, tip.Checksum)
	require.Equal(t, ver110.String(), tip.Release)
	require.Len(t, notRolled, 1)
	require.Equal(t, tip, notRolled[0])
}

func TestGitSemVerChild_GetRevision(t *testing.T) {
	c, mockRepo := setupGitSemVerChild(t, `v(\d+)\.(\d+)\.(\d+)`)

	ver100, err := c.semVerParser.Parse("v1.0.0")
	require.NoError(t, err)

	const hashA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	mockRepo.On("Tags", testutils.AnyContext).Return(map[string]string{
		ver100.String(): hashA,
	}, nil)
	mockRepo.On("URL").Return("https://github.com/google/skia")
	mockCommit := &vcsinfo.LongCommit{
		ShortCommit: &vcsinfo.ShortCommit{
			Hash: hashA,
		},
	}
	mockRepo.On("Details", testutils.AnyContext, ver100.String()).Return(mockCommit, nil)

	rev, err := c.GetRevision(t.Context(), ver100.String())
	require.NoError(t, err)
	require.Equal(t, hashA, rev.Id)
	require.Equal(t, hashA, rev.Checksum)
	require.Equal(t, ver100.String(), rev.Release)
}

func TestGitSemVerChild_LogRevisions(t *testing.T) {
	c, mockRepo := setupGitSemVerChild(t, `v(\d+)\.(\d+)\.(\d+)`)

	ver100, err := c.semVerParser.Parse("v1.0.0")
	require.NoError(t, err)
	ver110, err := c.semVerParser.Parse("v1.1.0")
	require.NoError(t, err)

	const hashA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const hashB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	fromRev := &revision.Revision{Id: hashA}
	toRev := &revision.Revision{Id: hashB}

	mockRepo.On("Tags", testutils.AnyContext).Return(map[string]string{
		ver100.String(): hashA,
		ver110.String(): hashB,
	}, nil)
	mockRepo.On("URL").Return("https://github.com/google/skia")
	returnedCommits := []*vcsinfo.LongCommit{
		{
			ShortCommit: &vcsinfo.ShortCommit{
				Hash: hashB,
			},
		},
	}
	mockRepo.On("LogFirstParent", testutils.AnyContext, fromRev.Id, toRev.Id).Return(returnedCommits, nil)

	revs, err := c.LogRevisions(t.Context(), fromRev, toRev)
	require.NoError(t, err)
	require.Len(t, revs, 1)
	require.Equal(t, hashB, revs[0].Id)
	require.Equal(t, hashB, revs[0].Checksum)
	require.Equal(t, ver110.String(), revs[0].Release)
}

func TestGitSemVerChild_Update_MultipleTags(t *testing.T) {
	c, mockRepo := setupGitSemVerChild(t, `v(\d+)\.(\d+)\.(\d+)`)

	ver100, err := c.semVerParser.Parse("v1.0.0")
	require.NoError(t, err)
	ver110, err := c.semVerParser.Parse("v1.1.0")
	require.NoError(t, err)
	ver120, err := c.semVerParser.Parse("v1.2.0")
	require.NoError(t, err)

	const hashA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const hashB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	lastRollRev := &revision.Revision{
		Id: hashA,
	}

	mockRepo.On("Tags", testutils.AnyContext).Return(map[string]string{
		ver100.String(): hashA,
		ver110.String(): hashB,
		ver120.String(): hashB,
	}, nil)
	mockRepo.On("URL").Return("https://github.com/google/skia")
	tipCommit := &vcsinfo.LongCommit{
		ShortCommit: &vcsinfo.ShortCommit{
			Hash: hashB,
		},
		Timestamp: time.Now(),
	}
	mockRepo.On("Details", testutils.AnyContext, hashB).Return(tipCommit, nil)
	mockRepo.On("LogFirstParent", testutils.AnyContext, hashA, hashB).Return([]*vcsinfo.LongCommit{tipCommit}, nil)

	tip, notRolled, err := c.Update(t.Context(), lastRollRev)
	require.NoError(t, err)
	require.Equal(t, hashB, tip.Id)
	require.Equal(t, hashB, tip.Checksum)
	require.Equal(t, ver120.String(), tip.Release)
	require.Len(t, notRolled, 1)
	require.Equal(t, tip, notRolled[0])
}

func TestGitSemVerChild_fixupRevision_NoTag(t *testing.T) {
	c, _ := setupGitSemVerChild(t, `v(\d+)\.(\d+)\.(\d+)`)

	rev := &revision.Revision{
		Id: "cccccccccccccccccccccccccccccccccccccccc",
	}
	hashToTags := map[string][]*semver.Version{}

	c.fixupRevision(rev, hashToTags)
	require.Equal(t, "cccccccccccccccccccccccccccccccccccccccc", rev.Id)
	require.Empty(t, rev.Release)
	require.Equal(t, "No associated tag matching the configured regex.", rev.InvalidReason)
}
