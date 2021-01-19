package sqltjstore

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils/unittest"
	ci "go.skia.org/infra/golden/go/continuous_integration"
	"go.skia.org/infra/golden/go/sql/sqltest"
	"go.skia.org/infra/golden/go/tjstore"
)

func TestGetTryJob_OnlyExistsAfterPut(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	store := New(db)

	const cis = "buildbucket"

	expectedID := "987654"
	psID := tjstore.CombinedPSID{
		CL:  "1234",
		CRS: "github",
		PS:  "abcd",
	}

	// Should not exist initially
	_, err := store.GetTryJob(ctx, expectedID, cis)
	require.Error(t, err)
	assert.Equal(t, tjstore.ErrNotFound, err)

	tj := ci.TryJob{
		System:      cis,
		SystemID:    expectedID,
		DisplayName: "My-Test",
		Updated:     time.Date(2019, time.August, 13, 12, 11, 10, 0, time.UTC),
	}

	err = store.PutTryJob(ctx, psID, tj)
	require.NoError(t, err)

	actual, err := store.GetTryJob(ctx, expectedID, cis)
	require.NoError(t, err)
	assert.Equal(t, tj, actual)

	// TODO(kjlubick) verify SQL in DB
}
