package resultstore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/ct_pixel_diff/go/common"
	"go.skia.org/infra/ct_pixel_diff/go/dynamicdiff"
	"go.skia.org/infra/go/fileutil"
)

const (
	// Sort parameter constants.
	NUM_DIFF     = "numDiff"
	PERCENT_DIFF = "percentDiff"
	RED_DIFF     = "redDiff"
	GREEN_DIFF   = "greenDiff"
	BLUE_DIFF    = "blueDiff"
	RANK         = "rank"
	DSC          = "descending"

	// URL search constants.
	HTTP  = "http://"
	HTTPS = "https://"
	WWW   = "www."
	TEXT  = "text"
	VALUE = "value"

	// Constants used to report statistics.
	BUCKET_0            = "[0-10)"
	BUCKET_1            = "[10-20)"
	BUCKET_2            = "[20-30)"
	BUCKET_3            = "[30-40)"
	BUCKET_4            = "[40-50)"
	BUCKET_5            = "[50-60)"
	BUCKET_6            = "[60-70)"
	BUCKET_7            = "[70-80)"
	BUCKET_8            = "[80-90)"
	BUCKET_9            = "[90-100]"
	NUM_TOTAL_RESULTS   = "numTotalResults"
	NUM_DYNAMIC_CONTENT = "numDynamicContent"
	NUM_ZERO_DIFF       = "numZeroDiff"

	// Number of ResultRec instances to render in one render request.
	CHUNK_SIZE = 20
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
	NoPatchImg common.ImageID

	// WithPatchImg is the imageID of the screenshot taken with the patch.
	WithPatchImg common.ImageID

	// DiffMetrics are the results of diffing NoPatchImg and WithPatchImg.
	DiffMetrics *dynamicdiff.DynamicDiffMetrics
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

	// GetFiltered returns cached results within the given bounds for the given runID.
	GetFiltered(runID string, startIdx int, min float32, max float32) ([]*ResultRec, int, error)

	// SortRun sorts the cached results for the given runID using the sort
	// parameters.
	SortRun(runID, sortField, sortOrder string) error

	// GetURLs returns the URLs of all cached results for the given runID.
	GetURLs(runID string) ([]map[string]string, error)

	// GetStats returns various statistics and a histogram using cached results
	// for the given runID.
	GetStats(runID string) (map[string]int, map[string]int, error)
}

// BoltResultStore implements the ResultStore interface with a boltDB instance.
type BoltResultStore struct {
	db *bolt.DB

	// Map of runIDs to list of ResultRecs. The cache only contains entries that
	// have computed diff metrics.
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

// GetAll returns all the ResultRecs containing diff metrics that are stored in
// the bucket associated with the given runID.
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
		// and append them to the list if they contain diff metrics.
		err := b.ForEach(func(k, v []byte) error {
			rec := &ResultRec{}
			if err := json.Unmarshal(v, &rec); err != nil {
				return err
			}
			if rec.DiffMetrics != nil {
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
// diff metrics, it is also added to the cache.
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

	// Add the ResultRec to the cache if it has diff metrics.
	if rec.DiffMetrics != nil {
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

// GetFiltered returns at most CHUNK_SIZE ResultRecs whose PixelDiffPercent is
// greater than or equal to the given min and less than or equal to the given
// max. The filtering starts at the given start index for the runID's cached
// results. Also returns the next index so that the frontend knows where to
// start its next filter query in the cache. Returns an error if there is no
// data cached for the runID.
func (b *BoltResultStore) GetFiltered(runID string, startIdx int, min float32, max float32) ([]*ResultRec, int, error) {
	results, ok := b.cache[runID]
	if !ok {
		return nil, -1, fmt.Errorf("No cached results for run %s", runID)
	}

	ret := []*ResultRec{}
	i := 0
	for i < CHUNK_SIZE {
		if startIdx >= len(results) {
			return ret, len(results), nil
		}
		percent := results[startIdx].DiffMetrics.PixelDiffPercent
		if min <= percent && percent <= max {
			ret = append(ret, results[startIdx])
			i++
		}
		startIdx++
	}
	return ret, startIdx, nil
}

// SortRun sorts the cached ResultRecs for the given runID using the given sort
// parameter and sort order (ascending/descending). Returns an error if there
// is no data cached for the runID.
func (b *BoltResultStore) SortRun(runID, sortField, sortOrder string) error {
	results, ok := b.cache[runID]
	if !ok {
		return fmt.Errorf("No cached results for run %s", runID)
	}

	var lessFn resultRecLessFn
	switch sortField {
	// The ResultRecs are sorted by URL if they have equal values for the sort
	// parameter.
	case NUM_DIFF:
		lessFn = sortByNumDiffPixels
	case PERCENT_DIFF:
		lessFn = sortByPercentDiffPixels
	case RED_DIFF:
		lessFn = sortByMaxRedDiff
	case GREEN_DIFF:
		lessFn = sortByMaxGreenDiff
	case BLUE_DIFF:
		lessFn = sortByMaxBlueDiff
	case RANK:
		lessFn = sortByRank
	}
	sortSlice := sort.Interface(newResultRecSlice(results, lessFn))
	if sortOrder == DSC {
		sortSlice = sort.Reverse(sortSlice)
	}
	sort.Sort(sortSlice)
	return nil
}

// Function signature for a ResultRec comparator.
type resultRecLessFn func(r *resultRecSlice, i, j int) bool

// resultRecSlice wraps around a list of ResultRec instances and implements
// sort.Interface.
type resultRecSlice struct {
	lessFn resultRecLessFn
	data   []*ResultRec
}

// Constructor takes in a slice of ResultRec instances and a custom less
// function that is called during sorting.
func newResultRecSlice(data []*ResultRec, lessFn resultRecLessFn) *resultRecSlice {
	return &resultRecSlice{lessFn: lessFn, data: data}
}

// Implementation of sort.Interface.
func (r *resultRecSlice) Len() int           { return len(r.data) }
func (r *resultRecSlice) Less(i, j int) bool { return r.lessFn(r, i, j) }
func (r *resultRecSlice) Swap(i, j int)      { r.data[i], r.data[j] = r.data[j], r.data[i] }

// Sorts the slice using the number of different pixels.
func sortByNumDiffPixels(r *resultRecSlice, i, j int) bool {
	left := r.data[i].DiffMetrics.NumDiffPixels
	right := r.data[j].DiffMetrics.NumDiffPixels
	if left == right {
		return r.data[i].URL < r.data[j].URL
	}
	return left < right
}

// Sorts the slice using the percentage of different pixels.
func sortByPercentDiffPixels(r *resultRecSlice, i, j int) bool {
	left := r.data[i].DiffMetrics.PixelDiffPercent
	right := r.data[j].DiffMetrics.PixelDiffPercent
	if left == right {
		return r.data[i].URL < r.data[j].URL
	}
	return left < right
}

// Sorts the slice using the maximum red difference.
func sortByMaxRedDiff(r *resultRecSlice, i, j int) bool {
	left := r.data[i].DiffMetrics.MaxRGBDiffs[0]
	right := r.data[j].DiffMetrics.MaxRGBDiffs[0]
	if left == right {
		return r.data[i].URL < r.data[j].URL
	}
	return left < right
}

// Sorts the slice using the maximum green difference.
func sortByMaxGreenDiff(r *resultRecSlice, i, j int) bool {
	left := r.data[i].DiffMetrics.MaxRGBDiffs[1]
	right := r.data[j].DiffMetrics.MaxRGBDiffs[1]
	if left == right {
		return r.data[i].URL < r.data[j].URL
	}
	return left < right
}

// Sorts the slice using the maximum blue difference.
func sortByMaxBlueDiff(r *resultRecSlice, i, j int) bool {
	left := r.data[i].DiffMetrics.MaxRGBDiffs[2]
	right := r.data[j].DiffMetrics.MaxRGBDiffs[2]
	if left == right {
		return r.data[i].URL < r.data[j].URL
	}
	return left < right
}

// Sorts the slice using the site popularity rank. Two ResultRec instances
// within the same slice will never have the same rank, so there is no need for
// an equality check.
func sortByRank(r *resultRecSlice, i, j int) bool {
	return r.data[i].Rank > r.data[j].Rank
}

// GetURLs returns the urls of the cached results for the given runID. Urls are
// returned as map[string]string objects, where the entries are as follows:
// "text":URL stripped of prefixes, "value":stripped prefixes. These fields are
// required by the frontend element responsible for making url suggestions.
// Returns an error if there is no data cached for the runID.
func (b *BoltResultStore) GetURLs(runID string) ([]map[string]string, error) {
	results, ok := b.cache[runID]
	if !ok {
		return nil, fmt.Errorf("No cached results for run %s", runID)
	}

	urls := []map[string]string{}
	for _, result := range results {
		url := map[string]string{}
		var prefix bytes.Buffer
		formattedURL := result.URL

		// Strips either "http://" or "https://"
		if strings.HasPrefix(formattedURL, HTTP) {
			prefix.WriteString(HTTP)
			formattedURL = strings.TrimPrefix(formattedURL, HTTP)
		} else if strings.HasPrefix(formattedURL, HTTPS) {
			prefix.WriteString(HTTPS)
			formattedURL = strings.TrimPrefix(formattedURL, HTTPS)
		}

		// Strips "www." prefix
		if strings.HasPrefix(formattedURL, WWW) {
			prefix.WriteString(WWW)
			formattedURL = strings.TrimPrefix(formattedURL, WWW)
		}

		url[TEXT] = formattedURL
		url[VALUE] = prefix.String()
		urls = append(urls, url)
	}
	return urls, nil
}

// GetStats returns two maps, one that contains basic count statistics such as
// the total number of diff results and number of results with dynamic content,
// and another that represents a histogram of pixel diff percentages. Returns an
// error if there is no data cached for the runID.
func (b *BoltResultStore) GetStats(runID string) (map[string]int, map[string]int, error) {
	results, ok := b.cache[runID]
	if !ok {
		return nil, nil, fmt.Errorf("No cached results for run %s", runID)
	}

	stats := map[string]int{
		NUM_TOTAL_RESULTS:   0,
		NUM_DYNAMIC_CONTENT: 0,
		NUM_ZERO_DIFF:       0,
	}
	histogram := map[string]int{
		BUCKET_0: 0,
		BUCKET_1: 0,
		BUCKET_2: 0,
		BUCKET_3: 0,
		BUCKET_4: 0,
		BUCKET_5: 0,
		BUCKET_6: 0,
		BUCKET_7: 0,
		BUCKET_8: 0,
		BUCKET_9: 0,
	}
	for _, result := range results {
		// Increment the total number of diff results.
		stats[NUM_TOTAL_RESULTS]++

		// Increment the number of results with dynamic content if the result has
		// at least one dynamic pixel.
		if result.DiffMetrics.NumDynamicPixels > 0 {
			stats[NUM_DYNAMIC_CONTENT]++
		}

		percent := result.DiffMetrics.PixelDiffPercent
		// Place the percent in the correct histogram bin.
		if 0 <= percent && percent < 10 {
			// Increment the number of results with zero diff if the result has
			// a pixel diff percentage of exactly 0%.
			if percent == 0 {
				stats[NUM_ZERO_DIFF]++
			}
			histogram[BUCKET_0]++
		} else if 10 <= percent && percent < 20 {
			histogram[BUCKET_1]++
		} else if 20 <= percent && percent < 30 {
			histogram[BUCKET_2]++
		} else if 30 <= percent && percent < 40 {
			histogram[BUCKET_3]++
		} else if 40 <= percent && percent < 50 {
			histogram[BUCKET_4]++
		} else if 50 <= percent && percent < 60 {
			histogram[BUCKET_5]++
		} else if 60 <= percent && percent < 70 {
			histogram[BUCKET_6]++
		} else if 70 <= percent && percent < 80 {
			histogram[BUCKET_7]++
		} else if 80 <= percent && percent < 90 {
			histogram[BUCKET_8]++
		} else if 90 <= percent && percent <= 100 {
			histogram[BUCKET_9]++
		}
	}
	return stats, histogram, nil
}
