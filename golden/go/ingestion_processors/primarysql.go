package ingestion_processors

import (
	"context"
	"crypto/md5"
	"strconv"
	"time"

	lru "github.com/hashicorp/golang-lru"

	"go.skia.org/infra/golden/go/jsonio"
	"go.skia.org/infra/golden/go/sql/schema"

	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/go/skerr"

	"go.opencensus.io/trace"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/golden/go/ingestion"
)

const (
	sqlTileWidthConfig = "TileWidth"

	commitsCacheSize = 1000
)

type sqlPrimaryIngester struct {
	db        *pgxpool.Pool
	source    ingestion.Source
	tileWidth int

	commitsCache *lru.Cache
}

// PrimaryBranchSQL creates a Processor that writes to the SQL backend and returns it.
func PrimaryBranchSQL(_ context.Context, src ingestion.Source, configParams map[string]string, db *pgxpool.Pool) (*sqlPrimaryIngester, error) {
	tw := configParams[sqlTileWidthConfig]
	tileWidth := 10
	if tw != "" {
		var err error
		tileWidth, err = strconv.Atoi(tw)
		if err != nil {
			return nil, skerr.Wrapf(err, "Invalid TileWidth")
		}
	}
	commitsCache, err := lru.New(commitsCacheSize)
	if err != nil {
		return nil, skerr.Wrap(err) // should only throw error on invalid size
	}

	return &sqlPrimaryIngester{
		db:           db,
		source:       src,
		tileWidth:    tileWidth,
		commitsCache: commitsCache,
	}, nil
}

// HandlesFile returns true if the underlying source handles the given file
func (s *sqlPrimaryIngester) HandlesFile(name string) bool {
	return s.source.HandlesFile(name)
}

// Process take the content of the given file and writes it to the various SQL tables required
// by the schema.
func (s *sqlPrimaryIngester) Process(ctx context.Context, fileName string) error {
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

	commitID, tileID, shouldWriteCommit, err := s.getCommitAndTileID(ctx, gr)
	if err != nil {
		return skerr.Wrapf(err, "identifying commit id for file %s", fileName)
	}
	if err := s.writeData(ctx, gr, commitID, tileID); err != nil {
		sklog.Errorf("Error writing data for file %s: %s", fileName, err)
		return ingestion.ErrRetryable
	}

	if shouldWriteCommit {
		if err := s.insertIntoCommitsWithData(ctx, commitID, tileID); err != nil {
			sklog.Errorf("Error writing to CommitsWithData for file %s: %s", fileName, err)
			return ingestion.ErrRetryable
		}
		s.updateCommitCache(gr, commitID, tileID)
	}
	if err := s.upsertSourceFile(ctx, fileName); err != nil {
		sklog.Errorf("Error writing to SourceFiles for file %s: %s", fileName, err)
		return ingestion.ErrRetryable
	}
	return nil
}

type commitCacheEntry struct {
	commitID schema.CommitID
	tileID   schema.TileID
}

// getCommitID gets the commit id from the information provided in the given jsonio. Currently,
// This looks up the GitHash to determine the commit_id (i.e. a sequence number), but this could
// be more flexible (e.g. To support multiple repos).
func (s *sqlPrimaryIngester) getCommitAndTileID(ctx context.Context, gr *jsonio.GoldResults) (schema.CommitID, schema.TileID, bool, error) {
	ctx, span := trace.StartSpan(ctx, "ingestion_getCommitID")
	defer span.End()
	if gr.GitHash == "" {
		return "", 0, false, skerr.Fmt("missing GitHash")
	}
	if c, ok := s.commitsCache.Get(gr.GitHash); ok {
		cce, ok := c.(commitCacheEntry)
		if ok {
			return cce.commitID, cce.tileID, false, nil
		}
		sklog.Warningf("Corrupt entry in commits cache: %#v", c)
		s.commitsCache.Remove(gr.GitHash)
	}
	// Cache miss - go to DB; We can't assume it's in the CommitsWithData table yet.
	row := s.db.QueryRow(ctx, `SELECT commit_id FROM GitCommits WHERE git_hash = $1`, gr.GitHash)
	var rv schema.CommitID
	if err := row.Scan(&rv); err != nil {
		return "", 0, false, skerr.Wrapf(err, "Looking up git_hash = %q", gr.GitHash)
	}
	// TODO(kjlubick) do tile computation here.
	return rv, 0, true, nil
}

func (s *sqlPrimaryIngester) insertIntoCommitsWithData(ctx context.Context, id schema.CommitID, tileID schema.TileID) error {
	_, err := s.db.Exec(ctx, `
INSERT INTO CommitsWithData (commit_id, tile_id) VALUES ($1, $2)
ON CONFLICT DO NOTHING`, id, tileID)
	return skerr.Wrap(err)
}

func (s *sqlPrimaryIngester) updateCommitCache(gr *jsonio.GoldResults, id schema.CommitID, tileID schema.TileID) {
	if s.commitsCache.Contains(gr.GitHash) {
		return
	}
	s.commitsCache.Add(gr.GitHash, commitCacheEntry{
		commitID: id,
		tileID:   tileID,
	})
}

// upsertSourceFile creates a row in SourceFiles for the given file or updates the existing row's
// last_ingested timestamp with now.
func (s *sqlPrimaryIngester) upsertSourceFile(ctx context.Context, fileName string) error {
	const statement = `UPSERT INTO SourceFiles (source_file_id, source_file, last_ingested)
VALUES ($1, $2, $3)`
	sourceID := md5.Sum([]byte(fileName))
	_, err := s.db.Exec(ctx, statement, sourceID[:], fileName, now(ctx))
	return skerr.Wrap(err)
}

func (s *sqlPrimaryIngester) writeData(ctx context.Context, gr *jsonio.GoldResults, commitID schema.CommitID, tileID schema.TileID) error {
	return nil
}

// overwriteNowKey is used by tests to make the time deterministic.
const overwriteNowKey = contextKey("overwriteNow")

type contextKey string

// now returns the current time or the time from the context.
func now(ctx context.Context) time.Time {
	if ts := ctx.Value(overwriteNowKey); ts != nil {
		return ts.(time.Time)
	}
	return time.Now()
}

// Make sure sqlPrimaryIngester implements the ingestion.Processor interface.
var _ ingestion.Processor = (*sqlPrimaryIngester)(nil)
