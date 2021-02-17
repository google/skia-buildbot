package sqltjstore

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"strings"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.opencensus.io/trace"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	ci "go.skia.org/infra/golden/go/continuous_integration"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/tjstore"
	"go.skia.org/infra/golden/go/types"
)

type StoreImpl struct {
	db *pgxpool.Pool
	// keyValueCache keeps track of which Traces, Groupings, and Options have been already created
	// and thus don't need to be created again.
	keyValueCache *lru.Cache
}

// New returns a SQL-backed tjstore.Store.
func New(db *pgxpool.Pool) *StoreImpl {
	// 10 million should be sufficient to avoid many unnecessary puts to the Traces, Groupings,
	// and Options tables without taking up too much RAM.
	kc, err := lru.New(10_000_000) // ~20 bytes per entry = 200M
	if err != nil {
		panic(err) // this should only happen if the value passed into New is negative
	}
	return &StoreImpl{
		db:            db,
		keyValueCache: kc,
	}
}

// getGrouping returns a grouping for a given set of params. If we need to make grouping dependent
// on, for example, the corpus, this is where we could affect that (probably passed in to the
// creation of this store.
func (s *StoreImpl) getGrouping(traceParams paramtools.Params) paramtools.Params {
	return paramtools.Params{
		types.CorpusField:     traceParams[types.CorpusField],
		types.PrimaryKeyField: traceParams[types.PrimaryKeyField],
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

// PutTryJob implements the tjstore.Store interface
func (s *StoreImpl) PutTryJob(ctx context.Context, cID tjstore.CombinedPSID, tj ci.TryJob) error {
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

// PutResults implements the tjstore.Store interface. In exploratory design, ingesting a file
// with many results in a transaction yielded in very very slow ingestion due to a lot of contention
// on tables like Traces. As a result, we do not make all these changes in a transaction.
func (s *StoreImpl) PutResults(ctx context.Context, cID tjstore.CombinedPSID, sourceFile string, results []tjstore.TryJobResult, ts time.Time) error {
	clID := sql.Qualify(cID.CRS, cID.CL)
	psID := sql.Qualify(cID.CRS, cID.PS)
	sf := md5.Sum([]byte(sourceFile))
	sourceID := sf[:]
	// Put sourcefile
	_, err := s.db.Exec(ctx, `
UPSERT INTO SourceFiles (source_file_id, source_file, last_ingested)
VALUES ($1, $2, $3)`, sourceID, sourceFile, ts)
	if err != nil {
		return skerr.Wrapf(err, "upserting sourcefile %x-%s", sourceID, sourceFile)
	}
	// Find all traceIDs, groupings, options
	tracesToAdd := map[schema.MD5Hash]paramtools.Params{}
	groupingsToAdd := map[schema.MD5Hash]paramtools.Params{}
	optionsToAdd := map[schema.MD5Hash]paramtools.Params{}
	rows := make([]schema.SecondaryBranchValueRow, 0, len(results))
	uniqueTryjobs := map[string]bool{}
	for _, result := range results {
		tjID := sql.Qualify(result.System, result.TryjobID)
		uniqueTryjobs[tjID] = true
		digestBytes, err := sql.DigestToBytes(result.Digest)
		if err != nil {
			return skerr.Wrap(err)
		}
		keyParams := paramtools.Params{}
		keyParams.Add(result.GroupParams, result.ResultParams)
		_, traceID := sql.SerializeMap(keyParams)
		groupingParams := s.getGrouping(keyParams)
		_, groupingID := sql.SerializeMap(groupingParams)
		_, optionsID := sql.SerializeMap(result.Options)

		if !s.keyValueCache.Contains(string(traceID)) {
			tracesToAdd[sql.AsMD5Hash(traceID)] = keyParams
		}
		if !s.keyValueCache.Contains(string(groupingID)) {
			groupingsToAdd[sql.AsMD5Hash(groupingID)] = groupingParams
		}
		if !s.keyValueCache.Contains(string(optionsID)) {
			optionsToAdd[sql.AsMD5Hash(optionsID)] = result.Options
		}

		rows = append(rows, schema.SecondaryBranchValueRow{
			BranchName:   clID,
			VersionName:  psID,
			TraceID:      traceID,
			Digest:       digestBytes,
			GroupingID:   groupingID,
			OptionsID:    optionsID,
			SourceFileID: sourceID,
			TryjobID:     tjID,
		})
	}

	// Insert those all if needed (e.g. not in cache)
	if err := s.batchCreateKeys(ctx, insertGroupings, groupingsToAdd); err != nil {
		return skerr.Wrap(err)
	}
	if err := s.batchCreateKeys(ctx, insertOptions, optionsToAdd); err != nil {
		return skerr.Wrap(err)
	}
	if err := s.batchCreateTraces(ctx, tracesToAdd); err != nil {
		return skerr.Wrap(err)
	}
	// Insert into SecondaryBranchValues
	if err := s.batchInsertResultValues(ctx, rows); err != nil {
		return skerr.Wrap(err)
	}

	// Update all the Tryjobs with the correct timestamp now that everything else has succeeded.
	for tjID := range uniqueTryjobs {
		_, err = s.db.Exec(ctx, `
UPDATE Tryjobs SET last_ingested_data = $1 WHERE tryjob_id = $2`, ts, tjID)
		if err != nil {
			return skerr.Wrapf(err, "updating tryjob %s", tjID)
		}
	}

	return nil
}

const insertGroupings = `INSERT INTO Groupings (grouping_id, keys) VALUES `
const insertOptions = `INSERT INTO Options (options_id, keys) VALUES `

// batchCreateKeys adds the provided groupings or options to the sql database. On success, the cache
// is updated so they aren't added again (the rows are immutable).
func (s *StoreImpl) batchCreateKeys(ctx context.Context, insert string, toCreate map[schema.MD5Hash]paramtools.Params) error {
	if len(toCreate) == 0 {
		return nil
	}
	type keyValue struct {
		id   []byte
		keys paramtools.Params
	}
	createSlice := make([]keyValue, 0, len(toCreate))
	for id, keys := range toCreate {
		// Taking a slice of an array that is a loop variable does not work as expected.
		copyID := make([]byte, md5.Size)
		copy(copyID, id[:])
		createSlice = append(createSlice, keyValue{id: copyID, keys: keys})
	}
	// This can be somewhat high because in the steady state case there is not a lot of contention
	// on this table.
	const chunkSize = 500
	err := util.ChunkIter(len(createSlice), chunkSize, func(startIdx int, endIdx int) error {
		batch := createSlice[startIdx:endIdx]
		if len(batch) == 0 {
			return nil
		}
		statement := insert
		const valuesPerRow = 2
		statement += sql.ValuesPlaceholders(valuesPerRow, len(batch))
		arguments := make([]interface{}, 0, valuesPerRow*len(batch))
		for _, value := range batch {
			arguments = append(arguments, value.id, value.keys)
		}
		// ON CONFLICT DO NOTHING because if the rows already exist, then the data we are writing
		// is immutable.
		statement += ` ON CONFLICT DO NOTHING;`

		_, err := s.db.Exec(ctx, statement, arguments...)
		return skerr.Wrap(err)
	})
	if err != nil {
		return skerr.Wrapf(err, "storing %d JSON entries with insert %s", len(toCreate), insert)
	}
	// Update the cache now that these have all landed.
	for _, kv := range createSlice {
		s.keyValueCache.Add(string(kv.id), true)
	}
	return nil
}

// batchCreateTraces adds the provided trace rows to the database. If the traces already exist,
// the new data will be ignored. On success, the cache is updated to contain the provided trace ids.
func (s *StoreImpl) batchCreateTraces(ctx context.Context, toCreate map[schema.MD5Hash]paramtools.Params) error {
	if len(toCreate) == 0 {
		return nil
	}
	rows := make([]schema.TraceRow, 0, len(toCreate))
	for id, keys := range toCreate {
		groupingParams := s.getGrouping(keys)
		_, groupingID := sql.SerializeMap(groupingParams)
		// Taking a slice of an array that is a loop variable does not work as expected.
		copyID := make([]byte, md5.Size)
		copy(copyID, id[:])
		rows = append(rows, schema.TraceRow{
			TraceID:    copyID,
			GroupingID: groupingID,
			Keys:       keys,
		})
	}

	// In most cases, the trace already exists, so we go in smaller batches to avoid contention.
	const chunkSize = 100
	err := util.ChunkIter(len(rows), chunkSize, func(startIdx int, endIdx int) error {
		batch := rows[startIdx:endIdx]
		if len(batch) == 0 {
			return nil
		}
		statement := `INSERT INTO Traces (trace_id, grouping_id, keys) VALUES `
		const valuesPerRow = 3
		statement += sql.ValuesPlaceholders(valuesPerRow, len(batch))
		arguments := make([]interface{}, 0, valuesPerRow*len(batch))
		for _, value := range batch {
			arguments = append(arguments, value.TraceID, value.GroupingID, value.Keys)
		}
		// ON CONFLICT DO NOTHING because if the rows already exist, then the data we are writing
		// is immutable.
		statement += ` ON CONFLICT DO NOTHING;`

		_, err := s.db.Exec(ctx, statement, arguments...)
		return skerr.Wrap(err)
	})
	if err != nil {
		return skerr.Wrapf(err, "storing %d traces", len(toCreate))
	}

	for _, r := range rows {
		s.keyValueCache.Add(string(r.TraceID), true)
	}
	return nil
}

// batchInsertResultValues inserts the provided tryjob results in batches.
func (s *StoreImpl) batchInsertResultValues(ctx context.Context, rows []schema.SecondaryBranchValueRow) error {
	if len(rows) == 0 {
		return nil
	}
	// Start at this chunk size for now. This table will likely receive a fair amount of data
	// and smaller batch sizes can reduce the contention/retries.
	const chunkSize = 300
	err := util.ChunkIter(len(rows), chunkSize, func(startIdx int, endIdx int) error {
		batch := rows[startIdx:endIdx]
		if len(batch) == 0 {
			return nil
		}
		statement := `UPSERT INTO SecondaryBranchValues
(branch_name, version_name, secondary_branch_trace_id, digest, grouping_id, options_id,
source_file_id, tryjob_id) VALUES `
		const valuesPerRow = 8
		statement += sql.ValuesPlaceholders(valuesPerRow, len(batch))
		arguments := make([]interface{}, 0, valuesPerRow*len(batch))
		for _, value := range batch {
			arguments = append(arguments, value.BranchName, value.VersionName, value.TraceID,
				value.Digest, value.GroupingID, value.OptionsID, value.SourceFileID, value.TryjobID)
		}

		_, err := s.db.Exec(ctx, statement, arguments...)
		return skerr.Wrap(err)
	})
	if err != nil {
		return skerr.Wrapf(err, "storing %d tryjob results", len(rows))
	}
	return nil
}

// Make sure StoreImpl fulfills the tjstore.Store interface.
var _ tjstore.Store = (*StoreImpl)(nil)
