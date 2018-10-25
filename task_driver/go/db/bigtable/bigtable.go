package bigtable

/*
   This DB implementation uses BigTable to store information about Task Drivers.
*/

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/task_driver/go/db"
	"go.skia.org/infra/task_driver/go/td"
	"go.skia.org/infra/task_scheduler/go/db/local_db"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
)

const (
	// We use a single BigTable instance for Task Drivers per project.
	BT_INSTANCE = "task-driver"

	// We use a single BigTable table for storing Task Driver runs.
	BT_TABLE = "task-driver-runs"

	// We use a single BigTable column family.
	BT_COLUMN_FAMILY = "MSGS"

	// We use a single BigTable column which stores gob-encoded td.Messages.
	BT_COLUMN = "MSG"
)

var (
	// Fully-qualified BigTable column name.
	BT_COLUMN_FULL = fmt.Sprintf("%s:%s", BT_COLUMN_FAMILY, BT_COLUMN)
)

// rowKey returns a BigTable row key for the given message, based on the given
// Task Driver ID.
func rowKey(id string, msg *td.Message) string {
	return id + "#" + msg.Timestamp.Format(local_db.TIMESTAMP_FORMAT)
}

// btDB is an implementation of db.DB which uses BigTable.
type btDB struct {
	client *bigtable.Client
}

// NewBigTableDB returns a db.DB instance which uses BigTable.
func NewBigTableDB(ctx context.Context, project, instance string, ts oauth2.TokenSource) (db.DB, error) {
	client, err := bigtable.NewClient(ctx, project, instance, option.WithTokenSource(ts))
	if err != nil {
		return nil, fmt.Errorf("Failed to create BigTable client: %s", err)
	}
	return &btDB{
		client: client,
	}, nil
}

// See documentation for db.DB interface.
func (d *btDB) Close() error {
	return d.client.Close()
}

// See documentation for db.DB interface.
func (d *btDB) GetTaskDriver(id string) (*db.TaskDriverRun, error) {
	// Retrieve all messages for the Task Driver from BigTable.
	msgs := []*td.Message{}
	var decodeErr error
	if err := d.client.Open(BT_TABLE).ReadRows(context.Background(), bigtable.PrefixRange(id), func(row bigtable.Row) bool {
		for _, ri := range row[BT_COLUMN_FAMILY] {
			if ri.Column == BT_COLUMN_FULL {
				var msg td.Message
				decodeErr = gob.NewDecoder(bytes.NewReader(ri.Value)).Decode(&msg)
				if decodeErr != nil {
					return false
				}
				msgs = append(msgs, &msg)
				// We only store one message per row.
				return true
			}
		}
		return true
	}); err != nil {
		return nil, fmt.Errorf("Failed to retrieve data from BigTable: %s", err)
	}
	if decodeErr != nil {
		return nil, fmt.Errorf("Failed to gob-decode message: %s", decodeErr)
	}

	// If we have no messages, the TaskDriverRun does not exist in our DB.
	// Per the doc on the db.DB interface, we should return nil with no
	// error.
	if len(msgs) == 0 {
		return nil, nil
	}

	// Apply all messages to the TaskDriver.
	t := &db.TaskDriverRun{
		TaskId: id,
	}
	for _, msg := range msgs {
		if err := t.UpdateFromMessage(msg); err != nil {
			return nil, fmt.Errorf("Failed to apply update to TaskDriverRun: %s", err)
		}
	}
	return t, nil
}

// See documentation for db.DB interface.
func (d *btDB) UpdateTaskDriver(id string, msg *td.Message) error {
	// Encode the Message.
	buf := bytes.Buffer{}
	if err := gob.NewEncoder(&buf).Encode(msg); err != nil {
		return fmt.Errorf("Failed to gob-encode Message: %s", err)
	}
	// Insert the message into BigTable.
	mt := bigtable.NewMutation()
	mt.Set(BT_COLUMN_FAMILY, BT_COLUMN, bigtable.ServerTime, buf.Bytes())
	rk := rowKey(id, msg)
	return d.client.Open(BT_TABLE).Apply(context.Background(), rk, mt)
}
