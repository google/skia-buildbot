// Package sqlclstore contains a SQL implementation of a clstore.Store.
package sqlclstore

import (
	"context"

	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.opencensus.io/trace"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/clstore"
	"go.skia.org/infra/golden/go/code_review"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/schema"
)

type StoreImpl struct {
	db       *pgxpool.Pool
	systemID string
}

// New returns a SQL-backed clstore.Store for the given system.
func New(db *pgxpool.Pool, systemID string) *StoreImpl {
	return &StoreImpl{
		db:       db,
		systemID: systemID,
	}
}

// We don't have an enterprise license and don't benefit from follower reads.
// https://www.cockroachlabs.com/docs/stable/follower-reads.html
// If we did, we would want the AS OF SYSTEM TIME to be 4.8 seconds (or 5 for ease).
// Setting a non-zero delay reduces some contention because the query doesn't have to
// be retried if data is written while the query is being executed (which is common for
// instances that stream in results).
const statementAll = `
SELECT changelist_id, status, owner_email, subject, last_ingested_data
FROM Changelists AS OF SYSTEM TIME '-0.1s'
WHERE system = $1 and last_ingested_data > $2
ORDER BY last_ingested_data DESC
LIMIT $3 OFFSET $4
`

const statementOpenOnly = `
SELECT changelist_id, status, owner_email, subject, last_ingested_data
FROM Changelists AS OF SYSTEM TIME '-0.1s'
WHERE system = $1 and last_ingested_data > $2 and status = 'open'
ORDER BY last_ingested_data DESC
LIMIT $3 OFFSET $4
`

// GetChangelists implements clstore.Store.
func (s *StoreImpl) GetChangelists(ctx context.Context, opts clstore.SearchOptions) ([]code_review.Changelist, int, error) {
	ctx, span := trace.StartSpan(ctx, "sqlclstore_GetChangelists")
	defer span.End()
	if opts.Limit <= 0 {
		return nil, 0, skerr.Fmt("must supply a limit")
	}
	if opts.StartIdx < 0 {
		return nil, 0, skerr.Fmt("start index must be positive")
	}
	statement := statementAll
	if opts.OpenCLsOnly {
		statement = statementOpenOnly
	}
	rows, err := s.db.Query(ctx, statement, s.systemID, opts.After, opts.Limit, opts.StartIdx)
	if err != nil {
		return nil, -1, skerr.Wrapf(err, "querying for options %s - %#v", s.systemID, opts)
	}
	defer rows.Close()
	var rv []code_review.Changelist
	for rows.Next() {
		var r schema.ChangelistRow
		err := rows.Scan(&r.ChangelistID, &r.Status, &r.OwnerEmail, &r.Subject, &r.LastIngestedData)
		if err != nil {
			return nil, -1, skerr.Wrapf(err, "Scanning data for changelists %s - %#v", s.systemID, opts)
		}
		rv = append(rv, code_review.Changelist{
			SystemID: sql.Unqualify(r.ChangelistID),
			Owner:    r.OwnerEmail,
			Status:   convertToStatusEnum(r.Status),
			Subject:  r.Subject,
			Updated:  r.LastIngestedData.UTC(),
		})
	}

	const totalStatement = `SELECT count(*) FROM Changelists WHERE system = $1`
	var total int
	countRow := s.db.QueryRow(ctx, totalStatement, s.systemID)
	if err := countRow.Scan(&total); err != nil {
		return nil, -1, skerr.Wrapf(err, "counting changelists")
	}
	return rv, total, nil
}

// GetChangelist implements clstore.Store.
func (s *StoreImpl) GetChangelist(ctx context.Context, id string) (code_review.Changelist, error) {
	qID := sql.Qualify(s.systemID, id)
	row := s.db.QueryRow(ctx, `
SELECT status, owner_email, subject, last_ingested_data FROM Changelists WHERE changelist_id = $1`, qID)
	var r schema.ChangelistRow
	err := row.Scan(&r.Status, &r.OwnerEmail, &r.Subject, &r.LastIngestedData)
	if err != nil {
		if err == pgx.ErrNoRows {
			sklog.Warningf("No SQL CL found for changelist_id = %q", qID)
			return code_review.Changelist{}, clstore.ErrNotFound
		}
		return code_review.Changelist{}, skerr.Wrapf(err, "querying for id %s", qID)
	}
	return code_review.Changelist{
		SystemID: id,
		Owner:    r.OwnerEmail,
		Status:   convertToStatusEnum(r.Status),
		Subject:  r.Subject,
		Updated:  r.LastIngestedData.UTC(),
	}, nil
}

// PutChangelist implements clstore.Store.
func (s *StoreImpl) PutChangelist(ctx context.Context, cl code_review.Changelist) error {
	qID := sql.Qualify(s.systemID, cl.SystemID)
	const statement = `
UPSERT INTO Changelists (changelist_id, system, status, owner_email, subject, last_ingested_data)
VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := s.db.Exec(ctx, statement, qID, s.systemID, convertFromStatusEnum(cl.Status), cl.Owner, cl.Subject, cl.Updated)
	if err != nil {
		return skerr.Wrapf(err, "Inserting CL %#v", cl)
	}
	return nil
}

// GetPatchsets implements clstore.Store.
func (s *StoreImpl) GetPatchsets(ctx context.Context, clID string) ([]code_review.Patchset, error) {
	qID := sql.Qualify(s.systemID, clID)
	rows, err := s.db.Query(ctx, `
SELECT patchset_id, ps_order, git_hash, commented_on_cl, created_ts
FROM Patchsets WHERE changelist_id = $1 ORDER BY created_ts DESC, ps_order ASC, git_hash ASC`, qID)
	if err != nil {
		return nil, skerr.Wrapf(err, "querying for cl %s", qID)
	}
	defer rows.Close()
	var rv []code_review.Patchset
	for rows.Next() {
		var r schema.PatchsetRow
		var created pgtype.Timestamptz
		err := rows.Scan(&r.PatchsetID, &r.Order, &r.GitHash, &r.CommentedOnCL, &created)
		if err != nil {
			return nil, skerr.Wrapf(err, "Scanning data for cl %s", qID)
		}
		ps := code_review.Patchset{
			SystemID:      sql.Unqualify(r.PatchsetID),
			ChangelistID:  clID,
			Order:         r.Order,
			GitHash:       r.GitHash,
			CommentedOnCL: r.CommentedOnCL,
		}
		if created.Status == pgtype.Present {
			ps.Created = created.Time.UTC()
		}
		rv = append(rv, ps)
	}
	return rv, nil
}

// GetPatchset implements clstore.Store.
func (s *StoreImpl) GetPatchset(ctx context.Context, _, psID string) (code_review.Patchset, error) {
	qID := sql.Qualify(s.systemID, psID)
	row := s.db.QueryRow(ctx, `
SELECT changelist_id, ps_order, git_hash, commented_on_cl, created_ts
FROM Patchsets WHERE patchset_id = $1`, qID)
	var r schema.PatchsetRow
	var created pgtype.Timestamptz
	err := row.Scan(&r.ChangelistID, &r.Order, &r.GitHash, &r.CommentedOnCL, &created)
	if err != nil {
		if err == pgx.ErrNoRows {
			return code_review.Patchset{}, clstore.ErrNotFound
		}
		return code_review.Patchset{}, skerr.Wrapf(err, "querying for id %s", qID)
	}
	ps := code_review.Patchset{
		SystemID:      psID,
		ChangelistID:  sql.Unqualify(r.ChangelistID),
		Order:         r.Order,
		GitHash:       r.GitHash,
		CommentedOnCL: r.CommentedOnCL,
	}
	if created.Status == pgtype.Present {
		ps.Created = created.Time.UTC()
	}
	return ps, nil
}

// GetPatchsetByOrder implements clstore.Store.
func (s *StoreImpl) GetPatchsetByOrder(ctx context.Context, clID string, psOrder int) (code_review.Patchset, error) {
	qID := sql.Qualify(s.systemID, clID)
	// Add the limit 1 to work around issues with git force pushing (skbug.com/12093)
	row := s.db.QueryRow(ctx, `
SELECT patchset_id, git_hash, commented_on_cl, created_ts
FROM Patchsets WHERE changelist_id = $1 AND ps_order = $2
ORDER BY created_ts DESC, ps_order ASC, git_hash ASC LIMIT 1`, qID, psOrder)
	var r schema.PatchsetRow
	var created pgtype.Timestamptz
	err := row.Scan(&r.PatchsetID, &r.GitHash, &r.CommentedOnCL, &created)
	if err != nil {
		if err == pgx.ErrNoRows {
			return code_review.Patchset{}, clstore.ErrNotFound
		}
		return code_review.Patchset{}, skerr.Wrapf(err, "querying for order %s-%d", qID, psOrder)
	}
	ps := code_review.Patchset{
		SystemID:      sql.Unqualify(r.PatchsetID),
		ChangelistID:  clID,
		Order:         psOrder,
		GitHash:       r.GitHash,
		CommentedOnCL: r.CommentedOnCL,
	}
	if created.Status == pgtype.Present {
		ps.Created = created.Time.UTC()
	}
	return ps, nil
}

// PutPatchset implements clstore.Store. Note that due to foreign key constraints, it will fail
// if the Changelist does not already exist.
func (s *StoreImpl) PutPatchset(ctx context.Context, ps code_review.Patchset) error {
	psID := sql.Qualify(s.systemID, ps.SystemID)
	clID := sql.Qualify(s.systemID, ps.ChangelistID)
	const statement = `
UPSERT INTO Patchsets (patchset_id, system, changelist_id, ps_order, git_hash,
  commented_on_cl, created_ts)
VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := s.db.Exec(ctx, statement, psID, s.systemID, clID, ps.Order, ps.GitHash,
		ps.CommentedOnCL, ps.Created)
	if err != nil {
		return skerr.Wrapf(err, "Inserting PS %#v", ps)
	}
	return nil
}

func convertToStatusEnum(status schema.ChangelistStatus) code_review.CLStatus {
	switch status {
	case schema.StatusAbandoned:
		return code_review.Abandoned
	case schema.StatusOpen:
		return code_review.Open
	case schema.StatusLanded:
		return code_review.Landed
	}
	sklog.Warningf("Unknown status: %s", status)
	return code_review.Abandoned
}

func convertFromStatusEnum(status code_review.CLStatus) schema.ChangelistStatus {
	switch status {
	case code_review.Abandoned:
		return schema.StatusAbandoned
	case code_review.Open:
		return schema.StatusOpen
	case code_review.Landed:
		return schema.StatusLanded
	}
	sklog.Warningf("Unknown status: %d", status)
	return schema.StatusAbandoned
}

// Make sure StoreImpl fulfills the clstore.Store interface.
var _ clstore.Store = (*StoreImpl)(nil)
