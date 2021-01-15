package sqltjstore

import (
	"context"
	"time"

	"go.skia.org/infra/go/skerr"

	"github.com/jackc/pgx/v4/pgxpool"
	ci "go.skia.org/infra/golden/go/continuous_integration"
	"go.skia.org/infra/golden/go/tjstore"
)

type StoreImpl struct {
	db *pgxpool.Pool
}

// New returns a SQL-backed tjstore.Store.
func New(db *pgxpool.Pool) *StoreImpl {
	return &StoreImpl{db: db}
}

func (s StoreImpl) GetTryJobs(ctx context.Context, psID tjstore.CombinedPSID) ([]ci.TryJob, error) {
	panic("implement me")
}

func (s StoreImpl) GetTryJob(ctx context.Context, id, cisName string) (ci.TryJob, error) {
	panic("implement me")
}

// PutTryJob implements the tjstore.Store interface
func (s StoreImpl) PutTryJob(ctx context.Context, cID tjstore.CombinedPSID, tj ci.TryJob) error {
	tjID := qualify(tj.System, tj.SystemID)
	clID := qualify(cID.CRS, cID.CL)
	psID := qualify(cID.CRS, cID.PS)
	const statement = `
UPSERT INTO Tryjobs (tryjob_id, system, changelist_id, patchset_id, display_name, last)
VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := s.db.Exec(ctx, statement, tjID, tj.System, clID, psID, tj.DisplayName, tj.Updated)
	if err != nil {
		return skerr.Wrapf(err, "Inserting CL %#v", cl)
	}
	return nil
}

func (s StoreImpl) GetResults(ctx context.Context, psID tjstore.CombinedPSID, updatedAfter time.Time) ([]tjstore.TryJobResult, error) {
	panic("implement me")
}

func (s StoreImpl) PutResults(ctx context.Context, psID tjstore.CombinedPSID, tjID, cisName string, r []tjstore.TryJobResult, ts time.Time) error {
	panic("implement me")
}

// qualify prefixes the given CL or PS id with the given system. In the SQL database, we use these
// qualified IDs to make the queries easier, that is, we don't have to do a join over id and system,
// we can just use the combined ID.
func qualify(system, id string) string {
	return system + "_" + id
}

// Make sure StoreImpl fulfills the tjstore.Store interface.
var _ tjstore.Store = (*StoreImpl)(nil)
