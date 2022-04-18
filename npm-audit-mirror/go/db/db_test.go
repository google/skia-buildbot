package db

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/firestore/testutils"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/testutils/unittest"
)

const (
	testDBKey     = "test_key"
	testIssueName = "projects/test_project/issues/100"
)

// newDBClientForTesting returns a FirestoreDB client and ensures that it will
// connect to the Firestore emulator. The Client's instance name will be
// randomized to ensure concurrent tests don't interfere with each other. It also
// returns a CleanupFunc that closes the Client.
func newDBClientForTesting(ctx context.Context, t sktest.TestingT) *FirestoreDB {
	c, cleanup := testutils.NewClientForTesting(ctx, t)
	t.Cleanup(cleanup)
	return &FirestoreDB{
		client: c,
	}
}

func TestGetFromDB_EmptyResults_NoErrors(t *testing.T) {
	unittest.LargeTest(t)
	ctx := context.Background()
	db := newDBClientForTesting(ctx, t)

	// DB should be empty.
	cherrypickData, err := db.GetFromDB(ctx, testDBKey)
	require.Nil(t, cherrypickData)
	require.NoError(t, err)
}

func TestPutInDB_OneEntry_ExpectedChangeNum(t *testing.T) {
	unittest.LargeTest(t)
	db := newDBClientForTesting(context.Background(), t)
	ctx := context.Background()
	created := time.Now().UTC()

	// DB should be empty. Add one entry.
	err := db.PutInDB(ctx, testDBKey, testIssueName, created)
	require.NoError(t, err)

	// Get the added entry and assert.
	npmAuditData, err := db.GetFromDB(ctx, testDBKey)
	require.Equal(t, testIssueName, npmAuditData.IssueName)
	require.Equal(t, created.Format("2006-01-02"), npmAuditData.Created.Format("2006-01-02"))
	require.NoError(t, err)
}
