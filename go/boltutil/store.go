// Package boltutil provides higher level primitives to work with bolt DB.
package boltutil

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/boltdb/bolt"
	"go.skia.org/infra/go/util"
)

const (
	// Maximum length of the value for an index. This results from us storing the
	// length in a two byte uint.
	MAX_INDEX_VAL_LEN = 65535

	// META_DATA_BUCKET_PREFIX is the prefix of the bucket name used for meta data.
	META_DATA_BUCKET_PREFIX = "_meta_"

	// META_DATA_KEY is the key under which meta data are stored.
	META_DATA_KEY = "meta"

	// BUILD_INDEX_BATCH_SIZE is the batchsize used when building new indices.
	BUILD_INDEX_BATCH_SIZE = 100
)

// Record is the interface that has to be implemented by a client to store
// instance of a type in a IndexedBucket instance. It allows to extract relevant
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
	// Note: Individual values for each index cannot be longer than 65535.
	IndexValues() map[string][]string
}

// IndexedBucket embeds a bolt database and adds higher level functions to add and maintain
// indices. Indices allow to read records by keys other than the primary key.
// It aims at following the boltdb patterns and allows to still use the
// underlying bolt DB directly if necessary.
type IndexedBucket struct {
	// BoltDB instance used by this store.
	DB *bolt.DB

	// indices is the set of indices this IndexedBucket maintains.
	indices util.StringSet

	// mainBucket is the name of the bucket where the records are stored.
	mainBucket []byte

	// codec provides functions to serialize and deserialize records.
	codec util.Codec
}

// Config contains the configuration values to set up a IndexedBucket instance.
type Config struct {
	// DB is the boltdb instance to store the indexed bucket.
	DB *bolt.DB

	// Name is the primary name of the database.
	Name string

	// Indices is the list of indices that should be created for every record stored.
	Indices []string

	// Codec is used to serialize and deserialize records. It has to consume
	// and produce instances that implement the Record interface.
	Codec util.Codec
}

// NewIndexedBucket returns a new instance of IndexedBucket. Since it uses an existing
// BoltDB instance, it is up to the caller to close it.
func NewIndexedBucket(config *Config) (*IndexedBucket, error) {
	// Make sure there is not collision with any internal buckets, i.e. for meta data.
	if config.Name == "" || strings.HasPrefix(config.Name, "_") {
		return nil, fmt.Errorf("Bucket name cannot be empty or start with %q", "_")
	}

	ret := &IndexedBucket{
		DB:         config.DB,
		indices:    util.NewStringSet(config.Indices),
		mainBucket: []byte(config.Name),
		codec:      config.Codec,
	}

	if err := ret.initBuckets(); err != nil {
		return nil, err
	}

	return ret, nil
}

// Read reads the records identified by keys. The returned slice has the
// exact same number of elements as keys. If a record could not be found the
// corresponding entry in the return value will be nil.
func (ix *IndexedBucket) Read(keys []string) ([]Record, error) {
	return ix.readRecs(keys, nil)
}

// Same as Read but use an existing transaction.
func (ix *IndexedBucket) ReadTx(tx *bolt.Tx, keys []string) ([]Record, error) {
	return ix.readRecs(keys, tx)
}

// ReadIndex returns the record keys for the given index name and list of index values.
// The return value maps from index values (secondary key values) to a list of primary keys.
func (ix *IndexedBucket) ReadIndex(idx string, keys []string) (map[string][]string, error) {
	return ix.ReadIndexTx(nil, idx, keys)
}

// ReadIndexTx does the same as ReadIndex but uses an existing transaction.
func (ix *IndexedBucket) ReadIndexTx(tx *bolt.Tx, idx string, keys []string) (map[string][]string, error) {
	if _, ok := ix.indices[idx]; !ok {
		return nil, fmt.Errorf("Invalid index: %q", idx)
	}

	ret := make(map[string][]string, len(keys))
	viewFn := func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(idx))
		for _, key := range keys {
			ret[key] = ix.indexLookup(bucket, key)
		}
		return nil
	}

	var err error
	if tx != nil {
		err = viewFn(tx)
	} else {
		err = ix.DB.View(viewFn)
	}
	return ret, err
}

// ReadRaw reads the bytes for a given key. If the key does not exist
// the returned byte slice is nil.
func (ix *IndexedBucket) ReadRaw(key string) ([]byte, error) {
	// Key cannot be empty.
	if key == "" {
		return nil, nil
	}

	var ret []byte = nil
	viewFn := func(tx *bolt.Tx) error {
		bucket := tx.Bucket(ix.mainBucket)
		contentBytes := bucket.Get([]byte(key))
		if contentBytes != nil {
			// If we have data make a copy.
			ret = append(ret, contentBytes...)
		}
		return nil
	}

	err := ix.DB.View(viewFn)
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
func (ix *IndexedBucket) Update(inputRecs []Record, writeFn WriteFn) error {
	return ix.writeRecs(inputRecs, writeFn, nil)
}

// UpdateTx is the same as Update but re-uses an existing transaction.
func (ix *IndexedBucket) UpdateTx(tx *bolt.Tx, inputRecs []Record, writeFn WriteFn) error {
	return ix.writeRecs(inputRecs, writeFn, tx)
}

func (ix *IndexedBucket) Insert(recs []Record) error {
	return ix.writeRecs(recs, nil, nil)
}

// writeRecs writes given records with the given transaction or by creating a new  transaction.
// See Write(...) for details how it works.
func (ix *IndexedBucket) writeRecs(inputRecs []Record, writeFn WriteFn, tx *bolt.Tx) error {
	if len(inputRecs) == 0 {
		return nil
	}

	// Extract the keys and make sure they are unique and not empty.
	uniqueKeys := make(util.StringSet, len(inputRecs))
	keys := make([]string, len(inputRecs), len(inputRecs))
	for i, rec := range inputRecs {
		uniqueKeys[rec.Key()] = true
		keys[i] = rec.Key()
	}
	if (len(uniqueKeys) != len(inputRecs)) || uniqueKeys[""] {
		return fmt.Errorf("Keys of input records have to be unique and cannot be empty.")
	}

	updateFn := func(tx *bolt.Tx) error {
		foundRecs, err := ix.readRecs(keys, tx)
		if err != nil {
			return err
		}

		// Capture the index values before they are changed.
		origIndexState, err := ix.getIndexState(foundRecs)
		if err != nil {
			return err
		}

		writeRecs := inputRecs
		if writeFn != nil {
			if err := writeFn(tx, foundRecs); err != nil {
				return err
			}
			writeRecs = foundRecs

			// Make sure the keys are still in the same order as before.
			for i, writeRec := range writeRecs {
				if (writeRec != nil) && (inputRecs[i].Key() != writeRec.Key()) {
					return fmt.Errorf("Order of keys in the input to Update needs to match the order of keys after call to write function.")
				}
			}
		}

		// TODO(stephana): Make this parallel since encoding can be CPU intensive.

		// Get the main bucket and write the entries.
		bucket := tx.Bucket(ix.mainBucket)
		for _, rec := range writeRecs {
			if rec != nil {
				contentBytes, err := ix.codec.Encode(rec)
				if err != nil {
					return err
				}
				if err := bucket.Put([]byte(rec.Key()), contentBytes); err != nil {
					return err
				}
			}
		}

		if err := ix.updateIndices(tx, writeRecs, origIndexState, ix.indices); err != nil {
			return err
		}

		return nil
	}

	// If we were given a transaction then use it otherwise create a new one.
	if tx != nil {
		return updateFn(tx)
	}

	return ix.DB.Update(updateFn)
}

// Delete deletes the records identified by keys from the database and updates
// the indices accordingly.
func (ix *IndexedBucket) Delete(keys []string) error {
	return ix.deleteRecs(keys, nil)
}

// DeleteTx does the same as Delete but uses an existing transaction.
func (ix *IndexedBucket) DeleteTx(tx *bolt.Tx, keys []string) error {
	return ix.deleteRecs(keys, tx)
}

// ReIndex rebuilds the indices that were specified when the instance was created.
// This will block until all indices are rebuild.
func (ix *IndexedBucket) ReIndex() error {
	updateFn := func(tx *bolt.Tx) error {
		// Build all indices. This will remove the indices before bulding if they exist.
		return ix.buildIndices(tx, ix.indices)
	}

	return ix.DB.Update(updateFn)
}

// deleteRecs deletes the records identified by keys. It uses tx if it's not
// nil or creates a new transaction otherwise.
func (ix *IndexedBucket) deleteRecs(keys []string, tx *bolt.Tx) error {
	updateFn := func(tx *bolt.Tx) error {
		entries, err := ix.readRecs(keys, tx)
		if err != nil {
			return err
		}

		// TODO(stephana): Combine the above readRecs step with the delete
		// step to avoid having to traverse the keyspace twice. Use a cursor.

		// Get the main bucket and delete the records from it.
		bucket := tx.Bucket(ix.mainBucket)
		for _, key := range keys {
			if err := bucket.Delete([]byte(key)); err != nil {
				return err
			}
		}

		if err := ix.deleteIndices(tx, entries); err != nil {
			return err
		}

		return nil
	}

	if tx != nil {
		return updateFn(tx)
	}

	return ix.DB.Update(updateFn)
}

// TODO(stephana): Revisit the signature of List and how it is implemented
// based on the use cases.

// List returns a list of records sorted in ascending order by the primary
// key starting with record at offset. If size <= 0 all records will be
// returned.
// Note: This can be take a long time for large values of offset and/or size.
// Results might not be consistent across multiple calls since they might
// be interleaved with write operations.
func (ix *IndexedBucket) List(offset, size int) ([]Record, int, error) {
	total := 0
	var ret []Record = nil
	viewFn := func(tx *bolt.Tx) error {
		bucket := tx.Bucket(ix.mainBucket)
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

		allEntries := make([]Record, resultSize, resultSize)
		var err error
		var iRec interface{}
		// Note: We are guaranteed to have resultSize records in the database.
		for i := 0; i < resultSize; i++ {
			if iRec, err = ix.codec.Decode(val); err != nil {
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

	err := ix.DB.View(viewFn)
	return ret, total, err
}

// readRecs reads the records identified by keys. It returns an array
// of records with the same length as the keys array. If a record cannot be
// found the corresponding value is nil.
func (ix *IndexedBucket) readRecs(keys []string, tx *bolt.Tx) ([]Record, error) {
	var ret []Record = nil
	viewFn := func(tx *bolt.Tx) error {
		bucket := tx.Bucket(ix.mainBucket)
		ret = make([]Record, len(keys), len(keys))

		// TODO(stephana): Make this parallel since decoding can be CPU intensive.
		for i, key := range keys {
			contentBytes := bucket.Get([]byte(key))
			if contentBytes != nil {
				rec, err := ix.codec.Decode(contentBytes)
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
		err = ix.DB.View(viewFn)
	}

	return ret, err
}

// indexLookup returns the primary keys for the given (bucket,key) pair.
// The bucket is assumed to contain an index.
func (ix *IndexedBucket) indexLookup(bucket *bolt.Bucket, key string) []string {
	searchPrefix := []byte(key)
	searchPrefixLen := len(searchPrefix)
	ret := []string{}
	cursor := bucket.Cursor()
	for k, _ := cursor.Seek(searchPrefix); bytes.HasPrefix(k, searchPrefix); k, _ = cursor.Next() {
		keyPrefixLen, val := fromIndexEntry(k)
		// Make sure the index value matches the target exactly.
		if keyPrefixLen != searchPrefixLen {
			continue
		}
		ret = append(ret, val)
	}
	return ret
}

// getIndexState captures the index values for the records in the given list.
// It returns an array where each element maps to the corresponding element
// in the input array.
func (ix *IndexedBucket) getIndexState(recs []Record) ([]map[string][]string, error) {
	ret := make([]map[string][]string, len(recs), len(recs))
	var err error
	for i, rec := range recs {
		if rec != nil {
			if ret[i], err = ix.getIndexValues(rec, ix.indices); err != nil {
				return nil, err
			}
		}
	}
	return ret, nil
}

// getIndexValues extracts the index values from rec and returns a deep copy.
func (ix *IndexedBucket) getIndexValues(rec Record, targetIndices util.StringSet) (map[string][]string, error) {
	iv := rec.IndexValues()
	ret := make(map[string][]string, len(iv))
	for idxName, values := range iv {
		if _, ok := ix.indices[idxName]; !ok {
			return nil, fmt.Errorf("Unknown index name: %s. All indices returned by IndexValues must be listed in initial Config.", idxName)
		}

		// Make sure the index values do not exceed the maximum length.
		for _, val := range values {
			if len(val) > MAX_INDEX_VAL_LEN {
				return nil, fmt.Errorf("Value for index %s too long. Cannot exceed %d.", idxName, MAX_INDEX_VAL_LEN)
			}
		}

		// Only consider the indices in targetIndices.
		if !targetIndices[idxName] {
			continue
		}

		ret[idxName] = append([]string(nil), values...)
	}
	return ret, nil
}

// toIndexEntry concatenates the index value (the value for which we wish to search)
// and parent id (primary key) and the length of the index value. This allows
// to separate them without inserting any kind of separator. By appending the
// length at the end, we are still able to use the result in a prefix scan.
func toIndexEntry(indexVal string, parentID string) string {
	len1, len2 := len(indexVal), len(parentID)
	ret := make([]byte, len1+len2+2, len1+len2+2)
	copy(ret, indexVal)
	copy(ret[len1:], parentID)
	ret[len1+len2] = byte(len1 >> 8)
	ret[len1+len2+1] = byte(len1)
	return string(ret)
}

// fromIndexEntry splits a byte slice generated by the toIndexEntry and returns
// the primary key value and the length of the index value. We ignore the
// prefixed index value under the assumption that this entry has already been
// chosen because of having a specific prefix.
func fromIndexEntry(entry []byte) (int, string) {
	first := len(entry) - 2
	keyLen := (int(entry[first]) << 8) | int(entry[first+1])
	ret := append([]byte(nil), entry[keyLen:first]...)
	return keyLen, string(ret)
}

// updateIndices updates the indices the given records
func (ix *IndexedBucket) updateIndices(tx *bolt.Tx, recs []Record, baseState []map[string][]string, indices util.StringSet) error {
	addChanges := make(map[string][]string, len(indices))
	delChanges := make(map[string][]string, len(indices))
	var indexVals map[string][]string
	var err error
	for i, rec := range recs {
		// rec != nil indicates that this record will change in the database and
		// we need to update the index.
		if rec != nil {
			parentId := rec.Key()
			if indexVals, err = ix.getIndexValues(rec, indices); err != nil {
				return err
			}

			for idxName, indexVals := range indexVals {
				newVals := util.NewStringSet(indexVals)
				oldVals := util.NewStringSet(baseState[i][idxName])
				for indexVal := range newVals.Complement(oldVals) {
					addChanges[idxName] = append(addChanges[idxName], toIndexEntry(indexVal, parentId))
				}
				for indexVal := range oldVals.Complement(newVals) {
					delChanges[idxName] = append(delChanges[idxName], toIndexEntry(indexVal, parentId))
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
func (ix *IndexedBucket) deleteIndices(tx *bolt.Tx, entries []Record) error {
	changes := make(map[string][]string, len(ix.indices))
	var idxValsMap map[string][]string
	var err error
	for _, entry := range entries {
		if entry != nil {
			parentId := entry.Key()
			if idxValsMap, err = ix.getIndexValues(entry, ix.indices); err != nil {
				return err
			}

			for idxName, indexVals := range idxValsMap {
				for _, indexVal := range indexVals {
					changes[idxName] = append(changes[idxName], toIndexEntry(indexVal, parentId))
				}
			}
		}
	}
	return writeIndexChanges(tx, changes, deleteIndexOp)
}

// metaData captures the meta data that are stored for each IndexedBucket.
type metaData struct {
	// Version is for future use when the bucket layout changes.
	Version int

	// Indices contains the indices currently maintained for a bucket.
	Indices []string
}

// initBuckets makes sure all needed buckets exist and the configured indices
// are correct in the database. If indices were added or removed it will
// update the database accordingly.
func (ix *IndexedBucket) initBuckets() error {
	return ix.DB.Update(func(tx *bolt.Tx) error {
		// Create the main bucket if it does not exist.
		if _, err := tx.CreateBucketIfNotExists(ix.mainBucket); err != nil {
			return err
		}

		var removeIndices util.StringSet = nil
		var addIndices util.StringSet = nil

		// Get the metadata bucket and load the meta data.
		metaBucketName := []byte(META_DATA_BUCKET_PREFIX + string(ix.mainBucket))
		codec := util.NewJSONCodec(&metaData{})
		bucket := tx.Bucket(metaBucketName)
		var err error

		// If the bucket doesn't exist or is empty, we create all indices.
		if bucket == nil {
			if bucket, err = tx.CreateBucket(metaBucketName); err != nil {
				return err
			}
			addIndices = ix.indices
		} else if metaBytes := bucket.Get([]byte(META_DATA_KEY)); metaBytes == nil {
			addIndices = ix.indices
		} else {
			// deserialize the meta data.
			iMeta, err := codec.Decode(metaBytes)
			if err != nil {
				return err
			}
			metaRec := iMeta.(*metaData)

			// Find the indices that need to be added or removed.
			existingIndices := util.NewStringSet(metaRec.Indices)
			addIndices = ix.indices.Complement(existingIndices)
			removeIndices = existingIndices.Complement(ix.indices)
		}

		// Delete all unnecessary indices.
		for idxName := range removeIndices {
			if err := tx.DeleteBucket([]byte(idxName)); err != nil {
				return err
			}
		}

		// Add the necessary indices.
		if len(addIndices) > 0 {
			if err := ix.buildIndices(tx, addIndices); err != nil {
				return err
			}
		}

		// Write the meta data.
		metaRec := &metaData{Version: 1, Indices: ix.indices.Keys()}
		metaBytes, err := codec.Encode(metaRec)
		if err != nil {
			return err
		}
		return bucket.Put([]byte(META_DATA_KEY), metaBytes)
	})
}

// buildIndices iterates over all records and creates the provided indices.
func (ix *IndexedBucket) buildIndices(tx *bolt.Tx, indices util.StringSet) error {
	// Clear the indices first to be in a well defined state.
	for idxName := range indices {
		bucket := tx.Bucket([]byte(idxName))
		if bucket != nil {
			if err := tx.DeleteBucket([]byte(idxName)); err != nil {
				return err
			}
		}
		if _, err := tx.CreateBucket([]byte(idxName)); err != nil {
			return err
		}
	}

	// iterate over the entire database and build the indices.
	recBatch := make([]Record, 0, BUILD_INDEX_BATCH_SIZE)
	emptyBaseState := make([]map[string][]string, BUILD_INDEX_BATCH_SIZE, BUILD_INDEX_BATCH_SIZE)
	c := tx.Bucket(ix.mainBucket).Cursor()
	for k, v := c.First(); k != nil; k, v = c.Next() {
		rec, err := ix.codec.Decode(v)
		if err != nil {
			return fmt.Errorf("Error while (re)building indices, while reading record with key %q: %s", string(k), err)
		}
		recBatch = append(recBatch, rec.(Record))
		if len(recBatch) >= BUILD_INDEX_BATCH_SIZE {
			if err := ix.updateIndices(tx, recBatch, emptyBaseState[:len(recBatch)], indices); err != nil {
				return err
			}
			// Note: This might prevent some instances of Record from being garbage collected
			// right away which is accpetable in this case.
			recBatch = recBatch[:0]
		}
	}

	// Process any records that haven't been indexed yet.
	if len(recBatch) > 0 {
		if err := ix.updateIndices(tx, recBatch, emptyBaseState[:len(recBatch)], indices); err != nil {
			return err
		}
	}
	return nil
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
