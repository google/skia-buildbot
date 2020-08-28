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
	q := NewQuery(coll)
	search := func(q Query, expect ...*firestore.DocumentRef) {
		res, newQ, err := q.Search(ctx)
		require.NoError(t, err)
		require.Equal(t, len(expect), len(res))
		for i, expectRef := range expect {
			require.Equal(t, expectRef.ID, res[i].Ref.ID)
		}
		require.True(t, newQ.Done())
		require.Equal(t, "", newQ.GetCursor())
	}

	// Basic test cases for the various operators.
	search(q.Where("Name", "==", "Item 1").OrderBy("Name", firestore.Asc), d1)
	search(q.Where("Name", "<", "Item 3").OrderBy("Name", firestore.Asc), d1, d2)
	search(q.Where("Timestamp", "<", t3).OrderBy("Timestamp", firestore.Asc), d1, d2)
	search(q.Where("Timestamp", "<=", t3).OrderBy("Timestamp", firestore.Asc), d1, d2, d3)
	search(q.Where("Timestamp", ">", t3).OrderBy("Timestamp", firestore.Asc))
	search(q.Where("Timestamp", ">=", t3).OrderBy("Timestamp", firestore.Asc), d3)
	search(q.Where("Items", "array-contains", "c").OrderBy("Timestamp", firestore.Asc), d1, d3)
	search(q.Where("Items", "array-contains-any", []string{"b", "f"}).OrderBy("Timestamp", firestore.Asc), d1, d2)
	search(q.Where("Name", "in", []string{"Item 1", "Item 2"}).OrderBy("Timestamp", firestore.Asc), d1, d2)

	// Test pagination.
	res, q, err := q.OrderBy("Timestamp", firestore.Asc).Limit(2).Search(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, len(res))
	require.Equal(t, d1.ID, res[0].Ref.ID)
	require.Equal(t, d2.ID, res[1].Ref.ID)
	require.False(t, q.Done())
	require.NotEqual(t, "", q.GetCursor())

	res, q, err = q.Search(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, len(res))
	require.Equal(t, d3.ID, res[0].Ref.ID)
	require.True(t, q.Done())
	require.Equal(t, "", q.GetCursor())
}
