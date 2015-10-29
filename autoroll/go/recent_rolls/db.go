package recent_rolls

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"time"

	"github.com/boltdb/bolt"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/autoroll"
)

var (
	BUCKET_ROLLS         = []byte("rolls")
	BUCKET_ROLLS_BY_DATE = []byte("rollsByDate")
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
		if _, err := tx.CreateBucketIfNotExists(BUCKET_ROLLS); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(BUCKET_ROLLS_BY_DATE); err != nil {
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

// issueToRollKey converts an issue number to a BoltDB key.
func issueToRollKey(issue int64) []byte {
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, issue); err != nil {
		glog.Fatalf("Failed to serialize int64: %d", issue)
	}
	return buf.Bytes()
}

// rollKey returns a BoltDB key for the given AutoRollIssue based on its issue
// number.
func rollKey(a *autoroll.AutoRollIssue) []byte {
	return issueToRollKey(a.Issue)
}

// timeToKey returns a BoltDB key for the given time.Time.
func timeToKey(t time.Time) []byte {
	return []byte(t.Format(time.RFC3339Nano))
}

// timeKey returns a BoltDB key for the given AutoRollIssue based on its
// last-modified time.
func timeKey(a *autoroll.AutoRollIssue) []byte {
	return timeToKey(a.Modified)
}

// insertRoll inserts the given AutoRollIssue into the database within the
// given transaction.
func insertRoll(tx *bolt.Tx, a *autoroll.AutoRollIssue) error {
	rolls := tx.Bucket(BUCKET_ROLLS)
	rollsByDate := tx.Bucket(BUCKET_ROLLS_BY_DATE)

	serialized, err := json.Marshal(a)
	if err != nil {
		return err
	}

	if err := rolls.Put(rollKey(a), serialized); err != nil {
		return err
	}
	return rollsByDate.Put(timeKey(a), rollKey(a))
}

// deleteRoll deletes the given AutoRollIssue from the database within the
// given transaction.
func deleteRoll(tx *bolt.Tx, a *autoroll.AutoRollIssue) error {
	rolls := tx.Bucket(BUCKET_ROLLS)
	rollsByDate := tx.Bucket(BUCKET_ROLLS_BY_DATE)

	// The Modified time may have changed, so use the one we already have in the DB.
	serialized := rolls.Get(rollKey(a))
	if serialized == nil {
		return fmt.Errorf("The given issue (%d) does not exist in %s", a.Issue, string(BUCKET_ROLLS))
	}
	var oldIssue autoroll.AutoRollIssue
	if err := json.Unmarshal(serialized, &oldIssue); err != nil {
		return err
	}

	oldByDate := rollsByDate.Get(timeKey(&oldIssue))
	if oldByDate == nil {
		return fmt.Errorf("The given issue (%d) does not exist in %s", a.Issue, string(BUCKET_ROLLS_BY_DATE))
	}

	if err := rollsByDate.Delete(timeKey(&oldIssue)); err != nil {
		return err
	}
	if err := rolls.Delete(rollKey(a)); err != nil {
		return err
	}
	return nil
}

// InsertRoll inserts the given AutoRollIssue into the database.
func (d *db) InsertRoll(a *autoroll.AutoRollIssue) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		return insertRoll(tx, a)
	})
}

// DeleteRoll deletes the given AutoRollIssue from the database.
func (d *db) DeleteRoll(a *autoroll.AutoRollIssue) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		return deleteRoll(tx, a)
	})
}

// UpdateRoll updates the given AutoRollIssue in the database.
func (d *db) UpdateRoll(a *autoroll.AutoRollIssue) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		if err := deleteRoll(tx, a); err != nil {
			return err
		}
		return insertRoll(tx, a)
	})
}

// GetRoll retrieves the given issue from the database.
func (d *db) GetRoll(issue int64) (*autoroll.AutoRollIssue, error) {
	var a *autoroll.AutoRollIssue
	if err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(BUCKET_ROLLS)

		serialized := b.Get(issueToRollKey(issue))
		if serialized == nil {
			return nil
		}
		a = &autoroll.AutoRollIssue{}
		if err := json.Unmarshal(serialized, a); err != nil {
			return err
		}
		a.Modified = a.Modified.UTC()
		return nil
	}); err != nil {
		return nil, err
	}
	return a, nil
}

// GetRecentRolls retrieves the most recent N issues from the database.
func (d *db) GetRecentRolls(N int) ([]*autoroll.AutoRollIssue, error) {
	var rv []*autoroll.AutoRollIssue
	if err := d.db.View(func(tx *bolt.Tx) error {
		// Retrieve the issue keys from the by-date bucket.
		byDate := tx.Bucket(BUCKET_ROLLS_BY_DATE)
		c := byDate.Cursor()
		keys := make([][]byte, 0, N)
		for k, v := c.Last(); k != nil && len(keys) < N; k, v = c.Prev() {
			keys = append(keys, v)
		}

		// Retrieve the issues themselves.
		b := tx.Bucket(BUCKET_ROLLS)
		rv = make([]*autoroll.AutoRollIssue, 0, len(keys))
		for _, k := range keys {
			serialized := b.Get(k)
			if serialized == nil {
				return fmt.Errorf("DB consistency error: bucket %s contains data not present in bucket %s!", BUCKET_ROLLS_BY_DATE, BUCKET_ROLLS)
			}
			var a autoroll.AutoRollIssue
			if err := json.Unmarshal(serialized, &a); err != nil {
				return err
			}
			a.Modified = a.Modified.UTC()
			rv = append(rv, &a)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return rv, nil
}
