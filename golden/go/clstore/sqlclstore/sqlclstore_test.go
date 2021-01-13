package sqlclstore

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/clstore"
	"go.skia.org/infra/golden/go/code_review"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/sqltest"
)

func TestPutChangelist_NotExistantCL_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTests(ctx, t)
	sqltest.CreateProductionSchema(ctx, t, db)
	store := New(db, "gerrit")

	const unqualifiedID = "987654"

	// Should not exist initially
	_, err := store.GetChangelist(ctx, unqualifiedID)
	require.Error(t, err)
	require.Equal(t, clstore.ErrNotFound, err)

	cl := code_review.Changelist{
		SystemID: unqualifiedID,
		Owner:    "test@example.com",
		Status:   code_review.Abandoned,
		Subject:  "some code",
		Updated:  time.Date(2019, time.August, 13, 12, 11, 10, 0, time.UTC),
	}

	err = store.PutChangelist(ctx, cl)
	require.NoError(t, err)

	actual, err := store.GetChangelist(ctx, unqualifiedID)
	require.NoError(t, err)
	assert.Equal(t, cl, actual)

	// Check the SQL directly so we can trust GetChangelist in other tests.
	row := db.QueryRow(ctx, `SELECT * FROM Changelists LIMIT 1`)
	var r schema.ChangelistRow
	require.NoError(t, row.Scan(&r.ChangelistID, &r.System, &r.Status, &r.OwnerEmail, &r.Subject, &r.LastIngestedData))
	r.LastIngestedData = r.LastIngestedData.UTC()
	assert.Equal(t, schema.ChangelistRow{
		ChangelistID:     "gerrit_987654",
		System:           "gerrit",
		Status:           schema.StatusAbandoned,
		OwnerEmail:       "test@example.com",
		Subject:          "some code",
		LastIngestedData: time.Date(2019, time.August, 13, 12, 11, 10, 0, time.UTC),
	}, r)
}
