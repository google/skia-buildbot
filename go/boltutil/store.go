// Package boltutil provides higher level primitives to work with bolt DB.
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

// Record is the interface that has to be implemented by a client to store
// instance of a type in a Store instance. It allows to extract relevant
// key information about each record.
type Record interface {
	// Key returns the primary key of the record. The return value does not have
	// to be a human readable string it can be any sequence of bytes. The
	// returned string has to uniquely identify this instance.
	Key() string

	// IndexValues returns the values of the type for the attributes we want to
	// index. The return value maps from the index names to the list of values for
	// that index (= secondary key values): map[index_name][]{val_1, val_2, val_n}.
	// Any key in the returned map must also be given in Indices field of the
	// Config instance passed to NewStore(...).
	IndexValues() map[string][]string
}

// Store embeds a bolt database and adds higher level functions to add and maintain
// indices. Indices allow to read records by keys other than the primary key.
// It aims at following the boltdb patterns and allows to still use the
// underlying bolt DB directly if necessary.
type Store struct {
	// BoltDB instance used by this store.
	DB *bolt.DB

	// indices is the list of indices this store maintains. The key and values
	// are different representations of the same value for convenience.
	indices map[string][]byte

	// mainBucket is the name of the bucket where the records are stored.
	mainBucket []byte

	// codec provides functions to serialize and deserialize records.
	codec util.LRUCodec
}

// Config contains the configuration values to set up a Store instance.
type Config struct {
	// Directory is the path to the directory where the database should be stored.
	Directory string

	// Name is the primary name of the database.
	Name string

	// Indices is the list of indices that should be created for every record stored.
	Indices []string

	// Codec is used to serialize and deserialize records. It has to consume
	// and produce instances that implement the Record interface.
	Codec util.LRUCodec
}

// NewStore returns a new instance of Store. Since it embeds bolt.DB it is
// up to the caller to call Close() when the Store instance is no longer needed.
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

	indices := make(map[string][]byte, len(config.Indices))
	for _, idx := range config.Indices {
		if idx == "" {
			return nil, fmt.Errorf("Index name cannot be empty string.")
		}
		indices[idx] = []byte(idx)
	}

	ret := &Store{
		DB:         db,
		indices:    indices,
		mainBucket: []byte(config.Name),
		codec:      config.Codec,
	}

	if err := ret.initBuckets(); err != nil {
		util.Close(ret.DB)
		return nil, err
	}

	return ret, nil
}

// Read reads the records identified by keys. The returned slice has the
// exact same number of elements as keys. If a record could not be found the
// corresponding entry in the return value will be nil.
func (s *Store) Read(keys []string) ([]Record, error) {
	return s.readRecs(keys, nil)
}

// Same as Read but use an existing transaction.
func (s *Store) ReadTx(keys []string, tx *bolt.Tx) ([]Record, error) {
	return s.readRecs(keys, tx)
}

// ReadIndex returns the record keys for the given index name and list of index values.
// The return value maps from input key to a list of primary keys.
func (s *Store) ReadIndex(idx string, keys []string) (map[string][]string, error) {
	if _, ok := s.indices[idx]; !ok {
		return nil, fmt.Errorf("Invalid index: %q", idx)
	}

	ret := make(map[string][]string, len(keys))
	viewFn := func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(idx))
		for _, key := range keys {
			ret[key] = s.indexLookup(bucket, key)
		}
		return nil
	}

	err := s.DB.View(viewFn)
	return ret, err
}

// WriteFn is the type of the callback function that can be handed to the
// Write method to update existing records before they are overwritten with a
// new instance. The passed records are the current records in the database
// (before new values are written). The WriteFn can then modify records in place
// and they will be written to disk. To prevent a record from being written back
// into the database it should set the entry to nil.
type WriteFn func(*bolt.Tx, []Record) error

// Update allows to update records in the database by following these steps:
//   - read existing records from the database, identified by the Key() value
//     of inputRecs.
//   - call the writeFn to allow the client to update the records. If it does
//     not want a record to be updated it will set its entry to nil.
func (s *Store) Update(inputRecs []Record, writeFn WriteFn) error {
	return s.writeRecs(inputRecs, writeFn, nil)
}

// WriteTx is the same as Write but re-uses an existing transaction.
func (s *Store) WriteTx(inputRecs []Record, writeFn WriteFn, tx *bolt.Tx) error {
	return s.writeRecs(inputRecs, writeFn, tx)
}

func (s *Store) Insert(recs []Record) error {
	return s.writeRecs(recs, nil, nil)
}

// writeRecs writes given records with the given transaction or by creating a new  transaction.
// See Write(...) for details how it works.
func (s *Store) writeRecs(inputRecs []Record, writeFn WriteFn, tx *bolt.Tx) error {
	if len(inputRecs) == 0 {
		return nil
	}

	// Extract the keys and make sure they are unique and not empty.
	uniqueKeys := make(util.StringSet, len(inputRecs))
	for _, rec := range inputRecs {
		uniqueKeys[rec.Key()] = true
	}

	if (len(uniqueKeys) != len(inputRecs)) || uniqueKeys[""] {
		return fmt.Errorf("Keys of input records have to be unique and cannot be empty.")
	}
	keys := uniqueKeys.Keys()

	updateFn := func(tx *bolt.Tx) error {
		foundRecs, err := s.readRecs(keys, tx)
		if err != nil {
			return err
		}

		// Capture the index values before they are changed.
		origIndexState := s.getIndexState(foundRecs)

		writeRecs := inputRecs
		if writeFn != nil {
			if err := writeFn(tx, foundRecs); err != nil {
				return err
			}
			writeRecs = foundRecs

			// Make sure the keys still match.
			for i, writeRec := range writeRecs {
				if inputRecs[i].Key() != writeRec.Key() {
					return fmt.Errorf("Order of keys in the input to Update needs to match the order of keys after call to write function.")
				}
			}

		}

		// TODO(stephana): Make this parallel since encoding can be CPU intensive.

		// Get the main bucket and write the entries.
		bucket := tx.Bucket(s.mainBucket)
		for _, rec := range writeRecs {
			if rec != nil {
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

	// If we were given a transaction then use it otherwise create a new one.
	if tx != nil {
		return updateFn(tx)
	}

	return s.DB.Update(updateFn)
}

// Delete deletes the records identified by keys from the database and updates
// the indices accordingly.
func (s *Store) Delete(keys []string) error {
	return s.deleteRecs(keys, nil)
}

// DeleteTx does the same as Delete but uses an existing transaction.
func (s *Store) DeleteTx(keys []string, tx *bolt.Tx) error {
	return s.deleteRecs(keys, tx)
}

// deleteRecs deletes the records identified by keys. If uses tx if it's not
// nil or creates a new transaction otherwise.
func (s *Store) deleteRecs(keys []string, tx *bolt.Tx) error {
	updateFn := func(tx *bolt.Tx) error {
		entries, err := s.readRecs(keys, tx)
		if err != nil {
			return err
		}

		// TODO(stephana): Combine the above readRecs step with the delete
		// step to avoid having to traverse the keyspace twice. Use a cursor.

		// Get the main bucket and delete the records from it.
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

	if tx != nil {
		return updateFn(tx)
	}

	return s.DB.Update(updateFn)
}

// TODO(stephana): Revisit the signature of List and how it is implemented
// based on the use cases.

// List returns a list of records sorted in ascending order by the primary
// key starting with record at offset. If size <= 0 all records will be
// returned.
// Note: This can be take a long time for large values of offset and/or size.
// Results might not be consistent across multiple calls since they might
// be interleaved with write operations.
func (s *Store) List(offset, size int) ([]Record, int, error) {
	total := 0
	var ret []Record = nil
	viewFn := func(tx *bolt.Tx) error {
		bucket := tx.Bucket(s.mainBucket)
		nEntries := bucket.Stats().KeyN

		// If size <= 0 then we want all entries.
		if size <= 0 {
			size = nEntries
		}
		resultSize := util.MaxInt(0, util.MinInt(nEntries-offset, size))
		if resultSize == 0 {
			ret = []Record{}
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

		allEntries := make([]Record, resultSize)
		var err error
		var iRec interface{}
		// Note: We are guaranteed to have resultSize records in the database.
		for i := 0; i < resultSize; i++ {
			if iRec, err = s.codec.Decode(val); err != nil {
				return err
			}
			allEntries[i] = iRec.(Record)
			_, val = cursor.Next()
		}

		// Success assign the results now.
		ret = allEntries
		total = nEntries
		return nil
	}

	err := s.DB.View(viewFn)
	return ret, total, err
}

// readRecs reads the records identified by keys. It returns an array
// of records with the same length as the keys array. If a record cannot be
// found the corresponding value is nil.
func (s *Store) readRecs(keys []string, tx *bolt.Tx) ([]Record, error) {
	var ret []Record = nil
	viewFn := func(tx *bolt.Tx) error {
		bucket := tx.Bucket(s.mainBucket)
		ret = make([]Record, len(keys))

		// TODO(stephana): Make this parallel since decoding can be CPU intensive.
		for i, key := range keys {
			contentBytes := bucket.Get([]byte(key))
			if contentBytes != nil {
				rec, err := s.codec.Decode(contentBytes)
				if err != nil {
					return err
				}
				ret[i] = rec.(Record)
			}
		}
		return nil
	}

	// if a transaction was supplied then use it.
	var err error
	if tx != nil {
		err = viewFn(tx)
	} else {
		err = s.DB.View(viewFn)
	}

	return ret, err
}

// indexLookup returns the primary keys for the given (bucket,key) pair.
// The bucket is assumed to contain an index.
func (s *Store) indexLookup(bucket *bolt.Bucket, key string) []string {
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
// It returns an array where each element maps to the corresponding element
// in the input array.
func (s *Store) getIndexState(recs []Record) []map[string][]string {
	ret := make([]map[string][]string, len(recs))
	for i, rec := range recs {
		if rec != nil {
			ret[i] = rec.IndexValues()
		}
	}
	return ret
}

func (s *Store) getIndexValues(rec Record) (map[string][]string, error) {
	ret := rec.IndexValues()
	for idxName := range ret {
		if _, ok := s.indices[idxName]; !ok {
			return nil, fmt.Errorf("Unknown index name: %s. All indices returned by IndexValues must be listed in initial Config.", idxName)
		}
	}
	return ret, nil
}

// updateIndices updates the indices the given records
func (s *Store) updateIndices(tx *bolt.Tx, recs []Record, baseState []map[string][]string) error {
	addChanges := make(map[string][]string, len(s.indices))
	delChanges := make(map[string][]string, len(s.indices))
	var indexVals map[string][]string
	var err error
	for i, rec := range recs {
		// rec != nil indicates that this record will change in the database and
		// we need to update the index.
		if rec != nil {
			parentId := rec.Key()
			if indexVals, err = s.getIndexValues(rec); err != nil {
				return err
			}

			for idxName, indexVals := range indexVals {
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
func (s *Store) deleteIndices(tx *bolt.Tx, entries []Record) error {
	changes := make(map[string][]string, len(s.indices))
	for _, entry := range entries {
		if entry != nil {
			parentId := entry.Key()
			for idxName, indexVals := range entry.IndexValues() {
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

		for _, idxName := range s.indices {
			if _, err := tx.CreateBucketIfNotExists(idxName); err != nil {
				return err
			}
		}
		return nil
	})
}

// indexOp is a helper type that allows to define different operations for writeIndexChanges.
type indexOp func(*bolt.Bucket, []byte) error

// deleteIndexOp is the operation used to delete from the indices.
func deleteIndexOp(bucket *bolt.Bucket, k []byte) error {
	return bucket.Delete(k)
}

// addIndexOp is the operation used to add to an index.
func addIndexOp(bucket *bolt.Bucket, k []byte) error {
	return bucket.Put(k, []byte{})
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
