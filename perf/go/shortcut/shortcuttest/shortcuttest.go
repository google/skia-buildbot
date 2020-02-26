// Package shortcuttest has common code for tests of implementations of
// shortcut.Store.
package shortcuttest

import (
	"bytes"
	"context"
	"encoding/json"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/perf/go/shortcut"
)

// InsertGet does the core testing of an instance of shortcut.Store.
func InsertGet(t *testing.T, store shortcut.Store) {
	ctx := context.Background()
	// Write a shortcut.
	sh := &shortcut.Shortcut{
		Keys: []string{
			"https://foo",
			"https://bar",
			"https://baz",
		},
	}
	b, err := json.Marshal(sh)
	buf := bytes.NewBuffer(b)
	id, err := store.Insert(ctx, buf)
	require.NoError(t, err)
	assert.NotEqual(t, "", id)

	// Read it back, confirm it is unchanged, except for being sorted.
	sh2, err := store.Get(ctx, id)
	require.NoError(t, err)
	assert.NotEqual(t, sh, sh2)
	sort.Strings(sh.Keys)
	assert.Equal(t, sh, sh2)
}
