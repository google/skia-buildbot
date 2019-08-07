package baseline_test

import (
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/baseline"
	data "go.skia.org/infra/golden/go/testutils/data_three_devices"
	"go.skia.org/infra/golden/go/tryjobstore"
	"go.skia.org/infra/golden/go/types"
)

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

	assert.Equal(t, &baseline.Baseline{
		Issue:        issue,
		Expectations: both,
		MD5:          bothMD5,
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
			Hash:       emptyCommitHash,
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

const (
	emptyCommitHash = "eea258b693f2fc53501ac341f3029860b3b57a10"

	bothMD5 = "821b673f7f240953aecf3c8fa32d5655"
)

var (

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
