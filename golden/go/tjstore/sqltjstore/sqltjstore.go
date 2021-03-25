package sqltjstore

import (
	"context"
	"encoding/hex"
	"strings"
	"time"

	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.opencensus.io/trace"

	"go.skia.org/infra/go/skerr"
	ci "go.skia.org/infra/golden/go/continuous_integration"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/tjstore"
	"go.skia.org/infra/golden/go/types"
)

type StoreImpl struct {
	db *pgxpool.Pool
}

// New returns a SQL-backed tjstore.Store.
func New(db *pgxpool.Pool) *StoreImpl {
	return &StoreImpl{
		db: db,
	}
}

// GetTryJobs implements the tjstore.Store interface.
func (s *StoreImpl) GetTryJobs(ctx context.Context, cID tjstore.CombinedPSID) ([]ci.TryJob, error) {
	clID := sql.Qualify(cID.CRS, cID.CL)
	psID := sql.Qualify(cID.CRS, cID.PS)
	rows, err := s.db.Query(ctx, `
SELECT tryjob_id, system, display_name, last_ingested_data FROM Tryjobs
WHERE changelist_id = $1 AND patchset_id = $2
ORDER by display_name`, clID, psID)
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
func (s *StoreImpl) GetTryJob(ctx context.Context, id, cisName string) (ci.TryJob, error) {
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

// A possible optimization for RAM usage / network would be to request the option ids only
// and then run a followup request to fetch those and re-use the maps. This is the simplest
// possible query that might work.
const resultNoTime = `SELECT Traces.keys, digest, Options.keys, SecondaryBranchValues.tryjob_id FROM
SecondaryBranchValues JOIN Traces
ON SecondaryBranchValues.secondary_branch_trace_id = Traces.trace_id
JOIN Options
ON SecondaryBranchValues.options_id = Options.options_id
WHERE branch_name = $1 AND version_name = $2`

const resultWithTime = `
SELECT Traces.keys, digest, Options.keys, SecondaryBranchValues.tryjob_id FROM
SecondaryBranchValues JOIN Traces
ON SecondaryBranchValues.secondary_branch_trace_id = Traces.trace_id
JOIN Options
ON SecondaryBranchValues.options_id = Options.options_id
JOIN Tryjobs
ON SecondaryBranchValues.tryjob_id = Tryjobs.tryjob_id
WHERE branch_name = $1 AND version_name = $2 and last_ingested_data > $3`

// GetResults implements the tjstore.Store interface. Of note, it always returns a nil GroupParams
// because the way the data is stored, there is no way to know which params were ingested together.
func (s *StoreImpl) GetResults(ctx context.Context, cID tjstore.CombinedPSID, updatedAfter time.Time) ([]tjstore.TryJobResult, error) {
	ctx, span := trace.StartSpan(ctx, "sqltjstore_GetResults")
	defer span.End()
	clID := sql.Qualify(cID.CRS, cID.CL)
	psID := sql.Qualify(cID.CRS, cID.PS)

	statement := resultNoTime
	arguments := []interface{}{clID, psID}
	if !updatedAfter.IsZero() {
		statement = resultWithTime
		arguments = append(arguments, updatedAfter)
	}
	rows, err := s.db.Query(ctx, statement, arguments...)
	if err != nil {
		return nil, skerr.Wrapf(err, "getting values for tryjobs on %#v", cID)
	}
	defer rows.Close()
	var rv []tjstore.TryJobResult
	for rows.Next() {
		var digestBytes schema.DigestBytes
		var result tjstore.TryJobResult
		var qualifiedTryjobID pgtype.Text
		err := rows.Scan(&result.ResultParams, &digestBytes, &result.Options, &qualifiedTryjobID)
		if err != nil {
			return nil, skerr.Wrapf(err, "scanning values for tryjobs %#v", cID)
		}
		result.Digest = types.Digest(hex.EncodeToString(digestBytes))

		if qualifiedTryjobID.Status == pgtype.Present {
			parts := strings.SplitN(qualifiedTryjobID.String, "_", 2)
			result.System = parts[0]
			result.TryjobID = parts[1]
		}

		rv = append(rv, result)
	}
	return rv, nil
}

// Make sure StoreImpl fulfills the tjstore.Store interface.
var _ tjstore.Store = (*StoreImpl)(nil)
