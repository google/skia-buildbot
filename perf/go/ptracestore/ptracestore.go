// Package pstracestore is a database for Perf data.
package ptracestore

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/util"

	"github.com/boltdb/bolt"
	"github.com/golang/groupcache/lru"
	"github.com/skia-dev/glog"
)

const (
	COMMITS_PER_TILE = 50

	MAX_CACHED_TILES = 5

	TRACE_VALUES_BUCKET_NAME  = "traces"
	TRACE_SOURCES_BUCKET_NAME = "sources"
	SOURCE_LIST_BUCKET_NAME   = "sourceList"

	LAST_SOURCE_ID_KEY = "lastSourceIndex"
)

var (
	// safeRe is used in CommitID.Filename() to replace unsafe chars in a filename.
	safeRe = regexp.MustCompile("[^a-zA-Z0-9]")
)

// CommitID represents the time of a particular commit, where a commit could either be
// a real commit into the repo, or an event like running a trybot.
type CommitID struct {
	Offset int    // The index number of the commit from beginning of time, or patch number in Reitveld.
	Source string // The branch name, e.g. "master", or the Reitveld issue id.
}

// Filename returns a safe filename to be used as part of the underlying BoltDB tile name.
func (c CommitID) Filename() string {
	return fmt.Sprintf("%s-%06d.bdb", safeRe.ReplaceAllLiteralString(c.Source, "_"), c.Offset/COMMITS_PER_TILE)
}

// Traces is a placeholder for later CLs. It represents a set of Traces.
type Traces struct {
}

// PTraceStore is an interface for storing Perf data.
//
// PTraceStore doesn't know anything about git hashes or Rietveld issue IDs,
// that will be handled at a level above this.
//
// TODO(jcgregorio) How to list all the Sources?
type PTraceStore interface {
	// Add new values to the datastore at the given commitID.
	//
	// values - A map from the trace id to a float32 value.
	// sourceFile - The full path of the file where this information came from,
	//   usually the Google Storage URL.
	Add(commitID *CommitID, values map[string]float32, sourceFile string) error

	// Retrieve the source and value for a given measurement in a given trace,
	// and a non-nil error if no such point was found.
	Details(commitID *CommitID, traceID string) (string, float32, error)

	// Match returns Traces that match the given Query and slice of CommitIDs.
	//
	// The returned Traces will contain a slice of Trace, and that list will be
	// empty if there are no matches.
	Match(commitIDs []*CommitID, q query.Query) (*Traces, error)
}

// BoltTraceStore is an implementation of PTraceStore that uses BoltDB.
type BoltTraceStore struct {
	// mutex protects access to cache.
	mutex sync.Mutex

	// cache is a cache of opened tiles.
	cache *lru.Cache

	// dir is the directory where tiles are stored.
	dir string
}

// closer is a callback we pass to the lru cache to close bolt.DBs once they've
// been evicted from the cache.
func closer(key lru.Key, value interface{}) {
	if db, ok := value.(*bolt.DB); ok {
		util.Close(db)
	} else {
		glog.Errorf("Found a non-bolt.DB in the cache at key %q", key)
	}
}

// New creates a new BoltTraceStore that stores tiles in the given directory.
func New(dir string) *BoltTraceStore {
	cache := lru.New(MAX_CACHED_TILES)
	cache.OnEvicted = closer

	return &BoltTraceStore{
		dir:   dir,
		cache: cache,
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
func (b *BoltTraceStore) getBoltDB(commitID *CommitID) (*bolt.DB, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	name := commitID.Filename()
	if idb, ok := b.cache.Get(name); ok {
		if db, ok := idb.(*bolt.DB); ok {
			return db, nil
		}
	}
	filename := filepath.Join(b.dir, commitID.Filename())
	db, err := bolt.Open(filename, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("Unable to open %q: %s", filename, err)
	}
	b.cache.Add(name, db)
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

func (b *BoltTraceStore) Add(commitID *CommitID, values map[string]float32, sourceFile string) error {
	index := commitID.Offset % COMMITS_PER_TILE
	db, err := b.getBoltDB(commitID)
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

		// Write the source.
		if err := t.Put(uint64ToBytes(lastSourceIndex), []byte(sourceFile)); err != nil {
			return fmt.Errorf("Failed to write the source file: %s", err)
		}
		return nil
	}

	if err := db.Update(addSource); err != nil {
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

	if err := db.Update(addValues); err != nil {
		return fmt.Errorf("Error while writing values: %s", err)
	}

	return nil
}

func (b *BoltTraceStore) Details(commitID *CommitID, traceID string) (string, float32, error) {
	db, err := b.getBoltDB(commitID)
	if err != nil {
		return "", 0, fmt.Errorf("Unable to open datastore: %s", err)
	}

	localIndex := int64(commitID.Offset % COMMITS_PER_TILE)
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
		buf := bytes.NewBuffer(v.Get([]byte(traceID)))
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
		buf = bytes.NewBuffer(s.Get([]byte(traceID)))
		source := sourceValue{
			Index: -1,
		}
		var sourceIndex uint64
		for {
			err := binary.Read(buf, binary.LittleEndian, &source)
			if err != nil {
				break
			}
			if value.Index == localIndex {
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

	if err := db.View(get); err != nil {
		return "", 0, fmt.Errorf("Error while reading value: %s", err)
	}

	return sourceRet, valueRet, nil
}

func (b *BoltTraceStore) Match(commitIDs []*CommitID, q query.Query) (*Traces, error) {
	// TODO(jcgregorio) Implement.
	return nil, fmt.Errorf("Not implemented.")
}

// Ensure that *BoltTraceStore implements PTraceStore.
var _ PTraceStore = &BoltTraceStore{}
