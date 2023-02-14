package status

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/firestore/testutils"
)

func TestFirestoreDB(t *testing.T) {
	ctx := context.Background()
	client, cleanup := testutils.NewClientForTesting(ctx, t)
	defer cleanup()
	db, err := NewFirestoreDB(ctx, client)
	require.NoError(t, err)
	testDB(t, db)
}
