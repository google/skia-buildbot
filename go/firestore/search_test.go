package firestore

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestSearch(t *testing.T) {
	unittest.LargeTest(t)

	// Setup.
	sklog.Errorf("Start setup.")
	ctx := context.Background()
	c, cleanup := NewClientForTesting(ctx, t)
	defer cleanup()
	coll := c.Collection("testSearch")
	sklog.Errorf("Finished setup.")

	// Insert data to search.
	type searchable struct {
		Name      string
		Timestamp time.Time
		Items     []string
	}
	t1 := FixTimestamp(time.Unix(1598548300, 0))
	d1, _, err := coll.Add(ctx, searchable{
		Name:      "Item 1",
		Timestamp: t1,
		Items:     []string{"a", "b", "c"},
	})
	require.NoError(t, err)
	t2 := FixTimestamp(time.Unix(1598548500, 0))
	d2, _, err := coll.Add(ctx, searchable{
		Name:      "Item 2",
		Timestamp: t2,
		Items:     []string{"d", "e", "f"},
	})
	require.NoError(t, err)
	t3 := FixTimestamp(time.Unix(1598548800, 0))
	d3, _, err := coll.Add(ctx, searchable{
		Name:      "Item 3",
		Timestamp: t3,
		Items:     []string{"c", "d", "e"},
	})
	require.NoError(t, err)

	// Helper function which performs a search and asserts that the expected
	// results were obtained.
	search := func(path, op string, value interface{}, expect ...*firestore.DocumentRef) {
		// We'd prefer to order by timestamp, but Firestore doesn't allow range
		// queries with orderBy on a different field. Unfortunately, instead of
		// returning a helpful error message, it just gives an empty set of
		// results.
		orderBy := "Timestamp"
		if path == "Name" {
			orderBy = path
		}
		cursor, res, err := Search(ctx, coll.Query.OrderBy(orderBy, firestore.Asc), 0, "", Where(path, op, value))
		require.NoError(t, err)
		require.Equal(t, "", cursor)
		require.Equal(t, len(expect), len(res))
		for i, expectRef := range expect {
			require.Equal(t, expectRef.ID, res[i].Ref.ID)
		}
	}

	// Basic test cases for the various operators.
	search("Name", "==", "Item 1", d1)
	search("Name", "<", "Item 3", d1, d2)
	search("Timestamp", "<", t3, d1, d2)
	search("Timestamp", "<=", t3, d1, d2, d3)
	search("Timestamp", ">", t3)
	search("Timestamp", ">=", t3, d3)
	search("Items", "array-contains", "c", d1, d3)
	search("Items", "array-contains-any", []string{"b", "f"}, d1, d2)
	search("Name", "in", []string{"Item 1", "Item 2"}, d1, d2)

	// Test pagination.
	cursor, res, err := Search(ctx, coll.Query.OrderBy("Timestamp", firestore.Asc), 2, "")
	require.NoError(t, err)
	require.Equal(t, d2.ID, cursor)
	require.Equal(t, 2, len(res))
	require.Equal(t, d1.ID, res[0].Ref.ID)
	require.Equal(t, d2.ID, res[1].Ref.ID)

	cursor, res, err = Search(ctx, coll.Query.OrderBy("Timestamp", firestore.Asc), 2, cursor)
	require.NoError(t, err)
	require.Equal(t, "", cursor)
	require.Equal(t, 1, len(res))
	require.Equal(t, d3.ID, res[0].Ref.ID)
}
