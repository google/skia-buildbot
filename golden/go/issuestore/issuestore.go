package issuestore

import (
	"bytes"
	"encoding/json"
	"path"

	"github.com/boltdb/bolt"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/util"
)

// IssueStore captures the functions necessary to persist the connection between
// Monorail issues and digests, traces, tests and ignores.
type IssueStore interface {
	// ByDigest returns the ids of all issue associated with the given digest.
	ByDigest(digest string) ([]string, error) // list of issues

	// ByDigest returns the ids of all issue associated with the given digest.
	ByIgnore(ignoreID string) ([]string, error) // list of issues

	// ByDigest returns the ids of all issue associated with the given digest.
	ByTrace(traceID string) ([]string, error) // list of issues

	// ByDigest returns the ids of all issue associated with the given digest.
	ByTest(testName string) ([]string, error) // list of issues

	// Add allows to create an issue annotation or add to an existing annotation.
	// If the issue identified by delta.IssueID exists, delta will be merged into
	// the existing annotation.
	Add(delta *Annotation) error

	// Subtract removes from an existing issue annotation. The values in
	// delta are subtracted from an existing annotation.
	Subtract(delta *Annotation) error

	// Get returns the annotations for the given list of issue ids.
	Get(issueIDs []string) ([]*Annotation, error)

	// List returns a list of all issues that are currently annotated with
	// support of paging. The first 'offset' annotations will be skipped and
	// the returned array has at most 'size'. If 'size' <= 0 there is no limit
	// on the number of annotations returned.
	List(offset int, size int) ([]*Annotation, int, error)

	// Delete the given issue annotations.
	Delete(issueIDs []string) error
}

// Annotation captures annotations for the issue identified by IssueID.
type Annotation struct {
	IssueID   string   // id of the issue in Monorail
	Digests   []string // Image digests connected to this issue
	Traces    []string // Trace ids connected to this issues.
	Ignores   []string // Ignore ids connected to this issue.
	TestNames []string // Test names connected to this issue.
}

// Adds the digests, traces, ignores and tests in delta to the current annotation.
// and deduplicates in the process.
func (r *Annotation) Add(deltaRec *Annotation) bool {
	updated := mergeStrings(&r.Digests, deltaRec.Digests)
	updated = mergeStrings(&r.Traces, deltaRec.Traces) || updated
	updated = mergeStrings(&r.Ignores, deltaRec.Ignores) || updated
	return mergeStrings(&r.TestNames, deltaRec.TestNames) || updated
}

// Subtract removes the digests, traces, ignores and tests in delta from the current annotation.
func (r *Annotation) Subtract(deltaRec *Annotation) bool {
	updated := removeStrings(&r.Digests, deltaRec.Digests)
	updated = removeStrings(&r.Traces, deltaRec.Traces) || updated
	updated = removeStrings(&r.Ignores, deltaRec.Ignores) || updated
	return removeStrings(&r.TestNames, deltaRec.TestNames) || updated
}

// IsEmpty returns true if all all annotations are empty.
func (r *Annotation) IsEmpty() bool {
	return (len(r.Digests) + len(r.Traces) + len(r.Ignores) + len(r.TestNames)) == 0
}

var (
	// Bucket names in boltdb. 'INDEX' in the name indicates an index.
	ISSUE_BUCKET = []byte("issues")
	DIGEST_INDEX = []byte("digest-idx")
	TRACE_INDEX  = []byte("trace-idx")
	IGNORE_INDEX = []byte("ignore-idx")
	TEST_INDEX   = []byte("test-idx")
)

// Separator used to separate child and parent id in indices.
const IDX_SEPARATOR = "|"

// boltIssueStore implements the IssueStore interface.
type boltIssueStore struct {
	db *bolt.DB
}

// New returns a new instance of IssueStore that is stored in the given directory.
func New(baseDir string) (IssueStore, error) {
	baseDir, err := fileutil.EnsureDirExists(baseDir)
	if err != nil {
		return nil, err
	}

	dbName := path.Join(baseDir, "ignorestore.db")
	db, err := bolt.Open(dbName, 0600, nil)
	if err != nil {
		return nil, err
	}

	ret := &boltIssueStore{
		db: db,
	}

	return ret, ret.initBuckets()
}

// ByDigest, see IgnoreStore interface.
func (b *boltIssueStore) ByDigest(digest string) ([]string, error) {
	return b.getIndexedData(DIGEST_INDEX, digest)
}

// ByIgnore, see IgnoreStore interface.
func (b *boltIssueStore) ByIgnore(ignoreID string) ([]string, error) {
	return b.getIndexedData(IGNORE_INDEX, ignoreID)
}

// ByTrace, see IgnoreStore interface.
func (b *boltIssueStore) ByTrace(traceID string) ([]string, error) {
	return b.getIndexedData(TRACE_INDEX, traceID)
}

// ByTest, see IgnoreStore interface.
func (b *boltIssueStore) ByTest(testName string) ([]string, error) {
	return b.getIndexedData(TEST_INDEX, testName)
}

// Get, see IgnoreStore interface.
func (b *boltIssueStore) Get(issueIDs []string) ([]*Annotation, error) {
	if len(issueIDs) == 0 {
		return []*Annotation{}, nil
	}

	var ret []*Annotation
	viewFn := func(tx *bolt.Tx) error {
		bucket := tx.Bucket(ISSUE_BUCKET)
		found := make([]*Annotation, 0, len(issueIDs))
		for _, issueID := range issueIDs {
			issueBytes := bucket.Get([]byte(issueID))
			if issueBytes != nil {
				issue, err := b.deserialize(issueBytes)
				if err != nil {
					return err
				}
				found = append(found, issue)
			}
		}
		// Only assign the result if we succeed.
		ret = found
		return nil
	}
	return ret, b.db.View(viewFn)
}

// Add, see IgnoreStore interface.
func (b *boltIssueStore) Add(delta *Annotation) error {
	return b.update(nil, delta, addOP)
}

// List, see IgnoreStore interface.
func (b *boltIssueStore) List(offset int, size int) ([]*Annotation, int, error) {
	var ret []*Annotation
	total := 0
	viewFn := func(tx *bolt.Tx) error {
		bucket := tx.Bucket(ISSUE_BUCKET)
		nEntries := bucket.Stats().KeyN

		// If size <= 0 then we want all entries.
		if size <= 0 {
			size = nEntries
		}
		resultSize := util.MaxInt(0, util.MinInt(nEntries-offset, size))
		if resultSize == 0 {
			ret = []*Annotation{}
			return nil
		}

		cursor := bucket.Cursor()
		_, v := cursor.First()

		// TODO(stephana): This is definitely inefficient, but there is no
		// option in boltdb to address a specific element by index. If this is
		// too slow we can add an additional pseudo index for issue ids.

		// Skip the first entries if offset > 0.
		for i := 0; i < offset; i++ {
			_, v = cursor.Next()
		}

		allEntries := make([]*Annotation, resultSize)
		var err error
		for i := 0; i < resultSize; i++ {
			if allEntries[i], err = b.deserialize(v); err != nil {
				return err
			}
			_, v = cursor.Next()
		}

		// Success assign the results now.
		ret = allEntries
		total = nEntries
		return nil
	}

	return ret, total, b.db.View(viewFn)
}

// Subtract, see IgnoreStore interface.
func (b *boltIssueStore) Subtract(delta *Annotation) error {
	return b.update(nil, delta, subtractOP)
}

// Delete, see IgnoreStore interface.
func (b *boltIssueStore) Delete(issueIDs []string) error {
	deleteFn := func(tx *bolt.Tx) error {
		var rec Annotation
		for _, issueID := range issueIDs {
			rec.IssueID = issueID
			if err := b.update(tx, &rec, deleteOP); err != nil {
				return err
			}
		}
		return nil
	}
	return b.db.Update(deleteFn)
}

// initBuckets makes sure all needed buckets exist.
func (b *boltIssueStore) initBuckets() error {
	return b.db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(ISSUE_BUCKET); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(DIGEST_INDEX); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(TRACE_INDEX); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(IGNORE_INDEX); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(TEST_INDEX); err != nil {
			return err
		}
		return nil
	})
}

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
func (b *boltIssueStore) update(tx *bolt.Tx, delta *Annotation, op updateOP) error {
	issueID := []byte(delta.IssueID)
	updateFn := func(tx *bolt.Tx) error {
		// load the current entry
		bucket := tx.Bucket(ISSUE_BUCKET)

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

// serialize serializes an annotation to bytes.
func (b *boltIssueStore) serialize(rec *Annotation) ([]byte, error) {
	return json.Marshal(rec)
}

// deserialize deserializes an annotation from bytes.
func (b *boltIssueStore) deserialize(issueBytes []byte) (*Annotation, error) {
	ret := &Annotation{}
	return ret, json.Unmarshal(issueBytes, ret)
}

// index adds or removes an issue annotation from all the indices.
func (b *boltIssueStore) index(tx *bolt.Tx, rec *Annotation, subtract bool) error {
	if err := b.updateIndex(tx, DIGEST_INDEX, rec.Digests, rec.IssueID, subtract); err != nil {
		return err
	}

	if err := b.updateIndex(tx, TRACE_INDEX, rec.Traces, rec.IssueID, subtract); err != nil {
		return err
	}

	if err := b.updateIndex(tx, IGNORE_INDEX, rec.Ignores, rec.IssueID, subtract); err != nil {
		return err
	}

	return b.updateIndex(tx, TEST_INDEX, rec.TestNames, rec.IssueID, subtract)
}

// updateIndex adds or removes all (childID, parentID) pairs from an index.
func (b *boltIssueStore) updateIndex(tx *bolt.Tx, indexID []byte, childIDs []string, parentID string, subtract bool) error {
	indexBucket := tx.Bucket(indexID)
	var doFn func([]byte) error
	if subtract {
		doFn = func(key []byte) error { return indexBucket.Delete(key) }
	} else {
		doFn = func(key []byte) error { return indexBucket.Put(key, []byte{}) }
	}

	for _, childID := range childIDs {
		if err := doFn([]byte(childID + IDX_SEPARATOR + parentID)); err != nil {
			return err
		}
	}
	return nil
}

// mergeStrings merges the strings of src into tgt. true is returned if the
// strings in tgt changed as a result of the merge.
func mergeStrings(tgt *[]string, src []string) bool {
	if t := util.NewStringSet(*tgt, src); len(t) != len(*tgt) {
		*tgt = t.Keys()
		return true
	}
	return false
}

// removeStrings removes all strings from tgt that also appear in src. true is returned
// if tgt changed as part of the removal.
func removeStrings(tgt *[]string, src []string) bool {
	if t := util.NewStringSet(*tgt).Complement(util.NewStringSet(src)); len(t) != len(*tgt) {
		*tgt = t.Keys()
		return true
	}
	return false
}

// getIndexedData retrieves all parentIDs that have the given prefix in the index
// identified by indexName.
func (b *boltIssueStore) getIndexedData(indexName []byte, indexPrefix string) ([]string, error) {
	if indexPrefix == "" {
		return []string{}, nil
	}

	prefix := []byte(indexPrefix + IDX_SEPARATOR)
	lenPrefix := len(prefix)
	ret := []string{}
	viewFn := func(tx *bolt.Tx) error {
		cursor := tx.Bucket(indexName).Cursor()
		for k, _ := cursor.Seek(prefix); bytes.HasPrefix(k, prefix); k, _ = cursor.Next() {
			ret = append(ret, string(append([]byte(nil), k[lenPrefix:]...)))
		}
		return nil
	}

	return ret, b.db.View(viewFn)
}
