package status

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/skerr"
)

type TestDB map[string]*AutoRollStatus

func NewTestDB() TestDB {
	return map[string]*AutoRollStatus{}
}

func (d TestDB) Close() error {
	return nil
}

func (d TestDB) Get(ctx context.Context, rollerName string) (*AutoRollStatus, error) {
	rv, ok := d[rollerName]
	if !ok {
		return nil, skerr.Fmt("not found")
	}
	return rv, nil
}

func (d TestDB) Set(ctx context.Context, rollerName string, st *AutoRollStatus) error {
	d[rollerName] = st
	return nil
}

func TestMultiDB(t *testing.T) {
	db, err := NewMultiDB([]DB{NewTestDB(), NewTestDB(), NewTestDB()})
	require.NoError(t, err)
	testDB(t, db)
}
