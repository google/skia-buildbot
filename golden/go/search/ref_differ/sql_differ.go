package ref_differ

import (
	"context"
	"math"

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

func NewSQLImpl(sqlDB *pgxpool.Pool, exp expectations.Classifier, idx indexer.IndexSearcher) *SQLImpl {
	return &SQLImpl{
		sqlDB: sqlDB,
		exp:   exp,
		idx:   idx,
	}
}

// FillRefDiffs implements the RefDiffer interface by querying a SQL database.
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
	dCount := s.idx.DigestCountsByTest(iState)[d.Test]
	for ref, srdd := range ret {
		if srdd != nil {
			// Fill in the missing fields.
			srdd.Status = s.exp.Classification(d.Test, srdd.Digest)
			srdd.ParamSet = paramsByDigest[srdd.Digest]
			srdd.OccurrencesInTile = dCount[srdd.Digest]

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
