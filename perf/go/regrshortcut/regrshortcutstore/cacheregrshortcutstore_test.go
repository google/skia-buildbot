package regrshortcutstore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/cache/local"
)

func TestCacheCreateAndGet_Success(t *testing.T) {
	ctx := context.Background()
	cacheClient, err := local.New(1000)
	require.NoError(t, err)
	store := NewCacheRegressionsShortcutStore(cacheClient)

	regrIds := []string{"regr1", "regr2"}
	shortcut, err := store.Create(ctx, regrIds)
	require.NoError(t, err)
	assert.NotEmpty(t, shortcut)

	isLegacy, fetchedIds, err := store.Get(ctx, shortcut)
	require.NoError(t, err)
	assert.True(t, isLegacy.Valid)
	assert.False(t, isLegacy.Bool)
	assert.ElementsMatch(t, regrIds, fetchedIds)
}

func TestCacheGet_NotFound(t *testing.T) {
	ctx := context.Background()
	cacheClient, err := local.New(1000)
	require.NoError(t, err)
	store := NewCacheRegressionsShortcutStore(cacheClient)

	isLegacy, fetchedIds, err := store.Get(ctx, "nonexistent")
	require.NoError(t, err)
	assert.False(t, isLegacy.Valid)
	assert.Empty(t, fetchedIds)
}

func TestCacheCreate_EmptyListFails(t *testing.T) {
	ctx := context.Background()
	cacheClient, err := local.New(1000)
	require.NoError(t, err)
	store := NewCacheRegressionsShortcutStore(cacheClient)

	_, err = store.Create(ctx, []string{})
	require.Error(t, err)
}
