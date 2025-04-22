package sqltracestore

import (
	"context"

	"github.com/jackc/pgx/v4"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/perf/go/tracestore"
)

type sqlstatement int

const (
	insert sqlstatement = iota
	read
	getsourcefileid
)

// statements are already constructed SQL statements.
var sqlstatements = map[sqlstatement]string{
	insert: `INSERT INTO
		Metadata
		VALUES ($1,$2)
		ON CONFLICT (source_file_id) DO NOTHING`,

	read: `SELECT links FROM Metadata WHERE source_file_id=$1`,
	getsourcefileid: `
        SELECT
            source_file_id
        FROM
            SourceFiles
        WHERE
            source_file=$1`,
}

// SQLMetadataStore implements the MetadataStore interface.
type SQLMetadataStore struct {
	// db is the SQL database instance.
	db pool.Pool
}

// NewSQLMetadataStore returns a new instance of the SQLMetadataStore.
func NewSQLMetadataStore(db pool.Pool) *SQLMetadataStore {
	return &SQLMetadataStore{
		db: db,
	}
}

// InsertMetadata inserts the metadata for the source file.
func (s *SQLMetadataStore) InsertMetadata(ctx context.Context, sourceFileName string, links map[string]string) error {
	var sourceFileId int
	row := s.db.QueryRow(ctx, sqlstatements[getsourcefileid], sourceFileName)
	if err := row.Scan(&sourceFileId); err != nil {
		if err == pgx.ErrNoRows {
			return skerr.Wrapf(err, "Source file %s does not exist in the database.", sourceFileName)
		}
		return skerr.Wrap(err)
	}

	if _, err := s.db.Exec(ctx, sqlstatements[insert], sourceFileId, links); err != nil {
		return skerr.Wrap(err)
	}

	return nil
}

// GetMetadata returns the metadata for the given source file.
func (s *SQLMetadataStore) GetMetadata(ctx context.Context, sourceFileName string) (map[string]string, error) {
	var sourceFileId int
	sourceFileRow := s.db.QueryRow(ctx, sqlstatements[getsourcefileid], sourceFileName)
	if err := sourceFileRow.Scan(&sourceFileId); err != nil {
		if err == pgx.ErrNoRows {
			return nil, skerr.Wrapf(err, "Source file %s does not exist in the database.", sourceFileName)
		}
		return nil, skerr.Wrap(err)
	}

	row := s.db.QueryRow(ctx, sqlstatements[read], sourceFileId)
	var links map[string]string
	if err := row.Scan(&links); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, skerr.Wrap(err)
	}

	return links, nil
}

var _ tracestore.MetadataStore = (*SQLMetadataStore)(nil)
