package strategy

// TODO(borenet): Remove this file once all rollers have been upgraded.

import (
	"encoding/json"
	"time"

	"github.com/boltdb/bolt"
)

var (
	BUCKET_STRATEGY_HISTORY = []byte("strategyHistory")
)

// db is a struct used for interacting with a database.
type db struct {
	db *bolt.DB
}

// openDB returns a db instance.
func openDB(filename string) (*db, error) {
	d, err := bolt.Open(filename, 0600, nil)
	if err != nil {
		return nil, err
	}

	if err := d.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(BUCKET_STRATEGY_HISTORY); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return &db{d}, nil
}

// Close closes the db.
func (d *db) Close() error {
	return d.db.Close()
}

// timeToKey returns a BoltDB key for the given time.Time.
func timeToKey(t time.Time) []byte {
	return []byte(t.Format(time.RFC3339Nano))
}

// SetStrategy inserts a strategy change into the database.
func (d *db) SetStrategy(m *StrategyChange) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(BUCKET_STRATEGY_HISTORY)
		serialized, err := json.Marshal(m)
		if err != nil {
			return err
		}
		return b.Put(timeToKey(m.Time), serialized)
	})
}

// GetStrategyHistory returns the last N strategy changes.
func (d *db) GetStrategyHistory(N int) ([]*StrategyChange, error) {
	history := []*StrategyChange{}
	if err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(BUCKET_STRATEGY_HISTORY)
		c := b.Cursor()
		for k, v := c.Last(); k != nil && len(history) < N; k, v = c.Prev() {
			var m StrategyChange
			if err := json.Unmarshal(v, &m); err != nil {
				return err
			}
			m.Time = m.Time.UTC()
			history = append(history, &m)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return history, nil
}
