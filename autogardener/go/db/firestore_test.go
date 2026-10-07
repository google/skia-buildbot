package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autogardener/go/types"
	"go.skia.org/infra/go/firestore/testutils"
	"golang.org/x/sync/errgroup"
)

func TestMergeFailureClassUpdates(t *testing.T) {
	t1 := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 9, 30, 10, 5, 0, 0, time.UTC)

	u1 := &types.FailureClassUpdate{
		Timestamp: t1,
		User:      "autogardener",
		Flaky:     types.Ptr(true),
		Culprits:  types.Ptr([]string{"commit-a"}),
	}
	u2 := &types.FailureClassUpdate{
		Timestamp:  t2,
		User:       "dev@google.com",
		Message:    "Not flaky, fixed by commit-b",
		Flaky:      types.Ptr(false),
		ResolvedBy: types.Ptr([]string{"commit-b"}),
		Bugs:       types.Ptr([]string{"b/12345"}),
	}

	// Merging disjoint updates preserves both in chronological order.
	merged := mergeFailureClassUpdates([]*types.FailureClassUpdate{u2}, []*types.FailureClassUpdate{u1})
	require.Equal(t, []*types.FailureClassUpdate{u1, u2}, merged)

	// Merging overlapping updates deduplicates by (Timestamp, User).
	merged = mergeFailureClassUpdates([]*types.FailureClassUpdate{u1, u2}, []*types.FailureClassUpdate{u1})
	require.Equal(t, []*types.FailureClassUpdate{u1, u2}, merged)
}

func TestFirestoreDB_PutFailureClass_ConcurrentUpdates(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c, cleanup := testutils.NewClientForTesting(ctx, t)
	defer cleanup()

	db := &firestoreDB{client: c}

	baseTime := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)
	u0 := &types.FailureClassUpdate{
		Timestamp:    baseTime,
		User:         "autogardener",
		ErrorMessage: types.Ptr("error 1"),
		Analysis:     types.Ptr("analysis 1"),
		Flaky:        types.Ptr(true),
		Culprits:     types.Ptr([]string{"commit-0"}),
	}
	initial := &types.FailureClass{
		Id:       "fc-concurrent",
		LastSeen: baseTime,
		Repo:     "https://skia.googlesource.com/skia.git",
		Updates:  []*types.FailureClassUpdate{u0},
	}
	require.NoError(t, db.PutFailureClass(ctx, initial))

	// Spawn multiple concurrent writers that each start from a stale snapshot
	// (containing only u0) and write their own update and LastSeen timestamp.
	const numWorkers = 5
	expectedUpdates := []*types.FailureClassUpdate{u0}
	for i := 1; i <= numWorkers; i++ {
		ts := baseTime.Add(time.Duration(i) * time.Minute)
		expectedUpdates = append(expectedUpdates, &types.FailureClassUpdate{
			Timestamp: ts,
			User:      fmt.Sprintf("user-%d@google.com", i),
			Message:   fmt.Sprintf("update %d", i),
			Bugs:      types.Ptr([]string{fmt.Sprintf("b/%d", i)}),
		})
	}

	var eg errgroup.Group
	for i := 1; i <= numWorkers; i++ {
		i := i
		eg.Go(func() error {
			u := expectedUpdates[i]
			fcCopy := &types.FailureClass{
				Id:       initial.Id,
				LastSeen: u.Timestamp,
				Repo:     initial.Repo,
				Updates:  []*types.FailureClassUpdate{u0, u},
			}
			return db.PutFailureClass(ctx, fcCopy)
		})
	}
	require.NoError(t, eg.Wait())

	got, err := db.GetFailureClass(ctx, initial.Id)
	require.NoError(t, err)
	require.Equal(t, baseTime.Add(numWorkers*time.Minute), got.LastSeen)
	require.Equal(t, expectedUpdates, got.Updates)
}

func TestFirestoreDB_LegacyFailureClass(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c, cleanup := testutils.NewClientForTesting(ctx, t)
	defer cleanup()

	db := &firestoreDB{client: c}
	baseTime := time.Now().UTC().Truncate(time.Microsecond)

	// Write a legacy FailureClass document with top-level ErrorMessage and
	// Analysis and no Updates.
	ref := c.Collection(collectionFailureClass).Doc("fc-legacy")
	_, err := c.Set(ctx, ref, map[string]any{
		"Id":           "fc-legacy",
		"ErrorMessage": "legacy error",
		"Analysis":     "legacy analysis",
		"LastSeen":     baseTime,
		"Repo":         "https://skia.googlesource.com/skia.git",
	}, defaultAttempts, defaultTimeout)
	require.NoError(t, err)

	got, err := db.GetFailureClass(ctx, "fc-legacy")
	require.NoError(t, err)
	require.Equal(t, "legacy error", got.ErrorMessage())
	require.Equal(t, "legacy analysis", got.Analysis())
	require.Len(t, got.Updates, 1)
	require.Equal(t, "autogardener", got.Updates[0].User)

	ch := db.ModifiedFailureClassesCh(ctx, time.Hour)
	fcs := <-ch
	require.Len(t, fcs, 1)
	require.Equal(t, "legacy error", fcs[0].ErrorMessage())
	require.Equal(t, "legacy analysis", fcs[0].Analysis())
}

func TestFirestoreDB_GetRecentFailureClasses_Duplicates(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c, cleanup := testutils.NewClientForTesting(ctx, t)
	defer cleanup()

	db := &firestoreDB{client: c}
	repo := "https://skia.googlesource.com/skia.git"
	oldTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	recentTime := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)

	// fc1 is recent and lists fc2 in Duplicates.
	fc1 := &types.FailureClass{
		Id:       "fc1",
		LastSeen: recentTime,
		Repo:     repo,
		Updates: []*types.FailureClassUpdate{
			{
				Timestamp:    recentTime,
				User:         "autogardener",
				ErrorMessage: types.Ptr("canonical error"),
				Duplicates:   types.Ptr([]string{"fc2"}),
			},
		},
	}
	// fc2 has LastSeen before the query window, and points back via DuplicateOf.
	fc2 := &types.FailureClass{
		Id:       "fc2",
		LastSeen: oldTime,
		Repo:     repo,
		Updates: []*types.FailureClassUpdate{
			{
				Timestamp:    oldTime,
				User:         "autogardener",
				ErrorMessage: types.Ptr("duplicate error"),
				DuplicateOf:  types.Ptr("fc1"),
			},
		},
	}
	require.NoError(t, db.PutFailureClass(ctx, fc1))
	require.NoError(t, db.PutFailureClass(ctx, fc2))

	got, err := db.GetRecentFailureClasses(ctx, repo, recentTime.Add(-time.Hour), 0)
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, "fc1", got[0].Id)
	require.Equal(t, "fc2", got[1].Id)
}

func TestFirestoreDB_PutFailureClass_CycleDetection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c, cleanup := testutils.NewClientForTesting(ctx, t)
	defer cleanup()

	db := &firestoreDB{client: c}
	nowTime := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)

	// Self-loop (fc1 -> fc1) should be rejected.
	selfLoop := &types.FailureClass{
		Id:       "fc1",
		LastSeen: nowTime,
		Updates: []*types.FailureClassUpdate{
			{
				Timestamp:   nowTime,
				User:        "dev@google.com",
				DuplicateOf: types.Ptr("fc1"),
			},
		},
	}
	err := db.PutFailureClass(ctx, selfLoop)
	require.ErrorContains(t, err, "cycle detected")

	// Valid chain fc1 -> fc2 -> fc3 succeeds, then fc3 -> fc1 fails.
	fc3 := &types.FailureClass{
		Id:       "fc3",
		LastSeen: nowTime,
		Updates: []*types.FailureClassUpdate{
			{
				Timestamp: nowTime,
				User:      "autogardener",
			},
		},
	}
	fc2 := &types.FailureClass{
		Id:       "fc2",
		LastSeen: nowTime,
		Updates: []*types.FailureClassUpdate{
			{
				Timestamp:   nowTime,
				User:        "dev@google.com",
				DuplicateOf: types.Ptr("fc3"),
			},
		},
	}
	fc1 := &types.FailureClass{
		Id:       "fc1",
		LastSeen: nowTime,
		Updates: []*types.FailureClassUpdate{
			{
				Timestamp:   nowTime,
				User:        "dev@google.com",
				DuplicateOf: types.Ptr("fc2"),
			},
		},
	}
	require.NoError(t, db.PutFailureClass(ctx, fc3))
	require.NoError(t, db.PutFailureClass(ctx, fc2))
	require.NoError(t, db.PutFailureClass(ctx, fc1))

	// Attempting to make fc3 a DuplicateOf fc1 would create fc3 -> fc1 -> fc2 -> fc3.
	fc3Cycle := &types.FailureClass{
		Id:       "fc3",
		LastSeen: nowTime.Add(time.Minute),
		Updates: []*types.FailureClassUpdate{
			{
				Timestamp:   nowTime.Add(time.Minute),
				User:        "dev@google.com",
				DuplicateOf: types.Ptr("fc1"),
			},
		},
	}
	err = db.PutFailureClass(ctx, fc3Cycle)
	require.ErrorContains(t, err, "cycle detected")
}
