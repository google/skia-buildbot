// Package search2 encapsulates various queries we make against Gold's data. It is backed
// by the SQL database and aims to replace the current search package.
package search2

import (
	"context"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.opencensus.io/trace"
	"golang.org/x/sync/errgroup"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/golden/go/sql"
)

type API interface {
	// NewAndUntriagedSummaryForCL returns a summarized look at the new digests produced by a CL
	// (that is, digests not currently on the primary branch for this grouping at all) as well as
	// how many of the newly produced digests are currently untriaged.
	NewAndUntriagedSummaryForCL(ctx context.Context, crs, clID string) (NewAndUntriagedSummary, error)

	ChangelistLastUpdated(ctx context.Context, crs, clID string) (time.Time, error)
}

// NewAndUntriagedSummary is a summary of the results associated with a given CL. It focuses on
// the untriaged and new images produced.
type NewAndUntriagedSummary struct {
	// ChangelistID is the nonqualified id of the CL.
	ChangelistID string
	// PatchsetSummaries is a summary for all Patchsets for which we have data.
	PatchsetSummaries []PatchsetNewAndUntriagedSummary
	// LastUpdated returns the timestamp of the CL, which corresponds to the last datapoint for
	// this CL.
	LastUpdated time.Time
}

// PatchsetNewAndUntriagedSummary is the summary for a specific PS. It focuses on the untriaged
// and new images produced.
type PatchsetNewAndUntriagedSummary struct {
	// NewImages is the number of new images (digests) that were produced by this patchset by
	// non-ignored traces and not seen on the primary branch.
	NewImages int
	// NewUntriagedImages is the number of NewImages which are still untriaged. It is less than or
	// equal to NewImages.
	NewUntriagedImages int
	// TotalUntriagedImages is the number of images produced by this patchset by non-ignored traces
	// that are untriaged. This includes images that are untriaged and observed on the primary
	// branch (i.e. might not be the fault of this CL/PS). It is greater than or equal to
	// NewUntriagedImages.
	TotalUntriagedImages int
	// PatchsetID is the nonqualified id of the patchset. This is usually a git hash.
	PatchsetID string
	// PatchsetOrder is represents the chronological order the patchsets are in. It starts at 1.
	PatchsetOrder int
}

type Impl struct {
	db *pgxpool.Pool
}

// New returns an implementation of API.
func New(sqlDB *pgxpool.Pool) *Impl {
	return &Impl{db: sqlDB}
}

// NewAndUntriagedSummaryForCL queries all the patchsets in parallel (to keep the query less
// complex). If there are no patchsets for the provided CL, it returns an error.
func (s *Impl) NewAndUntriagedSummaryForCL(ctx context.Context, crs, clID string) (NewAndUntriagedSummary, error) {
	ctx, span := trace.StartSpan(ctx, "search2_NewAndUntriagedSummaryForCL")
	defer span.End()

	qCLID := sql.Qualify(crs, clID)
	patchsets, err := s.getPatchsets(ctx, qCLID)
	if err != nil {
		return NewAndUntriagedSummary{}, skerr.Wrap(err)
	}
	if len(patchsets) == 0 {
		return NewAndUntriagedSummary{}, skerr.Fmt("CL %q not found", qCLID)
	}

	eg, ctx := errgroup.WithContext(ctx)
	rv := make([]PatchsetNewAndUntriagedSummary, len(patchsets))
	for i, p := range patchsets {
		idx, ps := i, p
		eg.Go(func() error {
			sum, err := s.getSummaryForPS(ctx, qCLID, ps.id)
			if err != nil {
				return skerr.Wrap(err)
			}
			sum.PatchsetID = sql.Unqualify(ps.id)
			sum.PatchsetOrder = ps.order
			rv[idx] = sum
			return nil
		})
	}
	var updatedTS time.Time
	eg.Go(func() error {
		row := s.db.QueryRow(ctx, `SELECT last_ingested_data
FROM Changelists WHERE changelist_id = $1`, qCLID)
		return skerr.Wrap(row.Scan(&updatedTS))
	})
	if err := eg.Wait(); err != nil {
		return NewAndUntriagedSummary{}, skerr.Wrapf(err, "Getting counts for CL %q and %d PS", qCLID, len(patchsets))
	}
	return NewAndUntriagedSummary{
		ChangelistID:      clID,
		PatchsetSummaries: rv,
		LastUpdated:       updatedTS.UTC(),
	}, nil
}

type psIDAndOrder struct {
	id    string
	order int
}

// getPatchsets returns the qualified ids and orders of the patchsets sorted by ps_order.
func (s *Impl) getPatchsets(ctx context.Context, qualifiedID string) ([]psIDAndOrder, error) {
	ctx, span := trace.StartSpan(ctx, "getPatchsets")
	defer span.End()
	rows, err := s.db.Query(ctx, `SELECT patchset_id, ps_order
FROM Patchsets WHERE changelist_id = $1 ORDER BY ps_order ASC`, qualifiedID)
	if err != nil {
		return nil, skerr.Wrapf(err, "getting summary for cl %q", qualifiedID)
	}
	defer rows.Close()
	var rv []psIDAndOrder
	for rows.Next() {
		var row psIDAndOrder
		if err := rows.Scan(&row.id, &row.order); err != nil {
			return nil, skerr.Wrap(err)
		}
		rv = append(rv, row)
	}
	return rv, nil
}

// getSummaryForPS looks at all the data produced for a given PS and returns the a summary of the
// newly produced digests and untriaged digests.
func (s *Impl) getSummaryForPS(ctx context.Context, clid, psID string) (PatchsetNewAndUntriagedSummary, error) {
	ctx, span := trace.StartSpan(ctx, "getSummaryForPS")
	defer span.End()
	const statement = `
WITH
  CLDigests AS (
    SELECT secondary_branch_trace_id, digest, grouping_id
    FROM SecondaryBranchValues
    WHERE branch_name = $1 and version_name = $2
  ),
  NonIgnoredCLDigests AS (
    -- We only want to count a digest once per grouping, no matter how many times it shows up
    -- because group those together (by trace) in the frontend UI.
    SELECT DISTINCT digest, CLDigests.grouping_id
    FROM CLDigests
    JOIN Traces
    ON secondary_branch_trace_id = trace_id
    WHERE Traces.matches_any_ignore_rule = False
  ),
  CLExpectations AS (
    SELECT grouping_id, digest, label
    FROM SecondaryBranchExpectations
    WHERE branch_name = $1
  ),
  NewDigests AS (
    SELECT NonIgnoredCLDigests.digest, NonIgnoredCLDigests.grouping_id
    FROM NonIgnoredCLDigests
    LEFT JOIN TiledTraceDigests
    ON NonIgnoredCLDigests.grouping_id = TiledTraceDigests.grouping_id AND
      NonIgnoredCLDigests.digest = TiledTraceDigests.digest
    WHERE TiledTraceDigests.tile_id IS NULL
  ),
  LabeledNewDigests AS (
    SELECT COALESCE(CLExpectations.label, 'u') as label
    FROM NewDigests
    LEFT JOIN CLExpectations
    ON NewDigests.grouping_id = CLExpectations.grouping_id AND
      NewDigests.digest = CLExpectations.digest
  ),
  LabeledDigests AS (
    SELECT COALESCE(CLExpectations.label, COALESCE(Expectations.label, 'u')) as label
    FROM NonIgnoredCLDigests
    LEFT JOIN Expectations
    ON NonIgnoredCLDigests.grouping_id = Expectations.grouping_id AND
      NonIgnoredCLDigests.digest = Expectations.digest
    LEFT JOIN CLExpectations
    ON NonIgnoredCLDigests.grouping_id = CLExpectations.grouping_id AND
      NonIgnoredCLDigests.digest = CLExpectations.digest
  )
SELECT count(*) as "num_digests", '"new"' as "name"
FROM NewDigests
UNION
SELECT count(*) as "num_digests", '"new_unt"' as "name"
FROM LabeledNewDigests
WHERE label = 'u'
UNION
SELECT count(*) as "num_digests", '"unt"' as "name"
FROM LabeledDigests
WHERE label = 'u'`

	rows, err := s.db.Query(ctx, statement, clid, psID)
	if err != nil {
		return PatchsetNewAndUntriagedSummary{}, skerr.Wrapf(err, "getting summary for ps %q in cl %q", psID, clid)
	}
	defer rows.Close()
	rows.Next()
	var ignore string // to get the union to work, I need to label the values (otherwise they get
	// deduplicated), but I don't actually need to read this in, since it's always in the provided
	// order.
	rv := PatchsetNewAndUntriagedSummary{}
	if err := rows.Scan(&rv.NewImages, &ignore); err != nil {
		return PatchsetNewAndUntriagedSummary{}, skerr.Wrap(err)
	}
	rows.Next()
	if err := rows.Scan(&rv.NewUntriagedImages, &ignore); err != nil {
		return PatchsetNewAndUntriagedSummary{}, skerr.Wrap(err)
	}
	rows.Next()
	if err := rows.Scan(&rv.TotalUntriagedImages, &ignore); err != nil {
		return PatchsetNewAndUntriagedSummary{}, skerr.Wrap(err)
	}
	return rv, nil
}

// ChangelistLastUpdated implements the API interface.
func (s *Impl) ChangelistLastUpdated(ctx context.Context, crs, clID string) (time.Time, error) {
	ctx, span := trace.StartSpan(ctx, "search2_ChangelistLastUpdated")
	defer span.End()
	qCLID := sql.Qualify(crs, clID)
	var updatedTS time.Time
	row := s.db.QueryRow(ctx, `SELECT last_ingested_data
FROM Changelists AS OF SYSTEM TIME '-0.1s' WHERE changelist_id = $1`, qCLID)
	if err := row.Scan(&updatedTS); err != nil {
		return time.Time{}, skerr.Wrapf(err, "Getting last updated ts for cl %q", qCLID)
	}
	return updatedTS.UTC(), nil
}

// Make sure Impl implements the API interface.
var _ API = (*Impl)(nil)
