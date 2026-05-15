package regrshortcutstore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/perf/go/sql/sqltest"
)

func TestCreate_Success_ReturnsShortcut(t *testing.T) {
	db := sqltest.NewSpannerDBForTests(t, "regrshortcuts")
	store := New(db)
	ctx := context.Background()

	regrIds := []string{"regr1", "regr2"}
	shortcut, err := store.Create(ctx, regrIds)
	require.NoError(t, err)
	assert.NotEmpty(t, shortcut)

	// Verify idempotency or that duplicate creation returns the same shortcut.
	shortcutDuplicate, err := store.Create(ctx, regrIds)
	require.NoError(t, err)
	assert.Equal(t, shortcut, shortcutDuplicate)
}

func TestCreate_DifferentOrderIdList_ReturnsSameShortcut(t *testing.T) {
	db := sqltest.NewSpannerDBForTests(t, "regrshortcuts")
	store := New(db)
	ctx := context.Background()

	regrIds1 := []string{"regr1", "regr2"}
	regrIds2 := []string{"regr2", "regr1"}

	shortcut1, err := store.Create(ctx, regrIds1)
	require.NoError(t, err)

	shortcut2, err := store.Create(ctx, regrIds2)
	require.NoError(t, err)

	assert.Equal(t, shortcut1, shortcut2)
}

func TestGet_Success(t *testing.T) {
	db := sqltest.NewSpannerDBForTests(t, "regrshortcuts")
	store := New(db)
	ctx := context.Background()

	regrIds := []string{"regr1", "regr2"}
	shortcut, err := store.Create(ctx, regrIds)
	require.NoError(t, err)

	isLegacy, fetchedIds, err := store.Get(ctx, shortcut)
	require.NoError(t, err)
	assert.True(t, isLegacy.Valid)
	assert.False(t, isLegacy.Bool)
	assert.ElementsMatch(t, regrIds, fetchedIds)
}

func TestGet_NotFound(t *testing.T) {
	db := sqltest.NewSpannerDBForTests(t, "regrshortcuts")
	store := New(db)
	ctx := context.Background()

	isLegacy, fetchedIds, err := store.Get(ctx, "nonexistent")
	require.NoError(t, err)
	assert.False(t, isLegacy.Valid)
	assert.Empty(t, fetchedIds)
}

func TestCreate_EmptyListFails(t *testing.T) {
	db := sqltest.NewSpannerDBForTests(t, "regrshortcuts")
	store := New(db)
	ctx := context.Background()

	regrIds1 := []string{}

	_, err := store.Create(ctx, regrIds1)
	require.Error(t, err)
}

// Should be caught earlier
func TestCreate_SingleShortcut(t *testing.T) {
	db := sqltest.NewSpannerDBForTests(t, "regrshortcuts")
	store := New(db)
	ctx := context.Background()

	regrIds1 := []string{"this is caught earlier"}

	_, err := store.Create(ctx, regrIds1)
	require.NoError(t, err)
}

// This test documents a possible evil actor scenario.
// We don't guard against it, since it's of minimal impact.
func TestCreate_MD5CollisionsAreNotDetected(t *testing.T) {
	db := sqltest.NewSpannerDBForTests(t, "regrshortcuts")
	store := New(db)
	ctx := context.Background()

	// Below we simulate an action where an evil actor finds and chooses regression ids to cause
	// a collision
	regrIds1 := []string{"hacky"}
	shortcut1, err := store.Create(ctx, regrIds1)
	require.NoError(t, err)
	_, err = db.Exec(ctx, "UPDATE RegressionsShortcuts SET anomaly_ids = '{hackier}' where sid = $1", shortcut1)
	require.NoError(t, err)

	// Note how the old record has a different regr ids list
	shortcut2, err := store.Create(ctx, regrIds1)
	// The problem is not detected, we return a graph with different regressions.
	assert.Equal(t, shortcut1, shortcut2)
}
