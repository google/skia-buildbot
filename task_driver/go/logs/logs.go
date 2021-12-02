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
	"time"

	"cloud.google.com/go/bigtable"
	"go.opencensus.io/trace"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"

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

// LogsManager is a struct which provides an interface for inserting and
// retrieving Task Driver logs in Cloud BigTable.
type LogsManager struct {
	client *bigtable.Client
	table  *bigtable.Table
}

// NewLogsManager returns a LogsManager instance.
func NewLogsManager(ctx context.Context, project, instance string, ts oauth2.TokenSource) (*LogsManager, error) {
	client, err := bigtable.NewClient(ctx, project, instance, option.WithTokenSource(ts))
	if err != nil {
		return nil, fmt.Errorf("Failed to create BigTable client: %s", err)
	}
	table := client.Open(BT_TABLE)
	return &LogsManager{
		client: client,
		table:  table,
	}, nil
}

// Close the LogsManager.
func (m *LogsManager) Close() error {
	return m.client.Close()
}

// Insert the given log entry.
func (m *LogsManager) Insert(ctx context.Context, e *Entry) error {
	ctx, span := trace.StartSpan(ctx, "LogsManager_Insert")
	defer span.End()
	// Encode the log entry.
	buf := bytes.Buffer{}
	if err := gob.NewEncoder(&buf).Encode(e); err != nil {
		return fmt.Errorf("Failed to gob-encode log entry: %s", err)
	}
	// Insert the log entry into BigTable.
	mt := bigtable.NewMutation()
	mt.Set(BT_COLUMN_FAMILY, BT_COLUMN, bigtable.Time(e.Timestamp), buf.Bytes())
	taskId, ok := e.Labels["taskId"]
	if !ok {
		// TODO(borenet): We should Ack() the message in this case.
		return fmt.Errorf("Log entry is missing a task ID! %+v", e)
	}
	stepId, ok := e.Labels["stepId"]
	if !ok {
		stepId = "no_step_id"
	}
	logId, ok := e.Labels["logId"]
	if !ok {
		logId = "no_log_id"
	}
	rk := rowKey(taskId, stepId, logId, e.Timestamp, e.InsertID)
	ctx, cancel := context.WithTimeout(ctx, INSERT_TIMEOUT)
	defer cancel()
	return m.table.Apply(ctx, rk, mt)
}

// Search returns Entries matching the given search terms.
func (m *LogsManager) Search(ctx context.Context, taskId, stepId, logId string) ([]*Entry, error) {
	ctx, span := trace.StartSpan(ctx, "LogsManager_Search")
	defer span.End()
	prefix := rowKey(taskId, stepId, logId, time.Time{}, "")
	sklog.Infof("Searching for entries with prefix: %s", prefix)
	entries := []*Entry{}
	var decodeErr error
	ctx, cancel := context.WithTimeout(ctx, QUERY_TIMEOUT)
	defer cancel()
	if err := m.table.ReadRows(ctx, bigtable.PrefixRange(prefix), func(row bigtable.Row) bool {
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
	}, bigtable.RowFilter(bigtable.LatestNFilter(1))); err != nil {
		return nil, fmt.Errorf("Failed to retrieve data from BigTable: %s", err)
	}
	if decodeErr != nil {
		return nil, fmt.Errorf("Failed to gob-decode entry: %s", decodeErr)
	}
	return entries, nil
}
