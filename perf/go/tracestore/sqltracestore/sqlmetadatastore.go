package sqltracestore

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/jackc/pgx/v4"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/tracestore"
)

type sqlstatement int

const (
	insert sqlstatement = iota
	read
	readMultiple
	getsourcefileid
	getsourcefileidMultiple
)

// statements are already constructed SQL statements.
var sqlstatements = map[sqlstatement]string{
	insert: `INSERT INTO
		Metadata
		VALUES ($1,$2)
		ON CONFLICT (source_file_id) DO NOTHING`,

	read:         `SELECT links FROM Metadata WHERE source_file_id=$1`,
	readMultiple: `SELECT source_file_id, links FROM Metadata WHERE source_file_id IN `,
	getsourcefileid: `
        SELECT
            source_file_id
        FROM
            SourceFiles
        WHERE
            source_file=$1`,
	getsourcefileidMultiple: `
        SELECT
            source_file, source_file_id
        FROM
            SourceFiles
        WHERE
            source_file IN `,
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

// GetMetadataMultiple returns the metadata for the given list of source files.
// The return value is a map where the key is the source file name and value is the map of links.
func (s *SQLMetadataStore) GetMetadataMultiple(ctx context.Context, sourceFileNames []string) (map[string]map[string]string, error) {
	fileLinksAggregate := map[string]map[string]string{}
	mutex := sync.Mutex{}
	err := util.ChunkIterParallelPool(ctx, len(sourceFileNames), 200, 50, func(ctx context.Context, startIdx, endIdx int) error {
		sourceFileChunk := sourceFileNames[startIdx:endIdx]
		var sb strings.Builder
		for _, sourceFile := range sourceFileChunk {
			sb.WriteString("'" + sourceFile + "', ")
		}

		argString := sb.String()
		// Trim the last 2 chars (", ")
		argString = argString[:len(argString)-2]
		sourceFileRows, err := s.db.Query(ctx, sqlstatements[getsourcefileidMultiple]+fmt.Sprintf("(%s)", argString))
		if err != nil {
			if err == pgx.ErrNoRows {
				return skerr.Wrapf(err, "Source files %s do not exist in the database.", sourceFileNames)
			}
			return skerr.Wrap(err)
		}
		defer sourceFileRows.Close()
		sourceMap := map[int]string{}
		sourceFileIds := []string{}
		for sourceFileRows.Next() {
			var sourceFileName string
			var sourceFileId int
			if err := sourceFileRows.Scan(&sourceFileName, &sourceFileId); err != nil {
				return skerr.Wrapf(err, "Failed to scan source file row data.")
			}
			sourceMap[sourceFileId] = sourceFileName
			sourceFileIds = append(sourceFileIds, strconv.Itoa(sourceFileId))
		}

		sql := sqlstatements[readMultiple] + fmt.Sprintf("(%s)", strings.Join(sourceFileIds, ","))
		rows, err := s.db.Query(ctx, sql)
		if err != nil {
			if err == pgx.ErrNoRows {
				return nil
			}
			return skerr.Wrap(err)
		}
		// Need to add a mutex to avoid concurrent map writes on fileLinksAggregate.
		addMetadata := func(fileName string, links map[string]string) {
			mutex.Lock()
			defer mutex.Unlock()
			fileLinksAggregate[fileName] = links
		}
		for rows.Next() {
			var source_file_id int
			var links map[string]string
			if err := rows.Scan(&source_file_id, &links); err != nil {
				return skerr.Wrapf(err, "Failed to scan links data.")
			}
			fileName := sourceMap[source_file_id]
			addMetadata(fileName, links)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return fileLinksAggregate, nil
}

func (s *SQLMetadataStore) GetMetadataForSourceFileIDs(ctx context.Context, sourceFileIDs []int64) (map[int64]map[string]string, error) {
	if len(sourceFileIDs) == 0 {
		sklog.Info("sourceFileIDs list is empty, returning")
		return map[int64]map[string]string{}, nil
	}
	fileLinksAggregate := map[int64]map[string]string{}
	mutex := sync.Mutex{}
	err := util.ChunkIterParallelPool(ctx, len(sourceFileIDs), 200, 50, func(ctx context.Context, startIdx, endIdx int) error {
		sourceFileIDChunk := sourceFileIDs[startIdx:endIdx]
		sb := strings.Builder{}
		for _, sourceFileID := range sourceFileIDChunk {
			sb.WriteString(strconv.FormatInt(sourceFileID, 10) + ", ")
		}

		sql := sqlstatements[readMultiple] + fmt.Sprintf("(%s)", sb.String()[:len(sb.String())-2])
		rows, err := s.db.Query(ctx, sql)
		if err != nil {
			if err == pgx.ErrNoRows {
				return nil
			}
			return skerr.Wrap(err)
		}
		defer rows.Close()
		// Need to add a mutex to avoid concurrent map writes on fileLinksAggregate.
		addMetadata := func(sourceFileId int64, links map[string]string) {
			mutex.Lock()
			defer mutex.Unlock()
			fileLinksAggregate[sourceFileId] = links
		}
		for rows.Next() {
			var source_file_id int64
			var links map[string]string
			if err := rows.Scan(&source_file_id, &links); err != nil {
				return skerr.Wrapf(err, "Failed to scan links data.")
			}
			addMetadata(source_file_id, links)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return fileLinksAggregate, nil
}

var _ tracestore.MetadataStore = (*SQLMetadataStore)(nil)
