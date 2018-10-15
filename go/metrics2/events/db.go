package events

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"time"

	"github.com/boltdb/bolt"
	"go.skia.org/infra/go/util"
)

// EventDB is an interface used for storing Events in a BoltDB.
type EventDB interface {
	Append(string, []byte) error
	Close() error
	Insert(*Event) error
	Range(string, time.Time, time.Time) ([]*Event, error)
}

// eventDB is a struct used for storing Events in a BoltDB.
type eventDB struct {
	db *bolt.DB
}

// NewEventDB returns an EventDB instance.
func NewEventDB(filename string) (EventDB, error) {
	db, err := bolt.Open(filename, 0600, nil)
	if err != nil {
		return nil, err
	}

	rv := &eventDB{
		db: db,
	}

	return rv, nil
}

// Close cleans up the eventDB.
func (m *eventDB) Close() error {
	return m.db.Close()
}

// Insert inserts the given Event into DB.
func (m *eventDB) Insert(e *Event) error {
	if util.TimeIsZero(e.Timestamp) {
		return fmt.Errorf("Cannot insert an event without a timestamp.")
	}
	if e.Stream == "" {
		return fmt.Errorf("Cannot insert an event without a stream.")
	}

	k, err := encodeKey(e.Timestamp)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(e); err != nil {
		return err
	}
	return m.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(e.Stream))
		if err != nil {
			return err
		}
		return b.Put(k, buf.Bytes())
	})
}

// Append inserts the given data into the given stream at the current time.
func (m *eventDB) Append(stream string, data []byte) error {
	return m.Insert(&Event{
		Stream:    stream,
		Timestamp: time.Now(),
		Data:      data,
	})
}

// Range returns all Events in the given range from the given stream.
func (m *eventDB) Range(stream string, start, end time.Time) ([]*Event, error) {
	min, err := encodeKey(start)
	if err != nil {
		return nil, err
	}
	max, err := encodeKey(end)
	if err != nil {
		return nil, err
	}

	rv := []*Event{}
	if err := m.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(stream))
		if b == nil {
			return nil
		}
		c := b.Cursor()
		for k, v := c.Seek(min); k != nil && bytes.Compare(k, max) <= 0; k, v = c.Next() {
			e := new(Event)
			if err := gob.NewDecoder(bytes.NewBuffer(v)).Decode(e); err != nil {
				return err
			}
			rv = append(rv, e)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return rv, nil
}
