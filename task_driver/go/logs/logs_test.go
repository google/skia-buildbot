// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

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

func TestLogsManager_Search_MultiStreamOrdering(t *testing.T) {
	project, instance, cleanup := bt_testutil.SetupBigTable(t, BT_TABLE, BT_COLUMN_FAMILY)
	defer cleanup()

	ctx := t.Context()
	lm, err := NewLogsManager(ctx, project, instance, nil)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, lm.Close())
	}()

	// Insert 10 entries alternating between two steps ("step-z" and "step-a")
	// and two log IDs ("log-z" and "log-a") so that lexicographical row-key
	// order differs from chronological order.
	const numEntries = 10
	ts := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < numEntries; i++ {
		sID := "step-z"
		lID := "log-z"
		if i%2 == 1 {
			sID = "step-a"
			lID = "log-a"
		}
		e := &Entry{
			InsertID:         uuid.NewString(),
			Labels:           map[string]string{"taskId": taskID, "stepId": sID, "logId": lID},
			LogName:          "log-name",
			ReceiveTimestamp: ts,
			Severity:         "INFO",
			TextPayload:      fmt.Sprintf("%d", i),
			Timestamp:        ts,
		}
		require.NoError(t, lm.Insert(ctx, e))
		// Also insert an entry into a single step ("step-combined") with alternating log IDs.
		eStep := &Entry{
			InsertID:         uuid.NewString(),
			Labels:           map[string]string{"taskId": taskID, "stepId": "step-combined", "logId": lID},
			LogName:          "log-name",
			ReceiveTimestamp: ts,
			Severity:         "INFO",
			TextPayload:      fmt.Sprintf("step-%d", i),
			Timestamp:        ts,
		}
		require.NoError(t, lm.Insert(ctx, eStep))
		ts = ts.Add(time.Second)
	}

	getPayloads := func(entries []*Entry) []string {
		var payloads []string
		for _, e := range entries {
			payloads = append(payloads, e.TextPayload)
		}
		return payloads
	}

	// 1. Search all logs for "step-combined" (logId == "") unpaginated.
	entries, nextCursor, err := lm.Search(ctx, taskID, "step-combined", "", "", 0, false)
	require.NoError(t, err)
	require.Empty(t, nextCursor)
	require.Equal(t, []string{
		"step-0", "step-1", "step-2", "step-3", "step-4",
		"step-5", "step-6", "step-7", "step-8", "step-9",
	}, getPayloads(entries))

	// 2. Search all logs for "step-combined" (logId == "") with forward pagination (limit 3).
	var allForward []*Entry
	cursor := ""
	pages := 0
	for {
		var page []*Entry
		page, cursor, err = lm.Search(ctx, taskID, "step-combined", "", cursor, 3, false)
		require.NoError(t, err)
		allForward = append(allForward, page...)
		pages++
		if cursor == "" {
			break
		}
	}
	require.Equal(t, 4, pages)
	require.Equal(t, []string{
		"step-0", "step-1", "step-2", "step-3", "step-4",
		"step-5", "step-6", "step-7", "step-8", "step-9",
	}, getPayloads(allForward))

	// 3. Search all logs for "step-combined" (logId == "") with reverse pagination (limit 3).
	var allReverse []*Entry
	cursor = ""
	pages = 0
	for {
		var page []*Entry
		page, cursor, err = lm.Search(ctx, taskID, "step-combined", "", cursor, 3, true)
		require.NoError(t, err)
		allReverse = append(allReverse, page...)
		pages++
		if cursor == "" {
			break
		}
	}
	require.Equal(t, 4, pages)
	require.Equal(t, []string{
		"step-7", "step-8", "step-9",
		"step-4", "step-5", "step-6",
		"step-1", "step-2", "step-3",
		"step-0",
	}, getPayloads(allReverse))

	// 4. Search all logs across steps for taskID (stepId == "", logId == "") where
	// "step-z" has even timestamps and "step-a" has odd timestamps, and "step-combined"
	// has entries at the same timestamps with fixed InsertIDs to test tie-breaking.
	const multiStepTaskID = "task-multi-step"
	ts = time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < numEntries; i++ {
		sID := "step-z"
		lID := "log-z"
		if i%2 == 1 {
			sID = "step-a"
			lID = "log-a"
		}
		e := &Entry{
			InsertID:         "same-insert-id",
			Labels:           map[string]string{"taskId": multiStepTaskID, "stepId": sID, "logId": lID},
			LogName:          "log-name",
			ReceiveTimestamp: ts,
			Severity:         "INFO",
			TextPayload:      fmt.Sprintf("%d", i),
			Timestamp:        ts,
		}
		require.NoError(t, lm.Insert(ctx, e))
		ts = ts.Add(time.Second)
	}

	taskEntries, nextCursor, err := lm.Search(ctx, multiStepTaskID, "", "", "", 0, false)
	require.NoError(t, err)
	require.Empty(t, nextCursor)
	require.Equal(t, []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}, getPayloads(taskEntries))

	// 5. Forward pagination across steps (stepId == "", logId == "", limit 3).
	allForward = nil
	cursor = ""
	pages = 0
	for {
		var page []*Entry
		page, cursor, err = lm.Search(ctx, multiStepTaskID, "", "", cursor, 3, false)
		require.NoError(t, err)
		allForward = append(allForward, page...)
		pages++
		if cursor == "" {
			break
		}
	}
	require.Equal(t, 4, pages)
	require.Equal(t, []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}, getPayloads(allForward))

	// 6. Reverse pagination across steps (stepId == "", logId == "", limit 3).
	allReverse = nil
	cursor = ""
	pages = 0
	for {
		var page []*Entry
		page, cursor, err = lm.Search(ctx, multiStepTaskID, "", "", cursor, 3, true)
		require.NoError(t, err)
		allReverse = append(allReverse, page...)
		pages++
		if cursor == "" {
			break
		}
	}
	require.Equal(t, 4, pages)
	require.Equal(t, []string{
		"7", "8", "9",
		"4", "5", "6",
		"1", "2", "3",
		"0",
	}, getPayloads(allReverse))
}
