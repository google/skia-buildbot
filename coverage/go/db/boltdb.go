package db

import (
	"bytes"
	"encoding/gob"
	"fmt"

	"github.com/boltdb/bolt"
	"go.skia.org/infra/coverage/go/common"
	"go.skia.org/infra/go/sklog"
)

// The CoverageCache interface abstracts the logic for caching CoverageSummaries.
// A CoverageCache should be thread-safe.
type CoverageCache interface {
	// Returns the element from the cache if it exists, otherwise the second arg will be false.
	CheckCache(key string) (common.CoverageSummary, bool)
	// Stores the coverage data to cache, returning any errors.
	StoreToCache(key string, cov common.CoverageSummary) error
}

type boltDB struct {
	DB *bolt.DB
}

const COVERAGE_SUMMARY_KEY = "coverage_summary_key"

// NewBoltDB returns a CoverageCache implemented by boltdb.
func NewBoltDB(path string) (*boltDB, error) {
	d := boltDB{}
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("Could not open coverage cache file: %s", err)
	}
	d.DB = db
	return &d, nil
}

// CheckCache implements the CoverageCache interface.
func (b *boltDB) CheckCache(key string) (common.CoverageSummary, bool) {
	ok := false
	coverage := common.CoverageSummary{}
	loadFunc := func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(key))
		if b == nil {
			return fmt.Errorf("Cache for key %s does not exist", key)
		}
		c := b.Get([]byte(COVERAGE_SUMMARY_KEY))
		if c == nil {
			return fmt.Errorf("Could not extract coverage summary for key %s", key)
		}
		ok = true
		dec := gob.NewDecoder(bytes.NewBuffer(c))
		if err := dec.Decode(&coverage); err != nil {
			return fmt.Errorf("Could not decode report: %s", err)
		}
		return nil
	}
	if err := b.DB.View(loadFunc); err != nil {
		sklog.Infof("Coverage Summary not found in cache: %s", err)
	}
	return coverage, ok
}

// StoreToCache implements the CoverageCache interface.
func (b *boltDB) StoreToCache(key string, cov common.CoverageSummary) error {
	storeFunc := func(tx *bolt.Tx) error {
		bkt, err := tx.CreateBucketIfNotExists([]byte(key))
		if err != nil {
			return fmt.Errorf("Could not make cache/bucket for %s", key)
		}
		var buffCov bytes.Buffer
		enc := gob.NewEncoder(&buffCov)
		if err := enc.Encode(cov); err != nil {
			return fmt.Errorf("Problem encoding report: %s", err)
		}
		if err := bkt.Put([]byte(COVERAGE_SUMMARY_KEY), buffCov.Bytes()); err != nil {
			return fmt.Errorf("Problem storing %d bytes of report: %s", buffCov.Len(), err)
		}
		return nil
	}
	return b.DB.Update(storeFunc)
}

// Close closes the underlying boltDB.
func (b *boltDB) Close() error {
	return b.DB.Close()
}
