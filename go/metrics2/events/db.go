package events

import (
	"bytes"
	"time"

	"github.com/boltdb/bolt"
)

// eventDB is a struct used for storing Events in a BoltDB.
type eventDB struct {
	db *bolt.DB
}

// newEventDB returns an eventDB instance.
func newEventDB(filename string) (*eventDB, error) {
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

// InsertAt inserts the given Event into the given stream at the given time.
func (m *eventDB) InsertAt(stream string, ts time.Time, e Event) error {
	k, err := encodeKey(ts)
	if err != nil {
		return err
	}
	return m.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(stream))
		if err != nil {
			return err
		}
		return b.Put(k, e)
	})
}

// Insert inserts the given Event into the given stream at the current time.
func (m *eventDB) Insert(stream string, e Event) error {
	return m.InsertAt(stream, time.Now(), e)
}

// GetRange returns all Events in the given range from the given stream.
func (m *eventDB) GetRange(stream string, start, end time.Time) ([]time.Time, []Event, error) {
	min, err := encodeKey(start)
	if err != nil {
		return nil, nil, err
	}
	max, err := encodeKey(end)
	if err != nil {
		return nil, nil, err
	}

	ts := []time.Time{}
	es := []Event{}
	if err := m.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte(stream)).Cursor()
		for k, v := c.Seek(min); k != nil && bytes.Compare(k, max) <= 0; k, v = c.Next() {
			t, err := decodeKey(k)
			if err != nil {
				return err
			}
			ts = append(ts, t)
			es = append(es, v)
		}
		return nil
	}); err != nil {
		return nil, nil, err
	}
	return ts, es, nil
}
