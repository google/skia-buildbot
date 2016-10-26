package issuestore

import (
	"bytes"
	"encoding/json"
	"path"

	"github.com/boltdb/bolt"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/util"
)

var (
	// Bucket names in boltdb.
	ISSUE_BUCKET = []byte("issues")
	DIGEST_INDEX = []byte("digest-idx")
	TRACE_INDEX  = []byte("trace-idx")
	IGNORE_INDEX = []byte("ignore-idx")
	TEST_INDEX   = []byte("test-idx")
)

const IDX_SEPARATOR = "|"

type IssueStore interface {
	ByDigest(digest string) ([]string, error)   // list of issues
	ByIgnore(ignoreID string) ([]string, error) // list of issues
	ByTrace(traceID string) ([]string, error)   // list of issues
	ByTest(testName string) ([]string, error)   // list of issues
	Add(delta *Rec) error
	Remove(delta *Rec) error
	Get(issueIDs []string) ([]*Rec, error)
	List(offset int, size int) ([]*Rec, int, error)
	Delete(issueID string) error
}

type Rec struct {
	IssueID   string   // id of the issue we want to annotate.
	Digests   []string // Example digests we are interested in.
	Traces    []string // Traces we are interested in.
	Ignores   []string // Ignores
	TestNames []string // TestNames
}

// returns true if this was updated.
func (r *Rec) Merge(deltaRec *Rec) bool {
	updated := mergeStrings(&r.Digests, deltaRec.Digests)
	updated = mergeStrings(&r.Traces, deltaRec.Traces) || updated
	updated = mergeStrings(&r.Ignores, deltaRec.Ignores) || updated
	return mergeStrings(&r.TestNames, deltaRec.TestNames) || updated
}

func (r *Rec) Remove(deltaRec *Rec) bool {
	updated := removeStrings(&r.Digests, deltaRec.Digests)
	updated = removeStrings(&r.Traces, deltaRec.Traces) || updated
	updated = removeStrings(&r.Ignores, deltaRec.Ignores) || updated
	return removeStrings(&r.TestNames, deltaRec.TestNames) || updated
}

type boltIssueStore struct {
	db *bolt.DB
}

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

func (b *boltIssueStore) ByDigest(digest string) ([]string, error) {
	return b.getIndexedData(DIGEST_INDEX, digest)
}

func (b *boltIssueStore) ByIgnore(ignoreID string) ([]string, error) {
	return b.getIndexedData(IGNORE_INDEX, ignoreID)
}

func (b *boltIssueStore) ByTrace(traceID string) ([]string, error) {
	return b.getIndexedData(TRACE_INDEX, traceID)
}

func (b *boltIssueStore) ByTest(testName string) ([]string, error) {
	return b.getIndexedData(TEST_INDEX, testName)
}

func (b *boltIssueStore) Get(issueIDs []string) ([]*Rec, error) {
	if len(issueIDs) == 0 {
		return []*Rec{}, nil
	}

	var ret []*Rec
	viewFn := func(tx *bolt.Tx) error {
		bucket := tx.Bucket(ISSUE_BUCKET)
		found := make([]*Rec, 0, len(issueIDs))
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
		// Only assing the result if we succeed.
		ret = found
		return nil
	}
	return ret, b.db.View(viewFn)
}

func (b *boltIssueStore) Add(delta *Rec) error {
	return b.addOrRemove(delta, false)
}

func (b *boltIssueStore) List(offset int, size int) ([]*Rec, int, error) {
	var ret []*Rec
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
			ret = []*Rec{}
			return nil
		}

		cursor := bucket.Cursor()
		_, v := cursor.First()

		// TODO(stephana): This is definitely inefficient, but ther there is no
		// option in boltdb to address a specific element by index. If this is
		// too slow we can add an additional pseudo index for issue ids.
		// Skip the first entries if offset > 0.
		if offset > 0 {
			for i := 0; i < offset; i++ {
				_, v = cursor.Next()
			}
		}

		allEntries := make([]*Rec, resultSize)
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

func (b *boltIssueStore) Remove(delta *Rec) error {
	return b.addOrRemove(delta, true)
}

func (b *boltIssueStore) Delete(issueID string) error {
	return nil
}

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

func (b *boltIssueStore) addOrRemove(delta *Rec, remove bool) error {
	issueID := []byte(delta.IssueID)
	updateFn := func(tx *bolt.Tx) error {
		// load the current entry
		bucket := tx.Bucket(ISSUE_BUCKET)

		// merge the new entry into the existing entry.
		var issue *Rec = delta
		var updated = true
		var err error
		issueBytes := bucket.Get(issueID)

		// if we are removing and there is no previous record then stop.
		if remove && issueBytes == nil {
			return nil
		}

		if issueBytes != nil {
			issue, err = b.deserialize(issueBytes)
			if err != nil {
				return err
			}
			if remove {
				updated = issue.Remove(delta)
			} else {
				updated = issue.Merge(delta)
			}
		}

		// If nothing was updated we are don't need to write the record.
		if !updated {
			return nil
		}

		// Index the delta.
		if err := b.index(tx, delta, remove); err != nil {
			return err
		}

		if issueBytes, err = b.serialize(issue); err != nil {
			return err
		}

		return bucket.Put([]byte(issueID), issueBytes)
	}

	return b.db.Update(updateFn)
}

func (b *boltIssueStore) serialize(rec *Rec) ([]byte, error) {
	return json.Marshal(rec)
}

func (b *boltIssueStore) deserialize(issueBytes []byte) (*Rec, error) {
	ret := &Rec{}
	return ret, json.Unmarshal(issueBytes, ret)
}

func (b *boltIssueStore) index(tx *bolt.Tx, rec *Rec, remove bool) error {
	if err := b.updateIndex(tx, DIGEST_INDEX, rec.Digests, rec.IssueID, remove); err != nil {
		return err
	}

	if err := b.updateIndex(tx, TRACE_INDEX, rec.Traces, rec.IssueID, remove); err != nil {
		return err
	}

	if err := b.updateIndex(tx, IGNORE_INDEX, rec.Ignores, rec.IssueID, remove); err != nil {
		return err
	}

	if err := b.updateIndex(tx, TEST_INDEX, rec.TestNames, rec.IssueID, remove); err != nil {
		return err
	}
	return nil
}

func (b *boltIssueStore) updateIndex(tx *bolt.Tx, indexID []byte, childIDs []string, parentID string, remove bool) error {
	indexBucket := tx.Bucket(indexID)
	var doFn func([]byte) error
	if remove {
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

func mergeStrings(tgt *[]string, src []string) bool {
	if t := util.NewStringSet(*tgt, src); len(t) != len(*tgt) {
		*tgt = t.Keys()
		return true
	}
	return false
}

func removeStrings(tgt *[]string, src []string) bool {
	if t := util.NewStringSet(*tgt).Complement(util.NewStringSet(src)); len(t) != len(*tgt) {
		*tgt = t.Keys()
		return true
	}
	return false
}

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
