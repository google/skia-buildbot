package ingestion_processors

import (
	"context"
	"crypto/md5"
	"strconv"
	"time"

	"github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgx"
	lru "github.com/hashicorp/golang-lru"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.opencensus.io/trace"
	"golang.org/x/sync/errgroup"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/ingestion"
	"go.skia.org/infra/golden/go/jsonio"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/types"
)

const (
	// SQLPrimaryBranch indicates that the primary branch ingester use the SQL backend.
	SQLPrimaryBranch = "sql_primary"

	sqlTileWidthConfig = "TileWidth"

	commitsCacheSize         = 1000
	expectationsCacheSize    = 10_000_000
	optionsGroupingCacheSize = 1_000_000
	paramsCacheSize          = 100_000
	traceCacheSize           = 10_000_000

	cacheSizeMetric = "gold_ingestion_cache_sizes"
)

type sqlPrimaryIngester struct {
	db        *pgxpool.Pool
	source    ingestion.Source
	tileWidth int

	commitsCache        *lru.Cache
	expectationsCache   *lru.Cache
	optionGroupingCache *lru.Cache
	paramsCache         *lru.Cache
	traceCache          *lru.Cache

	filesProcessed  metrics2.Counter
	filesSuccess    metrics2.Counter
	resultsIngested metrics2.Counter
}

// PrimaryBranchSQL creates a Processor that writes to the SQL backend and returns it.
// It panics if configuration is invalid.
func PrimaryBranchSQL(src ingestion.Source, configParams map[string]string, db *pgxpool.Pool) *sqlPrimaryIngester {
	tw := configParams[sqlTileWidthConfig]
	tileWidth := 10
	if tw != "" {
		var err error
		tileWidth, err = strconv.Atoi(tw)
		if err != nil {
			panic(skerr.Wrapf(err, "Invalid TileWidth"))
		}
	}
	commitsCache, err := lru.New(commitsCacheSize)
	if err != nil {
		panic(err) // should only throw error on invalid size
	}
	eCache, err := lru.New(expectationsCacheSize)
	if err != nil {
		panic(err) // should only throw error on invalid size
	}
	ogCache, err := lru.New(optionsGroupingCacheSize)
	if err != nil {
		panic(err) // should only throw error on invalid size
	}
	paramsCache, err := lru.New(paramsCacheSize)
	if err != nil {
		panic(err) // should only throw error on invalid size
	}
	tCache, err := lru.New(traceCacheSize)
	if err != nil {
		panic(err) // should only throw error on invalid size
	}

	return &sqlPrimaryIngester{
		db:                  db,
		source:              src,
		tileWidth:           tileWidth,
		commitsCache:        commitsCache,
		expectationsCache:   eCache,
		optionGroupingCache: ogCache,
		paramsCache:         paramsCache,
		traceCache:          tCache,

		filesProcessed:  metrics2.GetCounter("gold_primarysqlingestion_files_processed"),
		filesSuccess:    metrics2.GetCounter("gold_primarysqlingestion_files_success"),
		resultsIngested: metrics2.GetCounter("gold_primarysqlingestion_results_ingested"),
	}
}

// HandlesFile returns true if the underlying source handles the given file
func (s *sqlPrimaryIngester) HandlesFile(name string) bool {
	return s.source.HandlesFile(name)
}

// Process take the content of the given file and writes it to the various SQL tables required
// by the schema.
// If there is a SQL error, we return ingestion.ErrRetryable but do NOT rollback the data. During
// exploratory design, it was observed that if we tried to put all the data from a single file
// in a transaction, the whole ingestion process ground to a halt as many big transactions would
// conflict with each other. Instead, the data is written to the DB in an order to minimize
// short-term errors if it partially succeeds and relies on retrying ingestion of files to
// deal with SQL errors.
func (s *sqlPrimaryIngester) Process(ctx context.Context, fileName string) error {
	s.filesProcessed.Inc(1)
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	ctx, span := trace.StartSpan(ctx, "ingestion_SQLPrimaryBranchProcess")
	defer span.End()
	r, err := s.source.GetReader(ctx, fileName)
	if err != nil {
		return skerr.Wrap(err)
	}
	gr, err := processGoldResults(ctx, r)
	if err != nil {
		return skerr.Wrapf(err, "could not process file %s from source %s", fileName, s.source)
	}
	if len(gr.Results) == 0 {
		sklog.Infof("file %s had no results", fileName)
		return nil
	}
	span.AddAttributes(trace.Int64Attribute("num_results", int64(len(gr.Results))))
	sklog.Infof("Ingesting %d results from file %s", len(gr.Results), fileName)

	commitID, tileID, err := s.getCommitAndTileID(ctx, gr)
	if err != nil {
		return skerr.Wrapf(err, "identifying commit id for file %s", fileName)
	}
	sourceFileID := md5.Sum([]byte(fileName))

	if err := s.writeData(ctx, gr, commitID, tileID, sourceFileID[:]); err != nil {
		sklog.Errorf("Error writing data for file %s: %s", fileName, err)
		return ingestion.ErrRetryable
	}

	if err := s.upsertSourceFile(ctx, sourceFileID[:], fileName); err != nil {
		sklog.Errorf("Error writing to SourceFiles for file %s: %s", fileName, err)
		return ingestion.ErrRetryable
	}
	s.filesSuccess.Inc(1)
	s.resultsIngested.Inc(int64(len(gr.Results)))
	return nil
}

type commitCacheEntry struct {
	commitID schema.CommitID
	tileID   schema.TileID
}

// getCommitAndTileID gets the commit id and corresponding tile id from the information provided
// in the given jsonio. Currently, this looks up the GitHash to determine the commit_id
// (i.e. a sequence number), but this could be more flexible (e.g. To support multiple repos).
// The tileID is derived from existing tileIDs of surrounding commits.
func (s *sqlPrimaryIngester) getCommitAndTileID(ctx context.Context, gr *jsonio.GoldResults) (schema.CommitID, schema.TileID, error) {
	ctx, span := trace.StartSpan(ctx, "getCommitAndTileID")
	defer span.End()
	var commitID schema.CommitID
	var key string
	if gr.GitHash != "" {
		key = gr.GitHash
		if c, ok := s.commitsCache.Get(key); ok {
			cce, ok := c.(commitCacheEntry)
			if ok {
				return cce.commitID, cce.tileID, nil
			}
			sklog.Warningf("Corrupt entry in commits cache: %#v", c)
			s.commitsCache.Remove(key)
		}
		// Cache miss - go to DB; We can't assume it's in the CommitsWithData table yet.
		row := s.db.QueryRow(ctx, `SELECT commit_id FROM GitCommits WHERE git_hash = $1`, gr.GitHash)
		if err := row.Scan(&commitID); err != nil {
			return "", 0, skerr.Wrapf(err, "looking up git_hash = %q", gr.GitHash)
		}
	} else if gr.CommitID != "" && gr.CommitMetadata != "" {
		key = gr.CommitID
		if _, ok := s.commitsCache.Get(key); !ok {
			// On a cache miss, make sure we store it to the table.
			const statement = `INSERT INTO MetadataCommits (commit_id, commit_metadata)
VALUES ($1, $2) ON CONFLICT DO NOTHING`
			_, err := s.db.Exec(ctx, statement, gr.CommitID, gr.CommitMetadata)
			if err != nil {
				return "", 0, skerr.Wrapf(err, "storing metadata commit %q : %q", gr.CommitID, gr.CommitMetadata)
			}
		}
		commitID = schema.CommitID(gr.CommitID)
	} else {
		// should never happen, assuming the jsonio.Validate works
		return "", 0, skerr.Fmt("missing GitHash and CommitID/CommitMetadata")
	}

	tileID, err := s.getAndWriteTileIDForCommit(ctx, commitID)
	if err != nil {
		return "", 0, skerr.Wrapf(err, "computing tile id for %s", commitID)
	}
	s.updateCommitCache(gr, key, commitID, tileID)
	return commitID, tileID, nil
}

// getAndWriteTileIDForCommit determines the tileID that should be used for this commit and writes
// it to the DB using a transaction. We use a transaction here (but not in general) because this
// should be a relatively fast read/write sequence and done somewhat infrequently (e.g. new commit
// has data created for it and it's not cached).
func (s *sqlPrimaryIngester) getAndWriteTileIDForCommit(ctx context.Context, targetCommit schema.CommitID) (schema.TileID, error) {
	ctx, span := trace.StartSpan(ctx, "getAndWriteTileIDForCommit")
	defer span.End()
	var returnTileID schema.TileID
	err := crdbpgx.ExecuteTx(ctx, s.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		tileID, err := s.getTileIDForCommit(ctx, tx, targetCommit)
		if err != nil {
			return err // Don't wrap - crdbpgx might retry
		}
		// Rows in this table are immutable once written
		_, err = tx.Exec(ctx, `
INSERT INTO CommitsWithData (commit_id, tile_id) VALUES ($1, $2)
ON CONFLICT DO NOTHING`, targetCommit, tileID)
		if err != nil {
			return err // Don't wrap - crdbpgx might retry
		}
		returnTileID = tileID
		return nil
	})
	if err != nil {
		return 0, skerr.Wrap(err)
	}
	return returnTileID, nil
}

// getTileIDForCommit determines the tileID for this commit by looking at surrounding commits
// (if any) for clues. If we have no commits before us, we are at tile 0. If we have no commits
// after us, we are on the latest tile, or if that is full, we start the next tile. If there are
// commits on either side of us, we use the tile for the commit after us.
func (s *sqlPrimaryIngester) getTileIDForCommit(ctx context.Context, tx pgx.Tx, targetCommit schema.CommitID) (schema.TileID, error) {
	ctx, span := trace.StartSpan(ctx, "getTileIDForCommit")
	defer span.End()
	row := tx.QueryRow(ctx, `SELECT commit_id, tile_id FROM CommitsWithData WHERE
commit_id <= $1 ORDER BY commit_id DESC LIMIT 1`, targetCommit)
	var cID schema.CommitID
	var beforeTileID schema.TileID
	if err := row.Scan(&cID, &beforeTileID); err == pgx.ErrNoRows {
		// If there is no data before this commit, we are, by definition, at tile 0
		return 0, nil
	} else if err != nil {
		return 0, err // Don't wrap - crdbpgx might retry
	}
	if cID == targetCommit {
		return beforeTileID, nil // already been computed
	}

	row = tx.QueryRow(ctx, `SELECT tile_id FROM CommitsWithData WHERE
commit_id >= $1 ORDER BY commit_id ASC LIMIT 1`, targetCommit)
	var afterTileID schema.TileID
	if err := row.Scan(&afterTileID); err == pgx.ErrNoRows {
		// If there is no data after this commit, we are at the highest tile id.
		row := tx.QueryRow(ctx, `SELECT count(*) FROM CommitsWithData WHERE
tile_id = $1`, beforeTileID)
		var count int
		if err := row.Scan(&count); err != nil {
			return 0, err // Don't wrap - crdbpgx might retry
		}
		// If the size of the previous tile exceeds our tile width, go to the next tile.
		if count >= s.tileWidth {
			return beforeTileID + 1, nil
		}
		return beforeTileID, nil
	} else if err != nil {
		return 0, err // Don't wrap - crdbpgx might retry
	}
	// We have CommitsWithData both before and after the targetCommit. We always return the tile
	// after this one, because it is less likely that tile is full.
	return afterTileID, nil
}

// updateCommitCache updates the local cache of "CommitsWithData" if necessary (so we know we do
// not have to write to the table the next time).
func (s *sqlPrimaryIngester) updateCommitCache(gr *jsonio.GoldResults, key string, id schema.CommitID, tileID schema.TileID) {
	s.commitsCache.Add(key, commitCacheEntry{
		commitID: id,
		tileID:   tileID,
	})
}

// upsertSourceFile creates a row in SourceFiles for the given file or updates the existing row's
// last_ingested timestamp. The time can be overridden via the context.
func (s *sqlPrimaryIngester) upsertSourceFile(ctx context.Context, srcID schema.SourceFileID, fileName string) error {
	ctx, span := trace.StartSpan(ctx, "upsertSourceFile")
	defer span.End()
	const statement = `UPSERT INTO SourceFiles (source_file_id, source_file, last_ingested)
VALUES ($1, $2, $3)`
	err := crdbpgx.ExecuteTx(ctx, s.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, statement, srcID, fileName, now.Now(ctx))
		return err // Don't wrap - crdbpgx might retry
	})
	return skerr.Wrap(err)
}

// writeData writes all the data from the processed JSON file, associating it with the given
// commitID, tileID, and sourceFile. This has to write to several tables in accordance with the
// schema/design. It makes use of caches where possible to avoid writing to tables with immutable
// data that we know is there already (e.g. a previous write succeeded).
func (s *sqlPrimaryIngester) writeData(ctx context.Context, gr *jsonio.GoldResults, commitID schema.CommitID, tileID schema.TileID, srcID schema.SourceFileID) error {
	ctx, span := trace.StartSpan(ctx, "writeData")
	span.AddAttributes(trace.Int64Attribute("results", int64(len(gr.Results))))
	defer span.End()

	var groupingsToCreate []schema.GroupingRow
	var optionsToCreate []schema.OptionsRow
	var tracesToCreate []schema.TraceRow
	var traceValuesToUpdate []schema.TraceValueRow
	var valuesAtHeadToUpdate []schema.ValueAtHeadRow

	newCacheEntries := map[string]bool{}
	paramset := paramtools.ParamSet{} // All params for this set of data points
	for _, result := range gr.Results {
		keys, options := paramsAndOptions(gr, result)
		if err := shouldIngest(keys, options); err != nil {
			sklog.Infof("Not ingesting a result: %s", err)
			continue
		}
		digestBytes, err := sql.DigestToBytes(result.Digest)
		if err != nil {
			sklog.Errorf("Invalid digest %s: %s", result.Digest, err)
			continue
		}
		_, traceID := sql.SerializeMap(keys)
		paramset.AddParams(keys)

		_, optionsID := sql.SerializeMap(options)
		paramset.AddParams(options)

		grouping := groupingFor(keys)
		_, groupingID := sql.SerializeMap(grouping)

		if h := string(optionsID); !s.optionGroupingCache.Contains(h) && !newCacheEntries[h] {
			optionsToCreate = append(optionsToCreate, schema.OptionsRow{
				OptionsID: optionsID,
				Keys:      options,
			})
			newCacheEntries[h] = true
		}

		if h := string(groupingID); !s.optionGroupingCache.Contains(h) && !newCacheEntries[h] {
			groupingsToCreate = append(groupingsToCreate, schema.GroupingRow{
				GroupingID: groupingID,
				Keys:       grouping,
			})
			newCacheEntries[h] = true
		}

		th := string(traceID)
		if newCacheEntries[th] {
			continue // already seen data for this trace
		}
		newCacheEntries[th] = true
		if !s.traceCache.Contains(th) {
			tracesToCreate = append(tracesToCreate, schema.TraceRow{
				TraceID:              traceID,
				GroupingID:           groupingID,
				Keys:                 keys,
				MatchesAnyIgnoreRule: schema.NBNull,
			})
		}
		valuesAtHeadToUpdate = append(valuesAtHeadToUpdate, schema.ValueAtHeadRow{
			TraceID:              traceID,
			MostRecentCommitID:   commitID,
			Digest:               digestBytes,
			OptionsID:            optionsID,
			GroupingID:           groupingID,
			Keys:                 keys,
			MatchesAnyIgnoreRule: schema.NBNull,
		})
		traceValuesToUpdate = append(traceValuesToUpdate, schema.TraceValueRow{
			Shard:        sql.ComputeTraceValueShard(traceID),
			TraceID:      traceID,
			CommitID:     commitID,
			Digest:       digestBytes,
			GroupingID:   groupingID,
			OptionsID:    optionsID,
			SourceFileID: srcID,
		})
	}

	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return skerr.Wrap(batchCreateGroupings(ctx, s.db, groupingsToCreate, s.optionGroupingCache))
	})
	eg.Go(func() error {
		return skerr.Wrap(batchCreateOptions(ctx, s.db, optionsToCreate, s.optionGroupingCache))
	})
	eg.Go(func() error {
		return skerr.Wrap(batchCreateTraces(ctx, s.db, tracesToCreate, s.traceCache))
	})
	eg.Go(func() error {
		return skerr.Wrap(s.batchCreateUntriagedExpectations(ctx, traceValuesToUpdate))
	})
	eg.Go(func() error {
		return skerr.Wrap(s.batchUpdateTraceValues(ctx, traceValuesToUpdate))
	})
	eg.Go(func() error {
		return skerr.Wrap(s.batchUpdateValuesAtHead(ctx, valuesAtHeadToUpdate))
	})
	eg.Go(func() error {
		return skerr.Wrap(s.batchCreatePrimaryBranchParams(ctx, paramset, tileID))
	})
	eg.Go(func() error {
		return skerr.Wrap(s.batchCreateTiledTraceDigests(ctx, traceValuesToUpdate, tileID))
	})
	return skerr.Wrap(eg.Wait())
}

// batchCreateGroupings writes the given grouping rows to the Groupings table if they aren't
// already there (they are immutable once written). It updates the cache after a successful write.
func batchCreateGroupings(ctx context.Context, db crdbpgx.Conn, rows []schema.GroupingRow, cache *lru.Cache) error {
	if len(rows) == 0 {
		return nil
	}
	ctx, span := trace.StartSpan(ctx, "batchCreateGroupings")
	span.AddAttributes(trace.Int64Attribute("groupings", int64(len(rows))))
	defer span.End()
	const chunkSize = 200 // Arbitrarily picked
	err := util.ChunkIter(len(rows), chunkSize, func(startIdx int, endIdx int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		batch := rows[startIdx:endIdx]
		if len(batch) == 0 {
			return nil
		}
		statement := `INSERT INTO Groupings (grouping_id, keys) VALUES `
		const valuesPerRow = 2
		statement += sql.ValuesPlaceholders(valuesPerRow, len(batch))
		arguments := make([]interface{}, 0, valuesPerRow*len(batch))
		for _, row := range batch {
			arguments = append(arguments, row.GroupingID, row.Keys)
		}
		// ON CONFLICT DO NOTHING because if the rows already exist, then the data we are writing
		// is immutable.
		statement += ` ON CONFLICT DO NOTHING;`

		err := crdbpgx.ExecuteTx(ctx, db, pgx.TxOptions{}, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, statement, arguments...)
			return err // Don't wrap - crdbpgx might retry
		})
		return skerr.Wrap(err)
	})
	if err != nil {
		return skerr.Wrapf(err, "storing %d groupings", len(rows))
	}
	// We've successfully written them to the DB, add them to the cache.
	for _, r := range rows {
		cache.Add(string(r.GroupingID), struct{}{})
	}
	return nil
}

// batchCreateOptions writes the given options rows to the Options table if they aren't
// already there (they are immutable once written). It updates the cache after a successful write.
func batchCreateOptions(ctx context.Context, db crdbpgx.Conn, rows []schema.OptionsRow, cache *lru.Cache) error {
	if len(rows) == 0 {
		return nil
	}
	ctx, span := trace.StartSpan(ctx, "batchCreateOptions")
	span.AddAttributes(trace.Int64Attribute("options", int64(len(rows))))
	defer span.End()
	const chunkSize = 200 // Arbitrarily picked
	err := util.ChunkIter(len(rows), chunkSize, func(startIdx int, endIdx int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		batch := rows[startIdx:endIdx]
		if len(batch) == 0 {
			return nil
		}
		statement := `INSERT INTO Options (options_id, keys) VALUES `
		const valuesPerRow = 2
		statement += sql.ValuesPlaceholders(valuesPerRow, len(batch))
		arguments := make([]interface{}, 0, valuesPerRow*len(batch))
		for _, row := range batch {
			arguments = append(arguments, row.OptionsID, row.Keys)
		}
		// ON CONFLICT DO NOTHING because if the rows already exist, then the data we are writing
		// is immutable.
		statement += ` ON CONFLICT DO NOTHING;`

		err := crdbpgx.ExecuteTx(ctx, db, pgx.TxOptions{}, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, statement, arguments...)
			return err // Don't wrap - crdbpgx might retry
		})
		return skerr.Wrap(err)
	})
	if err != nil {
		return skerr.Wrapf(err, "storing %d options", len(rows))
	}
	// We've successfully written them to the DB, add them to the cache.
	for _, r := range rows {
		cache.Add(string(r.OptionsID), struct{}{})
	}
	return nil
}

// batchCreateTraces writes the given trace rows to the Traces table if they aren't
// already there. The values we write are immutable once written. It updates the cache after a
// successful write.
func batchCreateTraces(ctx context.Context, db crdbpgx.Conn, rows []schema.TraceRow, cache *lru.Cache) error {
	if len(rows) == 0 {
		return nil
	}
	ctx, span := trace.StartSpan(ctx, "batchCreateTraces")
	span.AddAttributes(trace.Int64Attribute("traces", int64(len(rows))))
	defer span.End()
	const chunkSize = 200 // Arbitrarily picked
	err := util.ChunkIter(len(rows), chunkSize, func(startIdx int, endIdx int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		batch := rows[startIdx:endIdx]
		if len(batch) == 0 {
			return nil
		}
		statement := `INSERT INTO Traces (trace_id, grouping_id, keys) VALUES `
		const valuesPerRow = 3
		statement += sql.ValuesPlaceholders(valuesPerRow, len(batch))
		arguments := make([]interface{}, 0, valuesPerRow*len(batch))
		for _, row := range batch {
			arguments = append(arguments, row.TraceID, row.GroupingID, row.Keys)
		}
		// ON CONFLICT DO NOTHING because if the rows already exist, then the data we are writing
		// is immutable (we aren't writing to matches_any_ignore_rule).
		statement += ` ON CONFLICT DO NOTHING;`

		err := crdbpgx.ExecuteTx(ctx, db, pgx.TxOptions{}, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, statement, arguments...)
			return err // Don't wrap - crdbpgx might retry
		})
		return skerr.Wrap(err)
	})
	if err != nil {
		return skerr.Wrapf(err, "storing %d traces", len(rows))
	}
	// We've successfully written them to the DB, add them to the cache.
	for _, r := range rows {
		cache.Add(string(r.TraceID), struct{}{})
	}
	return nil
}

// batchCreateUntriagedExpectations creates a set of grouping+digest from all the provided values.
// It then ensures these exist in the Expectations table, storing the "untriaged" label if they
// are not there already. As per the schema/design this allows us to query for untriaged digests
// more easily than doing a big scan of TraceValues. It updates the cache on a successful write.
func (s *sqlPrimaryIngester) batchCreateUntriagedExpectations(ctx context.Context, values []schema.TraceValueRow) error {
	if len(values) == 0 {
		return nil
	}
	ctx, span := trace.StartSpan(ctx, "batchCreateUntriagedExpectations")
	span.AddAttributes(trace.Int64Attribute("trace_values", int64(len(values))))
	defer span.End()

	// Create ExpectationRows defaulting to untriaged for every data point in values.
	// We want to make sure it's unique because it's a SQL error to try to create multiple
	// of the same row in the same statement.
	newExpectations := map[string]bool{}
	var expectationRows []schema.ExpectationRow
	for _, valueRow := range values {
		newKey := string(valueRow.GroupingID) + string(valueRow.Digest)
		if newExpectations[newKey] || s.expectationsCache.Contains(newKey) {
			continue // We already are going to make this expectation row.
		}
		expectationRows = append(expectationRows, schema.ExpectationRow{
			GroupingID: valueRow.GroupingID,
			Digest:     valueRow.Digest,
			Label:      schema.LabelUntriaged,
		})
		newExpectations[newKey] = true
	}
	if err := s.batchCreateExpectations(ctx, expectationRows); err != nil {
		return skerr.Wrap(err)
	}
	// We've successfully written them to the DB, add them to the cache.
	for key := range newExpectations {
		s.expectationsCache.Add(key, struct{}{})
	}
	return nil
}

// batchCreateExpectations actually writes the provided expectation rows to the database. If any
// rows are already there, we don't overwrite the contents because it might already have been
// triaged.
func (s *sqlPrimaryIngester) batchCreateExpectations(ctx context.Context, rows []schema.ExpectationRow) error {
	if len(rows) == 0 {
		return nil
	}
	ctx, span := trace.StartSpan(ctx, "batchCreateExpectations")
	span.AddAttributes(trace.Int64Attribute("expectations", int64(len(rows))))
	defer span.End()
	const chunkSize = 200 // Arbitrarily picked
	err := util.ChunkIter(len(rows), chunkSize, func(startIdx int, endIdx int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		batch := rows[startIdx:endIdx]
		if len(batch) == 0 {
			return nil
		}
		statement := `INSERT INTO Expectations (grouping_id, digest, label) VALUES `
		const valuesPerRow = 3
		statement += sql.ValuesPlaceholders(valuesPerRow, len(batch))
		arguments := make([]interface{}, 0, valuesPerRow*len(batch))
		for _, row := range batch {
			arguments = append(arguments, row.GroupingID, row.Digest, row.Label)
		}
		// ON CONFLICT DO NOTHING because if the rows already exist, then we already have an
		// expectation, either previously automatically created or changed by a user. In either
		// case, we don't want to overwrite it.
		statement += ` ON CONFLICT DO NOTHING;`

		err := crdbpgx.ExecuteTx(ctx, s.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, statement, arguments...)
			return err // Don't wrap - crdbpgx might retry
		})
		return skerr.Wrap(err)
	})
	if err != nil {
		return skerr.Wrapf(err, "storing %d expectations", len(rows))
	}
	return nil
}

// batchUpdateTraceValues stores all the given rows.
func (s *sqlPrimaryIngester) batchUpdateTraceValues(ctx context.Context, rows []schema.TraceValueRow) error {
	if len(rows) == 0 {
		return nil
	}
	ctx, span := trace.StartSpan(ctx, "batchUpdateTraceValues")
	span.AddAttributes(trace.Int64Attribute("values", int64(len(rows))))
	defer span.End()
	const chunkSize = 200 // Arbitrarily picked
	err := util.ChunkIter(len(rows), chunkSize, func(startIdx int, endIdx int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		batch := rows[startIdx:endIdx]
		if len(batch) == 0 {
			return nil
		}
		const statement = `UPSERT INTO TraceValues (shard, trace_id, commit_id, digest,
grouping_id, options_id, source_file_id) VALUES `
		const valuesPerRow = 7
		arguments := make([]interface{}, 0, valuesPerRow*len(batch))
		for _, row := range batch {
			arguments = append(arguments, row.Shard, row.TraceID, row.CommitID, row.Digest,
				row.GroupingID, row.OptionsID, row.SourceFileID)
		}
		vp := sql.ValuesPlaceholders(valuesPerRow, len(batch))
		err := crdbpgx.ExecuteTx(ctx, s.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, statement+vp, arguments...)
			return err // Don't wrap - crdbpgx might retry
		})
		return skerr.Wrap(err)
	})
	if err != nil {
		return skerr.Wrapf(err, "storing %d trace values", len(rows))
	}
	return nil
}

// batchUpdateValuesAtHead stores all the given rows, if the given most_recent_commit_id is newer
// than the existing one in the DB.
func (s *sqlPrimaryIngester) batchUpdateValuesAtHead(ctx context.Context, rows []schema.ValueAtHeadRow) error {
	if len(rows) == 0 {
		return nil
	}
	ctx, span := trace.StartSpan(ctx, "batchUpdateValuesAtHead")
	span.AddAttributes(trace.Int64Attribute("values", int64(len(rows))))
	defer span.End()
	const chunkSize = 50 // Arbitrarily picked (smaller because more likely to contend)
	err := util.ChunkIter(len(rows), chunkSize, func(startIdx int, endIdx int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		batch := rows[startIdx:endIdx]
		if len(batch) == 0 {
			return nil
		}
		statement := `INSERT INTO ValuesAtHead (trace_id, most_recent_commit_id, digest,
options_id, grouping_id, keys) VALUES `
		const valuesPerRow = 6
		statement += sql.ValuesPlaceholders(valuesPerRow, len(batch))
		arguments := make([]interface{}, 0, valuesPerRow*len(batch))
		for _, row := range batch {
			arguments = append(arguments, row.TraceID, row.MostRecentCommitID, row.Digest,
				row.OptionsID, row.GroupingID, row.Keys)
		}
		// If the row already exists, we'll update these three fields if and only if the
		// commit_id comes after the stored commit_id.
		statement += `
ON CONFLICT (trace_id)
DO UPDATE SET (most_recent_commit_id, digest, options_id) =
    (excluded.most_recent_commit_id, excluded.digest, excluded.options_id)
WHERE excluded.most_recent_commit_id > ValuesAtHead.most_recent_commit_id`
		err := crdbpgx.ExecuteTx(ctx, s.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, statement, arguments...)
			return err // Don't wrap - crdbpgx might retry
		})
		return skerr.Wrap(err)
	})
	if err != nil {
		return skerr.Wrapf(err, "updating %d values at head", len(rows))
	}
	return nil
}

// batchCreatePrimaryBranchParams turns the given paramset into tile-key-value tuples and stores
// them to the DB. It updates the cache on success.
func (s *sqlPrimaryIngester) batchCreatePrimaryBranchParams(ctx context.Context, paramset paramtools.ParamSet, tile schema.TileID) error {
	ctx, span := trace.StartSpan(ctx, "batchCreatePrimaryBranchParams")
	defer span.End()
	var rows []schema.PrimaryBranchParamRow
	for key, values := range paramset {
		for _, value := range values {
			pr := schema.PrimaryBranchParamRow{
				TileID: tile,
				Key:    key,
				Value:  value,
			}
			if s.paramsCache.Contains(pr) {
				continue // don't need to store it again.
			}
			rows = append(rows, pr)
		}
	}

	if len(rows) == 0 {
		return nil
	}
	span.AddAttributes(trace.Int64Attribute("key_value_pairs", int64(len(rows))))

	const chunkSize = 200 // Arbitrarily picked
	err := util.ChunkIter(len(rows), chunkSize, func(startIdx int, endIdx int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		batch := rows[startIdx:endIdx]
		if len(batch) == 0 {
			return nil
		}
		statement := `INSERT INTO PrimaryBranchParams (tile_id, key, value) VALUES `
		const valuesPerRow = 3
		statement += sql.ValuesPlaceholders(valuesPerRow, len(batch))
		arguments := make([]interface{}, 0, valuesPerRow*len(batch))
		for _, row := range batch {
			arguments = append(arguments, row.TileID, row.Key, row.Value)
		}
		// ON CONFLICT DO NOTHING because if the rows already exist, the data is immutable.
		statement += ` ON CONFLICT DO NOTHING;`

		err := crdbpgx.ExecuteTx(ctx, s.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, statement, arguments...)
			return err // Don't wrap - crdbpgx might retry
		})
		return skerr.Wrap(err)
	})
	if err != nil {
		return skerr.Wrapf(err, "storing %d primary branch params", len(rows))
	}

	for _, r := range rows {
		s.paramsCache.Add(r, struct{}{})
	}
	return nil
}

// batchCreateTiledTraceDigests creates trace-tile-digest tuples and stores them to the DB.
func (s *sqlPrimaryIngester) batchCreateTiledTraceDigests(ctx context.Context, values []schema.TraceValueRow, tileID schema.TileID) error {
	if len(values) == 0 {
		return nil
	}
	// TODO(kjlubick) perhaps have a cache for these as well?
	ctx, span := trace.StartSpan(ctx, "batchCreateTiledTraceDigests")
	defer span.End()

	span.AddAttributes(trace.Int64Attribute("values", int64(len(values))))

	const chunkSize = 200 // Arbitrarily picked
	err := util.ChunkIter(len(values), chunkSize, func(startIdx int, endIdx int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		batch := values[startIdx:endIdx]
		if len(batch) == 0 {
			return nil
		}
		statement := `INSERT INTO TiledTraceDigests (trace_id, tile_id, digest, grouping_id) VALUES `
		const valuesPerRow = 4
		statement += sql.ValuesPlaceholders(valuesPerRow, len(batch))
		arguments := make([]interface{}, 0, valuesPerRow*len(batch))
		for _, row := range batch {
			arguments = append(arguments, row.TraceID, tileID, row.Digest, row.GroupingID)
		}
		// ON CONFLICT DO NOTHING only update things if the grouping_id is different.
		// We don't expect the grouping_id to be different often, only when a grouping is changed.
		statement += ` ON CONFLICT (trace_id, tile_id, digest)
DO UPDATE SET grouping_id = excluded.grouping_id
WHERE TiledTraceDigests.grouping_id != excluded.grouping_id;`

		err := crdbpgx.ExecuteTx(ctx, s.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, statement, arguments...)
			return err // Don't wrap - crdbpgx might retry
		})
		return skerr.Wrap(err)
	})
	if err != nil {
		return skerr.Wrapf(err, "storing %d Tiled Trace Digest rows", len(values))
	}
	return nil
}

// MonitorCacheMetrics starts a goroutine to report the cache sizes every minute
func (s *sqlPrimaryIngester) MonitorCacheMetrics(ctx context.Context) {
	commitSize := metrics2.GetInt64Metric(cacheSizeMetric, map[string]string{"cache_name": "commits"})
	expectationSize := metrics2.GetInt64Metric(cacheSizeMetric, map[string]string{"cache_name": "expectations"})
	optionGroupingSize := metrics2.GetInt64Metric(cacheSizeMetric, map[string]string{"cache_name": "optionGrouping"})
	paramsSize := metrics2.GetInt64Metric(cacheSizeMetric, map[string]string{"cache_name": "params"})
	traceSize := metrics2.GetInt64Metric(cacheSizeMetric, map[string]string{"cache_name": "trace"})
	go util.RepeatCtx(ctx, time.Minute, func(ctx context.Context) {
		commitSize.Update(int64(s.commitsCache.Len()))
		expectationSize.Update(int64(s.expectationsCache.Len()))
		optionGroupingSize.Update(int64(s.optionGroupingCache.Len()))
		paramsSize.Update(int64(s.paramsCache.Len()))
		traceSize.Update(int64(s.traceCache.Len()))
	})
}

func groupingFor(keys map[string]string) map[string]string {
	// This could one day be configurable per corpus or something.
	return map[string]string{
		types.CorpusField:     keys[types.CorpusField],
		types.PrimaryKeyField: keys[types.PrimaryKeyField],
	}
}

// Make sure sqlPrimaryIngester implements the ingestion.Processor interface.
var _ ingestion.Processor = (*sqlPrimaryIngester)(nil)
