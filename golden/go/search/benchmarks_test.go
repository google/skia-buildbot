package search

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/golden/go/clstore"
	mock_clstore "go.skia.org/infra/golden/go/clstore/mocks"
	"go.skia.org/infra/golden/go/code_review"
	"go.skia.org/infra/golden/go/expectations"
	mock_index "go.skia.org/infra/golden/go/indexer/mocks"
	"go.skia.org/infra/golden/go/search/query"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/tjstore"
	mock_tjstore "go.skia.org/infra/golden/go/tjstore/mocks"
	"go.skia.org/infra/golden/go/types"
)

const (
	// These consts were arbitrarily picked to approximate a representative Skia Changelist
	numResultParams  = 6
	numOptionsParams = 8
	numGroupParams   = 12
	numIgnoreRules   = 100
	// numIgnorableValues can be tuned higher to ignore fewer values and lower to ignore more.
	// Right now, this value ignores 10% on average in BenchmarkExtractChangelistDigests
	numIgnorableValues = 500
)

// BenchmarkExtractChangelistDigests benchmarks extractChangelistDigests, specifically
// focusing on the filtering logic after the TryJobResults are returned.
func BenchmarkExtractChangelistDigests(b *testing.B) {
	const gerritCRS = "gerrit"
	mis := &mock_index.IndexSearcher{}
	mcls := &mock_clstore.Store{}
	mtjs := &mock_tjstore.Store{}

	clID := "123"
	psOrder := 1
	// 500k was observed as a typical number of results for Skia, as were 15 unique options and
	// 15 unique groups.
	xtr := genTryJobResults(500000, 15, 15)

	mcls.On("GetPatchsets", testutils.AnyContext, clID).Return([]code_review.Patchset{
		{
			SystemID:     "first_one",
			ChangelistID: clID,
			Order:        psOrder,
			// All the rest are ignored
		},
	}, nil)

	mis.On("GetIgnoreMatcher").Return(makeIgnoreRules())
	combinedID := tjstore.CombinedPSID{CL: "123", CRS: "gerrit", PS: "first_one"}
	// return a copy of the slice so we can mess around with the data however we like.
	mtjs.On("GetResults", testutils.AnyContext, combinedID).Return(func(context.Context, tjstore.CombinedPSID) []tjstore.TryJobResult {
		c := make([]tjstore.TryJobResult, len(xtr))
		copy(c, xtr)
		return c
	}, nil)

	reviewSystems := []clstore.ReviewSystem{
		{
			ID:    gerritCRS,
			Store: mcls,
			// Client and URLTemplate are unused here
		},
	}

	s := &SearchImpl{
		reviewSystems: reviewSystems,
		tryJobStore:   mtjs,
	}

	fn := func(_ types.TestName, _ types.Digest, _ paramtools.Params, _ tiling.TracePair) {}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		err := s.extractChangelistDigests(context.Background(), &query.Search{
			Patchsets:               []int64{int64(psOrder)},
			TraceValues:             map[string][]string{},
			IncludeUntriagedDigests: true,
			CodeReviewSystemID:      "gerrit",
			ChangelistID:            clID,
		}, mis, expectations.EmptyClassifier(), fn)
		require.NoError(b, err)
	}
}

// ignorableFields and ignorableValues allow us to have somewhat random inputs that can be
// re-used and thus matched upon by ignores.
var ignorableFields = []string{types.PrimaryKeyField, "config", "gpu", "os", "flavor", "smell", "weight"}
var ignorableValues []string = nil

func init() {
	for i := 0; i < numIgnorableValues; i++ {
		ignorableValues = append(ignorableValues, randValue())
	}
}

// makeIgnoreRules makes a set of synthetic ignore rules that approximately represents
// those found in Skia. It uses ignorableFields and ignorableValues to have a mix of things
// that could possibly line up with the values in genTryJobResults.
func makeIgnoreRules() paramtools.ParamMatcher {
	var pm paramtools.ParamMatcher
	for i := 0; i < numIgnoreRules; i++ {
		// 1-2 fields
		numFields := rand.Intn(2) + 1
		p := paramtools.ParamSet{}
		fieldPerm := rand.Perm(len(ignorableFields))
		for _, r := range fieldPerm[:numFields] {
			f := ignorableFields[r]

			// 1-4 values
			numValues := rand.Intn(4) + 1
			var v []string
			fieldPerm := rand.Perm(len(ignorableFields))
			for _, r := range fieldPerm[:numValues] {
				v = append(v, ignorableValues[r])
			}
			p[f] = v
		}
		pm = append(pm, p)
	}
	return pm
}

// genTryJobResults makes TryJobResults with synthetic data that approximately represents
// the data created by Skia. The data is structured such that some (~10%) of the results will
// be ignored by makeIgnoreRules(), but the vast majority will not. Additionally, the number of
// fields in the various Params is generally representative.
func genTryJobResults(results, uniqueOptions, uniqueGroups int) []tjstore.TryJobResult {
	opts := make([]paramtools.Params, 0, uniqueOptions)
	for i := 0; i < uniqueOptions; i++ {
		opts = append(opts, makeParams(numOptionsParams))
	}
	groups := make([]paramtools.Params, 0, uniqueGroups)
	for i := 0; i < uniqueGroups; i++ {
		groups = append(groups, makeParams(numGroupParams))
	}
	xtr := make([]tjstore.TryJobResult, 0, results)
	for i := 0; i < results; i++ {
		o := rand.Intn(uniqueOptions)
		g := rand.Intn(uniqueGroups)
		xtr = append(xtr, tjstore.TryJobResult{
			GroupParams:  groups[g],
			Options:      opts[o],
			ResultParams: makeResults(),
			Digest:       types.Digest(randValue()),
		})
	}

	return xtr
}

// makeParams makes a Params that is a blend of ignorable values, and non-ignorable values.
func makeParams(n int) paramtools.Params {
	p := paramtools.Params{}
	numIgnorables := 3
	for i := 0; len(p) < n && i < numIgnorables; i++ {
		r := rand.Intn(len(ignorableFields))
		f := ignorableFields[r]
		// don't have name for these - that should be in result params
		if f == types.PrimaryKeyField {
			f = ignorableFields[1]
		}
		r = rand.Intn(len(ignorableValues))
		v := ignorableValues[r]
		p[f] = v
	}

	numMaybes := 2
	for i := 0; len(p) < n && i < numMaybes; i++ {
		r := rand.Intn(len(ignorableFields))
		f := ignorableFields[r]
		// don't have name for these - that should be in result params
		if f == types.PrimaryKeyField {
			f = ignorableFields[1]
		}
		p[f] = randValue()
	}

	for len(p) < n {
		p[randValue()] = randValue()
	}
	return p
}

// makeResults returns a Params similar to those in Skia results. They always have a name.
// The rest of the fields are possibly ignorable.
func makeResults() paramtools.Params {
	p := paramtools.Params{}
	if rand.Float32() < 0.5 {
		p[types.PrimaryKeyField] = randValue()
	} else {
		r := rand.Intn(len(ignorableValues))
		p[types.PrimaryKeyField] = ignorableValues[r]
	}
	numIgnorables := 2
	for i := 1; len(p) < numResultParams && i < numIgnorables; i++ {
		r := rand.Intn(len(ignorableFields))
		f := ignorableFields[r]
		// don't have multiple names
		if f == types.PrimaryKeyField {
			f = ignorableFields[2]
		}
		r = rand.Intn(len(ignorableValues))
		v := ignorableValues[r]
		p[f] = v
	}

	for len(p) < numResultParams {
		r := rand.Intn(len(ignorableFields))
		f := ignorableFields[r]
		// don't have multiple names
		if f == types.PrimaryKeyField {
			f = ignorableFields[2]
		}
		p[f] = randValue()
	}

	return p
}

// randValue returns a random string. It happens to be a hex encoded Digest, but can be used
// any place a random medium-length string is needed.
func randValue() string {
	b := make([]byte, md5.Size)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
