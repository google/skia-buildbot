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

// digestFailureRec is a wrapper around diff.DigestFailure to implement the boltutil.Record interface.
type digestFailureRec struct {
	*diff.DigestFailure
}

// Key see boltutil.Record interface.
func (d *digestFailureRec) Key() string {
	return d.Digest
}

// IndexValues see boltutil.Record interface. No index at this point.
func (d *digestFailureRec) IndexValues() map[string][]string {
	return nil
}

// failureStore persists DigestFailures in boltDB database. It assumes that the
// number of unavailable digests is small and it keeps a copy of the entire list
// in memory at all time.
type failureStore struct {
	// store stores the digests that have failed to load.
	store *boltutil.IndexedBucket

	// cachedFailures caches all failures for fast lookup.
	cachedFailures map[string]*diff.DigestFailure

	// dbMutex protects changes to the database.
	dbMutex sync.Mutex

	// cacheMutex protects cachedFailures.
	cacheMutex sync.RWMutex
}

// newFailureStore returns a new instance of failureStore that opens a database in the given directory.
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

// unavailableDigests returns the current list of unavailable digests for fast lookup.
func (f *failureStore) unavailableDigests() map[string]*diff.DigestFailure {
	f.cacheMutex.RLock()
	defer f.cacheMutex.RUnlock()
	return f.cachedFailures
}

// addDigestFailureIfNew adds a digest failure to the database only if the
// there is no failure recorded for the given digest.
func (f *failureStore) addDigestFailureIfNew(failure *diff.DigestFailure) error {
	unavailable := f.unavailableDigests()
	if _, ok := unavailable[failure.Digest]; !ok {
		return f.addDigestFailure(failure)
	}
	return nil
}

// addDigestFailure adds a digest failure to the database or updates an
// existing failure.
func (f *failureStore) addDigestFailure(failure *diff.DigestFailure) error {
	f.dbMutex.Lock()
	defer f.dbMutex.Unlock()

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

// purgeDigestFailures removes the failures identified by digests from the database.
func (f *failureStore) purgeDigestFailures(digests []string) error {
	f.dbMutex.Lock()
	defer f.dbMutex.Unlock()

	targets := make([]string, 0, len(digests))
	unavailable := f.unavailableDigests()
	for _, d := range digests {
		if _, ok := unavailable[d]; ok {
			targets = append(targets, d)
		}
	}

	if len(targets) == 0 {
		return nil
	}

	if err := f.store.Delete(digests); err != nil {
		return err
	}

	return f.loadDigestFailures()
}

// loadDigestFailures loads all digest failures into memory and updates the current cache.
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

	f.cacheMutex.Lock()
	defer f.cacheMutex.Unlock()
	f.cachedFailures = ret
	return nil
}
