package sqlclstore

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v4"

	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/go/skerr"

	"github.com/jackc/pgx/v4/pgxpool"

	"go.skia.org/infra/golden/go/clstore"
	"go.skia.org/infra/golden/go/code_review"
	"go.skia.org/infra/golden/go/sql/schema"
)

type StoreImpl struct {
	db       *pgxpool.Pool
	systemID string
}

func New(db *pgxpool.Pool, systemID string) *StoreImpl {
	return &StoreImpl{db: db, systemID: systemID}
}

func (s StoreImpl) GetChangelist(ctx context.Context, id string) (code_review.Changelist, error) {
	qID := qualify(s.systemID, id)
	row := s.db.QueryRow(ctx, `SELECT * FROM Changelists WHERE changelist_id = $1`, qID)
	var r schema.ChangelistRow
	err := row.Scan(&r.ChangelistID, &r.System, &r.Status, &r.OwnerEmail, &r.Subject, &r.LastIngestedData)
	if err != nil {
		if err == pgx.ErrNoRows {
			return code_review.Changelist{}, clstore.ErrNotFound
		}
		return code_review.Changelist{}, skerr.Wrapf(err, "querying for id %s", qID)
	}
	return code_review.Changelist{
		SystemID: unqualify(r.ChangelistID),
		Owner:    r.OwnerEmail,
		Status:   convertToStatusEnum(r.Status),
		Subject:  r.Subject,
		Updated:  r.LastIngestedData.UTC(),
	}, nil
}

func qualify(system, id string) string {
	return system + "_" + id
}

func unqualify(id string) string {
	pieces := strings.SplitAfterN(id, "_", 2)
	if len(pieces) != 2 {
		sklog.Warningf("invalid changelist id %s", id)
		return id
	}
	return pieces[1]
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

func (s StoreImpl) GetPatchset(ctx context.Context, clID, psID string) (code_review.Patchset, error) {
	panic("implement me")
}

func (s StoreImpl) GetPatchsetByOrder(ctx context.Context, clID string, psOrder int) (code_review.Patchset, error) {
	panic("implement me")
}

func (s StoreImpl) GetChangelists(ctx context.Context, opts clstore.SearchOptions) ([]code_review.Changelist, int, error) {
	panic("implement me")
}

func (s StoreImpl) GetPatchsets(ctx context.Context, clID string) ([]code_review.Patchset, error) {
	panic("implement me")
}

func (s StoreImpl) PutChangelist(ctx context.Context, cl code_review.Changelist) error {
	qID := qualify(s.systemID, cl.SystemID)
	const statement = `
INSERT INTO Changelists (changelist_id, system, status, owner_email, subject, last_ingested_data)
VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := s.db.Exec(ctx, statement, qID, s.systemID, convertFromStatusEnum(cl.Status), cl.Owner, cl.Subject, cl.Updated)
	if err != nil {
		return skerr.Wrapf(err, "Inserting CL %#v", cl)
	}
	return nil
}

func (s StoreImpl) PutPatchset(ctx context.Context, ps code_review.Patchset) error {
	panic("implement me")
}

// Make sure StoreImpl fulfills the clstore.Store interface.
var _ clstore.Store = (*StoreImpl)(nil)
