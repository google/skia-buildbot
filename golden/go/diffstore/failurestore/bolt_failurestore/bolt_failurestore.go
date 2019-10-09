package bolt_failurestore

import (
	"path/filepath"
	"sync"

	"github.com/boltdb/bolt"
	"go.skia.org/infra/go/boltutil"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/diffstore/common"
	"go.skia.org/infra/golden/go/diffstore/failurestore"
	"go.skia.org/infra/golden/go/types"
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
	return string(d.Digest)
}

// IndexValues see boltutil.Record interface. No index at this point.
func (d *digestFailureRec) IndexValues() map[string][]string {
	return nil
}

// BoltImpl persists DigestFailures in boltDB database. It assumes that the
// number of unavailable digests is small and it keeps a copy of the entire list
// in memory at all time.
type BoltImpl struct {
	// store stores the digests that have failed to load.
	store *boltutil.IndexedBucket

	// cachedFailures caches all failures for fast lookup.
	cachedFailures map[types.Digest]*diff.DigestFailure

	// dbMutex protects changes to the database.
	dbMutex sync.Mutex

	// cacheMutex protects cachedFailures.
	cacheMutex sync.RWMutex
}

// New returns a new instance of BoltImpl that opens a database in the given directory.
func New(baseDir string) (*BoltImpl, error) {
	baseDir = filepath.Join(baseDir, FAILUREDB_NAME)
	db, err := common.OpenBoltDB(baseDir, FAILUREDB_NAME+".db")
	if err != nil {
		return nil, err
	}

	config := &boltutil.Config{
		DB:      db,
		Name:    FAILUREDB_NAME,
		Indices: []string{},
		Codec:   util.NewJSONCodec(digestFailureRec{}),
	}
	store, err := boltutil.NewIndexedBucket(config)
	if err != nil {
		return nil, err
	}

	ret := &BoltImpl{
		store: store,
	}

	if err := ret.loadDigestFailures(); err != nil {
		return nil, err
	}

	return ret, nil
}

// UnavailableDigests returns the current list of unavailable digests for fast lookup.
func (f *BoltImpl) UnavailableDigests() map[types.Digest]*diff.DigestFailure {
	f.cacheMutex.RLock()
	defer f.cacheMutex.RUnlock()
	return f.cachedFailures
}

// AddDigestFailureIfNew adds a digest failure to the database only if the
// there is no failure recorded for the given digest.
func (f *BoltImpl) AddDigestFailureIfNew(failure *diff.DigestFailure) error {
	unavailable := f.UnavailableDigests()
	if _, ok := unavailable[failure.Digest]; !ok {
		return f.AddDigestFailure(failure)
	}
	return nil
}

// AddDigestFailure adds a digest failure to the database or updates an
// existing failure.
func (f *BoltImpl) AddDigestFailure(failure *diff.DigestFailure) error {
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

// PurgeDigestFailures removes the failures identified by digests from the database.
func (f *BoltImpl) PurgeDigestFailures(digests types.DigestSlice) error {
	f.dbMutex.Lock()
	defer f.dbMutex.Unlock()

	targets := make([]string, 0, len(digests))
	unavailable := f.UnavailableDigests()
	for _, d := range digests {
		if _, ok := unavailable[d]; ok {
			targets = append(targets, string(d))
		}
	}

	if len(targets) == 0 {
		return nil
	}

	if err := f.store.Delete(targets); err != nil {
		return err
	}

	return f.loadDigestFailures()
}

// loadDigestFailures loads all digest failures into memory and updates the current cache.
func (f *BoltImpl) loadDigestFailures() error {
	allFailures, _, err := f.store.List(0, -1)
	if err != nil {
		return err
	}

	ret := make(map[types.Digest]*diff.DigestFailure, len(allFailures))
	for _, rec := range allFailures {
		failure := rec.(*digestFailureRec)
		ret[failure.Digest] = failure.DigestFailure
	}

	f.cacheMutex.Lock()
	defer f.cacheMutex.Unlock()
	f.cachedFailures = ret
	return nil
}

// Make sure BoltImpl fulfills the FailureStore interface
var _ failurestore.FailureStore = (*BoltImpl)(nil)
