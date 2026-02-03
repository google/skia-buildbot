package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/golden/go/sql/schema"
)

func TestBuildQuery(t *testing.T) {
	tableName := "Changelists"
	batchSize := 10

	t.Run("no progress, with createdat", func(t *testing.T) {
		orderByCols := []string{"createdat", "changelist_id"}
		q := buildQuery(tableName, true, orderByCols, nil, batchSize)
		assert.Equal(t, "SELECT * FROM changelists ORDER BY createdat, changelist_id LIMIT 10", q)
	})

	t.Run("no progress, without createdat", func(t *testing.T) {
		orderByCols := []string{"changelist_id"}
		q := buildQuery(tableName, false, orderByCols, nil, batchSize)
		assert.Equal(t, "SELECT *, '0001-01-01 00:00:00+00'::TIMESTAMPTZ as createdat FROM changelists ORDER BY changelist_id LIMIT 10", q)
	})

	t.Run("with progress", func(t *testing.T) {
		orderByCols := []string{"createdat", "changelist_id"}
		lastValues := []interface{}{time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC), "gerrit_123"}
		q := buildQuery(tableName, true, orderByCols, lastValues, batchSize)
		assert.Equal(t, "SELECT * FROM changelists WHERE (createdat, changelist_id) > ($1, $2) ORDER BY createdat, changelist_id LIMIT 10", q)
	})
}

func TestExtractProgressValues(t *testing.T) {
	row := schema.ChangelistRow{
		ChangelistID: "gerrit_123",
		System:       "gerrit",
		CreatedAt:    time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	t.Run("with createdat", func(t *testing.T) {
		orderByCols := []string{"createdat", "changelist_id"}
		vals := extractProgressValues(row, orderByCols)
		require.Len(t, vals, 2)
		assert.Equal(t, row.CreatedAt, vals[0])
		assert.Equal(t, row.ChangelistID, vals[1])
	})

	t.Run("without createdat", func(t *testing.T) {
		orderByCols := []string{"changelist_id"}
		vals := extractProgressValues(row, orderByCols)
		require.Len(t, vals, 1)
		assert.Equal(t, row.ChangelistID, vals[0])
	})
}

func TestGetRowType(t *testing.T) {
	assert.NotNil(t, getRowType("Changelists"))
	assert.NotNil(t, getRowType("TiledTraceDigests"))
	assert.Nil(t, getRowType("NonExistentTable"))
}
