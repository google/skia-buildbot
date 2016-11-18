package boltutil

import (
	"fmt"
	"path"

	"github.com/boltdb/bolt"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/util"
)

type WriteFn func(*bolt.Tx, interface{}) (interface{}, error)
type CreateFn func(*bolt.Tx, []Interface) error

type Config struct {
	Directory string
	Name      string
	Indices   []string
	Codec     util.LRUCodec
}

type Store struct {
	*bolt.DB
	indices    util.StringSet
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
		indices:    util.NewStringSet(config.Indices),
		mainBucket: []byte(config.Name),
		codec:      config.Codec,
	}

	return ret, ret.initBuckets()
}

func (s *Store) Read(keys []string, tx ...*bolt.Tx) ([]interface{}, error) {
	return s.read(s.mainBucket, keys, s.codec.Decode, tx...)
}

func (s *Store) ReadIndex(idx string, keys []string) ([]string, error) {
	if !s.indices[idx] {
		return nil, fmt.Errorf("Invalid index: '%s'", idx)
	}

	result, err := s.read([]byte(idx), keys, bytesToString)
	if err != nil {
		return nil, err
	}
	ret := make([]string, len(result))
	for i, val := range result {
		if val != nil {
			ret[i] = val.(string)
		}
	}
	return ret, nil
}

type decodeFn func([]byte) (interface{}, error)

func bytesToString(data []byte) (interface{}, error) {
	return string(data), nil
}

func (s *Store) read(bucket []byte, keys []string, decode decodeFn, tx ...*bolt.Tx) ([]interface{}, error) {
	var ret []interface{} = nil
	viewFn := func(tx *bolt.Tx) error {
		var err error

		bucket := tx.Bucket(bucket)
		ret = make([]interface{}, len(keys))
		for i, key := range keys {
			contentBytes := bucket.Get([]byte(key))
			if contentBytes != nil {
				ret[i], err = decode(contentBytes)
				if err != nil {
					return err
				}
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

func (s *Store) Write(keys []string, writeFn WriteFn, tx ...*bolt.Tx) error {
	return s.update(keys, writeFn)
}

func (s *Store) Create(recs []Interface, createFn CreateFn) error {
	return nil
}

func (s *Store) Delete(keys []string) error {
	return nil

	// deleteFn := func(tx *bolt.Tx) error {
	// 	var rec Annotation
	// 	for _, issueID := range issueIDs {
	// 		rec.IssueID = issueID
	// 		if err := b.update(tx, &rec, deleteOP); err != nil {
	// 			return err
	// 		}
	// 	}
	// 	return nil
	// }
	// return b.db.Update(deleteFn)
}

// TODO
func (s *Store) List(offset, size int) (interface{}, int, error) {
	return nil, 0, nil

	// total := 0
	// viewFn := func(tx *bolt.Tx) error {
	// 	bucket := tx.Bucket(ISSUE_BUCKET)
	// 	nEntries := bucket.Stats().KeyN
	//
	// 	// If size <= 0 then we want all entries.
	// 	if size <= 0 {
	// 		size = nEntries
	// 	}
	// 	resultSize := util.MaxInt(0, util.MinInt(nEntries-offset, size))
	// 	if resultSize == 0 {
	// 		ret = []*Annotation{}
	// 		return nil
	// 	}
	//f
	// 	cursor := bucket.Cursor()
	// 	_, v := cursor.First()
	//
	// 	// TODO(stephana): This is definitely inefficient, but there is no
	// 	// option in boltdb to address a specific element by index. If this is
	// 	// too slow we can add an additional pseudo index for issue ids.
	//
	// 	// Skip the first entries if offset > 0.
	// 	for i := 0; i < offset; i++ {
	// 		_, v = cursor.Next()
	// 	}
	//
	// 	allEntries := make([]*Annotation, resultSize)
	// 	var err error
	// 	for i := 0; i < resultSize; i++ {
	// 		if allEntries[i], err = b.deserialize(v); err != nil {
	// 			return err
	// 		}
	// 		_, v = cursor.Next()
	// 	}
	//
	// 	// Success assign the results now.
	// 	ret = allEntries
	// 	total = nEntries
	// 	return nil
	// }
	//
	// return ret, total, b.db.View(viewFn)
	//

}

type Interface interface{}

type IndexedDB struct{}

// getIndexedData retrieves all parentIDs that have the given prefix in the index
// identified by indexName.
// func (b *boltIssueStore) getIndexedData(indexName []byte, indexPrefix string) ([]string, error) {
// 	if indexPrefix == "" {
// 		return []string{}, nil
// 	}
//
// 	prefix := []byte(indexPrefix + IDX_SEPARATOR)
// 	lenPrefix := len(prefix)
// 	ret := []string{}
// 	viewFn := func(tx *bolt.Tx) error {
// 		cursor := tx.Bucket(indexName).Cursor()
// 		for k, _ := cursor.Seek(prefix); bytes.HasPrefix(k, prefix); k, _ = cursor.Next() {
// 			ret = append(ret, string(append([]byte(nil), k[lenPrefix:]...)))
// 		}
// 		return nil
// 	}
//
// 	return ret, b.db.View(viewFn)
// }

// updateOP and the constants below are  used to distinguish update operations.
type updateOP int

// available update operations.
const (
	addOP      updateOP = iota // add to an issue annotation.
	subtractOP                 // subtract from an issue annotation.
	deleteOP                   // delete an issue annotation.
)


// update allows to add and substract to/from an issue annotation, as well as
// delete an annotation. If 'tx' is nil a new update transaction is started otherwise
// tx is used.
func (s *Store) update(keys []string, writeFn WriteFn) error {
	updateFn := func(tx *bolt.Tx) error {
		entries, err := s.read(s.mainBucket, keys, s.codec.Decode, tx)
		if err != nil {
			return nil
		}

    // Get the index values before
    indices := s.indices.Keys()

    before := make([][]util.StringSet, len(entries))
    for i, entry := range entries {
      before[i] = entry.IndexValues(indices)
    }


		recs, err := writeFn(entries)
    if err != nil {
      return error
    }

    // Get the index values after.
    after := before.sub(entrie.Index)


    // find the delta of the index values

    // write the delta to disk.




    if len(recs) != len(keys) {
      return fmt.Errorf("Modified array size does not match original array size.")
    }

    for _, rec := range recs {
      if rec != nil {
        // Update the index with the new version of the item.
    		if err := b.reindex(tx, rec); err != nil {
    			return err
    		}

        // Write the issue back to the database.
    		if issueBytes, err = b.serialize(issue); err != nil {
    			return err
    		}

    		return bucket.Put(issueID, issueBytes)




    }





		// merge the new entry into the existing entry.
		var issue *Annotation = delta
		var updated = true
		var err error
		issueBytes := bucket.Get(issueID)

		// if we are removing and there is no previous record then stop.
		if ((op == subtractOP) || (op == deleteOP)) && issueBytes == nil {
			return nil
		}

		if issueBytes != nil {
			issue, err = b.deserialize(issueBytes)
			if err != nil {
				return err
			}

			// If we are deleting use the entire issue in the indexing operation below.
			if op == deleteOP {
				delta = issue
			} else if op == subtractOP {
				updated = issue.Subtract(delta)
			} else {
				updated = issue.Add(delta)
			}
		}

		// If nothing was updated we are don't need to write the record.
		if !updated {
			return nil
		}

		// If we are adding and this is an emtpy annotation we don't write anyting.
		if (op == addOP) && issue.IsEmpty() {
			return nil
		}

		// Update the index with the delta.
		if err := b.index(tx, delta, (op == subtractOP) || (op == deleteOP)); err != nil {
			return err
		}

		// If we are deleting or the issue is empty after subtraction then delete the whole entry.
		if (op == deleteOP) || ((op == subtractOP) && issue.IsEmpty()) {
			return bucket.Delete(issueID)
		}

		// Write the issue back to the database.
		if issueBytes, err = b.serialize(issue); err != nil {
			return err
		}

		return bucket.Put(issueID, issueBytes)
	}

	if tx != nil {
		return updateFn(tx)
	}
	return b.db.Update(updateFn)
}

//
// // serialize serializes an annotation to bytes.
// func (b *boltIssueStore) serialize(rec *Annotation) ([]byte, error) {
// 	return json.Marshal(rec)
// }
//
// // deserialize deserializes an annotation from bytes.
// func (b *boltIssueStore) deserialize(issueBytes []byte) (*Annotation, error) {
// 	ret := &Annotation{}
// 	return ret, json.Unmarshal(issueBytes, ret)
// }

// initBuckets makes sure all needed buckets exist.
func (s *Store) initBuckets() error {
	return s.DB.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(s.mainBucket); err != nil {
			return err
		}

		for idx := range s.indices {
			if idx == "" {
				return fmt.Errorf("Index name cannot be empty string.")
			}
			if _, err := tx.CreateBucketIfNotExists([]byte(idx)); err != nil {
				return err
			}
		}
		return nil
	})
}

// // index adds or removes an issue annotation from all the indices.
// func (b *boltIssueStore) index(tx *bolt.Tx, rec *Annotation, subtract bool) error {
// 	if err := b.updateIndex(tx, DIGEST_INDEX, rec.Digests, rec.IssueID, subtract); err != nil {
// 		return err
// 	}
//
// 	if err := b.updateIndex(tx, TRACE_INDEX, rec.Traces, rec.IssueID, subtract); err != nil {
// 		return err
// 	}
//
// 	if err := b.updateIndex(tx, IGNORE_INDEX, rec.Ignores, rec.IssueID, subtract); err != nil {
// 		return err
// 	}
//
// 	return b.updateIndex(tx, TEST_INDEX, rec.TestNames, rec.IssueID, subtract)
// }

// // updateIndex adds or removes all (childID, parentID) pairs from an index.
// func (b *boltIssueStore) updateIndex(tx *bolt.Tx, indexID []byte, childIDs []string, parentID string, subtract bool) error {
// 	indexBucket := tx.Bucket(indexID)
// 	var doFn func([]byte) error
// 	if subtract {
// 		doFn = func(key []byte) error { return indexBucket.Delete(key) }
// 	} else {
// 		doFn = func(key []byte) error { return indexBucket.Put(key, []byte{}) }
// 	}
//
// 	for _, childID := range childIDs {
// 		if err := doFn([]byte(childID + IDX_SEPARATOR + parentID)); err != nil {
// 			return err
// 		}
// 	}
// 	return nil
// }
