// Package shortcuttest has common code for tests of implementations of
// shortcut.Store.
package shortcuttest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/perf/go/shortcut"
)

// Shortcut_InsertGet does the core testing of an instance of shortcut.Store.
func Shortcut_InsertGet(t *testing.T, store shortcut.Store) {
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

func readAll(ch <-chan *shortcut.Shortcut) []*shortcut.Shortcut {
	ret := []*shortcut.Shortcut{}
	for s := range ch {
		ret = append(ret, s)
	}
	return ret
}

func Shortcut_GetAll(t *testing.T, store shortcut.Store) {
	ctx := context.Background()
	// Write a shortcuts.
	for i := 0; i < 12; i++ {
		sh := &shortcut.Shortcut{
			Keys: []string{
				fmt.Sprintf("https://foo/%d", i),
			},
		}
		_, err := store.InsertShortcut(ctx, sh)
		require.NoError(t, err)
	}
	ch, err := store.GetAll(ctx)
	require.NoError(t, err)
	all := readAll(ch)
	assert.Len(t, all, 12)
	assert.Equal(t, "https://foo/0", all[0].Keys[0])
}

// SubTestFunction is a func we will call to test one aspect of an
// implementation of regression.Store.
type SubTestFunction func(t *testing.T, store shortcut.Store)

// SubTests are all the subtests we have for regression.Store.
var SubTests = map[string]SubTestFunction{
	"Shortcut_GetAll":    Shortcut_GetAll,
	"Shortcut_InsertGet": Shortcut_InsertGet,
}
