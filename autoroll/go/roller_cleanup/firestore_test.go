package roller_cleanup

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/firestore/testutils"
)

func TestFirestoreDB_NeedsCleanup(t *testing.T) {
	c, cleanup := testutils.NewClientForTesting(context.Background(), t)
	defer cleanup()
	db, err := NewDB(context.Background(), c)
	require.NoError(t, err)
	testNeedsCleanup(t, db)
}

func TestFirestoreDB_History(t *testing.T) {
	c, cleanup := testutils.NewClientForTesting(context.Background(), t)
	defer cleanup()
	db, err := NewDB(context.Background(), c)
	require.NoError(t, err)
	testDB_History(t, db)
}
