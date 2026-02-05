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
	expectedCols := "changelist_id, system, status, owner_email, subject, last_ingested_data"

	t.Run("no progress", func(t *testing.T) {
		orderByCols := []string{"changelist_id"}
		q := buildQuery(tableName, orderByCols, nil, batchSize)
		assert.Equal(t, "SELECT "+expectedCols+" FROM changelists ORDER BY changelist_id LIMIT 10", q)
	})

	t.Run("with progress", func(t *testing.T) {
		orderByCols := []string{"changelist_id"}
		lastValues := []interface{}{"gerrit_123"}
		q := buildQuery(tableName, orderByCols, lastValues, batchSize)
		assert.Equal(t, "SELECT "+expectedCols+" FROM changelists WHERE (changelist_id) > ($1) ORDER BY changelist_id LIMIT 10", q)
	})
}

func TestExtractProgressValues(t *testing.T) {
	row := schema.ChangelistRow{
		ChangelistID: "gerrit_123",
		System:       "gerrit",
		CreatedAt:    time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	t.Run("basic", func(t *testing.T) {
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
