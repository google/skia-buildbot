package issuestore

import (
	"path"

	"github.com/boltdb/bolt"
	"go.skia.org/infra/go/boltutil"
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

var annotationIndices = []string{DIGEST_INDEX, TRACE_INDEX, IGNORE_INDEX, TEST_INDEX}

// Annotation captures annotations for the issue identified by IssueID.
type Annotation struct {
	IssueID   string   // id of the issue in Monorail
	Digests   []string // Image digests connected to this issue
	Traces    []string // Trace ids connected to this issues.
	Ignores   []string // Ignore ids connected to this issue.
	TestNames []string // Test names connected to this issue.
}

// Key see boltutil.Record interface.
func (a *Annotation) Key() string {
	return a.IssueID
}

// IndexValues see boltutil.Record interface.
func (a *Annotation) IndexValues() map[string][]string {
	ret := make(map[string][]string, len(annotationIndices))
	for _, idx := range annotationIndices {
		switch idx {
		case DIGEST_INDEX:
			ret[idx] = append(ret[idx], a.Digests...)
		case TRACE_INDEX:
			ret[idx] = append(ret[idx], a.Traces...)
		case IGNORE_INDEX:
			ret[idx] = append(ret[idx], a.Ignores...)
		case TEST_INDEX:
			ret[idx] = append(ret[idx], a.TestNames...)
		}
	}
	return ret
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

const (
	// Bucket names in boltdb. 'INDEX' in the name indicates an index.
	ISSUES_DB    = "issues"
	DIGEST_INDEX = "digest"
	TRACE_INDEX  = "trace"
	IGNORE_INDEX = "ignore"
	TEST_INDEX   = "test"
)

// Separator used to separate child and parent id in indices.
const IDX_SEPARATOR = "|"

// boltIssueStore implements the IssueStore interface.
type boltIssueStore struct {
	store *boltutil.IndexedBucket
}

// New returns a new instance of IssueStore that is stored in the given directory.
func New(baseDir string) (IssueStore, error) {
	baseDir, err := fileutil.EnsureDirExists(baseDir)
	if err != nil {
		return nil, err
	}

	db, err := bolt.Open(path.Join(baseDir, "issuestore.db"), 0600, nil)
	if err != nil {
		return nil, err
	}

	config := &boltutil.Config{
		DB:      db,
		Name:    "issues",
		Indices: annotationIndices,
		Codec:   util.JSONCodec(&Annotation{}),
	}

	store, err := boltutil.NewIndexedBucket(config)
	if err != nil {
		return nil, err
	}

	return &boltIssueStore{
		store: store,
	}, nil
}

// ByDigest, see IgnoreStore interface.
func (b *boltIssueStore) ByDigest(digest string) ([]string, error) {
	return b.readFromIndex(DIGEST_INDEX, digest)
}

// ByIgnore, see IgnoreStore interface.
func (b *boltIssueStore) ByIgnore(ignoreID string) ([]string, error) {
	return b.readFromIndex(IGNORE_INDEX, ignoreID)
}

// ByTrace, see IgnoreStore interface.
func (b *boltIssueStore) ByTrace(traceID string) ([]string, error) {
	return b.readFromIndex(TRACE_INDEX, traceID)
}

// ByTest, see IgnoreStore interface.
func (b *boltIssueStore) ByTest(testName string) ([]string, error) {
	return b.readFromIndex(TEST_INDEX, testName)
}

// Get, see IgnoreStore interface.
func (b *boltIssueStore) Get(issueIDs []string) ([]*Annotation, error) {
	if len(issueIDs) == 0 {
		return []*Annotation{}, nil
	}

	result, err := b.store.Read(issueIDs)
	if err != nil {
		return nil, err
	}

	ret := make([]*Annotation, len(result))
	for i, val := range result {
		ret[i] = val.(*Annotation)
	}
	return ret, nil
}

// Add, see IgnoreStore interface.
func (b *boltIssueStore) Add(delta *Annotation) error {
	if delta.IsEmpty() {
		return nil
	}

	writeFn := func(tx *bolt.Tx, result []boltutil.Record) error {
		if result[0] != nil {
			// If there a no change then don't write any records.
			if !result[0].(*Annotation).Add(delta) {
				result[0] = nil
			}
		} else {
			result[0] = delta
		}
		return nil
	}
	return b.store.Update([]boltutil.Record{delta}, writeFn)
}

// List, see IgnoreStore interface.
func (b *boltIssueStore) List(offset int, size int) ([]*Annotation, int, error) {
	result, total, err := b.store.List(offset, size)
	if err != nil {
		return nil, 0, err
	}
	ret := make([]*Annotation, len(result))
	for i, rec := range result {
		ret[i] = rec.(*Annotation)
	}
	return ret, total, nil
}

// Subtract, see IgnoreStore interface.
func (b *boltIssueStore) Subtract(delta *Annotation) error {
	writeFn := func(tx *bolt.Tx, result []boltutil.Record) error {
		found := result[0]
		if found != nil {
			rec := found.(*Annotation)
			// Subtract the delta and only take action if there was a change.
			if rec.Subtract(delta) {
				// If the resulting record is not empty, then we write it to disk.
				if !rec.IsEmpty() {
					return nil
				}

				// Delete the empty record.
				if err := b.store.DeleteTx(tx, []string{rec.IssueID}); err != nil {
					return err
				}
			}
		}
		result[0] = nil
		return nil
	}
	return b.store.Update([]boltutil.Record{delta}, writeFn)
}

// Delete, see IgnoreStore interface.
func (b *boltIssueStore) Delete(issueIDs []string) error {
	return b.store.Delete(issueIDs)
}

// readFromIndex does a lookup in the given index for the given value and
// returns all primary keys that match.
func (b *boltIssueStore) readFromIndex(index, value string) ([]string, error) {
	ret, err := b.store.ReadIndex(index, []string{value})
	if err != nil {
		return nil, err
	}
	return ret[value], nil
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
