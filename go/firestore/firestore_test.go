package firestore

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestAlphaNumID(t *testing.T) {
	unittest.SmallTest(t)

	require.Equal(t, 62, len(alphaNum))
	require.True(t, len(alphaNum) <= math.MaxInt8)

	// If there's a bug in the implementation, this test will be flaky...
	for i := 0; i < 100; i++ {
		id := AlphaNumID()
		require.Equal(t, ID_LEN, len(id))
		for _, char := range id {
			require.True(t, strings.ContainsRune(alphaNum, char))
		}
	}
}

func TestWithTimeout(t *testing.T) {
	unittest.MediumTest(t)

	errTimeout := errors.New("timeout")
	err := withTimeout(context.Background(), 200*time.Millisecond, func(ctx context.Context) error {
		for {
			select {
			case <-time.After(50 * time.Millisecond):
				// ...
			case <-ctx.Done():
				return errTimeout
			}
		}
	})
	require.Equal(t, errTimeout, err)
}

func TestWithTimeoutAndRetries(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := NewClientForTesting(t)
	defer cleanup()

	maxAttempts := 3
	timeout := 200 * time.Millisecond

	// No retries on success.
	attempted := 0
	err := c.withTimeoutAndRetries(context.Background(), maxAttempts, timeout, func(ctx context.Context) error {
		attempted++
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 1, attempted)

	// Retry whitelisted errors.
	attempted = 0
	e := status.Errorf(codes.ResourceExhausted, "Retry Me")
	err = c.withTimeoutAndRetries(context.Background(), maxAttempts, timeout, func(ctx context.Context) error {
		attempted++
		return e
	})
	require.EqualError(t, err, e.Error())
	require.Equal(t, maxAttempts, attempted)

	// No retry for non-whitelisted errors.
	attempted = 0
	err = c.withTimeoutAndRetries(context.Background(), maxAttempts, timeout, func(ctx context.Context) error {
		attempted++
		return errors.New("some other error")
	})
	require.EqualError(t, err, "some other error")
	require.Equal(t, 1, attempted)
}

type testEntry struct {
	Id    string
	Index int
	Label string
}

type testEntrySlice []*testEntry

func (s testEntrySlice) Len() int { return len(s) }

func (s testEntrySlice) Less(i, j int) bool {
	return s[i].Id < s[j].Id
}

func (s testEntrySlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func TestIterDocs(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := NewClientForTesting(t)
	defer cleanup()

	attempts := 3
	timeout := 300 * time.Second
	coll := c.Collection("TestIterDocs")
	labelValue := "my-label"
	q := coll.Where("Label", "==", labelValue)
	foundEntries := 0
	require.NoError(t, c.IterDocs(context.Background(), "TestIterDocs", "", q, attempts, timeout, func(doc *firestore.DocumentSnapshot) error {
		foundEntries++
		return nil
	}))
	require.Equal(t, 0, foundEntries)

	total := 100
	for i := 0; i < total; i++ {
		doc := coll.Doc(AlphaNumID())
		e := &testEntry{
			Id:    doc.ID,
			Index: i,
			Label: labelValue,
		}
		_, err := c.Create(context.Background(), doc, e, attempts, timeout)
		require.NoError(t, err)
	}

	found := make([]*testEntry, 0, total)
	appendEntry := func(doc *firestore.DocumentSnapshot) error {
		var e testEntry
		if err := doc.DataTo(&e); err != nil {
			return err
		}
		found = append(found, &e)
		return nil
	}
	require.NoError(t, c.IterDocs(context.Background(), "TestIterDocs", "", q, attempts, timeout, appendEntry))
	require.Equal(t, total, len(found))
	// Ensure that there were no duplicates.
	foundMap := make(map[string]*testEntry, len(found))
	for _, e := range found {
		_, ok := foundMap[e.Id]
		require.False(t, ok)
		foundMap[e.Id] = e
	}
	require.Equal(t, len(found), len(foundMap))
	require.True(t, sort.IsSorted(testEntrySlice(found)))

	// Verify that stop and resume works when we hit the timeout.
	found = make([]*testEntry, 0, total)
	numRestarts, err := c.iterDocsInner(context.TODO(), q, attempts, timeout, appendEntry, func(time.Time) bool {
		return len(found) == 50
	})
	require.NoError(t, err)
	require.Equal(t, 1, numRestarts)
	require.Equal(t, total, len(found))
	foundMap = make(map[string]*testEntry, len(found))
	for _, e := range found {
		_, ok := foundMap[e.Id]
		require.False(t, ok)
		foundMap[e.Id] = e
	}
	require.Equal(t, len(found), len(foundMap))
	require.True(t, sort.IsSorted(testEntrySlice(found)))

	// Verify that stop and resume works in the case of retried failures.
	alreadyFailed := false
	found = make([]*testEntry, 0, total)
	err = c.IterDocs(context.Background(), "TestIterDocs", "", q, attempts, timeout, func(doc *firestore.DocumentSnapshot) error {
		if len(found) == 50 && !alreadyFailed {
			alreadyFailed = true
			return status.Errorf(codes.ResourceExhausted, "retry me")
		} else {
			return appendEntry(doc)
		}
	})
	require.NoError(t, err)
	require.Equal(t, total, len(found))
	foundMap = make(map[string]*testEntry, len(found))
	for _, e := range found {
		_, ok := foundMap[e.Id]
		require.False(t, ok)
		foundMap[e.Id] = e
	}
	require.Equal(t, len(found), len(foundMap))
	require.True(t, sort.IsSorted(testEntrySlice(found)))

	// Test IterDocsInParallel.
	n := 5
	sliceSize := total / n
	foundSlices := make([][]*testEntry, n)
	queries := make([]firestore.Query, 0, n)
	for i := 0; i < n; i++ {
		start := i * sliceSize
		end := start + sliceSize
		q := coll.Where("Index", ">=", start).Where("Index", "<", end)
		queries = append(queries, q)
	}
	require.NoError(t, c.IterDocsInParallel(context.Background(), "TestIterDocs", "", queries, attempts, timeout, func(idx int, doc *firestore.DocumentSnapshot) error {
		var e testEntry
		if err := doc.DataTo(&e); err != nil {
			return err
		}
		foundSlices[idx] = append(foundSlices[idx], &e)
		return nil
	}))
	// Ensure that there were no duplicates and that each slice of results
	// is correct.
	foundMap = make(map[string]*testEntry, len(foundSlices))
	for _, entries := range foundSlices {
		require.Equal(t, sliceSize, len(entries))
		for _, e := range entries {
			_, ok := foundMap[e.Id]
			require.False(t, ok)
			foundMap[e.Id] = e
		}
	}
	require.Equal(t, total, len(foundMap))
}

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
