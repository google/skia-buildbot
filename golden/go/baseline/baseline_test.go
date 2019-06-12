package baseline_test

import (
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/baseline"
	"go.skia.org/infra/golden/go/mocks"
	data "go.skia.org/infra/golden/go/testutils/data_three_devices"
	"go.skia.org/infra/golden/go/tryjobstore"
	"go.skia.org/infra/golden/go/types"
)

func TestGetBaselinesPerCommitSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	mti := &mocks.TileInfo{}
	defer mti.AssertExpectations(t)

	mti.On("AllCommits").Return(makeSparseCommits())
	mti.On("DataCommits").Return(data.MakeTestCommits())
	mti.On("GetTile", types.ExcludeIgnoredTraces).Return(data.MakeTestTile())

	exp := data.MakeTestExpectations()

	bm, err := baseline.GetBaselinesPerCommit(exp, mti, makeExtraCommits())
	assert.NoError(t, err)

	extraCommits := makeExtraCommits()
	sparseCommits := makeSparseCommits()

	assert.Equal(t, map[string]*baseline.Baseline{
		data.FirstCommitHash: {
			StartCommit:  sparseCommits[0],
			EndCommit:    sparseCommits[0],
			Total:        6,
			Filled:       1,
			Issue:        types.MasterBranch,
			Expectations: betaOnly,
			MD5:          BetaMD5,
		},
		EmptyCommitHash: {
			StartCommit:  sparseCommits[1],
			EndCommit:    sparseCommits[1],
			Total:        6,
			Filled:       1,
			Issue:        types.MasterBranch,
			Expectations: betaOnly,
			MD5:          BetaMD5,
		},
		data.SecondCommitHash: {
			StartCommit:  sparseCommits[2],
			EndCommit:    sparseCommits[2],
			Total:        6,
			Filled:       1,
			Issue:        types.MasterBranch,
			Expectations: betaOnly,
			MD5:          BetaMD5,
		},
		data.ThirdCommitHash: {
			StartCommit:  sparseCommits[3],
			EndCommit:    sparseCommits[3],
			Total:        6,
			Filled:       2,
			Issue:        types.MasterBranch,
			Expectations: both,
			MD5:          BothMD5,
		},
		ExtraCommitHash: {
			StartCommit:  extraCommits[0],
			EndCommit:    extraCommits[0],
			Total:        6,
			Filled:       2,
			Issue:        types.MasterBranch,
			Expectations: both,
			MD5:          BothMD5,
		},
	}, bm)
}

func TestGetBaselineForIssueSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	issue := int64(117)

	// There were 2 tryjobs, one run when the CL had a parent of
	// SecondCommitHash and the other when it was rebased onto
	// ThirdCommitHash.
	tryjobs := []*tryjobstore.Tryjob{
		{
			MasterCommit: data.SecondCommitHash,
		},
		{
			MasterCommit: data.ThirdCommitHash,
		},
	}

	tryjobResults := make([][]*tryjobstore.TryjobResult, len(tryjobs))
	tryjobResults[0] = []*tryjobstore.TryjobResult{
		{
			TestName: data.AlphaTest,
			Digest:   data.AlphaBad1Digest,
		},
		{
			TestName: data.BetaTest,
			Digest:   data.BetaGood1Digest,
		},
	}
	tryjobResults[1] = []*tryjobstore.TryjobResult{
		{
			TestName: data.AlphaTest,
			Digest:   data.AlphaGood1Digest,
		},
		{
			TestName: data.BetaTest,
			Digest:   data.BetaUntriaged1Digest,
		},
	}

	b, err := baseline.GetBaselineForIssue(issue, tryjobs, tryjobResults,
		data.MakeTestExpectations(), makeSparseCommits())
	assert.NoError(t, err)

	sparseCommits := makeSparseCommits()
	assert.Equal(t, &baseline.Baseline{
		StartCommit:  sparseCommits[2],
		EndCommit:    sparseCommits[3],
		Total:        0,
		Filled:       0,
		Issue:        issue,
		Expectations: both,
		MD5:          BothMD5,
	}, b)
}

func makeSparseCommits() []*tiling.Commit {
	// Four commits, with completely arbitrary data.
	return []*tiling.Commit{
		{
			Hash:       data.FirstCommitHash,
			CommitTime: time.Date(2019, time.April, 26, 12, 0, 3, 0, time.UTC).Unix(),
			Author:     data.FirstCommitAuthor,
		},
		{
			Hash:       EmptyCommitHash,
			CommitTime: time.Date(2019, time.April, 26, 12, 5, 5, 0, time.UTC).Unix(),
			Author:     data.FirstCommitAuthor,
		},
		{
			Hash:       data.SecondCommitHash,
			CommitTime: time.Date(2019, time.April, 26, 12, 10, 18, 0, time.UTC).Unix(),
			Author:     data.SecondCommitAuthor,
		},
		{
			Hash:       data.ThirdCommitHash,
			CommitTime: time.Date(2019, time.April, 26, 13, 10, 8, 0, time.UTC).Unix(),
			Author:     data.ThirdCommitAuthor,
		},
	}
}

func makeExtraCommits() []*tiling.Commit {
	// One extra commit, completely arbitrary data.
	return []*tiling.Commit{
		{
			Hash:       ExtraCommitHash,
			CommitTime: time.Date(2019, time.April, 27, 5, 17, 1, 0, time.UTC).Unix(),
			Author:     data.SecondCommitAuthor,
		},
	}
}

const (
	EmptyCommitHash = "eea258b693f2fc53501ac341f3029860b3b57a10"
	ExtraCommitHash = "8181bc0cb0ff1c83e5839aac0e8b13dc0157047a"

	BetaMD5 = "fefa5ea9f0e646258516ed6cecff06c6"
	BothMD5 = "821b673f7f240953aecf3c8fa32d5655"
)

var (
	// For all but the last commit, BetaGood1Digest is the only
	// positive digest seen.
	betaOnly = types.Expectations{
		data.BetaTest: {
			data.BetaGood1Digest: types.POSITIVE,
		},
	}

	// For the last commit, we see AlphaGood1Digest, so it is added
	// to the Expectations.
	both = types.Expectations{
		data.AlphaTest: {
			data.AlphaGood1Digest: types.POSITIVE,
		},
		data.BetaTest: {
			data.BetaGood1Digest: types.POSITIVE,
		},
	}
)
