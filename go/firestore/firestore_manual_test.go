package firestore

import (
	"context"
	"fmt"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestGetAllDescendants(t *testing.T) {
	// The emulator does not support the query used in RecursiveDelete and
	// GetAllDescendantDocuments, so this must test against a real firestore
	// instance; hence it is a manual test.
	unittest.ManualTest(t)
	EnsureNotEmulator()

	project := "skia-firestore"
	app := "firestore_pkg_tests"
	instance := fmt.Sprintf("test-%s", uuid.New())
	c, err := NewClient(context.Background(), project, app, instance, nil)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, c.RecursiveDelete(context.Background(), c.ParentDoc, 5, 30*time.Second))
		require.NoError(t, c.Close())
	}()

	attempts := 3
	timeout := 5 * time.Second

	// Create some documents.
	add := func(coll *firestore.CollectionRef, name string) *firestore.DocumentRef {
		doc := coll.Doc(name)
		_, err := c.Create(context.Background(), doc, map[string]string{"name": name}, attempts, timeout)
		require.NoError(t, err)
		return doc
	}

	container := c.Collection("container")
	topLevelDoc := add(container, "TopLevel")

	states := topLevelDoc.Collection("states")
	ny := add(states, "NewYork")
	ca := add(states, "California")
	nc := add(states, "NorthCarolina")
	fl := add(states, "Florida")

	addCity := func(state *firestore.DocumentRef, name string) *firestore.DocumentRef {
		cities := state.Collection("cities")
		return add(cities, name)
	}
	nyc := addCity(ny, "NewYork")
	la := addCity(ca, "LosAngeles")
	sf := addCity(ca, "SanFrancisco")
	ch := addCity(nc, "ChapelHill")

	// Verify that descendants are found.
	check := func(parent *firestore.DocumentRef, expect []*firestore.DocumentRef) {
		actual, err := c.GetAllDescendantDocuments(context.Background(), parent, attempts, timeout)
		require.NoError(t, err)
		require.Equal(t, len(expect), len(actual))
		for idx, e := range expect {
			require.Equal(t, e.ID, actual[idx].ID)
		}
	}
	check(ny, []*firestore.DocumentRef{nyc})
	check(ca, []*firestore.DocumentRef{la, sf})
	check(nc, []*firestore.DocumentRef{ch})
	check(topLevelDoc, []*firestore.DocumentRef{ca, la, sf, fl, ny, nyc, nc, ch})

	// Check that we can find descendants of missing documents.
	_, err = c.Delete(context.Background(), ny, attempts, timeout)
	require.NoError(t, err)
	check(topLevelDoc, []*firestore.DocumentRef{ca, la, sf, fl, ny, nyc, nc, ch})
	_, err = c.Delete(context.Background(), nyc, attempts, timeout)
	require.NoError(t, err)
	check(topLevelDoc, []*firestore.DocumentRef{ca, la, sf, fl, nc, ch})

	// Also test RecursiveDelete.
	del := func(doc *firestore.DocumentRef, expect []*firestore.DocumentRef) {
		require.NoError(t, c.RecursiveDelete(context.Background(), doc, attempts, timeout))
		check(topLevelDoc, expect)
	}
	del(ca, []*firestore.DocumentRef{fl, nc, ch})
	del(fl, []*firestore.DocumentRef{nc, ch})
	del(topLevelDoc, []*firestore.DocumentRef{})
}
