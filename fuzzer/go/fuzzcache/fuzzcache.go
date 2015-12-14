package fuzzcache

import (
	"bytes"
	"encoding/gob"
	"fmt"

	"github.com/boltdb/bolt"
	"go.skia.org/infra/fuzzer/go/fuzz"
)

var BOLT_KEY = []byte("key")

type FuzzReportCache struct {
	DB *bolt.DB
}

// Open opens a connection to a bolt db at the given file location.
func New(dbPath string) (FuzzReportCache, error) {
	c := FuzzReportCache{}
	db, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		return c, err
	}
	c.DB = db
	return c, nil
}

// Load returns a *fuzz.fuzz.FuzzReport that corresponds to the passed in hash, or an error
// if such a Report does not exist.
func (b *FuzzReportCache) Load(hash string) (*fuzz.FuzzReport, error) {
	var staging fuzz.FuzzReport
	loadFunc := func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(hash))
		if b == nil {
			return fmt.Errorf("Cache for commit %s does not exist", hash)
		}
		var temp bytes.Buffer
		dec := gob.NewDecoder(&temp)
		c := b.Get(BOLT_KEY)
		if n, err := temp.Write(c); err != nil || n != len(c) {
			return fmt.Errorf("Only wrote %d/%d bytes to decoder: %v", n, len(c), err)
		}
		return dec.Decode(&staging)
	}
	return &staging, b.DB.View(loadFunc)
}

// Store stores a fuzz.fuzz.Fuzz report to the underlying fuzz.FuzzReportCache.  It creates a bucket with the
// name of the given hash and stores the report as a []byte under a simple key.
func (b *FuzzReportCache) Store(report fuzz.FuzzReport, hash string) error {
	storeFunc := func(tx *bolt.Tx) error {
		bkt, err := tx.CreateBucketIfNotExists([]byte(hash))
		if err != nil {
			return fmt.Errorf("Could not make cache/bucket for %s", hash)
		}
		var temp bytes.Buffer
		enc := gob.NewEncoder(&temp)
		if err := enc.Encode(report); err != nil {
			return fmt.Errorf("Problem encoding report: %s", err)
		}
		return bkt.Put(BOLT_KEY, temp.Bytes())
	}
	return b.DB.Update(storeFunc)
}

// Close closes the underlying fuzz.FuzzReportCache, returning any errors the instance returns.
func (b *FuzzReportCache) Close() error {
	return b.DB.Close()
}
