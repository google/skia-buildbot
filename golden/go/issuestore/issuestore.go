package issuestore

import (
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
	Get(issueIDs []string) ([]*Rec, error)
	Add(delta *Rec) error
	Remove(issueID string, delta *Rec) error
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
	updated = updated || mergeStrings(&r.Traces, deltaRec.Traces)
	updated = updated || mergeStrings(&r.Ignores, deltaRec.Ignores)
	return updated || mergeStrings(&r.TestNames, deltaRec.TestNames)
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
	return nil, nil
}

func (b *boltIssueStore) ByIgnore(ignoreID string) ([]string, error) {
	return nil, nil
}

func (b *boltIssueStore) ByTrace(traceID string) ([]string, error) {
	return nil, nil
}

func (b *boltIssueStore) ByTest(testName string) ([]string, error) {
	return nil, nil
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
	issueID := []byte(delta.IssueID)
	updateFn := func(tx *bolt.Tx) error {
		// load the current entry
		bucket := tx.Bucket(ISSUE_BUCKET)

		// merge the new entry into the existing entry.
		var issue *Rec = delta
		var updated = true
		var err error
		issueBytes := bucket.Get(issueID)
		if issueBytes != nil {
			issue, err = b.deserialize(issueBytes)
			if err != nil {
				return err
			}
			updated = issue.Merge(delta)
		}

		// If there was an update load the indices
		if !updated {
			return nil
		}

		// Index the issue and serialize it.
		if err := b.index(tx, issue); err != nil {
			return err
		}

		if issueBytes, err = b.serialize(issue); err != nil {
			return err
		}

		return bucket.Put([]byte(issueID), issueBytes)
	}

	return b.db.Update(updateFn)
}

func (b *boltIssueStore) Remove(issueID string, delta *Rec) error {
	return nil
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

func (b *boltIssueStore) serialize(rec *Rec) ([]byte, error) {
	return json.Marshal(rec)
}

func (b *boltIssueStore) deserialize(issueBytes []byte) (*Rec, error) {
	ret := &Rec{}
	return ret, json.Unmarshal(issueBytes, ret)
}

func (b *boltIssueStore) updateIndex(tx *bolt.Tx, indexID []byte, childIDs []string, parentID string) error {
	indexBucket := tx.Bucket(indexID)
	for _, childID := range childIDs {
		if err := indexBucket.Put([]byte(childID+IDX_SEPARATOR+parentID), []byte{}); err != nil {
			return err
		}
	}
	return nil
}

func (b *boltIssueStore) index(tx *bolt.Tx, rec *Rec) error {
	if err := b.updateIndex(tx, DIGEST_INDEX, rec.Digests, rec.IssueID); err != nil {
		return err
	}

	if err := b.updateIndex(tx, TRACE_INDEX, rec.Traces, rec.IssueID); err != nil {
		return err
	}

	if err := b.updateIndex(tx, IGNORE_INDEX, rec.Ignores, rec.IssueID); err != nil {
		return err
	}

	if err := b.updateIndex(tx, TEST_INDEX, rec.TestNames, rec.IssueID); err != nil {
		return err
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
