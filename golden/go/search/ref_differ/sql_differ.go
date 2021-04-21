// Package ref_differ contains routines to calculate reference diffs for digests.
package ref_differ

import (
	"context"
	"math"
	"sort"

	"go.skia.org/infra/go/util"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/search/common"
	"go.skia.org/infra/golden/go/search/frontend"
	"go.skia.org/infra/golden/go/search/query"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/types"
)

type SQLImpl struct {
	sqlDB *pgxpool.Pool
	exp   expectations.Classifier
	idx   indexer.IndexSearcher
}

// NewSQLImpl returns an implementation of FillRefDiffs that pulls diff metrics from
// FillRefDiffs
func NewSQLImpl(sqlDB *pgxpool.Pool, exp expectations.Classifier, idx indexer.IndexSearcher) *SQLImpl {
	return &SQLImpl{
		sqlDB: sqlDB,
		exp:   exp,
		idx:   idx,
	}
}

// FillRefDiffs fills in d with the closest negative and positive images (digests) to it.
// It uses "metric" to determine "closeness". If match is non-nil, it only returns those
// digests that match d's params for the keys in match. If rhsQuery is not empty, it only
// compares against digests that match rhsQuery.
func (s *SQLImpl) FillRefDiffs(ctx context.Context, d *frontend.SearchResult, metric string, match []string, rhsQuery paramtools.ParamSet, iState types.IgnoreState) error {
	paramsByDigest := s.idx.GetParamsetSummaryByTest(iState)[d.Test]

	// TODO(kjlubick) maybe make this use an errgroup
	posDigests := getDigestsWithLabel(s.exp, d, match, paramsByDigest, rhsQuery, expectations.Positive)
	negDigests := getDigestsWithLabel(s.exp, d, match, paramsByDigest, rhsQuery, expectations.Negative)

	var err error
	ret := make(map[common.RefClosest]*frontend.SRDiffDigest, 2)
	ret[common.PositiveRef], err = s.getClosestDiff(ctx, metric, d.Digest, posDigests)
	if err != nil {
		return skerr.Wrapf(err, "fetching positive diffs")
	}
	ret[common.NegativeRef], err = s.getClosestDiff(ctx, metric, d.Digest, negDigests)
	if err != nil {
		return skerr.Wrapf(err, "fetching negative diffs")
	}

	// Find the minimum according to the diff metric.
	closest := common.NoRef
	minDiff := float32(math.Inf(1))
	for ref, srdd := range ret {
		if srdd != nil {
			// Fill in the missing fields.
			srdd.Status = s.exp.Classification(d.Test, srdd.Digest)
			srdd.ParamSet = paramsByDigest[srdd.Digest]

			// Find the minimum.
			if srdd.QueryMetric < minDiff {
				closest = ref
				minDiff = srdd.QueryMetric
			}
		}
	}
	d.ClosestRef = closest
	d.RefDiffs = ret
	return nil
}

// getClosestDiff returns the closest diff between a digest and a set of digest. It uses the
// provided metric as a measure of closeness.
func (s *SQLImpl) getClosestDiff(ctx context.Context, metric string, left types.Digest, rightDigests types.DigestSlice) (*frontend.SRDiffDigest, error) {
	if len(rightDigests) == 0 {
		return nil, nil
	}
	const baseStatement = `SELECT encode(right_digest, 'hex'), num_pixels_diff, percent_pixels_diff, max_rgba_diffs,
combined_metric, dimensions_differ
FROM DiffMetrics AS OF SYSTEM TIME '-0.1s'
WHERE left_digest = $1 AND right_digest IN `
	orderStatement := ` ORDER BY combined_metric ASC LIMIT 1`
	switch metric {
	case query.PercentMetric:
		orderStatement = ` ORDER BY percent_pixels_diff ASC LIMIT 1`
	case query.PixelMetric:
		orderStatement = ` ORDER BY num_pixels_diff ASC LIMIT 1`
	}
	vp := sql.ValuesPlaceholders(len(rightDigests)+1, 1)
	arguments := make([]interface{}, 0, len(rightDigests)+1)
	lb, err := sql.DigestToBytes(left)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	arguments = append(arguments, lb)
	for _, d := range rightDigests {
		b, err := sql.DigestToBytes(d)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		arguments = append(arguments, b)
	}

	row := s.sqlDB.QueryRow(ctx, baseStatement+vp+orderStatement, arguments...)
	rv := frontend.SRDiffDigest{}
	err = row.Scan(&rv.Digest, &rv.NumDiffPixels, &rv.PixelDiffPercent, &rv.MaxRGBADiffs,
		&rv.CombinedMetric, &rv.DimDiffer)
	if err == pgx.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, skerr.Wrapf(err, "finding closest diff using %s and %d right digests", left, len(rightDigests))
	}

	rv.QueryMetric = rv.CombinedMetric
	switch metric {
	case query.PercentMetric:
		rv.QueryMetric = rv.PixelDiffPercent
	case query.PixelMetric:
		rv.QueryMetric = float32(rv.NumDiffPixels)
	}
	return &rv, nil
}

// getDigestsWithLabel return all digests within the given test that
// have the given label assigned to them and where the parameters
// listed in 'match' match.
func getDigestsWithLabel(exp expectations.Classifier, s *frontend.SearchResult, match []string, paramsByDigest map[types.Digest]paramtools.ParamSet, rhsQuery paramtools.ParamSet, targetLabel expectations.Label) types.DigestSlice {
	ret := types.DigestSlice{}
	for d, digestParams := range paramsByDigest {
		// Accept all digests that are:  in the set of allowed digests
		//                              match the target label and where the required
		//                              parameter fields match.
		if (len(rhsQuery) == 0 || rhsQuery.Matches(digestParams)) &&
			(exp.Classification(s.Test, d) == targetLabel) &&
			paramSetsMatch(match, s.ParamSet, digestParams) {
			ret = append(ret, d)
		}
	}
	// Sort for determinism
	sort.Sort(ret)
	return ret
}

// paramSetsMatch returns true if the two param sets have matching values for the parameters listed
// in 'match'. If one of them is nil or completely empty there is always a match.
func paramSetsMatch(match []string, p1, p2 paramtools.ParamSet) bool {
	if len(p1) == 0 || len(p2) == 0 {
		return true
	}

	for _, paramKey := range match {
		vals1, ok1 := p1[paramKey]
		vals2, ok2 := p2[paramKey]
		if !ok1 || !ok2 {
			return false
		}
		found := false
		for _, v := range vals1 {
			if util.In(v, vals2) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
