package data_manager

import (
	"context"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/go/skerr"
)

type statement int

const (
	getPositiveExpectationsToUpdateExpiry statement = iota
	updateExpectationsExpiry
)

// statements contains the SQL statements used for expiration monitoring.
var statements = map[statement]string{
	getPositiveExpectationsToUpdateExpiry: `
		SELECT encode(grouping_id, 'hex'), encode(digest, 'hex') FROM Expectations
		WHERE label = 'p' and expire_at <= $1
		ORDER BY expire_at LIMIT $2
	`,
	updateExpectationsExpiry: `
		UPDATE Expectations
		SET expire_at = $3
		WHERE grouping_id = decode($1, 'hex') AND digest = decode($2, 'hex')
	`,
}

// ExpectationKey provides a struct representing the key to an Expiration table row.
type ExpectationKey struct {
	Groupingid string
	Digest     string
}

// ExpiryDataManager provides an interface to manage data related to expirations.
type ExpiryDataManager interface {
	// GetExpiringExpectations returns expectations about to expire.
	GetExpiringExpectations(ctx context.Context) ([]ExpectationKey, error)

	// UpdateExpectationsExpiry updates the expiry for the provided expectations to the expirationTime.
	UpdateExpectationsExpiry(ctx context.Context, expectations []ExpectationKey, expirationTime time.Time) error
}

// NewExpiryDataManager returns a new instance of the ExpiryDataManager interface.
func NewExpiryDataManager(db *pgxpool.Pool, batchSize int) ExpiryDataManager {
	return &expiryDataManagerImpl{
		db:        db,
		batchSize: batchSize,
	}
}

type expiryDataManagerImpl struct {
	db        *pgxpool.Pool
	batchSize int
}

// GetExpiringExpectations returns expectations about to expire.
func (m *expiryDataManagerImpl) GetExpiringExpectations(ctx context.Context) ([]ExpectationKey, error) {
	oneMonthFromNow := time.Now().AddDate(0, 1, 0)
	rows, err := m.db.Query(ctx, statements[getPositiveExpectationsToUpdateExpiry], oneMonthFromNow, m.batchSize)
	if err != nil {
		return nil, skerr.Wrapf(err, "Error retrieving Expectations about to expire.")
	}

	rowsToUpdate := []ExpectationKey{}
	for rows.Next() {
		var grouping_id string
		var digest string
		if err := rows.Scan(&grouping_id, &digest); err != nil {
			return nil, skerr.Wrapf(err, "Failed to read expectation")
		}
		rowsToUpdate = append(rowsToUpdate, ExpectationKey{Groupingid: grouping_id, Digest: digest})
	}

	return rowsToUpdate, nil
}

// UpdateExpectationsExpiry updates the expiry for the provided expectations to the expirationTime.
func (m *expiryDataManagerImpl) UpdateExpectationsExpiry(ctx context.Context, expectations []ExpectationKey, expirationTime time.Time) error {
	for _, expectation := range expectations {
		_, err := m.db.Exec(ctx, statements[updateExpectationsExpiry], expectation.Groupingid, expectation.Digest, expirationTime)
		if err != nil {
			return skerr.Wrapf(err, "Error updating expiration for grouping: %s, digest: %s", expectation.Groupingid, expectation.Digest)
		}
	}

	return nil
}
