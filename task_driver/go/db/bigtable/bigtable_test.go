package bigtable

import (
	"context"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/bt"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/task_driver/go/db"
)

func setup(t *testing.T) (db.DB, func()) {
	testutils.LargeTest(t)
	project := "test-project"
	instance := "test-instance"

	// Set up the table and column families.
	assert.NoError(t, bt.InitBigtable(project, instance, bt.TableConfig{
		BT_TABLE: {
			BT_COLUMN_FAMILY,
		},
	}))

	d, err := NewBigTableDB(context.Background(), project, instance, nil)
	assert.NoError(t, err)
	return d, func() {
		testutils.AssertCloses(t, d)
	}
}

func TestBigTableDB(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()
	db.TestDB(t, d)
}

func TestBigTableDBMessageOrdering(t *testing.T) {
	d, cleanup := setup(t)
	defer cleanup()
	db.TestMessageOrdering(t, d)
}
