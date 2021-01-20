package sqltjstore

import (
	"context"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"

	"go.skia.org/infra/go/skerr"
	ci "go.skia.org/infra/golden/go/continuous_integration"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/tjstore"
)

type StoreImpl struct {
	db *pgxpool.Pool
}

// New returns a SQL-backed tjstore.Store.
func New(db *pgxpool.Pool) *StoreImpl {
	return &StoreImpl{db: db}
}

// GetTryJobs implements the tjstore.Store interface.
func (s StoreImpl) GetTryJobs(ctx context.Context, cID tjstore.CombinedPSID) ([]ci.TryJob, error) {
	clID := sql.Qualify(cID.CRS, cID.CL)
	psID := sql.Qualify(cID.CRS, cID.PS)
	rows, err := s.db.Query(ctx, `
SELECT tryjob_id, system, display_name, last_ingested_data FROM Tryjobs
WHERE changelist_id = $1 AND patchset_id = $2`, clID, psID)
	if err != nil {
		return nil, skerr.Wrapf(err, "fetching tryjobs for %#v", cID)
	}
	defer rows.Close()
	var rv []ci.TryJob
	for rows.Next() {
		var row schema.TryjobRow
		err := rows.Scan(&row.TryjobID, &row.System, &row.DisplayName, &row.LastIngestedData)
		if err != nil {
			return nil, skerr.Wrapf(err, "when fetching tryjobs for %#v", cID)
		}
		rv = append(rv, ci.TryJob{
			SystemID:    sql.Unqualify(row.TryjobID),
			System:      row.System,
			DisplayName: row.DisplayName,
			Updated:     row.LastIngestedData.UTC(),
		})
	}
	return rv, nil
}

// GetTryJob implements the tjstore.Store interface.
func (s StoreImpl) GetTryJob(ctx context.Context, id, cisName string) (ci.TryJob, error) {
	qID := sql.Qualify(cisName, id)
	row := s.db.QueryRow(ctx, `
SELECT display_name, last_ingested_data FROM Tryjobs WHERE tryjob_id = $1`, qID)
	var r schema.TryjobRow
	err := row.Scan(&r.DisplayName, &r.LastIngestedData)
	if err != nil {
		if err == pgx.ErrNoRows {
			return ci.TryJob{}, tjstore.ErrNotFound
		}
		return ci.TryJob{}, skerr.Wrapf(err, "querying for id %s", qID)
	}
	return ci.TryJob{
		SystemID:    id,
		System:      cisName,
		DisplayName: r.DisplayName,
		Updated:     r.LastIngestedData.UTC(),
	}, nil
}

// PutTryJob implements the tjstore.Store interface
func (s StoreImpl) PutTryJob(ctx context.Context, cID tjstore.CombinedPSID, tj ci.TryJob) error {
	tjID := sql.Qualify(tj.System, tj.SystemID)
	clID := sql.Qualify(cID.CRS, cID.CL)
	psID := sql.Qualify(cID.CRS, cID.PS)
	const statement = `
UPSERT INTO Tryjobs (tryjob_id, system, changelist_id, patchset_id, display_name, last_ingested_data)
VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := s.db.Exec(ctx, statement, tjID, tj.System, clID, psID, tj.DisplayName, tj.Updated)
	if err != nil {
		return skerr.Wrapf(err, "Inserting tryjob %#v", tj)
	}
	return nil
}

// GetResults implements the tjstore.Store interface.
func (s StoreImpl) GetResults(ctx context.Context, psID tjstore.CombinedPSID, updatedAfter time.Time) ([]tjstore.TryJobResult, error) {
	return nil, skerr.Fmt("TODO(kjlubick) implement me")
}

// PutResults implements the tjstore.Store interface.
func (s StoreImpl) PutResults(ctx context.Context, psID tjstore.CombinedPSID, tjID, cisName, sourceFile string, r []tjstore.TryJobResult, ts time.Time) error {
	return skerr.Fmt("TODO(kjlubick) implement me")
}

// Make sure StoreImpl fulfills the tjstore.Store interface.
var _ tjstore.Store = (*StoreImpl)(nil)
