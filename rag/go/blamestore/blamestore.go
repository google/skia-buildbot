package blamestore

import (
	"context"

	"cloud.google.com/go/spanner"
	"github.com/google/uuid"
	"go.skia.org/infra/go/skerr"
)

const (
	// spannerMutationLimit is the maximum number of mutations per commit.
	// From https://cloud.google.com/spanner/quotas#limits_for_creating_reading_updating_and_deleting_data
	// The official limit is 20,000. We use a slightly smaller number to be safe.
	spannerMutationLimit = 19000
)

// BlameStore defines an interface for interacting with the database for any blame data.
type BlameStore interface {
	// WriteBlame writes the blame data into the database.
	WriteBlame(ctx context.Context, blame *FileBlame) error

	// ReadBlame reads the blame information for the given file path.
	ReadBlame(ctx context.Context, filePath string) (*FileBlame, error)
}

type blameStoreImpl struct {
	// spannerClient is used to insert data into Spanner.
	spannerClient *spanner.Client
}

// WriteBlame writes the blame data into the database.
func (b *blameStoreImpl) WriteBlame(ctx context.Context, blame *FileBlame) error {
	_, err := b.spannerClient.ReadWriteTransaction(ctx, func(ctx context.Context, rwt *spanner.ReadWriteTransaction) error {
		// Check if the file already exists.
		stmt := spanner.NewStatement("SELECT id FROM BlamedFiles WHERE file_path = @filePath")
		stmt.Params["filePath"] = blame.FilePath
		var existingID string
		err := rwt.Query(ctx, stmt).Do(func(r *spanner.Row) error {
			return r.ColumnByName("id", &existingID)
		})
		if err != nil {
			return skerr.Wrap(err)
		}
		var mutations []*spanner.Mutation
		var blamedFileID string
		if existingID == "" {
			// Not found, create new.
			blamedFileID = uuid.New().String()
			m := spanner.InsertMap("BlamedFiles", map[string]interface{}{
				"id":           blamedFileID,
				"file_path":    blame.FilePath,
				"file_hash":    blame.FileHash,
				"version":      blame.Version,
				"commit_hash":  blame.CommitHash,
				"last_updated": spanner.CommitTimestamp,
			})
			mutations = append(mutations, m)
		} else {
			// Found, update.
			blamedFileID = existingID
			// Delete old line blames.
			mutations = append(mutations, spanner.Delete("LineBlames", spanner.KeyRange{
				Start: spanner.Key{blamedFileID},
				End:   spanner.Key{blamedFileID},
				Kind:  spanner.ClosedClosed,
			}))
			m := spanner.UpdateMap("BlamedFiles", map[string]interface{}{
				"id":           blamedFileID,
				"file_path":    blame.FilePath,
				"file_hash":    blame.FileHash,
				"version":      blame.Version,
				"commit_hash":  blame.CommitHash,
				"last_updated": spanner.CommitTimestamp,
			})
			mutations = append(mutations, m)
		}

		// Insert new line blames.
		for _, lineBlame := range blame.LineBlames {
			m := spanner.InsertMap("LineBlames", map[string]interface{}{
				"id":          blamedFileID,
				"blamed_file": blame.FilePath,
				"line_number": lineBlame.LineNumber,
				"commit_hash": lineBlame.CommitHash,
			})
			mutations = append(mutations, m)
			if len(mutations) >= spannerMutationLimit {
				if err := rwt.BufferWrite(mutations); err != nil {
					return skerr.Wrap(err)
				}
				mutations = nil
			}
		}

		if len(mutations) > 0 {
			return rwt.BufferWrite(mutations)
		}
		return nil
	})
	return err
}

// ReadBlame returns the file blame data for the file path provided.
func (b *blameStoreImpl) ReadBlame(ctx context.Context, filePath string) (*FileBlame, error) {
	ret := &FileBlame{
		FilePath: filePath,
	}
	stmt := spanner.NewStatement(`
		SELECT
			t1.file_hash,
			t1.version,
			t1.commit_hash,
			t2.line_number,
			t2.commit_hash AS line_commit_hash
		FROM BlamedFiles AS t1
		LEFT JOIN LineBlames AS t2 ON t1.id = t2.id
		WHERE t1.file_path = @filePath
	`)
	stmt.Params["filePath"] = filePath
	var fileBlamePopulated bool
	err := b.spannerClient.Single().Query(ctx, stmt).Do(func(r *spanner.Row) error {
		if !fileBlamePopulated {
			if err := r.ColumnByName("file_hash", &ret.FileHash); err != nil {
				return skerr.Wrap(err)
			}
			if err := r.ColumnByName("version", &ret.Version); err != nil {
				return skerr.Wrap(err)
			}
			if err := r.ColumnByName("commit_hash", &ret.CommitHash); err != nil {
				return skerr.Wrap(err)
			}
			fileBlamePopulated = true
		}

		var lineNumber spanner.NullInt64
		if err := r.ColumnByName("line_number", &lineNumber); err != nil {
			return skerr.Wrap(err)
		}
		var lineCommitHash spanner.NullString
		if err := r.ColumnByName("line_commit_hash", &lineCommitHash); err != nil {
			return skerr.Wrap(err)
		}

		if lineNumber.Valid && lineCommitHash.Valid {
			lb := &LineBlame{
				LineNumber: lineNumber.Int64,
				CommitHash: lineCommitHash.StringVal,
			}
			ret.LineBlames = append(ret.LineBlames, lb)
		}
		return nil
	})

	if err != nil {
		return nil, skerr.Wrap(err)
	}

	return ret, nil
}

// New returns a new BlameStore instance.
func New(spannerClient *spanner.Client) BlameStore {
	return &blameStoreImpl{
		spannerClient: spannerClient,
	}
}
