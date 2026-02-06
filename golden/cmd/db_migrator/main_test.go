package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	cols := []string{"changelist_id", "system", "status"}
	row := []interface{}{"gerrit_123", "gerrit", "open"}

	t.Run("basic", func(t *testing.T) {
		orderByCols := []string{"changelist_id"}
		vals := extractProgressValues(row, cols, orderByCols)
		require.Len(t, vals, 1)
		assert.Equal(t, "gerrit_123", vals[0])
	})

	t.Run("multiple", func(t *testing.T) {
		orderByCols := []string{"system", "changelist_id"}
		vals := extractProgressValues(row, cols, orderByCols)
		require.Len(t, vals, 2)
		assert.Equal(t, "gerrit", vals[0])
		assert.Equal(t, "gerrit_123", vals[1])
	})
}
