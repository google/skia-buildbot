package diffstore

import (
	"sync"

	"github.com/boltdb/bolt"

	"go.skia.org/infra/go/boltutil"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
)

const (
	// FAILUREDB_NAME is the name of the boltdb storing diff failures.
	FAILUREDB_NAME = "diffstore_failures"
)

type digestFailureRec struct {
	*diff.DigestFailure
}

// Key see boltutil.Record interface.
func (d *digestFailureRec) Key() string {
	return d.Digest
}

// IndexValues see boltutil.Record interface.
func (d *digestFailureRec) IndexValues() map[string][]string {
	return nil
}

// failureStore persists DigestFailures in bolDB database.
type failureStore struct {
	// store stores the digests that have failed to load.
	store *boltutil.IndexedBucket

	mutex          sync.RWMutex
	cachedFailures map[string]*diff.DigestFailure
}

func newFailureStore(baseDir string) (*failureStore, error) {
	db, err := openBoltDB(baseDir, FAILUREDB_NAME+".db")
	if err != nil {
		return nil, err
	}

	config := &boltutil.Config{
		DB:      db,
		Name:    FAILUREDB_NAME,
		Indices: []string{},
		Codec:   util.JSONCodec(digestFailureRec{}),
	}
	store, err := boltutil.NewIndexedBucket(config)
	if err != nil {
		return nil, err
	}

	ret := &failureStore{
		store: store,
	}

	if err := ret.loadDigestFailures(); err != nil {
		return nil, err
	}

	return ret, nil
}

func (f *failureStore) unavailableDigest() map[string]*diff.DigestFailure {
	f.mutex.RLock()
	defer f.mutex.RUnlock()
	return f.cachedFailures
}

// addDigestFailure adds a digest failure to the database or updates an
// existing failure if the provided one is newer.
func (f *failureStore) addDigestFailure(failure *diff.DigestFailure) error {
	inputRecs := []boltutil.Record{&digestFailureRec{DigestFailure: failure}}
	updateFn := func(tx *bolt.Tx, result []boltutil.Record) error {
		if result[0] != nil {
			if result[0].(*digestFailureRec).TS >= failure.TS {
				result[0] = nil
				return nil
			}
		}
		result[0] = inputRecs[0]
		return nil
	}

	if err := f.store.Update(inputRecs, updateFn); err != nil {
		return err
	}

	// Load the new failures into the cache.
	return f.loadDigestFailures()
}

// purgeDigestFailures removes the failure identified by digest from the database.
func (f *failureStore) purgeDigestFailures(digests []string) error {
	if err := f.store.Delete(digests); err != nil {
		return err
	}

	return f.loadDigestFailures()
}

// loadDigestFailures loads all digest failures from the failures database.
func (f *failureStore) loadDigestFailures() error {

	allFailures, _, err := f.store.List(0, -1)
	if err != nil {
		return err
	}

	ret := make(map[string]*diff.DigestFailure, len(allFailures))
	for _, rec := range allFailures {
		failure := rec.(*digestFailureRec)
		ret[failure.Digest] = failure.DigestFailure
	}

	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.cachedFailures = ret
	return nil
}
