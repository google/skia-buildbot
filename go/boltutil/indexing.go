package boltutil

import (
	"fmt"
	"path"

	"github.com/boltdb/bolt"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/util"
)

type WriteFn func(*bolt.Tx, []Interface) ([]Interface, error)
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

func (s *Store) Read(keys []string, tx ...*bolt.Tx) ([]Interface, error) {
	return s.readRecs(s.mainBucket, keys, tx...)
}

func (s *Store) ReadIndex(idx string, keys []string) ([]string, error) {
	if !s.indices[idx] {
		return nil, fmt.Errorf("Invalid index: '%s'", idx)
	}

	ret := make([]string, len(keys))
	viewFn := func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(idx))
		for i, key := range keys {
			contentBytes := bucket.Get([]byte(key))
			if contentBytes != nil {
				ret[i] = string(append([]byte(nil), contentBytes...))
			}
		}
		return nil
	}

	return ret, s.DB.View(viewFn)
}

func (s *Store) readRecs(bucket []byte, keys []string, tx ...*bolt.Tx) ([]Interface, error) {
	var ret []Interface = nil
	viewFn := func(tx *bolt.Tx) error {
		var err error

		bucket := tx.Bucket(bucket)
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

type Interface interface {
	Key() string
	IndexValues([]string) [][]string
}

type indexedEntries [][][]string

func (s *Store) getIndexVals(entries []Interface) indexedEntries {
	ret := make([][][]string, len(entries))
	for i, entry := range entries {
		ret[i] = entry.IndexValues(s.indices)
	}
	return ret
}

// TODO
func (s *Store) removeIndices(keys []string, iEntries indexedEntries) error {
	return nil
}

// TODO
func (s *Store) reindex(tx *bolt.Tx, rec Interface) error {
	return nil
}

// update allows to add and substract to/from an issue annotation, as well as
// delete an annotation. If 'tx' is nil a new update transaction is started otherwise
// tx is used.
func (s *Store) update(keys []string, writeFn WriteFn) error {
	updateFn := func(tx *bolt.Tx) error {
		entries, err := s.readRecs(s.mainBucket, keys, tx)
		if err != nil {
			return nil
		}

		if err := s.removeIndices(keys, s.getIndexVals(entries)); err != nil {
			return err
		}

		recs, err := writeFn(tx, entries)
		if err != nil {
			return err
		}

		if len(recs) != len(keys) {
			return fmt.Errorf("Modified array size does not match original array size.")
		}

		// Get the main bucket and write the entries plus indices to it.
		bucket := tx.Bucket(s.mainBucket)
		for _, rec := range recs {
			if rec != nil {
				// Update the index with the new version of the item.
				if err := s.reindex(tx, rec); err != nil {
					return err
				}

				// Write the issue back to the database.
				if contentBytes, err := s.codec.Encode(rec); err != nil {
					return err
				}
				if err := bucket.Put(rec.Key(), contentBytes); err != nil {
					return err
				}
			}
		}
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
