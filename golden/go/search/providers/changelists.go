package providers

import (
	"context"
	"sync"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/golden/go/search/common"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/schema"
	"golang.org/x/sync/errgroup"
)

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

type psIDAndOrder struct {
	id    string
	order int
}

// ChangelistProvider provides a struct for get changelist related data for search.
type ChangelistProvider struct {
	db    *pgxpool.Pool
	mutex sync.RWMutex

	// This caches the digests seen per grouping on the primary branch.
	digestsOnPrimary map[common.GroupingDigestKey]struct{}
}

// NewChangelistProvider returns a new instance of the ChangelistProvider.
func NewChangelistProvider(db *pgxpool.Pool) *ChangelistProvider {
	return &ChangelistProvider{
		db: db,
	}
}

// SetDigestsOnPrimary sets the primary digest cache on the provider.
func (s *ChangelistProvider) SetDigestsOnPrimary(digestsOnPrimary map[common.GroupingDigestKey]struct{}) {
	s.digestsOnPrimary = digestsOnPrimary
}

// GetNewAndUntriagedSummaryForCL returns the new and untriaged patchset summaries for the given CL.
func (s *ChangelistProvider) GetNewAndUntriagedSummaryForCL(ctx context.Context, clID string) ([]PatchsetNewAndUntriagedSummary, error) {
	patchsets, err := s.getPatchsets(ctx, clID)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	if len(patchsets) == 0 {
		return nil, skerr.Fmt("CL %q not found", clID)
	}

	eg, ctx := errgroup.WithContext(ctx)
	rv := make([]PatchsetNewAndUntriagedSummary, len(patchsets))
	for i, p := range patchsets {
		idx, ps := i, p
		eg.Go(func() error {
			sum, err := s.getSummaryForPS(ctx, clID, ps.id)
			if err != nil {
				return skerr.Wrap(err)
			}
			sum.PatchsetID = sql.Unqualify(ps.id)
			sum.PatchsetOrder = ps.order
			rv[idx] = sum
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return []PatchsetNewAndUntriagedSummary{}, skerr.Wrapf(err, "Error getting patchset summaries")
	}

	return rv, nil
}

// getPatchsets returns the qualified ids and orders of the patchsets sorted by ps_order.
func (s *ChangelistProvider) getPatchsets(ctx context.Context, qualifiedID string) ([]psIDAndOrder, error) {
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
func (s *ChangelistProvider) getSummaryForPS(ctx context.Context, clid, psID string) (PatchsetNewAndUntriagedSummary, error) {
	ctx, span := trace.StartSpan(ctx, "getSummaryForPS")
	defer span.End()
	const statement = `WITH
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
LabeledDigests AS (
	SELECT NonIgnoredCLDigests.grouping_id, NonIgnoredCLDigests.digest, COALESCE(CLExpectations.label, COALESCE(Expectations.label, 'u')) as label
	FROM NonIgnoredCLDigests
	LEFT JOIN Expectations
	ON NonIgnoredCLDigests.grouping_id = Expectations.grouping_id AND
		NonIgnoredCLDigests.digest = Expectations.digest
	LEFT JOIN CLExpectations
	ON NonIgnoredCLDigests.grouping_id = CLExpectations.grouping_id AND
		NonIgnoredCLDigests.digest = CLExpectations.digest
)
SELECT * FROM LabeledDigests;`

	rows, err := s.db.Query(ctx, statement, clid, psID)
	if err != nil {
		return PatchsetNewAndUntriagedSummary{}, skerr.Wrapf(err, "getting summary for ps %q in cl %q", psID, clid)
	}
	defer rows.Close()
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	var digest schema.DigestBytes
	var grouping schema.GroupingID
	var label schema.ExpectationLabel
	var key common.GroupingDigestKey
	keyGrouping := key.GroupingID[:]
	keyDigest := key.Digest[:]
	var rv PatchsetNewAndUntriagedSummary

	for rows.Next() {
		if err := rows.Scan(&grouping, &digest, &label); err != nil {
			return PatchsetNewAndUntriagedSummary{}, skerr.Wrap(err)
		}
		copy(keyGrouping, grouping)
		copy(keyDigest, digest)
		_, isExisting := s.digestsOnPrimary[key]
		if !isExisting {
			rv.NewImages++
		}
		if label == schema.LabelUntriaged {
			rv.TotalUntriagedImages++
			if !isExisting {
				rv.NewUntriagedImages++
			}
		}
	}
	return rv, nil
}
