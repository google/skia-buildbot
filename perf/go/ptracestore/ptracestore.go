// Package pstracestore is a database for Perf data.
package ptracestore

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/boltdb/bolt"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/constants"
	"go.skia.org/infra/perf/go/types"
)

const (
	MAX_CACHED_TILES = 20

	TRACE_VALUES_BUCKET_NAME  = "traces"
	TRACE_SOURCES_BUCKET_NAME = "sources"
	SOURCE_LIST_BUCKET_NAME   = "sourceList"
)

var (
	// tileNotExist is returned from getBoltDB only if 'readonly' is true and
	// the tile doesn't exist.
	tileNotExist = errors.New("Tile does not exist.")
)

// KeyMatches is a func that returns true if a key matches some criteria.
// Passed to Match().
type KeyMatches func(key string) bool

// PTraceStore is an interface for storing Perf data.
//
// PTraceStore doesn't know anything about git hashes or code review issue IDs,
// that will be handled at a level above this.
//
type PTraceStore interface {
	// Add new values to the datastore at the given commitID.
	//
	// values - A map from the trace id to a float32 value.
	// sourceFile - The full path of the file where this information came from,
	//   usually the Google Storage URL.
	// ts - The timestamp of the values being added (ignored).
	Add(commitID *cid.CommitID, values map[string]float32, sourceFile string, ts time.Time) error

	// Retrieve the source and value for a given measurement in a given trace,
	// and a non-nil error if no such point was found.
	Details(commitID *cid.CommitID, traceID string) (string, float32, error)

	// Match returns TraceSet that match the given Query and slice of cid.CommitIDs.
	//
	// The 'progess' callback will be called as each Tile is processed.
	//
	// The returned TraceSet will contain a slice of Trace, and that list will be
	// empty if there are no matches.
	Match(commitIDs []*cid.CommitID, matches KeyMatches, progress types.Progress) (types.TraceSet, error)
}

// BoltTraceStore is an implementation of PTraceStore that uses BoltDB.
type BoltTraceStore struct {
	// mutex protects access to cache.
	mutex sync.Mutex

	// cache is a cache of opened tiles.
	cache map[string]*bolt.DB

	// metrics
	cacheLen metrics2.Int64Metric

	// dir is the directory where tiles are stored.
	dir string
}

// New creates a new BoltTraceStore that stores tiles in the given directory.
func New(dir string) (*BoltTraceStore, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("Failed to create %q for ptracestore: %s", dir, err)
	}

	bs := &BoltTraceStore{
		dir:      dir,
		cache:    map[string]*bolt.DB{},
		cacheLen: metrics2.GetInt64Metric("perf_ptracestore_cache_len", nil),
	}
	go bs.metrics()
	return bs, nil
}

func (b *BoltTraceStore) metrics() {
	for _ = range time.Tick(time.Minute) {
		b.mutex.Lock()
		b.cacheLen.Update(int64(len(b.cache)))
		b.mutex.Unlock()
	}
}

// traceValue is used to encode/decode trace values.
type traceValue struct {
	Index int64
	Value float32
}

// sourceValue is used to encode/decode trace sources.
type sourceValue struct {
	Index  int64
	Source uint64
}

// getBoltDB returns a new/existing bolt.DB. Already opened db's are cached.
//
// If 'readonly' is true then getBoltDB will fail with a tileNotExist error
// instead of creating a new DB at that location.
//
// Calls must call Done() on the returned cacheEntry when they are done using it.
func (b *BoltTraceStore) getBoltDB(commitID *cid.CommitID, readonly bool) (*bolt.DB, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	name := commitID.Filename()
	if bdb, ok := b.cache[name]; ok {
		return bdb, nil
	}
	sklog.Infof("Opening %q for the first time.", name)

	filename := filepath.Join(b.dir, commitID.Filename())
	if _, err := os.Stat(filename); os.IsNotExist(err) && readonly {
		return nil, tileNotExist
	}
	db, err := bolt.Open(filename, 0600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("Unable to open %q: %s", filename, err)
	}
	b.cache[name] = db
	return db, nil
}

func uint64ToBytes(u uint64) []byte {
	b := make([]byte, 8, 8)
	binary.LittleEndian.PutUint64(b, u)
	return b
}

func serialize(i interface{}) ([]byte, error) {
	buf := &bytes.Buffer{}
	err := binary.Write(buf, binary.LittleEndian, i)
	if err != nil {
		return nil, fmt.Errorf("binary.Write of value failed: %s", err)
	}
	return buf.Bytes(), nil
}

func (b *BoltTraceStore) Add(commitID *cid.CommitID, values map[string]float32, sourceFile string, ts time.Time) error {
	sklog.Infof("Ingesting source file: %q", sourceFile)
	index := commitID.Offset % constants.COMMITS_PER_TILE
	bdb, err := b.getBoltDB(commitID, false)
	if err != nil {
		return fmt.Errorf("Unable to open datastore: %s", err)
	}

	var lastSourceIndex uint64
	// Add the source and get its index.
	addSource := func(tx *bolt.Tx) error {
		t, err := tx.CreateBucketIfNotExists([]byte(SOURCE_LIST_BUCKET_NAME))
		if err != nil {
			return fmt.Errorf("Failed to get bucket: %s", err)
		}
		lastSourceIndex, err = t.NextSequence()
		if err != nil {
			return fmt.Errorf("Failed to get source index: %s", err)
		}
		sklog.Infof("lastSourceIndex: %d", lastSourceIndex)

		// Write the source.
		if err := t.Put(uint64ToBytes(lastSourceIndex), []byte(sourceFile)); err != nil {
			return fmt.Errorf("Failed to write the source file: %s", err)
		}
		return nil
	}

	if err := bdb.Update(addSource); err != nil {
		return fmt.Errorf("Error while writing source list: %s", err)
	}

	// Now that we have lastSourceIndex we can add the trace values.
	addValues := func(tx *bolt.Tx) error {
		t, err := tx.CreateBucketIfNotExists([]byte(TRACE_VALUES_BUCKET_NAME))
		if err != nil {
			return fmt.Errorf("Failed to get bucket: %s", err)
		}
		s, err := tx.CreateBucketIfNotExists([]byte(TRACE_SOURCES_BUCKET_NAME))
		if err != nil {
			return fmt.Errorf("Failed to get bucket: %s", err)
		}

		// Add values and source index.
		for traceID, value := range values {
			// Write the value.
			valueBytes, err := serialize(traceValue{
				Index: int64(index),
				Value: value,
			})
			if err != nil {
				return err
			}
			// Append the serialized traceValue to the current trace value.
			if err := t.Put([]byte(traceID), append(t.Get([]byte(traceID)), valueBytes...)); err != nil {
				return fmt.Errorf("bucket.Put() of value failed: %s", err)
			}

			// Write the source.
			sourceBytes, err := serialize(sourceValue{
				Index:  int64(index),
				Source: lastSourceIndex,
			})
			if err != nil {
				return err
			}
			// Append the serialized sourceValue to the current trace value.
			if err := s.Put([]byte(traceID), append(s.Get([]byte(traceID)), sourceBytes...)); err != nil {
				return fmt.Errorf("bucket.Put() of source failed: %s", err)
			}
		}
		return nil
	}

	if err := bdb.Update(addValues); err != nil {
		return fmt.Errorf("Error while writing values: %s", err)
	}

	return nil
}

func (b *BoltTraceStore) Details(commitID *cid.CommitID, traceID string) (string, float32, error) {
	bdb, err := b.getBoltDB(commitID, true)
	if err != nil {
		return "", 0, fmt.Errorf("Unable to open datastore: %s", err)
	}

	localIndex := int64(commitID.Offset % constants.COMMITS_PER_TILE)
	var sourceRet string
	var valueRet float32

	get := func(tx *bolt.Tx) error {
		sl := tx.Bucket([]byte(SOURCE_LIST_BUCKET_NAME))
		if sl == nil {
			return fmt.Errorf("Failed to get bucket: %s", SOURCE_LIST_BUCKET_NAME)
		}
		v := tx.Bucket([]byte(TRACE_VALUES_BUCKET_NAME))
		if v == nil {
			return fmt.Errorf("Failed to get bucket: %s", TRACE_VALUES_BUCKET_NAME)
		}
		s := tx.Bucket([]byte(TRACE_SOURCES_BUCKET_NAME))
		if s == nil {
			return fmt.Errorf("Failed to get bucket: %s", TRACE_SOURCES_BUCKET_NAME)
		}

		// Read the value.
		rawValues := v.Get([]byte(traceID))
		if rawValues == nil {
			rawValues = []byte{}
		}
		rawValues = dup(rawValues)
		buf := bytes.NewBuffer(rawValues)
		value := traceValue{
			Index: -1,
		}
		for {
			err := binary.Read(buf, binary.LittleEndian, &value)
			if err != nil {
				break
			}
			if value.Index == localIndex {
				valueRet = value.Value
				// Don't break, we want the last value for index.
			}
		}
		if value.Index == -1 {
			return fmt.Errorf("Value not found: %q in %q", traceID, commitID.Filename())
		}

		// Read the source.
		rawSource := s.Get([]byte(traceID))
		if rawSource == nil {
			return fmt.Errorf("Source not found.")
		}
		rawSource = dup(rawSource)
		buf = bytes.NewBuffer(rawSource)
		source := sourceValue{
			Index: -1,
		}
		var sourceIndex uint64
		for {
			err := binary.Read(buf, binary.LittleEndian, &source)
			if err != nil {
				sklog.Infof("Failed binary.Read: %s", err)
				break
			}
			if source.Index == localIndex {
				sourceIndex = source.Source
				// Don't break, we want the last value for index.
			}
		}
		if value.Index == -1 {
			return fmt.Errorf("Source not found: %q in %q", traceID, commitID.Filename())
		}

		// Read the sourceFullname.
		sourceRet = string(sl.Get(uint64ToBytes(sourceIndex)))

		return nil
	}

	if err := bdb.View(get); err != nil {
		return "", 0, fmt.Errorf("Error while reading value: %s", err)
	}

	return sourceRet, valueRet, nil
}

type tileMap struct {
	commitID *cid.CommitID
	idxmap   map[int]int
}

// buildMapper transforms the slice of commitIDs passed to Match into a mapping
// from the location of the commit in the DB to the index for that commit in
// the Trace's returned from Match. I.e. it maps tiles to a map that says where
// each value stored in the tile trace needs to be copied into the destination
// Trace.
//
// For example, if given:
//
//	commitIDs := []*cid.CommitID{
//		&cid.CommitID{
//			Source: "master",
//			Offset: 49,
//		},
//		&cid.CommitID{
//			Source: "master",
//			Offset: 50,
//		},
//		&cid.CommitID{
//			Source: "master",
//			Offset: 51,
//		},
//	}
//
// This will return the following, presuming a tile size of 50:
//
//	map[string]*tileMap{
//		"master-000000.bdb": &tileMap{
//			commitID: &cid.CommitID{
//				Source: "master",
//				Offset: 49,
//			},
//			idxmap: map[int]int{
//				49: 0,
//			},
//		},
//		"master-000001.bdb": &tileMap{
//			commitID: &cid.CommitID{
//				Source: "master",
//				Offset: 50,
//			},
//			idxmap: map[int]int{
//				0: 1,
//				1: 2,
//			},
//		},
//	}
//
// The returned map is used when loading traces out of tiles.
func buildMapper(commitIDs []*cid.CommitID) map[string]*tileMap {
	mapper := map[string]*tileMap{}
	for targetIndex, commitID := range commitIDs {
		if tm, ok := mapper[commitID.Filename()]; !ok {
			mapper[commitID.Filename()] = &tileMap{
				commitID: commitID,
				idxmap:   map[int]int{commitID.Offset % constants.COMMITS_PER_TILE: targetIndex},
			}
		} else {
			tm.idxmap[commitID.Offset%constants.COMMITS_PER_TILE] = targetIndex
		}
	}
	return mapper
}

// dup makes a copy of a byte slice.
//
// Needed since values returned from BoltDB are only valid
// for the life of the transaction.
func dup(b []byte) []byte {
	ret := make([]byte, len(b))
	copy(ret, b)
	return ret
}

// loadMatches loads values into 'traceSet' that match the 'matches' from the
// tile in the BoltDB 'db'.  Only values at the offsets in 'idxmap' are
// actually loaded, and 'idxmap' determines where they are stored in the Trace.
func loadMatches(bdb *bolt.DB, idxmap map[int]int, matches KeyMatches, traceSet types.TraceSet, traceLen int) error {
	defer timer.New("loadMatches time").Stop()

	get := func(tx *bolt.Tx) error {
		defer timer.New("loadMatches TX time").Stop()
		bucket := tx.Bucket([]byte(TRACE_VALUES_BUCKET_NAME))
		if bucket == nil {
			// If the bucket doesn't exist then we've never written to this tile, it's not an error,
			// it just means it has no data.
			return nil
		}
		v := bucket.Cursor()
		value := traceValue{}
		// Loop over the entire bucket.
		for btraceid, rawValues := v.First(); btraceid != nil; btraceid, rawValues = v.Next() {
			// Does the trace id match the query?
			if !matches(string(btraceid)) {
				continue
			}
			// Get the trace.
			trace := traceSet[string(btraceid)]
			if trace == nil {
				// Don't make the copy until we know we are going to need it.
				traceid := string(dup(btraceid))
				traceSet[traceid] = types.NewTrace(traceLen)
				trace = traceSet[traceid]
			}

			// Decode all the [index, float32] pairs stored for the trace.
			buf := bytes.NewBuffer(rawValues)
			for {
				if err := binary.Read(buf, binary.LittleEndian, &value); err != nil {
					break
				}
				// Store the value in trace if the index appears in idxmap.
				if offset, ok := idxmap[int(value.Index)]; ok {
					trace[offset] = value.Value
					// Don't break, we want the last value for index.
				}
			}
		}
		return nil
	}

	return bdb.View(get)
}

func (b *BoltTraceStore) Match(commitIDs []*cid.CommitID, matches KeyMatches, progress types.Progress) (types.TraceSet, error) {
	ret := types.TraceSet{}
	mapper := buildMapper(commitIDs)
	i := 0
	for _, tm := range mapper {
		i++
		if progress != nil {
			progress(i, len(mapper))
		}
		bdb, err := b.getBoltDB(tm.commitID, true)
		if err == tileNotExist {
			sklog.Infof("Skipped non-existent db: %s", tm.commitID.Filename())
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("Failed to open tile from %s: %s", tm.commitID.Filename(), err)
		}
		if err := loadMatches(bdb, tm.idxmap, matches, ret, len(commitIDs)); err != nil {
			return nil, fmt.Errorf("Failed to load traces from %s: %s", tm.commitID.Filename(), err)
		}
	}
	if progress != nil {
		progress(len(mapper), len(mapper))
	}
	return ret, nil
}

var Default *BoltTraceStore

func Init(dir string) {
	if Default != nil {
		sklog.Fatalf("ptracestore should only be initialized once.")
	}
	var err error
	Default, err = New(dir)
	if err != nil {
		sklog.Fatalf("ptracestore failed to init: %s", err)
	}
}

// Ensure that *BoltTraceStore implements PTraceStore.
var _ PTraceStore = &BoltTraceStore{}
