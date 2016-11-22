package boltutil

import (
	"bytes"
	"fmt"
	"path"

	"github.com/boltdb/bolt"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/util"
)

// Separator used to separate child and parent id in indices.
const IDX_SEPARATOR = "\x00"

type Interface interface {
	Key() string
	IndexValues([]string) map[string][]string
}

type WriteFn func(*bolt.Tx, []Interface) ([]Interface, error)

type Config struct {
	Directory string
	Name      string
	Indices   []string
	Codec     util.LRUCodec
}

type Store struct {
	*bolt.DB
	indices    []string
	mainBucket []byte
	codec      util.LRUCodec
}

func NewStore(config *Config) (*Store, error) {
	baseDir, err := fileutil.EnsureDirExists(config.Directory)
	if err != nil {
		return nil, err
	}

	dbName := path.Join(baseDir, config.Name+".boltdb")
	db, err := bolt.Open(dbName, 0600, nil)
	if err != nil {
		return nil, err
	}

	indices := make([][]byte, len(config.Indices))
	for i, idx := range config.Indices {
		indices[i] = []byte(idx)
	}

	ret := &Store{
		DB:         db,
		indices:    config.Indices,
		mainBucket: []byte(config.Name),
		codec:      config.Codec,
	}

	return ret, ret.initBuckets()
}

func (s *Store) Read(keys []string, tx ...*bolt.Tx) ([]Interface, error) {
	return s.readRecs(keys, tx...)
}

func (s *Store) ReadIndex(idx string, keys []string) (map[string][]string, error) {
	if !util.In(idx, s.indices) {
		return nil, fmt.Errorf("Invalid index: '%s'", idx)
	}

	ret := make(map[string][]string, len(keys))
	viewFn := func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(idx))
		for _, key := range keys {
			ret[key] = s.readIndexValues(bucket, key)
		}
		return nil
	}

	return ret, s.DB.View(viewFn)
}

func (s *Store) Write(inputRecs []Interface, writeFn WriteFn, tx ...*bolt.Tx) error {
	// Extract the keys to read back the values.
	keys := make([]string, len(inputRecs))
	for i, rec := range inputRecs {
		keys[i] = rec.Key()
		if keys[i] == "" {
			return fmt.Errorf("Input key cannot be empty.")
		}
	}

	updateFn := func(tx *bolt.Tx) error {
		foundRecs, err := s.readRecs(keys, tx)
		if err != nil {
			return nil
		}

		// Capture the index values before they are changed.
		origIndexState := s.getIndexState(foundRecs)

		writeRecs := foundRecs
		if writeFn != nil {
			writeRecs, err = writeFn(tx, foundRecs)
			if err != nil {
				return err
			}
		}

		if len(writeRecs) != len(keys) {
			return fmt.Errorf("Modified array size does not match original array size.")
		}

		// Get the main bucket and write the entries plus indices to it.
		bucket := tx.Bucket(s.mainBucket)
		for _, rec := range writeRecs {
			if rec != nil {
				// Write the issue back to the database.
				contentBytes, err := s.codec.Encode(rec)
				if err != nil {
					return err
				}
				if err := bucket.Put([]byte(rec.Key()), contentBytes); err != nil {
					return err
				}
			}
		}

		if err := s.updateIndices(tx, writeRecs, origIndexState); err != nil {
			return err
		}

		return nil
	}

	// If we were given a transaction the use it otherwise create a new one.
	if len(tx) > 0 {
		return updateFn(tx[0])
	}

	return s.DB.Update(updateFn)
}

// Delete deletes the records identified by keys from the database and updates
// the indices accordingly.
func (s *Store) Delete(keys []string, tx ...*bolt.Tx) error {
	updateFn := func(tx *bolt.Tx) error {
		entries, err := s.readRecs(keys, tx)
		if err != nil {
			return nil
		}

		// Get the main bucket and write the entries plus indices to it.
		bucket := tx.Bucket(s.mainBucket)
		for _, key := range keys {
			if err := bucket.Delete([]byte(key)); err != nil {
				return err
			}
		}

		if err := s.deleteIndices(tx, entries); err != nil {
			return err
		}

		return nil
	}

	if len(tx) > 0 {
		return updateFn(tx[0])
	}

	return s.DB.Update(updateFn)
}

// List returns a list of records sorted in ascending order by the primary
// key starting with record at offset. If size <= 0 all records will be
// returned. Note: This can be inefficient for large values of size.
func (s *Store) List(offset, size int) ([]Interface, int, error) {
	total := 0
	var ret []Interface = nil
	viewFn := func(tx *bolt.Tx) error {
		bucket := tx.Bucket(s.mainBucket)
		nEntries := bucket.Stats().KeyN

		// If size <= 0 then we want all entries.
		if size <= 0 {
			size = nEntries
		}
		resultSize := util.MaxInt(0, util.MinInt(nEntries-offset, size))
		if resultSize == 0 {
			ret = []Interface{}
			return nil
		}

		cursor := bucket.Cursor()
		_, val := cursor.First()

		// TODO(stephana): This is definitely inefficient, but there is no
		// option in boltdb to address a specific element by index. If this is
		// too slow we can add an additional pseudo index for issue ids.

		// Skip the first entries if offset > 0.
		for i := 0; i < offset; i++ {
			_, val = cursor.Next()
		}

		allEntries := make([]Interface, resultSize)
		var err error
		var iRec interface{}
		for i := 0; i < resultSize; i++ {
			if iRec, err = s.codec.Decode(val); err != nil {
				return err
			}
			allEntries[i] = iRec.(Interface)
			_, val = cursor.Next()
		}

		// Success assign the results now.
		ret = allEntries
		total = nEntries
		return nil
	}

	return ret, total, s.DB.View(viewFn)
}

// readRecs reads the records identified by keys. It returns an array
// or records with the same length as the keys array. If a record cannot be
// found the corresponding value is nil.
func (s *Store) readRecs(keys []string, tx ...*bolt.Tx) ([]Interface, error) {
	var ret []Interface = nil
	viewFn := func(tx *bolt.Tx) error {
		bucket := tx.Bucket(s.mainBucket)
		ret = make([]Interface, len(keys))
		for i, key := range keys {
			contentBytes := bucket.Get([]byte(key))
			if contentBytes != nil {
				rec, err := s.codec.Decode(contentBytes)
				if err != nil {
					return err
				}
				ret[i] = rec.(Interface)
			}
		}
		return nil
	}

	// if a transaction was suzzzpplied then use it.
	if len(tx) > 0 {
		return ret, viewFn(tx[0])
	}

	return ret, s.DB.View(viewFn)
}

// readIndexValues returns the primary keys for the given (bucket,key) pair.
// The bucket is assumed to contain an index.
func (s *Store) readIndexValues(bucket *bolt.Bucket, key string) []string {
	prefix := []byte(key + IDX_SEPARATOR)
	lenPrefix := len(prefix)
	ret := []string{}
	cursor := bucket.Cursor()
	for k, _ := cursor.Seek(prefix); bytes.HasPrefix(k, prefix); k, _ = cursor.Next() {
		ret = append(ret, string(append([]byte(nil), k[lenPrefix:]...)))
	}
	return ret
}

// getIndexState captures the index values for the records in the given list.
// It returns an array where ther each element maps to the corresponding element
// in the input array.
func (s *Store) getIndexState(recs []Interface) []map[string][]string {
	ret := make([]map[string][]string, len(recs))
	for i, rec := range recs {
		if rec != nil {
			ret[i] = rec.IndexValues(s.indices)
		}
	}
	return ret
}

// updateIndices updates the indices the given records
func (s *Store) updateIndices(tx *bolt.Tx, recs []Interface, baseState []map[string][]string) error {
	addChanges := make(map[string][]string, len(s.indices))
	delChanges := make(map[string][]string, len(s.indices))
	for i, rec := range recs {
		if rec != nil {
			parentId := rec.Key()
			for idxName, indexVals := range rec.IndexValues(s.indices) {
				newVals := util.NewStringSet(indexVals)
				oldVals := util.NewStringSet(baseState[i][idxName])
				for indexVal := range newVals.Complement(oldVals) {
					addChanges[idxName] = append(addChanges[idxName], indexVal+IDX_SEPARATOR+parentId)
				}
				for indexVal := range oldVals.Complement(newVals) {
					delChanges[idxName] = append(delChanges[idxName], indexVal+IDX_SEPARATOR+parentId)
				}
			}
		}
	}

	// Delete all entries that need to be deleted.
	if err := writeIndexChanges(tx, delChanges, deleteIndexOp); err != nil {
		return err
	}

	// Add all the entries that need to be added.
	return writeIndexChanges(tx, addChanges, addIndexOp)
}

// deleteIndices deletes the indices associated with the the given records.
func (s *Store) deleteIndices(tx *bolt.Tx, entries []Interface) error {
	changes := make(map[string][]string, len(s.indices))
	for _, entry := range entries {
		if entry != nil {
			parentId := entry.Key()
			for idxName, indexVals := range entry.IndexValues(s.indices) {
				for _, indexVal := range indexVals {
					changes[idxName] = append(changes[idxName], indexVal+IDX_SEPARATOR+parentId)
				}
			}
		}
	}
	return writeIndexChanges(tx, changes, deleteIndexOp)
}

// initBuckets makes sure all needed buckets exist.
func (s *Store) initBuckets() error {
	return s.DB.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(s.mainBucket); err != nil {
			return err
		}

		for _, idx := range s.indices {
			if idx == "" {
				return fmt.Errorf("Index name cannot be empty.")
			}
			if _, err := tx.CreateBucketIfNotExists([]byte(idx)); err != nil {
				return err
			}
		}
		return nil
	})
}

// indexOp is a helper type that allows to define different operations in
// for writeIndexChanges.
type indexOp func(bucket *bolt.Bucket, kv []byte) error

// deleteIndexOp is the operation used to delete from the indices.
func deleteIndexOp(bucket *bolt.Bucket, kv []byte) error {
	return bucket.Delete(kv)
}

// addIndexOp is the operation used to add to an index.
func addIndexOp(bucket *bolt.Bucket, kv []byte) error {
	return bucket.Put(kv, []byte{})
}

// writeIndexChanges writes changes to indices.
//    changes map[indexName][]indexValues
func writeIndexChanges(tx *bolt.Tx, changes map[string][]string, op indexOp) error {
	for idxName, indexEntries := range changes {
		bucket := tx.Bucket([]byte(idxName))
		for _, entry := range indexEntries {
			if err := op(bucket, []byte(entry)); err != nil {
				return err
			}
		}
	}
	return nil
}
