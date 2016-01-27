package fuzzcache

import (
	"bytes"
	"encoding/gob"
	"fmt"

	"github.com/boltdb/bolt"
	"go.skia.org/infra/fuzzer/go/fuzz"
)

const REPORT_KEY = "report-"

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

func getReportKey(category string) []byte {
	return []byte(REPORT_KEY + category)
}

// Load returns a *fuzz.fuzz.FuzzReport that corresponds to the passed in revision,
// and the fuzz names associated with the report.
// It returns an error if such a Report does not exist.
func (b *FuzzReportCache) LoadTree(category, revision string) (*fuzz.FuzzReportTree, error) {
	var report fuzz.FuzzReportTree
	loadFunc := func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(revision))
		if b == nil {
			return fmt.Errorf("Cache for revision %s does not exist", revision)
		}
		c := b.Get(getReportKey(category))
		if c == nil {
			return fmt.Errorf("Could not find report for revision %s", revision)
		}
		dec := gob.NewDecoder(bytes.NewBuffer(c))
		if err := dec.Decode(&report); err != nil {
			return fmt.Errorf("Could not decode report: %s", err)
		}
		return nil
	}
	return &report, b.DB.View(loadFunc)
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

// Store stores a fuzz.FuzzReport and the fuzzNames associated with it to the underlying
// fuzz.FuzzReportCache. It creates a bucket with the
// name of the given revision and stores the report as a []byte under a simple key.
func (b *FuzzReportCache) StoreTree(report fuzz.FuzzReportTree, category, revision string) error {
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
		if err := bkt.Put(getReportKey(category), buffReport.Bytes()); err != nil {
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

// Close closes the underlying fuzz.FuzzReportCache, returning any errors the instance returns.
func (b *FuzzReportCache) Close() error {
	return b.DB.Close()
}
