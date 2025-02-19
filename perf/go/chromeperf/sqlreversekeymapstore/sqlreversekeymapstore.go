package sqlreversekeymapstore

import (
	"context"

	"github.com/jackc/pgx/v4"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sql/pool"
)

type statement int

const (
	getOriginalValue statement = iota
	createKeyMap
)

var statements = map[statement]string{
	getOriginalValue: `
		SELECT
			original_value
		FROM
			ReverseKeyMap
		WHERE
			modified_value =$1 AND param_key=$2
	`,
	createKeyMap: `
		INSERT INTO
			ReverseKeyMap (modified_value, param_key, original_value)
		VALUES
			($1, $2, $3)
		ON CONFLICT DO NOTHING
		RETURNING
			original_value
	`,
}

type ReverseKeyMapStoreImpl struct {
	db pool.Pool
}

// New returns a new *ReverseKeyMapStoreImpl.
func New(db pool.Pool) *ReverseKeyMapStoreImpl {
	return &ReverseKeyMapStoreImpl{
		db: db,
	}
}

// Use the updated value and the param key to retrieve the original value.
func (s *ReverseKeyMapStoreImpl) Get(ctx context.Context, modifiedValue string, key string) (string, error) {
	if modifiedValue == "" || key == "" {
		return "", skerr.Fmt("No value or key is given to get reverse key mapping. Key: %s, Value: %s", key, modifiedValue)
	}

	var originalValue string
	if err := s.db.QueryRow(ctx, statements[getOriginalValue], modifiedValue, key).Scan(&originalValue); err != nil {
		if err == pgx.ErrNoRows {
			// no old value is found.
			return "", nil
		}
		return "", skerr.Wrapf(err, "Failed to finish querying on old value for value: %s and key: %s", modifiedValue, key)
	}
	return originalValue, nil
}

// Create a new mapping from updated value and param key to the original value.
func (s *ReverseKeyMapStoreImpl) Create(ctx context.Context, modifiedValue string, key string, originalValue string) (string, error) {
	if modifiedValue == "" || key == "" || originalValue == "" {
		return "", skerr.Fmt("Value, key and old value are all required for reverse key mapping. Value: %s. Key: %s. Old value: %s", modifiedValue, key, originalValue)
	}
	returnedValue := ""
	if err := s.db.QueryRow(ctx, statements[createKeyMap], modifiedValue, key, originalValue).Scan(&returnedValue); err != nil {
		if err == pgx.ErrNoRows {
			// No row is created. Collision.
			return "", nil
		}
		return "", skerr.Wrapf(err, "Failed to create new reverse key map from value: %s, key: %s, to old value: %s", modifiedValue, key, originalValue)
	}
	return returnedValue, nil
}
