package baseline_test

import (
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/baseline"
	data "go.skia.org/infra/golden/go/testutils/data_three_devices"
	"go.skia.org/infra/golden/go/tryjobstore"
	"go.skia.org/infra/golden/go/types"
)

func TestGetBaselineForIssueSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	clID := "117"
	crs := "github"

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

	b, err := baseline.GetBaselineForCL(clID, crs, tryjobs, tryjobResults, data.MakeTestExpectations())
	assert.NoError(t, err)

	assert.Equal(t, &baseline.Baseline{
		Expectations:     both,
		MD5:              bothMD5,
		CodeReviewSystem: crs,
		ChangeListID:     clID,
	}, b)
}

const (
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
