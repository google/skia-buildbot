package fuzzcache

import (
	"bytes"
	"encoding/gob"
	"fmt"

	"github.com/boltdb/bolt"
	"go.skia.org/infra/fuzzer/go/fuzz"
)

var REPORT_KEY = []byte("report")
var BINARY_FUZZES_KEY = []byte("binary_fuzzes")

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

// Load returns a *fuzz.fuzz.FuzzReport that corresponds to the passed in revision,
// and the fuzz names associated with the report.
// It returns an error if such a Report does not exist.
func (b *FuzzReportCache) Load(revision string) (*fuzz.FuzzReportTree, []string, error) {
	var report fuzz.FuzzReportTree
	var binaryFuzzNames []string
	loadFunc := func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(revision))
		if b == nil {
			return fmt.Errorf("Cache for revision %s does not exist", revision)
		}
		c := b.Get(REPORT_KEY)
		if c == nil {
			return fmt.Errorf("Could not find report for revision %s", revision)
		}
		dec := gob.NewDecoder(bytes.NewBuffer(c))
		if err := dec.Decode(&report); err != nil {
			return fmt.Errorf("Could not decode report: %s", err)
		}

		c = b.Get(BINARY_FUZZES_KEY)
		if c == nil {
			return fmt.Errorf("Could not find stored binary fuzzes for revision %s", revision)
		}
		dec = gob.NewDecoder(bytes.NewBuffer(c))
		if err := dec.Decode(&binaryFuzzNames); err != nil {
			return fmt.Errorf("Could not decode binaryFuzzNames: %s", err)
		}
		return nil
	}
	return &report, binaryFuzzNames, b.DB.View(loadFunc)
}

// Store stores a fuzz.FuzzReport and the binaryFuzzNames associated with it to the underlying
// fuzz.FuzzReportCache. It creates a bucket with the
// name of the given revision and stores the report as a []byte under a simple key.
func (b *FuzzReportCache) Store(report fuzz.FuzzReportTree, binaryFuzzNames []string, revision string) error {
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
		if err := bkt.Put(REPORT_KEY, buffReport.Bytes()); err != nil {
			return fmt.Errorf("Problem storing %d bytes of report: %s", buffReport.Len(), err)
		}
		var buffNames bytes.Buffer
		enc = gob.NewEncoder(&buffNames)
		if err := enc.Encode(binaryFuzzNames); err != nil {
			return fmt.Errorf("Problem encoding fuzz names: %s", err)
		}
		if err := bkt.Put(BINARY_FUZZES_KEY, buffNames.Bytes()); err != nil {
			return fmt.Errorf("Problem storing %d bytes of binaryFuzzNames: %s", buffNames.Len(), err)
		}
		return nil
	}
	return b.DB.Update(storeFunc)
}

// Close closes the underlying fuzz.FuzzReportCache, returning any errors the instance returns.
func (b *FuzzReportCache) Close() error {
	return b.DB.Close()
}
