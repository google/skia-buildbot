// Package search2 encapsulates various queries we make against Gold's data. It is backed
// by the SQL database and aims to replace the current search package.
package search2

import (
	"context"
	"sort"

	"go.skia.org/infra/golden/go/sql/schema"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/golden/go/sql"

	"go.opencensus.io/trace"

	"github.com/jackc/pgx/v4/pgxpool"
)

type API interface {
	// NewAndUntriagedSummaryForCL returns a summarized look at the new digests produced by a CL
	// (that is, digests not currently on the primary branch for this grouping at all) as well as
	// how many of the newly produced digests are currently untriaged.
	NewAndUntriagedSummaryForCL(ctx context.Context, crs, clID string) (NewAndUntriagedSummary, error)
}

type NewAndUntriagedSummary struct {
	ChangelistID      string
	PatchsetSummaries []PatchsetNewAndUntriagedSummary
}

type PatchsetNewAndUntriagedSummary struct {
	PatchsetNewImages          int
	PatchsetNewUntriagedImages int

	PatchsetID    string
	PatchsetOrder int
}

type Impl struct {
	db *pgxpool.Pool
}

func New(sqlDB *pgxpool.Pool) *Impl {
	return &Impl{db: sqlDB}
}

func (s *Impl) NewAndUntriagedSummaryForCL(ctx context.Context, crs, clID string) (NewAndUntriagedSummary, error) {
	ctx, span := trace.StartSpan(ctx, "search2_NewAndUntriagedSummaryForCL")
	defer span.End()

	const statement = `
WITH
CLDigests AS (
  SELECT secondary_branch_trace_id, version_name, digest, grouping_id
  FROM SecondaryBranchValues
  WHERE branch_name = $1
), CLExpectations AS (
  SELECT grouping_id, digest, label
  FROM SecondaryBranchExpectations
  WHERE branch_name = $1
), NewDigests AS (
  SELECT DISTINCT CLDigests.version_name, CLDigests.digest, CLDigests.grouping_id
  FROM CLDigests
  LEFT JOIN TiledTraceDigests
  ON CLDigests.secondary_branch_trace_id = TiledTraceDigests.trace_id AND
    CLDigests.digest = TiledTraceDigests.digest
  WHERE TiledTraceDigests.tile_id IS NULL
), LabeledDigests AS (
  SELECT NewDigests.version_name, COALESCE(CLExpectations.label, 'u') as label
  FROM NewDigests
  LEFT JOIN CLExpectations
  ON NewDigests.grouping_id = CLExpectations.grouping_id AND
    NewDigests.digest = CLExpectations.digest
)
SELECT LabeledDigests.version_name, Patchsets.ps_order, LabeledDigests.label
From LabeledDigests
JOIN Patchsets
ON LabeledDigests.version_name = Patchsets.patchset_id;`

	qCLID := sql.Qualify(crs, clID)
	rows, err := s.db.Query(ctx, statement, qCLID)
	if err != nil {
		return NewAndUntriagedSummary{}, skerr.Wrapf(err, "getting summary for cl %q", qCLID)
	}
	defer rows.Close()
	patchsets := map[string]PatchsetNewAndUntriagedSummary{}
	for rows.Next() {
		var psID string
		var psOrder int
		var label schema.ExpectationLabel
		if err := rows.Scan(&psID, &psOrder, &label); err != nil {
			return NewAndUntriagedSummary{}, skerr.Wrap(err)
		}
		summary := patchsets[psID]
		summary.PatchsetNewImages++
		if label == schema.LabelUntriaged {
			summary.PatchsetNewUntriagedImages++
		}
		summary.PatchsetID = sql.Unqualify(psID)
		summary.PatchsetOrder = psOrder
		patchsets[psID] = summary
	}

	rv := NewAndUntriagedSummary{
		ChangelistID:      clID,
		PatchsetSummaries: make([]PatchsetNewAndUntriagedSummary, 0, len(patchsets)),
	}
	for _, s := range patchsets {
		rv.PatchsetSummaries = append(rv.PatchsetSummaries, s)
	}
	sort.Slice(rv.PatchsetSummaries, func(i, j int) bool {
		return rv.PatchsetSummaries[i].PatchsetOrder < rv.PatchsetSummaries[j].PatchsetOrder
	})
	return rv, nil
}

// Make sure Impl fulfills the API interface.
var _ API = (*Impl)(nil)
