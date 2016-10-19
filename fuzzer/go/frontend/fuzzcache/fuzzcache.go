package fuzzcache

// The FuzzReportCache is a wrapper around a bolt db that stores bad/grey fuzz names as
// well as the data structure that can be used to query for fuzz results.

import (
	"bytes"
	"encoding/gob"
	"fmt"

	"github.com/boltdb/bolt"
	"go.skia.org/infra/fuzzer/go/frontend/fuzzpool"
)

const POOL_KEY = "fuzzpool"

var FUZZ_NAMES_KEY = []byte("fuzz_names")

type FuzzReportCache struct {
	DB *bolt.DB
}

// Open opens a connection to a bolt db at the given file location.
func New(dbPath string) (*FuzzReportCache, error) {
	c := FuzzReportCache{}
	db, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("Could not open cache file: %s", err)
	}
	c.DB = db
	return &c, nil
}

// LoadPool fills the passed in *fuzzpool.FuzzPool with the data corresponding to the revision.
// It returns an error if such a FuzzPool does not exist.
func (b *FuzzReportCache) LoadPool(pool *fuzzpool.FuzzPool, revision string) error {
	loadFunc := func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(revision))
		if b == nil {
			return fmt.Errorf("Cache for revision %s does not exist", revision)
		}
		c := b.Get([]byte(POOL_KEY))
		if c == nil {
			return fmt.Errorf("Could not find report for revision %s", revision)
		}
		dec := gob.NewDecoder(bytes.NewBuffer(c))
		if err := dec.Decode(pool); err != nil {
			return fmt.Errorf("Could not decode report: %s", err)
		}
		return nil
	}
	return b.DB.View(loadFunc)
}

func (b *FuzzReportCache) LoadFuzzNames(revision string) ([]string, error) {
	var fuzzNames []string
	loadFunc := func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(revision))
		if b == nil {
			return fmt.Errorf("Cache for revision %s does not exist", revision)
		}
		c := b.Get(FUZZ_NAMES_KEY)
		if c == nil {
			return fmt.Errorf("Could not find stored bad fuzzes for revision %s", revision)
		}
		dec := gob.NewDecoder(bytes.NewBuffer(c))
		if err := dec.Decode(&fuzzNames); err != nil {
			return fmt.Errorf("Could not decode fuzzNames: %s", err)
		}
		return nil
	}
	return fuzzNames, b.DB.View(loadFunc)
}

// Store stores a fuzzpool.FuzzPool and the fuzzNames associated with it to the underlying
// data.FuzzReportCache. It creates a bucket with the
// name of the given revision and stores the report as a []byte under a simple key.
func (b *FuzzReportCache) StorePool(report *fuzzpool.FuzzPool, revision string) error {
	storeFunc := func(tx *bolt.Tx) error {
		bkt, err := tx.CreateBucketIfNotExists([]byte(revision))
		if err != nil {
			return fmt.Errorf("Could not make cache/bucket for %s", revision)
		}
		var buffReport bytes.Buffer
		enc := gob.NewEncoder(&buffReport)
		if err := enc.Encode(report); err != nil {
			return fmt.Errorf("Problem encoding report: %s", err)
		}
		if err := bkt.Put([]byte(POOL_KEY), buffReport.Bytes()); err != nil {
			return fmt.Errorf("Problem storing %d bytes of report: %s", buffReport.Len(), err)
		}
		return nil
	}
	return b.DB.Update(storeFunc)
}

func (b *FuzzReportCache) StoreFuzzNames(fuzzNames []string, revision string) error {
	storeFunc := func(tx *bolt.Tx) error {
		bkt, err := tx.CreateBucketIfNotExists([]byte(revision))
		if err != nil {
			return fmt.Errorf("Could not make cache/bucket for %s", revision)
		}
		var buffNames bytes.Buffer
		enc := gob.NewEncoder(&buffNames)
		if err := enc.Encode(fuzzNames); err != nil {
			return fmt.Errorf("Problem encoding fuzz names: %s", err)
		}
		if err := bkt.Put(FUZZ_NAMES_KEY, buffNames.Bytes()); err != nil {
			return fmt.Errorf("Problem storing %d bytes of fuzzNames: %s", buffNames.Len(), err)
		}
		return nil
	}
	return b.DB.Update(storeFunc)
}

// Close closes the underlying data.FuzzReportCache, returning any errors the instance returns.
func (b *FuzzReportCache) Close() error {
	return b.DB.Close()
}
