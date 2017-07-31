package resultstore

import (
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/boltdb/bolt"

	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/golden/go/diff"
)

const (
	NUM_DIFF     = "numDiff"
	PERCENT_DIFF = "percentDiff"
	RED_DIFF     = "redDiff"
	GREEN_DIFF   = "greenDiff"
	BLUE_DIFF    = "blueDiff"
	RANK         = "rank"

	ASC = "ascending"
	DSC = "descending"
)

var (
	// Used as start time in order to return all runs in GetRunIDs.
	BeginningOfTime = time.Date(2015, time.January, 02, 15, 04, 05, 0, time.UTC)
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

// HasBothImages checks if both the NoPatchImg and WithPatchImg for the
// ResultRec have been processed.
func (r *ResultRec) HasBothImages() bool {
	return r.NoPatchImg != "" && r.WithPatchImg != ""
}

// ResultStore is an interface for storing results extracted from Cluster
// Telemetry Pixel Diff JSON metadata.
type ResultStore interface {
	// Get returns a ResultRec from the ResultStore using the runID and url.
	Get(runID, url string) (*ResultRec, error)

	// GetAll returns all the ResultRecs associated with the runID.
	GetAll(runID string) ([]*ResultRec, error)

	// GetRunIDs returns all the runIDs in the database that fall in between the
	// start and end times.
	GetRunIDs(start time.Time, end time.Time) ([]string, error)

	// Put adds a ResultRec to the ResultStore using the runID and url.
	Put(runID, url string, rec *ResultRec) error

	// RemoveRun removes all the data associated with the runID from the
	// ResultStore.
	RemoveRun(runID string) error

	// GetRange returns cached results in the given range for the given runID.
	GetRange(runID string, startIdx, endIdx int) ([]*ResultRec, error)

	// SortRun sorts the cached results for the given runID using the sort
	// parameters.
	SortRun(runID, sortField, sortOrder string) error
}

// BoltResultStore implements the ResultStore interface with a boltDB instance.
type BoltResultStore struct {
	db *bolt.DB

	// Map of runIDs to list of ResultRecs, used to cache and sort entries that
	// contain both nopatch and withpatch images.
	cache map[string][]*ResultRec
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

	// Instantiate the cache.
	cache := map[string][]*ResultRec{}

	b := &BoltResultStore{
		db:    db,
		cache: cache,
	}

	// Fill the cache.
	if err = b.fillCache(); err != nil {
		return nil, err
	}
	return b, nil
}

// Fills the cache with the data in the boltDB instance. This is to ensure that
// the data in the boltDB and cache are consistent with each other even after
// a server crash or reboot, as the cache will be erased while the data in the
// boltDB will not.
func (b *BoltResultStore) fillCache() error {
	runIDs, err := b.GetRunIDs(BeginningOfTime, time.Now())
	if err != nil {
		return err
	}

	for _, runID := range runIDs {
		results, err := b.GetAll(runID)
		if err != nil {
			return err
		}
		b.cache[runID] = results
	}

	return nil
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
	recs := []*ResultRec{}
	viewFn := func(tx *bolt.Tx) error {
		// Retrieve bucket using the runID. If the bucket doesn't exist, returns an
		// empty list.
		b := tx.Bucket([]byte(runID))
		if b == nil {
			return nil
		}

		// Iterate through all the entries in the bucket, deserialize the values,
		// and append them to the list if they are complete.
		err := b.ForEach(func(k, v []byte) error {
			rec := &ResultRec{}
			if err := json.Unmarshal(v, &rec); err != nil {
				return err
			}
			if rec.HasBothImages() {
				recs = append(recs, rec)
			}
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

// GetRunIDs returns the IDs of all the runs that were completed in the given
// time range. If this func is called with the parameters
// resultstore.BeginningOfTime and time.Now(), all runIDs in the database are
// returned.
func (b *BoltResultStore) GetRunIDs(start, end time.Time) ([]string, error) {
	runIDs := []string{}
	viewFn := func(tx *bolt.Tx) error {
		// Iterate through each bucket name and create a Time struct using the
		// timestamp in the runID.
		err := tx.ForEach(func(name []byte, _ *bolt.Bucket) error {
			runID := string(name)
			timestamp := strings.Split(runID, "-")[1]
			runTime, err := time.Parse(util.TS_FORMAT, timestamp)
			if err != nil {
				return err
			}

			// Append the runID to the list if it falls in the specified range.
			if start.Before(runTime) && end.After(runTime) {
				runIDs = append(runIDs, runID)
			}

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

	return runIDs, nil
}

// Put uses the given runID to specify the storage bucket within the boltDB.
// Then, the ResultRec is encoded and put into the database with the url of
// the screenshots as the key. If a record already exists for the given runID
// and url, it is overwritten. If the update succeeds and the ResultRec has
// both images processed, it is also added to the cache.
func (b *BoltResultStore) Put(runID, url string, rec *ResultRec) error {
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

	err := b.db.Update(updateFn)
	if err != nil {
		return err
	}

	// Add the ResultRec to the cache.
	if rec.HasBothImages() {
		if results, ok := b.cache[runID]; ok {
			results = append(results, rec)
			b.cache[runID] = results
		} else {
			results = []*ResultRec{rec}
			b.cache[runID] = results
		}
	}
	return nil
}

// RemoveRun deletes the bucket specified by the runID from the boltDB instance.
// If the remove succeeds, the runID is also removed from the cache.
func (b *BoltResultStore) RemoveRun(runID string) error {
	updateFn := func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket([]byte(runID)); err != nil {
			return err
		}
		return nil
	}

	err := b.db.Update(updateFn)
	if err != nil {
		return err
	}

	// Remove the runID from the cache after verifying it's there.
	if _, ok := b.cache[runID]; ok {
		delete(b.cache, runID)
	}
	return nil
}

// GetRange returns all the ResultRecs in the specified range [startIdx,endIdx)
// for the given runID from the cache. Returns an empty slice if the indices are
// out of bounds and returns an error if there is no data cached for the runID.
func (b *BoltResultStore) GetRange(runID string, startIdx, endIdx int) ([]*ResultRec, error) {
	if results, ok := b.cache[runID]; ok {
		if startIdx > len(results) {
			startIdx = len(results)
		}

		if endIdx > len(results) {
			endIdx = len(results)
		}

		return results[startIdx:endIdx], nil
	} else {
		return nil, fmt.Errorf("No cached results for run %s", runID)
	}
}

// SortRun sorts the cached ResultRecs for the given runID using the given sort
// parameter and sort order (ascending/descending). Returns an error if there
// is no data cached for the runID.
func (b *BoltResultStore) SortRun(runID, sortField, sortOrder string) error {
	if results, ok := b.cache[runID]; ok {
		switch sortField {
		// The ResultRecs are sorted by URL if they have equal values for the sort
		// parameter.
		case NUM_DIFF:
			sortByNumDiffPixels(results, sortOrder)
		case PERCENT_DIFF:
			sortByPercentDiffPixels(results, sortOrder)
		case RED_DIFF:
			sortByRedDiff(results, sortOrder)
		case GREEN_DIFF:
			sortByGreenDiff(results, sortOrder)
		case BLUE_DIFF:
			sortByBlueDiff(results, sortOrder)
		case RANK:
			sortByRank(results, sortOrder)
		}
		return nil
	} else {
		return fmt.Errorf("No cached results for run %s", runID)
	}
}

func sortByNumDiffPixels(results []*ResultRec, sortOrder string) {
	sort.Slice(results, func(i, j int) bool {
		if results[i].DiffMetrics.NumDiffPixels == results[j].DiffMetrics.NumDiffPixels {
			return results[i].URL < results[j].URL
		}
		switch sortOrder {
		case ASC:
			return results[i].DiffMetrics.NumDiffPixels < results[j].DiffMetrics.NumDiffPixels
		case DSC:
			return results[i].DiffMetrics.NumDiffPixels > results[j].DiffMetrics.NumDiffPixels
		default:
			return false
		}
	})
}

func sortByPercentDiffPixels(results []*ResultRec, sortOrder string) {
	sort.Slice(results, func(i, j int) bool {
		if results[i].DiffMetrics.PixelDiffPercent == results[j].DiffMetrics.PixelDiffPercent {
			return results[i].URL < results[j].URL
		}
		switch sortOrder {
		case ASC:
			return results[i].DiffMetrics.PixelDiffPercent < results[j].DiffMetrics.PixelDiffPercent
		case DSC:
			return results[i].DiffMetrics.PixelDiffPercent > results[j].DiffMetrics.PixelDiffPercent
		default:
			return false
		}
	})
}

func sortByRedDiff(results []*ResultRec, sortOrder string) {
	sort.Slice(results, func(i, j int) bool {
		if results[i].DiffMetrics.MaxRGBADiffs[0] == results[j].DiffMetrics.MaxRGBADiffs[0] {
			return results[i].URL < results[j].URL
		}
		switch sortOrder {
		case ASC:
			return results[i].DiffMetrics.MaxRGBADiffs[0] < results[j].DiffMetrics.MaxRGBADiffs[0]
		case DSC:
			return results[i].DiffMetrics.MaxRGBADiffs[0] > results[j].DiffMetrics.MaxRGBADiffs[0]
		default:
			return false
		}
	})
}

func sortByGreenDiff(results []*ResultRec, sortOrder string) {
	sort.Slice(results, func(i, j int) bool {
		if results[i].DiffMetrics.MaxRGBADiffs[0] == results[j].DiffMetrics.MaxRGBADiffs[0] {
			return results[i].URL < results[j].URL
		}
		switch sortOrder {
		case ASC:
			return results[i].DiffMetrics.MaxRGBADiffs[1] < results[j].DiffMetrics.MaxRGBADiffs[1]
		case DSC:
			return results[i].DiffMetrics.MaxRGBADiffs[1] > results[j].DiffMetrics.MaxRGBADiffs[1]
		default:
			return false
		}
	})
}

func sortByBlueDiff(results []*ResultRec, sortOrder string) {
	sort.Slice(results, func(i, j int) bool {
		if results[i].DiffMetrics.MaxRGBADiffs[2] == results[j].DiffMetrics.MaxRGBADiffs[2] {
			return results[i].URL < results[j].URL
		}
		switch sortOrder {
		case ASC:
			return results[i].DiffMetrics.MaxRGBADiffs[2] < results[j].DiffMetrics.MaxRGBADiffs[2]
		case DSC:
			return results[i].DiffMetrics.MaxRGBADiffs[2] > results[j].DiffMetrics.MaxRGBADiffs[2]
		default:
			return false
		}
	})
}

func sortByRank(results []*ResultRec, sortOrder string) {
	sort.Slice(results, func(i, j int) bool {
		switch sortOrder {
		case ASC:
			return results[i].Rank > results[j].Rank
		case DSC:
			return results[i].Rank < results[j].Rank
		default:
			return false
		}
	})
}
