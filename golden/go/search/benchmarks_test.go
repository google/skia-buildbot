package search

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"math/rand"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils"
	mock_clstore "go.skia.org/infra/golden/go/clstore/mocks"
	"go.skia.org/infra/golden/go/code_review"
	mock_index "go.skia.org/infra/golden/go/indexer/mocks"
	"go.skia.org/infra/golden/go/search/common"
	"go.skia.org/infra/golden/go/search/query"
	"go.skia.org/infra/golden/go/tjstore"
	mock_tjstore "go.skia.org/infra/golden/go/tjstore/mocks"
	"go.skia.org/infra/golden/go/types"
)

const (
	numResultParams  = 6
	numOptionsParams = 8
	numGroupParams   = 12
	numIgnoreRules   = 100
	numTests         = 1000
)

func BenchmarkExtractChangeListDigests(b *testing.B) {
	mis := &mock_index.IndexSearcher{}
	mcls := &mock_clstore.Store{}
	mtjs := &mock_tjstore.Store{}

	clID := "123"
	psOrder := 1
	xtr := genTryJobResults(5000, 30, 1)

	mcls.On("GetPatchSets", testutils.AnyContext, clID).Return([]code_review.PatchSet{
		{
			SystemID:     "first_one",
			ChangeListID: clID,
			Order:        psOrder,
			// All the rest are ignored
		},
	}, nil)
	mcls.On("System").Return("gerrit")

	mis.On("GetIgnoreMatcher").Return(makeIgnoreRules())
	combinedID := tjstore.CombinedPSID{CL: "123", CRS: "gerrit", PS: "first_one"}
	// return a copy of the slice so we can mess around with the data however we like.
	mtjs.On("GetResults", testutils.AnyContext, combinedID).Return(func(context.Context, tjstore.CombinedPSID) []tjstore.TryJobResult {
		c := make([]tjstore.TryJobResult, len(xtr))
		copy(c, xtr)
		return c
	}, nil)

	s := &SearchImpl{
		changeListStore: mcls,
		tryJobStore:     mtjs,
	}

	fn := func(test types.TestName, digest types.Digest, params paramtools.Params) {}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		err := s.extractChangeListDigests(context.Background(), &query.Search{
			PatchSets:    []int64{int64(psOrder)},
			TraceValues:  map[string][]string{},
			Unt:          true,
			ChangeListID: clID,
		}, mis, common.ExpSlice{types.Expectations{}}, fn)
		assert.NoError(b, err)
	}
}

var ignorableFields = []string{"name", "config", "gpu", "os", "flavor", "smell", "weight"}
var ignorableValues []string = nil

func init() {
	for i := 0; i < 1000; i++ {
		ignorableValues = append(ignorableValues, randValue())
	}
}

func randValue() string {
	b := make([]byte, md5.Size)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func makeIgnoreRules() paramtools.ParamMatcher {
	var pm paramtools.ParamMatcher
	for i := 0; i < numIgnoreRules; i++ {
		// 1-3 fields
		numFields := rand.Intn(2) + 1
		p := paramtools.ParamSet{}
		for j := 0; j < numFields; j++ {
			r := rand.Intn(len(ignorableFields))
			f := ignorableFields[r]

			// 1-5 values
			numValues := rand.Intn(4) + 1
			var v []string
			for k := 0; k < numValues; k++ {
				r := rand.Intn(len(ignorableValues))
				v = append(v, ignorableValues[r])
			}
			p[f] = v
		}
		pm = append(pm, p)
	}
	return pm
}

func makeParams(n int) paramtools.Params {
	p := paramtools.Params{}
	numIgnorables := 3
	for i := 0; len(p) < n && i < numIgnorables; i++ {
		r := rand.Intn(len(ignorableFields))
		f := ignorableFields[r]
		// don't have name for these - that should be in result params
		if f == "name" {
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
		if f == "name" {
			f = ignorableFields[1]
		}
		p[f] = randValue()
	}

	for len(p) < n {
		p[randValue()] = randValue()
	}
	return p
}

func makeResults() paramtools.Params {
	p := paramtools.Params{}
	if rand.Float32() < 0.5 {
		p["name"] = randValue()
	} else {
		r := rand.Intn(len(ignorableValues))
		p["name"] = ignorableValues[r]
	}
	numIgnorables := 2
	for i := 1; len(p) < numResultParams && i < numIgnorables; i++ {
		r := rand.Intn(len(ignorableFields))
		f := ignorableFields[r]
		// don't have name for these - that should be in result params
		if f == "name" {
			f = ignorableFields[2]
		}
		r = rand.Intn(len(ignorableValues))
		v := ignorableValues[r]
		p[f] = v
	}

	for len(p) < numResultParams {
		r := rand.Intn(len(ignorableFields))
		f := ignorableFields[r]
		// don't have name for these - that should be in result params
		if f == "name" {
			f = ignorableFields[2]
		}
		p[f] = randValue()
	}

	return p
}

func genTryJobResults(results, uniqueOptions, uniqueGroups int) []tjstore.TryJobResult {
	opts := make([]paramtools.Params, 0, uniqueOptions)
	for i := 0; i < uniqueOptions; i++ {
		opts = append(opts, makeParams(numOptionsParams))
	}
	groups := make([]paramtools.Params, 0, uniqueGroups)
	for i := 0; i < uniqueOptions; i++ {
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
