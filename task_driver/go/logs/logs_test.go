package logs

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	bt_testutil "go.skia.org/infra/go/bt/testutil"
)

const (
	taskID = "task-1"
	logID  = "log-1"
	stepID = "step-1"
)

func TestLogsManager_Search_Pagination(t *testing.T) {
	project, instance, cleanup := bt_testutil.SetupBigTable(t, BT_TABLE, BT_COLUMN_FAMILY)
	defer cleanup()

	ctx := t.Context()
	lm, err := NewLogsManager(ctx, project, instance, nil)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, lm.Close())
	}()

	// Insert 10 entries with increasing timestamps.
	const numEntries = 10
	inserted := map[string][]*Entry{}
	ts := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	insertEntry := func(stepID, message string) {
		e := &Entry{
			InsertID:         uuid.NewString(),
			Labels:           map[string]string{"taskId": taskID, "stepId": stepID, "logId": logID},
			LogName:          "log-name",
			ReceiveTimestamp: ts,
			Severity:         "INFO",
			TextPayload:      message,
			Timestamp:        ts,
		}
		require.NoError(t, lm.Insert(ctx, e))
		inserted[stepID] = append(inserted[stepID], e)
		ts = ts.Add(time.Second)
	}
	for _, stepId := range []string{"step-1", "step-2", "step-3"} {
		for i := 0; i < numEntries; i++ {
			insertEntry(stepId, fmt.Sprintf("%s msg %d", stepId, i))
		}
	}

	// Helper function to extract payloads for easy comparison.
	getPayloads := func(entries []*Entry) []string {
		var payloads []string
		for _, e := range entries {
			payloads = append(payloads, e.TextPayload)
		}
		return payloads
	}

	// 1. Fetch all entries without limits.
	wantStepId := "step-2"
	entries, nextCursor, err := lm.Search(ctx, taskID, wantStepId, logID, "", 0, false)
	require.NoError(t, err)
	require.Len(t, entries, numEntries)
	require.Equal(t, "", nextCursor)

	// 2. Fetch with pagination (limit 3).
	limit := 3
	var allFetched []*Entry
	startCursor := ""
	pages := 0
	for {
		entries, nextCursor, err = lm.Search(ctx, taskID, wantStepId, logID, startCursor, limit, false)
		require.NoError(t, err)
		allFetched = append(allFetched, entries...)
		pages++
		if nextCursor == "" {
			break
		}
		startCursor = nextCursor
	}

	require.Equal(t, numEntries, len(allFetched))
	require.Equal(t, 4, pages) // 3 + 3 + 3 + 1 = 10 items over 4 pages
	require.Equal(t, getPayloads(inserted[wantStepId]), getPayloads(allFetched))

	// 3. Test exact division (limit 5).
	limit = 5
	allFetched = nil
	startCursor = ""
	pages = 0
	for {
		entries, nextCursor, err = lm.Search(ctx, taskID, wantStepId, logID, startCursor, limit, false)
		require.NoError(t, err)
		allFetched = append(allFetched, entries...)
		pages++
		if nextCursor == "" {
			break
		}
		startCursor = nextCursor
	}
	require.Equal(t, numEntries, len(allFetched))
	require.Equal(t, 2, pages) // 5 + 5 = 10 items over 2 pages (the last page returns empty nextCursor)
	require.Equal(t, getPayloads(inserted[wantStepId]), getPayloads(allFetched))
}

func TestLogsManager_Search_Reverse(t *testing.T) {
	project, instance, cleanup := bt_testutil.SetupBigTable(t, BT_TABLE, BT_COLUMN_FAMILY)
	defer cleanup()

	ctx := t.Context()
	lm, err := NewLogsManager(ctx, project, instance, nil)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, lm.Close())
	}()

	// Insert 10 entries with increasing timestamps.
	const numEntries = 10
	var inserted []*Entry
	ts := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < numEntries; i++ {
		e := &Entry{
			InsertID:         uuid.NewString(),
			Labels:           map[string]string{"taskId": taskID, "stepId": stepID, "logId": logID},
			LogName:          "log-name",
			ReceiveTimestamp: ts,
			Severity:         "INFO",
			TextPayload:      fmt.Sprintf("%d", i),
			Timestamp:        ts,
		}
		require.NoError(t, lm.Insert(ctx, e))
		inserted = append(inserted, e)
		ts = ts.Add(time.Second)
	}

	// Helper function to extract payloads for easy comparison.
	getPayloads := func(entries []*Entry) []string {
		var payloads []string
		for _, e := range entries {
			payloads = append(payloads, e.TextPayload)
		}
		return payloads
	}

	// Fetch with pagination (limit 3) and reverse=true.
	limit := 3
	pages := 0
	var allFetched []*Entry
	startCursor := ""
	for {
		var entries []*Entry
		entries, startCursor, err = lm.Search(ctx, taskID, stepID, logID, startCursor, limit, true)
		require.NoError(t, err)
		allFetched = append(allFetched, entries...)
		pages++
		if startCursor == "" {
			break
		}
	}

	require.Equal(t, numEntries, len(allFetched))
	require.Equal(t, 4, pages) // 3 + 3 + 3 + 1 = 10 items over 4 pages
	require.Equal(t, []string{
		// Each page contains entries in chronological order, but we load pages
		// from the end of the log stream.
		"7", "8", "9",
		"4", "5", "6",
		"1", "2", "3",
		"0",
	}, getPayloads(allFetched))
}
