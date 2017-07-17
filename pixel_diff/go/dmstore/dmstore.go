package dmstore

import (
	"encoding/json"
	"path"

	"github.com/boltdb/bolt"

	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/golden/go/diff"
)

// DMStore contains a boltDB instance to store diff metrics.
type DMStore struct {
	diffs *bolt.DB
}

func NewDMStore(boltdir, boltname string) (*DMStore, error) {
	// Make sure directory for diffs boltDB instance exists.
	boltdir, err := fileutil.EnsureDirExists(boltdir)
	if err != nil {
		return nil, err
	}

	// Create the diffs boltDB instance.
	diffs, err := bolt.Open(path.Join(boltdir, boltname), 0600, nil)
	if err != nil {
		return nil, err
	}

	return &DMStore {
		diffs: diffs,
	}, nil
}

// Add uses the given runID to specify the storage bucket within the diffs
// boltDB. Then, the diff metrics are encoded and put into diffs with the URL
// filename of the nopatch and withpatch images as the key.
func (d *DMStore) Add(runID, filename string, diffMetrics *diff.DiffMetrics) error {
	updateFn := func(tx *bolt.Tx) error {
		// Create bucket using the runID.
		b, err := tx.CreateBucketIfNotExists([]byte(runID))
		if err != nil {
			return err
		}

		// Serialize the diff metrics.
		encoded, err := json.Marshal(diffMetrics)
		if err != nil {
			return err
		}

		// Records in bucket: key = URL, value = diff metrics
		if err := b.Put([]byte(filename), encoded); err != nil {
			return err
		}

		return nil
	}

	return d.diffs.Update(updateFn)
}

// Remove deletes the bucket specified by the runID from the diffs boltDB
// instance. Used by the pixel diff server to delete run data.
func (d *DMStore) Remove(runID string) error {
	updateFn := func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket([]byte(runID)); err != nil {
			return err
		}
		return nil
	}

	return d.diffs.Update(updateFn)
}
