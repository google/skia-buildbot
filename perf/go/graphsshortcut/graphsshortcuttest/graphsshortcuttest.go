// Package graphsshortcuttest has common code for tests of implementations of
// graphsshortcut.Store.
package graphsshortcuttest

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/perf/go/graphsshortcut"
)

// GraphsShortcut_InsertGet does the core testing of an instance of graphsshortcut.Store.
func InsertGet(t *testing.T, store graphsshortcut.Store) {
	ctx := context.Background()
	// Write a shortcut, make sure the queries are out of sorted order.
	sh := &graphsshortcut.GraphsShortcut{
		Graphs: []graphsshortcut.GraphConfig{
			{
				Queries: []string{
					"arch=x86&config=8888",
					"arch=arm&config=8888",
				},
				Keys: "abcdef",
			},
		},
	}

	id, err := store.InsertShortcut(ctx, sh)
	require.NoError(t, err)
	assert.NotEqual(t, "", id)

	// Read it back, confirm it is unchanged, except for being sorted.
	sh2, err := store.GetShortcut(ctx, id)
	require.NoError(t, err)
	sort.Strings(sh.Graphs[0].Queries)
	assert.Equal(t, sh, sh2)
}

// GraphsShortcut_GetNonExistent tests that we fail when retrieving an unknown
// shortcut.
func GetNonExistent(t *testing.T, store graphsshortcut.Store) {
	ctx := context.Background()

	_, err := store.GetShortcut(ctx, "X-unknown")
	require.Error(t, err)
}

// SubTestFunction is a func we will call to test one aspect of an
// implementation of graphsshortcut.Store.
type SubTestFunction func(t *testing.T, store graphsshortcut.Store)

// SubTests are all the subtests we have for graphsshortcut.Store.
var SubTests = map[string]SubTestFunction{
	"GraphsShortcut_InsertGet":      InsertGet,
	"GraphsShortcut_GetNonExistent": GetNonExistent,
}
