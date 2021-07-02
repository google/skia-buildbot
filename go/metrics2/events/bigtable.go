package events

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"time"

	"cloud.google.com/go/bigtable"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
)

const (
	// BigTable configuration.

	// We use a single BigTable table to store all event metrics.
	BT_TABLE = "metrics-eventdb"

	// We use a single BigTable column family.
	BT_COLUMN_FAMILY = "EVTS"

	// We use a single BigTable column which stores gob-encoded Events.
	BT_COLUMN = "EVT"

	INSERT_TIMEOUT = 30 * time.Second
	QUERY_TIMEOUT  = 3 * 60 * time.Second
)

var (
	// Fully-qualified BigTable column name.
	BT_COLUMN_FULL = fmt.Sprintf("%s:%s", BT_COLUMN_FAMILY, BT_COLUMN)
)

// btRowKey returns a BigTable row key for the given stream and timestamp.
func btRowKey(stream string, ts time.Time) string {
	return fmt.Sprintf("%s#%s", stream, ts.UTC().Format(util.SAFE_TIMESTAMP_FORMAT))
}

// BigTable implementation of EventDB.
type btEventDB struct {
	client *bigtable.Client
	table  *bigtable.Table
}

// NewBTEventDB returns an EventDB which is backed by BigTable.
func NewBTEventDB(ctx context.Context, btProject, btInstance string, ts oauth2.TokenSource) (EventDB, error) {
	client, err := bigtable.NewClient(ctx, btProject, btInstance, option.WithTokenSource(ts))
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to create BigTable client")
	}
	table := client.Open(BT_TABLE)
	return &btEventDB{
		table: table,
	}, nil
}

// See documentation for EventDB interface.
func (db *btEventDB) Append(stream string, data []byte) error {
	return db.Insert(&Event{
		Stream:    stream,
		Timestamp: time.Now(),
		Data:      data,
	})
}

// See documentation for EventDB interface.
func (db *btEventDB) Close() error {
	return db.client.Close()
}

// See documentation for EventDB interface.
func (db *btEventDB) Insert(e *Event) error {
	if util.TimeIsZero(e.Timestamp) {
		return skerr.Fmt("Cannot insert an event without a timestamp.")
	}
	if e.Stream == "" {
		return skerr.Fmt("Cannot insert an event without a stream.")
	}
	rk := btRowKey(e.Stream, e.Timestamp)
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(e); err != nil {
		return skerr.Wrapf(err, "failed to encode event")
	}
	mut := bigtable.NewMutation()
	mut.Set(BT_COLUMN_FAMILY, BT_COLUMN, bigtable.ServerTime, buf.Bytes())
	ctx, cancel := context.WithTimeout(context.TODO(), INSERT_TIMEOUT)
	defer cancel()
	return db.table.Apply(ctx, rk, mut)
}

// See documentation for EventDB interface.
func (db *btEventDB) Range(stream string, start, end time.Time) ([]*Event, error) {
	var rv []*Event
	if err := util.IterTimeChunks(start, end, 24*time.Hour, func(start, end time.Time) error {
		var rvErr error
		s := btRowKey(stream, start)
		e := btRowKey(stream, end)
		ctx, cancel := context.WithTimeout(context.TODO(), QUERY_TIMEOUT)
		defer cancel()
		if err := db.table.ReadRows(ctx, bigtable.NewRange(s, e), func(row bigtable.Row) bool {
			for _, ri := range row[BT_COLUMN_FAMILY] {
				if ri.Column == BT_COLUMN_FULL {
					var ev Event
					rvErr = gob.NewDecoder(bytes.NewReader(ri.Value)).Decode(&ev)
					if rvErr != nil {
						return false
					}
					rv = append(rv, &ev)
					// We only store one event per row.
					return true
				}
			}
			return true
		}, bigtable.RowFilter(bigtable.LatestNFilter(1))); err != nil {
			return skerr.Wrapf(err, "failed to ReadRows from BigTable")
		}
		return skerr.Wrap(rvErr)
	}); err != nil {
		return nil, skerr.Wrap(err)
	}
	return rv, nil
}
