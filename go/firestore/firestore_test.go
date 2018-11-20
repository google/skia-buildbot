package firestore

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/google/uuid"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestWithTimeout(t *testing.T) {
	testutils.MediumTest(t)

	errTimeout := errors.New("timeout")
	err := withTimeout(200*time.Millisecond, func(ctx context.Context) error {
		for {
			select {
			case <-time.After(50 * time.Millisecond):
				// ...
			case <-ctx.Done():
				return errTimeout
			}
		}
	})
	assert.Equal(t, errTimeout, err)
}

func TestWithTimeoutAndRetries(t *testing.T) {
	testutils.LargeTest(t)

	maxAttempts := 3
	timeout := 200 * time.Millisecond

	// No retries on success.
	attempted := 0
	err := withTimeoutAndRetries(maxAttempts, timeout, func(ctx context.Context) error {
		attempted++
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, attempted)

	// Retry whitelisted errors.
	attempted = 0
	e := status.Errorf(codes.ResourceExhausted, "Retry Me")
	err = withTimeoutAndRetries(maxAttempts, timeout, func(ctx context.Context) error {
		attempted++
		return e
	})
	assert.EqualError(t, err, e.Error())
	assert.Equal(t, maxAttempts, attempted)

	// No retry for non-whitelisted errors.
	attempted = 0
	err = withTimeoutAndRetries(maxAttempts, timeout, func(ctx context.Context) error {
		attempted++
		return errors.New("some other error")
	})
	assert.EqualError(t, err, "some other error")
	assert.Equal(t, 1, attempted)
}

func setup(t *testing.T) (*Client, func()) {
	testutils.MediumTest(t)
	testutils.ManualTest(t)
	project := "skia-firestore"
	app := "firestore_pkg_tests"
	instance := fmt.Sprintf("test-%s", uuid.New())
	c, err := NewClient(context.Background(), project, app, instance, nil)
	assert.NoError(t, err)
	return c, func() {
		assert.NoError(t, c.Close())
	}
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
	c, cleanup := setup(t)
	defer cleanup()

	attempts := 3
	timeout := 300 * time.Second
	coll := c.Collection("TestIterDocs")
	labelValue := "my-label"
	q := coll.Where("Label", "==", labelValue)
	foundEntries := 0
	assert.NoError(t, IterDocs(q, attempts, timeout, func(doc *firestore.DocumentSnapshot) error {
		foundEntries++
		return nil
	}))
	assert.Equal(t, 0, foundEntries)

	total := 100
	for i := 0; i < total; i++ {
		doc := coll.NewDoc()
		e := &testEntry{
			Id:    doc.ID,
			Index: i,
			Label: labelValue,
		}
		_, err := Create(doc, e, attempts, timeout)
		assert.NoError(t, err)
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
	assert.NoError(t, IterDocs(q, attempts, timeout, appendEntry))
	assert.Equal(t, total, len(found))
	// Ensure that there were no duplicates.
	foundMap := make(map[string]*testEntry, len(found))
	for _, e := range found {
		_, ok := foundMap[e.Id]
		assert.False(t, ok)
		foundMap[e.Id] = e
	}
	assert.Equal(t, len(found), len(foundMap))
	assert.True(t, sort.IsSorted(testEntrySlice(found)))

	// Verify that stop and resume works when we hit the timeout.
	found = make([]*testEntry, 0, total)
	numRestarts, err := iterDocsInner(q, attempts, timeout, appendEntry, func(time.Time) bool {
		return len(found) == 50
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, numRestarts)
	assert.Equal(t, total, len(found))
	foundMap = make(map[string]*testEntry, len(found))
	for _, e := range found {
		_, ok := foundMap[e.Id]
		assert.False(t, ok)
		foundMap[e.Id] = e
	}
	assert.Equal(t, len(found), len(foundMap))
	assert.True(t, sort.IsSorted(testEntrySlice(found)))

	// Verify that stop and resume works in the case of retried failures.
	alreadyFailed := false
	found = make([]*testEntry, 0, total)
	err = IterDocs(q, attempts, timeout, func(doc *firestore.DocumentSnapshot) error {
		if len(found) == 50 && !alreadyFailed {
			alreadyFailed = true
			return status.Errorf(codes.ResourceExhausted, "retry me")
		} else {
			return appendEntry(doc)
		}
	})
	assert.NoError(t, err)
	assert.Equal(t, total, len(found))
	foundMap = make(map[string]*testEntry, len(found))
	for _, e := range found {
		_, ok := foundMap[e.Id]
		assert.False(t, ok)
		foundMap[e.Id] = e
	}
	assert.Equal(t, len(found), len(foundMap))
	assert.True(t, sort.IsSorted(testEntrySlice(found)))

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
	assert.NoError(t, IterDocsInParallel(queries, attempts, timeout, func(idx int, doc *firestore.DocumentSnapshot) error {
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
		assert.Equal(t, sliceSize, len(entries))
		for _, e := range entries {
			_, ok := foundMap[e.Id]
			assert.False(t, ok)
			foundMap[e.Id] = e
		}
	}
	assert.Equal(t, total, len(foundMap))
}
