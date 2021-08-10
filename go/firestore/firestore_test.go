package firestore

import (
	"context"
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// newClientForTesting returns a Client and ensures that it will connect to the
// Firestore emulator. The Client's instance name will be randomized to ensure
// concurrent tests don't interfere with each other. It also returns a
// CleanupFunc that closes the Client.
func newClientForTesting(ctx context.Context, t sktest.TestingT) (*Client, util.CleanupFunc) {
	unittest.RequiresFirestoreEmulator(t)
	return NewClientForTesting(ctx, t)
}

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
	c, cleanup := newClientForTesting(context.Background(), t)
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

	// Retry retryable errors.
	attempted = 0
	e := status.Errorf(codes.ResourceExhausted, "Retry Me")
	err = c.withTimeoutAndRetries(context.Background(), maxAttempts, timeout, func(ctx context.Context) error {
		attempted++
		return e
	})
	require.EqualError(t, err, e.Error())
	require.Equal(t, maxAttempts, attempted)

	// No retry for non-retryable errors.
	attempted = 0
	err = c.withTimeoutAndRetries(context.Background(), maxAttempts, timeout, func(ctx context.Context) error {
		attempted++
		return errors.New("some other error")
	})
	require.EqualError(t, err, "some other error")
	require.Equal(t, 1, attempted)
}

func TestWithCancelledContext(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := newClientForTesting(context.Background(), t)
	defer cleanup()

	maxAttempts := 3
	timeout := 200 * time.Millisecond

	// No retries on cancelled context.
	ctx, cancelFn := context.WithCancel(context.Background())
	cancelFn()
	attempted := 0
	err := c.withTimeoutAndRetries(ctx, maxAttempts, timeout, func(ctx context.Context) error {
		attempted++
		return nil
	})
	require.Error(t, err)
	require.Equal(t, 0, attempted)
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
	c, cleanup := newClientForTesting(context.Background(), t)
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
	numRestarts, err := c.iterDocsInner(context.Background(), q, attempts, timeout, "", "", appendEntry, func(time.Time) bool {
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

func TestWriteBatch_SmallBatches_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := newClientForTesting(context.Background(), t)
	defer cleanup()

	ctx := context.Background()
	const expectedWrites = 203
	const batchSize = 11 // selected to not evenly divide expectedWrites
	const timeout = 30 * time.Second
	const fruitKey = "fruit"
	coll := c.Collection("TestWriteBatch")

	err := c.BatchWrite(ctx, expectedWrites, batchSize, timeout, nil, func(b *firestore.WriteBatch, i int) error {
		a := strconv.Itoa(i)
		doc := coll.Doc("doc_" + a)
		b.Set(doc, map[string]string{
			fruitKey: "mango_" + a,
		})
		return nil
	})
	require.NoError(t, err)

	for i := 0; i < expectedWrites; i++ {
		a := strconv.Itoa(i)
		doc := coll.Doc("doc_" + a)
		ds, err := doc.Get(ctx)
		require.NoError(t, err)
		assert.Equal(t, "mango_"+a, ds.Data()[fruitKey])
	}
}

func TestWriteBatch_SmallBatchesWithProvidedBatch_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := newClientForTesting(context.Background(), t)
	defer cleanup()

	ctx := context.Background()
	const expectedWrites = 203
	const batchSize = 11 // selected to not evenly divide expectedWrites
	const timeout = 30 * time.Second
	const fruitKey = "fruit"
	coll := c.Collection("TestWriteBatch")

	b := c.Batch()
	bananaDoc := coll.Doc("doc_0")
	b.Set(bananaDoc, map[string]string{
		fruitKey: "banana",
	})

	err := c.BatchWrite(ctx, expectedWrites, batchSize, timeout, b, func(b *firestore.WriteBatch, i int) error {
		if i == 0 {
			return nil // it's already been written
		}
		a := strconv.Itoa(i)
		doc := coll.Doc("doc_" + a)
		b.Set(doc, map[string]string{
			fruitKey: "mango_" + a,
		})
		return nil
	})
	require.NoError(t, err)

	ds, err := bananaDoc.Get(ctx)
	require.NoError(t, err)
	assert.Equal(t, "banana", ds.Data()[fruitKey])

	// we stored mangos from 1 - n
	for i := 1; i < expectedWrites; i++ {
		a := strconv.Itoa(i)
		doc := coll.Doc("doc_" + a)
		ds, err = doc.Get(ctx)
		require.NoError(t, err)
		assert.Equal(t, "mango_"+a, ds.Data()[fruitKey])
	}
}

func TestWriteBatch_BigSingleBatch_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := newClientForTesting(context.Background(), t)
	defer cleanup()

	ctx := context.Background()
	const expectedWrites = 203
	const batchSize = MAX_TRANSACTION_DOCS
	const timeout = 30 * time.Second
	const fruitKey = "fruit"
	coll := c.Collection("TestWriteBatch")

	err := c.BatchWrite(ctx, expectedWrites, batchSize, timeout, nil, func(b *firestore.WriteBatch, i int) error {
		a := strconv.Itoa(i)
		doc := coll.Doc("doc_" + a)
		b.Set(doc, map[string]string{
			fruitKey: "mango_" + a,
		})
		return nil
	})
	require.NoError(t, err)

	for i := 0; i < expectedWrites; i++ {
		a := strconv.Itoa(i)
		doc := coll.Doc("doc_" + a)
		ds, err := doc.Get(ctx)
		require.NoError(t, err)
		assert.Equal(t, "mango_"+a, ds.Data()[fruitKey])
	}
}

func TestWriteBatch_BigBatches_Success(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := newClientForTesting(context.Background(), t)
	defer cleanup()

	ctx := context.Background()
	const expectedWrites = 1203 // Sized to have multiple batches.
	const batchSize = MAX_TRANSACTION_DOCS
	const timeout = 30 * time.Second
	const fruitKey = "fruit"
	coll := c.Collection("TestWriteBatch")

	err := c.BatchWrite(ctx, expectedWrites, batchSize, timeout, nil, func(b *firestore.WriteBatch, i int) error {
		a := strconv.Itoa(i)
		doc := coll.Doc("doc_" + a)
		b.Set(doc, map[string]string{
			fruitKey: "mango_" + a,
		})
		return nil
	})
	require.NoError(t, err)

	for i := 0; i < expectedWrites; i++ {
		a := strconv.Itoa(i)
		doc := coll.Doc("doc_" + a)
		ds, err := doc.Get(ctx)
		require.NoError(t, err)
		assert.Equal(t, "mango_"+a, ds.Data()[fruitKey])
	}
}

func TestWriteBatch_ExpiredContex_Error(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := newClientForTesting(context.Background(), t)
	defer cleanup()

	// These inputs don't really matter
	const expectedWrites = 1203
	const batchSize = MAX_TRANSACTION_DOCS
	const timeout = 30 * time.Second

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := c.BatchWrite(ctx, expectedWrites, batchSize, timeout, nil, func(_ *firestore.WriteBatch, i int) error {
		assert.Fail(t, "should not have seen any calls %d", i)
		return nil
	})
	require.Error(t, err)
}

func TestWriteBatch_BackoffRespectsExpiredContex_Error(t *testing.T) {
	unittest.LargeTest(t)
	c, cleanup := newClientForTesting(context.Background(), t)
	defer cleanup()

	// With a batchSize of 1, we force a context to be not canceled on the batch loop, and yet
	// to be canceled on the Commit() step.
	const expectedWrites = 5
	const batchSize = 1
	// this long time would normally time the test out, unless it respects the failed context
	const timeout = 30000 * time.Second
	coll := c.Collection("TestWriteBatch")

	ctx, cancel := context.WithCancel(context.Background())

	err := c.BatchWrite(ctx, expectedWrites, batchSize, timeout, nil, func(b *firestore.WriteBatch, i int) error {
		// This shouldn't actually get written because of the canceled context.
		bananaDoc := coll.Doc("doc_0")
		b.Set(bananaDoc, map[string]string{
			"fruit": "banana",
		})

		cancel()
		return nil
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exponential retry")

	docs, err := coll.Documents(context.Background()).GetAll()
	require.NoError(t, err)
	assert.Empty(t, docs)
}
