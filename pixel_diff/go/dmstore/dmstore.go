package dmstore

import (
	"encoding/json"
	"path"

	"github.com/boltdb/bolt"

	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/diff"
)

// DMStore contains a DiffStore to calculate diff metrics and a boltDB instance
// to store them.
type DMStore struct {
	ds  diff.DiffStore
	db  *bolt.DB
}

// Global DMStore referenced by silverProcessor.
var Default *DMStore

// Init() must be called before ingestion is started so that Default can be
// initialized properly and be ready for use by the processor.
func Init(ds diff.DiffStore, boltdir, boltname string) {
	if Default != nil {
		sklog.Fatalf("dmstore should only be initialized once.")
	}
	var err error
	Default, err = NewDMStore(ds, boltdir, boltname)
	if err != nil {
		sklog.Fatalf("Failed to initialize dmstore: %s", err)
	}
}

// NewDMStore creates a new DMStore with the given DiffStore, and names for the
// boltDB directory and instance.
func NewDMStore(ds diff.DiffStore, boltdir, boltname string) (*DMStore, error) {
	boltdir, err := fileutil.EnsureDirExists(boltdir)
	if err != nil {
		return nil, err
	}

	db, err := bolt.Open(path.Join(boltdir, boltname), 0600, nil)
	if err != nil {
		return nil, err
	}

	return &DMStore {
		ds: ds,
		db: db,
	}, nil
}

// Add uses the given runID and filename to get the paths of the nopatch and
// withpatch images in GS, then uses the DiffStore to calculate diff metrics.
// The metrics are serialized and put into the boltDB instance.
func (d *DMStore) Add(runID, filename string) error {
	nopatchImg, withpatchImg := getNoAndWithPatch(runID, filename)
	diff, err := d.ds.Get(diff.PRIORITY_NOW, nopatchImg, []string{withpatchImg})
	if err != nil {
		return err
	}

	diffMetrics := diff[withpatchImg]
	err = d.db.Update(func(tx *bolt.Tx) error {
		// Create bucket using runID.
		b, err := tx.CreateBucketIfNotExists([]byte(runID))
		if err != nil {
			return err
		}

		encoded, err := json.Marshal(diffMetrics)
		if err != nil {
			return err
		}

		if err := b.Put([]byte(filename), encoded); err != nil {
			return err
		}
		return nil
	})
	return err
}

// Remove deletes the bucket specified by runID from the boltDB instance.
func (d *DMStore) Remove(runID string) error {
	err := d.db.Update(func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket([]byte(runID)); err != nil {
			return err
		}
		return nil
	})
	return err
}

// Returns paths of nopatch and withpatch images.
func getNoAndWithPatch(runID, filename string) (string, string) {
	return runID + "/nopatch/" + filename, runID + "/withpatch/" + filename
}
