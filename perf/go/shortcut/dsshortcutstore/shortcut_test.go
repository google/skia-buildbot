package dsshortcutstore

import (
	"bytes"
	"context"
	"encoding/json"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/shortcut"
)

func TestInsertGet(t *testing.T) {
	unittest.LargeTest(t)
	cleanup := testutil.InitDatastore(t, ds.SHORTCUT)

	defer cleanup()

	store := New()

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
	assert.NoError(t, err)
	assert.NotEqual(t, "", id)

	// Read it back, confirm it is unchanged, except for being sorted.
	sh2, err := store.Get(ctx, id)
	assert.NoError(t, err)
	assert.NotEqual(t, sh, sh2)
	sort.Strings(sh.Keys)
	assert.Equal(t, sh, sh2)
}
