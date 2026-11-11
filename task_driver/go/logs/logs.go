// Copyright 2018 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package logs

/*
	The logs package provides an interface for inserting and retrieving
	Task Driver logs in Cloud BigTable.
*/

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"slices"
	"strings"
	"time"

	"cloud.google.com/go/bigtable"
	"go.opencensus.io/trace"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/task_driver/go/td"
)

const (
	// We use a single BigTable table for storing logs.
	BT_TABLE = "task-driver-logs"

	// We use a single BigTable column family.
	BT_COLUMN_FAMILY = "LOGS"

	// We use a single BigTable column which stores gob-encoded log entries.
	BT_COLUMN = "ENTRY"

	INSERT_TIMEOUT = 30 * time.Second
	QUERY_TIMEOUT  = 5 * time.Second
)

var (
	// Fully-qualified BigTable column name.
	BT_COLUMN_FULL = fmt.Sprintf("%s:%s", BT_COLUMN_FAMILY, BT_COLUMN)
)

// rowKey returns a BigTable row key for a log entry. If any of the parameters
// is empty, then only the row key prefix for the provided parameters is
// returned, which allows rowKey to be used for prefix searches.
func rowKey(taskId, stepId, logId string, ts time.Time, insertId string) string {
	// Full log for an entire task.
	rv := taskId

	// All logs related to a step.
	if stepId != "" {
		rv += "#" + stepId
	} else {
		return rv
	}

	// A single log stream.
	if logId != "" {
		rv += "#" + logId
	} else {
		return rv
	}

	// Timestamp.
	if !util.TimeIsZero(ts) {
		rv += "#" + ts.UTC().Format(util.SAFE_TIMESTAMP_FORMAT)
	} else {
		return rv
	}

	// Log insert ID. Included in case of multiple entries having the same
	// timestamp.
	if insertId != "" {
		rv += "#" + insertId
	} else {
		return rv
	}

	// Done.
	return rv
}

// Entry mimics logging.Entry, which for some reason does not include the
// jsonPayload field, and is not parsable via json.Unmarshal due to the Severity
// type.
type Entry struct {
	InsertID         string            `json:"insertId"`
	Labels           map[string]string `json:"labels"`
	LogName          string            `json:"logName"`
	ReceiveTimestamp time.Time         `json:"receiveTimestamp"`
	//Resource
	Severity    string      `json:"severity"`
	JsonPayload *td.Message `json:"jsonPayload"`
	TextPayload string      `json:"textPayload"`
	Timestamp   time.Time   `json:"timestamp"`
}

// LogsManager provides an interface for inserting and retrieving Task Driver
// logs in Cloud BigTable.
type LogsManager interface {
	// Close the LogsManager.
	Close() error

	// Insert the given log entry.
	Insert(ctx context.Context, e *Entry) error

	// Search returns Entries matching the given search terms.
	// If limit is provided, the results are paginated: a cursor will be returned
	// which, if non-empty, can be passed to the next call to Search.
	// If reverse is true, pages are loaded from the end of the stream but each page
	// contains entries in chronological order.
	Search(ctx context.Context, taskId, stepId, logId, cursor string, limit int, reverse bool) ([]*Entry, string, error)
}

// LogsManagerImpl is a struct which provides an interface for inserting and
// retrieving Task Driver logs in Cloud BigTable.
type LogsManagerImpl struct {
	client *bigtable.Client
	table  *bigtable.Table
}

// NewLogsManager returns a LogsManagerImpl instance.
func NewLogsManager(ctx context.Context, project, instance string, ts oauth2.TokenSource) (*LogsManagerImpl, error) {
	client, err := bigtable.NewClient(ctx, project, instance, option.WithTokenSource(ts))
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to create BigTable client")
	}
	table := client.Open(BT_TABLE)
	return &LogsManagerImpl{
		client: client,
		table:  table,
	}, nil
}

// Close the LogsManager.
func (m *LogsManagerImpl) Close() error {
	return m.client.Close()
}

// rowKey returns the BigTable row key for this Entry.
func (e *Entry) rowKey() string {
	taskId := e.Labels["taskId"]
	stepId, ok := e.Labels["stepId"]
	if !ok {
		stepId = "no_step_id"
	}
	logId, ok := e.Labels["logId"]
	if !ok {
		logId = "no_log_id"
	}
	return rowKey(taskId, stepId, logId, e.Timestamp, e.InsertID)
}

// Insert the given log entry.
func (m *LogsManagerImpl) Insert(ctx context.Context, e *Entry) error {
	ctx, span := trace.StartSpan(ctx, "LogsManager_Insert")
	defer span.End()
	// Encode the log entry.
	buf := bytes.Buffer{}
	if err := gob.NewEncoder(&buf).Encode(e); err != nil {
		return skerr.Wrapf(err, "failed to gob-encode log entry")
	}
	// Insert the log entry into BigTable.
	mt := bigtable.NewMutation()
	mt.Set(BT_COLUMN_FAMILY, BT_COLUMN, bigtable.Time(e.Timestamp), buf.Bytes())
	if _, ok := e.Labels["taskId"]; !ok {
		// TODO(borenet): We should Ack() the message in this case.
		return skerr.Fmt("Log entry is missing a task ID! %+v", e)
	}
	rk := e.rowKey()
	ctx, cancel := context.WithTimeout(ctx, INSERT_TIMEOUT)
	defer cancel()
	return m.table.Apply(ctx, rk, mt)
}

// Search returns Entries matching the given search terms.
// If limit is provided, the results are paginated: a cursor will be returned
// which, if non-empty, can be passed to the next call to Search.
// If reverse is true, pages are loaded from the end of the stream but each page
// contains entries in chronological order.
func (m *LogsManagerImpl) Search(ctx context.Context, taskId, stepId, logId, cursor string, limit int, reverse bool) ([]*Entry, string, error) {
	ctx, span := trace.StartSpan(ctx, "LogsManager_Search")
	defer span.End()
	prefix := rowKey(taskId, stepId, logId, time.Time{}, "")
	sklog.Infof("Searching for entries with prefix: %s", prefix)
	ctx, cancel := context.WithTimeout(ctx, QUERY_TIMEOUT)
	defer cancel()

	// Row keys are structured as <taskId>#<stepId>#<logId>#<timestamp>#<insertId>.
	// Within a single <taskId>#<stepId>#<logId> stream, BigTable rows are stored
	// in chronological order. When stepId or logId is empty, we first discover
	// all <taskId>#<stepId>#<logId> stream prefixes, and then query from each
	// stream starting at its cursor.
	var streamPrefixes []string
	if stepId != "" && logId != "" {
		streamPrefixes = []string{prefix}
	} else {
		searchPrefix := prefix + "#"
		startKey := searchPrefix
		endKey := searchPrefix + "\xff"
		for {
			var foundKey string
			if err := m.table.ReadRows(ctx, bigtable.NewRange(startKey, endKey), func(row bigtable.Row) bool {
				foundKey = row.Key()
				return false
			}, bigtable.LimitRows(1), bigtable.RowFilter(bigtable.StripValueFilter())); err != nil {
				return nil, "", skerr.Wrapf(err, "failed to discover log streams from BigTable")
			}
			if foundKey == "" {
				break
			}
			parts := strings.SplitN(foundKey, "#", 4)
			if len(parts) < 4 {
				startKey = foundKey + "\x00"
				continue
			}
			sp := strings.Join(parts[:3], "#")
			streamPrefixes = append(streamPrefixes, sp)
			startKey = sp + "#\xff"
		}
	}

	var cursorStreamPrefix, cursorChrono string
	if cursor != "" {
		parts := strings.SplitN(cursor, "#", 4)
		if len(parts) != 4 {
			return nil, "", skerr.Fmt("invalid cursor %q", cursor)
		}
		cursorStreamPrefix = strings.Join(parts[:3], "#")
		cursorChrono = parts[3]
	}

	opts := []bigtable.ReadOption{
		bigtable.RowFilter(bigtable.LatestNFilter(1)),
	}
	if limit > 0 {
		// Fetch one more than the limit so we know the next row key.
		opts = append(opts, bigtable.LimitRows(int64(limit+1)))
	}
	if reverse {
		opts = append(opts, bigtable.ReverseScan())
	}

	entries := []*Entry{}
	var decodeErr error
	for _, sp := range streamPrefixes {
		startKey := sp + "#"
		endKey := sp + "#\xff"
		if cursor != "" {
			if reverse {
				// BigTable's NewRange(start, limit) uses an exclusive upper bound:
				// [start, limit). For streams <= cursorStreamPrefix in our tie-break
				// order, entries at cursorChrono have not been returned yet, so we
				// append a null byte (\x00) to make the upper bound inclusive.
				if sp <= cursorStreamPrefix {
					endKey = sp + "#" + cursorChrono + "\x00"
				} else {
					endKey = sp + "#" + cursorChrono
				}
			} else {
				// NewRange(start, limit) is inclusive of start. For streams <
				// cursorStreamPrefix in our tie-break order, entries at cursorChrono
				// were already returned on the previous page, so we append \x00 to
				// start strictly after cursorChrono.
				if sp >= cursorStreamPrefix {
					startKey = sp + "#" + cursorChrono
				} else {
					startKey = sp + "#" + cursorChrono + "\x00"
				}
			}
		}
		rr := bigtable.NewRange(startKey, endKey)
		if err := m.table.ReadRows(ctx, rr, func(row bigtable.Row) bool {
			for _, ri := range row[BT_COLUMN_FAMILY] {
				if ri.Column == BT_COLUMN_FULL {
					var e Entry
					decodeErr = gob.NewDecoder(bytes.NewReader(ri.Value)).Decode(&e)
					if decodeErr != nil {
						return false
					}
					entries = append(entries, &e)
					// We only store one entry per row.
					return true
				}
			}
			return true
		}, opts...); err != nil {
			return nil, "", skerr.Wrapf(err, "failed to retrieve data from BigTable")
		}
		if decodeErr != nil {
			return nil, "", skerr.Wrapf(decodeErr, "failed to gob-decode entry")
		}
	}

	if len(streamPrefixes) > 1 {
		slices.SortFunc(entries, func(a, b *Entry) int {
			if cmp := a.Timestamp.Compare(b.Timestamp); cmp != 0 {
				return cmp
			}
			if cmp := strings.Compare(a.InsertID, b.InsertID); cmp != 0 {
				return cmp
			}
			return strings.Compare(a.rowKey(), b.rowKey())
		})
		if reverse {
			slices.Reverse(entries)
		}
	}

	nextCursor := ""
	if limit > 0 && len(entries) > limit {
		nextCursor = entries[limit].rowKey()
		entries = entries[:limit]
	}
	if reverse {
		slices.Reverse(entries)
	}
	return entries, nextCursor, nil
}

var _ LogsManager = &LogsManagerImpl{}
