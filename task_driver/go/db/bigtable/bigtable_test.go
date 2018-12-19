package bigtable

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/bt"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/task_driver/go/db"
)

func setup(t *testing.T) (db.DB, func()) {
	testutils.LargeTest(t)
	project := "test-project"
	instance := fmt.Sprintf("test-instance-%s", uuid.New())

	// Set up the table and column families.
	assert.NoError(t, bt.InitBigtable(project, instance, bt.TableConfig{
		BT_TABLE: {
			BT_COLUMN_FAMILY,
		},
	}))

	bt.PROJECT_FOR_INSTANCE[instance] = project
	d, err := NewBigTableDB(context.Background(), instance, nil)
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
