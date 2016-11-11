package diffstore

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/boltdb/bolt"
	"github.com/skia-dev/glog"

	"go.skia.org/infra/golden/go/diff"
)

type failureStore struct {
	// failureDB stores the digests that have failed to load.
	failureDB *bolt.DB

	mutex          sync.RWMutex
	cachedFailures map[string]*diff.DigestFailure
}

func newFailureStore(fileName string) (*failureStore, error) {
	failureDB, err := bolt.Open(fileName, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("Unable to open failureDB: %s", err)
	}

	ret := &failureStore{
		failureDB: failureDB,
	}

	return ret, ret.loadDigestFailures()
}

func (f *failureStore) unavailableDigest() map[string]*diff.DigestFailure {
	f.mutex.RLock()
	defer f.mutex.RUnlock()
	return f.cachedFailures
}

// addDigestFailure adds a digest failure to the database or updates an
// existing failure if the provided one is newer.
func (f *failureStore) addDigestFailure(failure *diff.DigestFailure) error {
	jsonData, err := json.Marshal(failure)
	if err != nil {
		return err
	}

	updateFn := func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(FAILURE_BUCKET))
		if err != nil {
			return err
		}

		foundFailureBytes := bucket.Get([]byte(failure.Digest))
		if foundFailureBytes != nil {
			var foundFailure diff.DigestFailure
			if err := json.Unmarshal(foundFailureBytes, &foundFailure); err != nil {
				return err
			}

			if foundFailure.TS >= failure.TS {
				return nil
			}
		}

		return bucket.Put([]byte(failure.Digest), jsonData)
	}

	if err := f.failureDB.Update(updateFn); err != nil {
		return err
	}

	// Load the new failures into the cache.
	return f.loadDigestFailures()
}

// purgeDigestFailures removes the failure identified by digest from the database.
func (f *failureStore) purgeDigestFailures(digests []string) error {
	updateFn := func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(FAILURE_BUCKET))
		if bucket == nil {
			return nil
		}

		for _, d := range digests {
			if bucket.Get([]byte(d)) != nil {
				if err := bucket.Delete([]byte(d)); err != nil {
					glog.Errorf("Unable to delete failure for digest %s. Got error: %s", d, err)
				}
			}
		}
		return nil
	}
	if err := f.failureDB.Update(updateFn); err != nil {
		return err
	}
	return f.loadDigestFailures()
}

// loadDigestFailures loads all digest failures from the failures database.
func (f *failureStore) loadDigestFailures() error {
	var allFailures map[string]*diff.DigestFailure = nil

	viewFn := func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(FAILURE_BUCKET))
		if bucket == nil {
			return nil
		}

		n := bucket.Stats().KeyN
		if n == 0 {
			return nil
		}

		allFailures = make(map[string]*diff.DigestFailure, n)
		cursor := bucket.Cursor()
		for k, v := cursor.First(); k != nil; k, v = cursor.Next() {
			dFailure := &diff.DigestFailure{}
			if err := json.Unmarshal(v, dFailure); err != nil {
				return err
			}
			allFailures[dFailure.Digest] = dFailure
		}
		return nil
	}

	if err := f.failureDB.View(viewFn); err != nil {
		return err
	}

	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.cachedFailures = allFailures
	return nil
}
