package resultstore

import (
	"encoding/json"
	"path"

	"github.com/boltdb/bolt"

	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/golden/go/diff"
)

// ResultRec defines the struct stored in ResultStore that can be queried over
// the web.
type ResultRec struct {
	// RunID is the unique ID of the CT pixel diff run, in the form
	// userID-timestamp.
	RunID string

	// URL identifies the web page that was screenshotted.
	URL string

	// Rank is the popularity rank of the web page.
	Rank int

	// NoPatchImg is the imageID of the screenshot taken without the page.
	NoPatchImg string

	// WithPatchImg is the imageID of the screenshot taken with the patch.
	WithPatchImg string

	// DiffMetrics are the results of diffing NoPatchImg and WithPatchImg.
	DiffMetrics *diff.DiffMetrics
}

// IsReadyForDiff checks if both the NoPatchImg and WithPatchImg for the
// ResultRec have been processed.
func (r *ResultRec) IsReadyForDiff() bool {
	return r.NoPatchImg != "" && r.WithPatchImg != ""
}

// ResultStore is an interface for storing results extracted from Cluster
// Telemetry Pixel Diff JSON metadata.
type ResultStore interface {
	// Get returns a ResultRec from the ResultStore using the runID and url.
	Get(runID, url string) (*ResultRec, error)

	// GetAll returns all the ResultRecs associated with the runID.
	GetAll(runID string) ([]*ResultRec, error)

	// Add adds a ResultRec to the ResultStore using the runID and url.
	Add(runID, url string, rec *ResultRec) error

	// RemoveRun removes all the data associated with the runID from the
	// ResultStore.
	RemoveRun(runID string) error
}

// BoltResultStore implements the ResultStore interface with a boltDB instance.
type BoltResultStore struct {
	db *bolt.DB
}

// NewBoltResultStore returns a new instance of BoltResultStore, using the given
// boltDir and boltName to create the boltDB instance.
func NewBoltResultStore(boltDir, boltName string) (ResultStore, error) {
	// Make sure directory for boltDB instance exists.
	boltDir, err := fileutil.EnsureDirExists(boltDir)
	if err != nil {
		return nil, err
	}

	// Create the boltDB instance.
	db, err := bolt.Open(path.Join(boltDir, boltName), 0600, nil)
	if err != nil {
		return nil, err
	}

	return &BoltResultStore{
		db: db,
	}, nil
}

// Get uses the given runID to specify the storage bucket within the boltDB.
// Then, it uses the url as the key to get the serialized ResultRec, and returns
// it after decoding.
func (b *BoltResultStore) Get(runID, url string) (*ResultRec, error) {
	rec := &ResultRec{}
	viewFn := func(tx *bolt.Tx) error {
		// Retrieve bucket using the runID. If the bucket doesn't exist, return nil.
		b := tx.Bucket([]byte(runID))
		if b == nil {
			rec = nil
			return nil
		}

		// Get the serialized ResultRec and decode it. If an entry doesn't exist
		// for a given url, return nil.
		bytes := b.Get([]byte(url))
		if bytes == nil {
			rec = nil
		} else {
			if err := json.Unmarshal(bytes, &rec); err != nil {
				return err
			}
		}

		return nil
	}

	err := b.db.View(viewFn)
	if err != nil {
		return nil, err
	}

	return rec, nil
}

// GetAll returns all the ResultRecs stored in the bucket associated with the
// given runID.
func (b *BoltResultStore) GetAll(runID string) ([]*ResultRec, error) {
	recs := make([]*ResultRec, 0)
	viewFn := func(tx *bolt.Tx) error {
		// Retrieve bucket using the runID. If the bucket doesn't exist, return nil.
		b := tx.Bucket([]byte(runID))
		if b == nil {
			recs = nil
			return nil
		}

		// Iterate through all the entries in the bucket, deserialize the values,
		// and append them to the list.
		err := b.ForEach(func(k, v []byte) error {
			rec := &ResultRec{}
			if err := json.Unmarshal(v, &rec); err != nil {
				return err
			}
			recs = append(recs, rec)
			return nil
		})
		if err != nil {
			return err
		}

		return nil
	}

	err := b.db.View(viewFn)
	if err != nil {
		return nil, err
	}

	return recs, nil
}

// Add uses the given runID to specify the storage bucket within the boltDB.
// Then, the ResultRec is encoded and put into the database with the url of
// the screenshots as the key. If a record already exists for the given runID
// and url, it is overwritten.
func (b *BoltResultStore) Add(runID, url string, rec *ResultRec) error {
	updateFn := func(tx *bolt.Tx) error {
		// Create or retrieve bucket using the runID.
		b, err := tx.CreateBucketIfNotExists([]byte(runID))
		if err != nil {
			return err
		}

		// Serialize the ResultRec.
		encoded, err := json.Marshal(rec)
		if err != nil {
			return err
		}

		// Put the record in the runID bucket: key = url, value = ResultRec
		if err := b.Put([]byte(url), encoded); err != nil {
			return err
		}

		return nil
	}

	return b.db.Update(updateFn)
}

// RemoveRun deletes the bucket specified by the runID from the boltDB instance.
func (b *BoltResultStore) RemoveRun(runID string) error {
	updateFn := func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket([]byte(runID)); err != nil {
			return err
		}
		return nil
	}

	return b.db.Update(updateFn)
}
