package digeststore

import (
	"encoding/json"
	"os"
	"path"

	"github.com/boltdb/bolt"
)

const (
	SUB_DIR_NAME   = "digeststore"
	DIGEST_DB_NAME = "digest_store.boltdb"
)

// DigestInfo aggregates all information we have about an individual digest.
type DigestInfo struct {
	// TestName for this digest.
	TestName string

	// Digest that uniquely identifies the digest within this test.
	Digest string

	// First containes the timestamp of the first occurance of this digest.
	First int64

	// Last contains the timestamp of the last time we have seen this digest.
	Last int64

	// Exception stores a string representing the exception that was encountered
	// retrieving this digest. If Exception is "" then there was no problem.
	Exception string

	// IssueIDs is a list of issue ids that are associated with this digest.
	IssueIDs []int
}

// UpdateTimestamps updates the time stamps of a DigestInfo based on the
// arguments. It returns true if the digest info was modified.
func (d *DigestInfo) UpdateTimestamps(first int64, last int64) bool {
	changed := false
	if first < d.First {
		d.First = first
		changed = true
	}
	if last > d.Last {
		d.Last = last
		changed = true
	}
	return changed
}

type DigestStore interface {
	// Get returns the information about the given testName/digest pair.
	Get(testName, digest string) (*DigestInfo, bool, error)

	// Update updates the stored information about the testname/digest
	// pairs identified in the list of DigestInfos.
	Update(digetInfos []*DigestInfo) error
}

type BoltDigestStore struct {
	digestDB *bolt.DB
}

func New(storageDir string) (DigestStore, error) {
	dbDir := path.Join(storageDir, SUB_DIR_NAME)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, err
	}
	db, err := bolt.Open(path.Join(dbDir, DIGEST_DB_NAME), 0666, nil)
	if err != nil {
		return nil, err
	}

	return &BoltDigestStore{digestDB: db}, nil
}

func (b *BoltDigestStore) Get(testName, digest string) (*DigestInfo, bool, error) {
	var ret *DigestInfo = nil
	err := b.digestDB.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(testName))
		if bucket == nil {
			return nil
		}

		if retBytes := bucket.Get([]byte(digest)); retBytes != nil {
			if err := json.Unmarshal(retBytes, &ret); err != nil {
				return err
			}
		}
		return nil
	})
	return ret, ret != nil, err
}

func (b *BoltDigestStore) Update(digestInfos []*DigestInfo) error {
	return b.digestDB.Update(func(tx *bolt.Tx) error {
		// Wrap everything into a single transaction. This avoids a write lock
		// by using the lock of the transaction.
		writeDigestInfos := make([]*DigestInfo, 0, len(digestInfos))
		for _, digestInfo := range digestInfos {
			di, found, err := b.Get(digestInfo.TestName, digestInfo.Digest)
			if err != nil {
				return err
			}

			// If the testname/digest was not found or needs to be updated we
			// record it.
			if !found {
				writeDigestInfos = append(writeDigestInfos, digestInfo)
			} else if di.UpdateTimestamps(digestInfo.First, digestInfo.Last) {
				writeDigestInfos = append(writeDigestInfos, di)
			}
		}

		// If no digest needs updating we are done.
		if len(writeDigestInfos) == 0 {
			return nil
		}

		for _, digestInfo := range writeDigestInfos {
			bucket, err := tx.CreateBucketIfNotExists([]byte(digestInfo.TestName))
			if err != nil {
				return err
			}

			jsonBytes, err := json.Marshal(digestInfo)
			if err != nil {
				return err
			}

			if err = bucket.Put([]byte(digestInfo.Digest), jsonBytes); err != nil {
				return err
			}
		}
		return nil
	})
}
