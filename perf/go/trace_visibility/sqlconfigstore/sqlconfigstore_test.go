package sqlconfigstore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/perf/go/sql/sqltest"
)

func TestSQLConfigStore_SetGetDelete(t *testing.T) {
	ctx := context.Background()
	db := sqltest.NewSpannerDBForTests(t, "tracevisibility")
	store := New(db)

	// Initially empty.
	configs, err := store.GetAll(ctx)
	require.NoError(t, err)
	assert.Empty(t, configs)

	// Set a config.
	scope1 := "bot=android-pixel6-perf"
	err = store.Set(ctx, scope1)
	assert.NoError(t, err)

	// Verify it's there.
	configs, err = store.GetAll(ctx)
	require.NoError(t, err)
	assert.Len(t, configs, 1)
	assert.Equal(t, scope1, configs[0].RuleExpression)

	// Set another config.
	scope2 := "benchmark=motionmark"
	err = store.Set(ctx, scope2)
	assert.NoError(t, err)

	// Verify both are there.
	configs, err = store.GetAll(ctx)
	require.NoError(t, err)
	assert.Len(t, configs, 2)

	// Update first config.
	err = store.Set(ctx, scope1)
	assert.NoError(t, err)

	// Still 2 configs.
	configs, err = store.GetAll(ctx)
	require.NoError(t, err)
	assert.Len(t, configs, 2)

	// Delete one.
	err = store.Delete(ctx, scope1)
	assert.NoError(t, err)

	// Only one left.
	configs, err = store.GetAll(ctx)
	require.NoError(t, err)
	assert.Len(t, configs, 1)
	assert.Equal(t, scope2, configs[0].RuleExpression)

	// Delete remaining.
	err = store.Delete(ctx, scope2)
	assert.NoError(t, err)

	// Empty again.
	configs, err = store.GetAll(ctx)
	require.NoError(t, err)
	assert.Empty(t, configs)
}
