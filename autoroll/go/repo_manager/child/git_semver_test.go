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

	const ver100 = "v1.0.0"
	const ver120 = "v1.2.0"
	const ver1150 = "v1.15.0"
	const ver1150Tag = "upstream/v1.15.0"

	const hashA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const hashB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	const hashC = "0000000000000000000000000000000000000000"

	v := func(tag string) *semver.Version {
		ver, err := c.semVerParser.Parse(tag)
		require.NoError(t, err)
		return ver
	}

	expectVersions := []*semver.Version{v(ver1150), v(ver120), v(ver100)}
	expectHashToVersions := map[string][]*semver.Version{
		hashA: {v(ver100)},
		hashB: {v(ver1150), v(ver120)},
	}
	expectVersionToHash := map[string]string{
		ver100:  hashA,
		ver120:  hashB,
		ver1150: hashB,
	}

	mockRepo.On("Tags", testutils.AnyContext).Return(map[string]string{
		ver100:     hashA,
		ver120:     hashB,
		ver1150Tag: hashB,
	}, nil)

	versions, versionToHash, hashToVersions, err := c.getVersions(t.Context())
	require.NoError(t, err)
	require.Equal(t, expectVersions, versions)
	require.Equal(t, expectVersionToHash, versionToHash)
	require.Equal(t, expectHashToVersions, hashToVersions)
}

func TestGitSemVerChild_Update(t *testing.T) {
	c, mockRepo := setupGitSemVerChild(t, `v(\d+)\.(\d+)\.(\d+)`)

	const ver100 = "v1.0.0"
	const ver110 = "v1.1.0"

	const hashA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const hashB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	lastRollRev := &revision.Revision{
		Id: hashA,
	}

	mockRepo.On("Tags", testutils.AnyContext).Return(map[string]string{
		ver100: hashA,
		ver110: hashB,
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
	require.Equal(t, "v1.1.0", tip.Release)
	require.Len(t, notRolled, 1)
	require.Equal(t, tip, notRolled[0])
}

func TestGitSemVerChild_GetRevision(t *testing.T) {
	c, mockRepo := setupGitSemVerChild(t, `v(\d+)\.(\d+)\.(\d+)`)

	const ver100 = "v1.0.0"
	const hashA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	mockRepo.On("Tags", testutils.AnyContext).Return(map[string]string{
		ver100: hashA,
	}, nil)
	mockRepo.On("URL").Return("https://github.com/google/skia")
	mockCommit := &vcsinfo.LongCommit{
		ShortCommit: &vcsinfo.ShortCommit{
			Hash: hashA,
		},
	}
	mockRepo.On("Details", testutils.AnyContext, "v1.0.0").Return(mockCommit, nil)

	rev, err := c.GetRevision(t.Context(), "v1.0.0")
	require.NoError(t, err)
	require.Equal(t, hashA, rev.Id)
	require.Equal(t, hashA, rev.Checksum)
	require.Equal(t, "v1.0.0", rev.Release)
}

func TestGitSemVerChild_LogRevisions(t *testing.T) {
	c, mockRepo := setupGitSemVerChild(t, `v(\d+)\.(\d+)\.(\d+)`)

	const ver100 = "v1.0.0"
	const ver110 = "v1.1.0"
	const ver120 = "v1.2.0"
	const ver130 = "v1.3.0"

	p, err := semver.NewParser(`v(\d+)\.(\d+)\.(\d+)`)
	require.NoError(t, err)
	v100, err := p.Parse(ver100)
	require.NoError(t, err)
	v110, err := p.Parse(ver110)
	require.NoError(t, err)
	v120, err := p.Parse(ver120)
	require.NoError(t, err)
	v130, err := p.Parse(ver130)
	require.NoError(t, err)

	commitA := &vcsinfo.LongCommit{
		ShortCommit: &vcsinfo.ShortCommit{
			Hash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		Timestamp: time.Unix(000000100, 0),
	}
	commitB := &vcsinfo.LongCommit{
		ShortCommit: &vcsinfo.ShortCommit{
			Hash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		},
		Timestamp: time.Unix(000000200, 0),
	}
	commitC := &vcsinfo.LongCommit{
		ShortCommit: &vcsinfo.ShortCommit{
			Hash: "cccccccccccccccccccccccccccccccccccccccc",
		},
		Timestamp: time.Unix(000000300, 0),
	}
	commitD := &vcsinfo.LongCommit{
		ShortCommit: &vcsinfo.ShortCommit{
			Hash: "dddddddddddddddddddddddddddddddddddddddd",
		},
		Timestamp: time.Unix(000000400, 0),
	}

	versionToHash := map[string]string{
		ver100: commitA.Hash,
		ver110: commitB.Hash,
		ver120: commitC.Hash,
		ver130: commitD.Hash,
	}
	hashToVersions := map[string][]*semver.Version{
		commitA.Hash: {v100},
		commitB.Hash: {v110},
		commitC.Hash: {v120},
		commitD.Hash: {v130},
	}

	const url = "https://github.com/google/skia"
	const revLinkTmpl = url + "/+show/%s"
	revA := revision.FromLongCommit(revLinkTmpl, "", commitA)
	revB := revision.FromLongCommit(revLinkTmpl, "", commitB)
	revC := revision.FromLongCommit(revLinkTmpl, "", commitC)
	revD := revision.FromLongCommit(revLinkTmpl, "", commitD)
	c.fixupRevision(revA, hashToVersions)
	c.fixupRevision(revB, hashToVersions)
	c.fixupRevision(revC, hashToVersions)
	c.fixupRevision(revD, hashToVersions)

	mockDetails := func(commits ...*vcsinfo.LongCommit) {
		for _, commit := range commits {
			mockRepo.On("Details", testutils.AnyContext, commit.Hash).Return(commit, nil).Once()
		}
	}

	check := func(fromRev, toRev *revision.Revision, expect []*revision.Revision) {
		actual, err := c.LogRevisions(t.Context(), fromRev, toRev)
		require.NoError(t, err)
		require.Equal(t, expect, actual)
		mockRepo.AssertExpectations(t)
	}

	mockRepo.On("Tags", testutils.AnyContext).Return(versionToHash, nil)
	mockRepo.On("URL").Return(url)

	t.Run("A to D", func(t *testing.T) {
		mockDetails(commitB, commitC, commitD)
		check(revA, revD, []*revision.Revision{revD, revC, revB})
	})
	t.Run("B to D", func(t *testing.T) {
		mockDetails(commitC, commitD)
		check(revB, revD, []*revision.Revision{revD, revC})
	})
	t.Run("B to C", func(t *testing.T) {
		mockDetails(commitC)
		check(revB, revC, []*revision.Revision{revC})
	})
	t.Run("from is not a release", func(t *testing.T) {
		fromRev := &revision.Revision{
			Id:        "someotherhash",
			Timestamp: time.Unix(000000150, 0), // Between A and B
		}
		mockDetails(commitA, commitB, commitC, commitD)
		check(fromRev, revD, []*revision.Revision{revD, revC, revB})
	})
	t.Run("from is older than all releases", func(t *testing.T) {
		fromRev := &revision.Revision{
			Id:        "someotherhash",
			Timestamp: time.Unix(000000050, 0), // Before A
		}
		mockDetails(commitA, commitB, commitC, commitD)
		check(fromRev, revD, []*revision.Revision{revD, revC, revB, revA})
	})
	t.Run("to is not a release", func(t *testing.T) {
		toRev := &revision.Revision{
			Id:        "someotherhash",
			Timestamp: time.Unix(000000350, 0), // Between C and D
		}
		mockDetails(commitB, commitC, commitD)
		check(revA, toRev, []*revision.Revision{revC, revB})
	})
	t.Run("to is newer than all releases", func(t *testing.T) {
		toRev := &revision.Revision{
			Id:        "someotherhash",
			Timestamp: time.Unix(000000450, 0), // After D
		}
		mockDetails(commitB, commitC, commitD)
		check(revA, toRev, []*revision.Revision{revD, revC, revB})
	})
	t.Run("neither are releases", func(t *testing.T) {
		fromRev := &revision.Revision{
			Id:        "fromHash",
			Timestamp: time.Unix(000000150, 0), // Between A and B.
		}
		toRev := &revision.Revision{
			Id:        "toHash",
			Timestamp: time.Unix(000000350, 0), // Between C and D.
		}
		mockDetails(commitA, commitB, commitC, commitD)
		check(fromRev, toRev, []*revision.Revision{revC, revB})
	})
	t.Run("from is newer than to", func(t *testing.T) {
		check(revC, revB, []*revision.Revision{})
	})
	t.Run("from is newer than to and neither are releases", func(t *testing.T) {
		fromRev := &revision.Revision{
			Id:        "fromHash",
			Timestamp: time.Unix(000000350, 0), // Between C and D.
		}
		toRev := &revision.Revision{
			Id:        "toHash",
			Timestamp: time.Unix(000000150, 0), // Between A and B.
		}
		mockDetails(commitA, commitB, commitC, commitD)
		check(fromRev, toRev, []*revision.Revision{})
	})
}

func TestGitSemVerChild_Update_MultipleTags(t *testing.T) {
	c, mockRepo := setupGitSemVerChild(t, `v(\d+)\.(\d+)\.(\d+)`)

	const ver100 = "v1.0.0"
	const ver110 = "v1.1.0"
	const ver120 = "v1.2.0"

	const hashA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const hashB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	lastRollRev := &revision.Revision{
		Id: hashA,
	}

	mockRepo.On("Tags", testutils.AnyContext).Return(map[string]string{
		ver100: hashA,
		ver110: hashB,
		ver120: hashB,
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
	require.Equal(t, "v1.2.0", tip.Release)
	// v1.1.0 points to the same revision as v1.2.0 (tip).
	rev110 := tip.Copy()
	rev110.Release = ver110
	rev110.Display = ver110
	require.Equal(t, []*revision.Revision{tip, rev110}, notRolled)
}

func TestGitSemVerChild_fixupRevision_NoTag(t *testing.T) {
	c, _ := setupGitSemVerChild(t, `v(\d+)\.(\d+)\.(\d+)`)

	rev := &revision.Revision{
		Id: "cccccccccccccccccccccccccccccccccccccccc",
	}
	hashToVersions := map[string][]*semver.Version{}

	c.fixupRevision(rev, hashToVersions)
	require.Equal(t, "cccccccccccccccccccccccccccccccccccccccc", rev.Id)
	require.Empty(t, rev.Release)
	require.Equal(t, "No associated tag matching the configured regex.", rev.InvalidReason)
}
