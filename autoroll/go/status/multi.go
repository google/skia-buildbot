package status

import (
	"context"

	"go.skia.org/infra/go/skerr"
)

// MultiDB combines two more more DBs.
type MultiDB []DB

// NewMultiDB returns a MultiDB instance. The first DB instance is used for all
// retrievals.
func NewMultiDB(dbs []DB) (MultiDB, error) {
	if len(dbs) < 1 {
		return nil, skerr.Fmt("At least one DB must be provided.")
	}
	return MultiDB(dbs), nil
}

// Close implements DB.
func (d MultiDB) Close() error {
	var rvErr error
	for _, db := range d {
		if err := db.Close(); err != nil {
			rvErr = skerr.Wrap(err)
		}
	}
	return rvErr
}

// Get implements DB.
func (d MultiDB) Get(ctx context.Context, rollerID string) (*AutoRollStatus, error) {
	return d[0].Get(ctx, rollerID)
}

// Set implements DB.
func (d MultiDB) Set(ctx context.Context, rollerID string, st *AutoRollStatus) error {
	var rvErr error
	for _, db := range d {
		if err := db.Set(ctx, rollerID, st); err != nil {
			rvErr = skerr.Wrap(err)
		}
	}
	return rvErr
}

var _ DB = MultiDB{}
