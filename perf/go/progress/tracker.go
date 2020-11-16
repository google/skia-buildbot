package progress

import (
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	lru "github.com/hashicorp/golang-lru"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

var timeNow = time.Now

// Tracker keeps track of long running processes.
//
// It will cache Progresses for a time after they complete.
type Tracker interface {
	// Add a Progress to the tracker. This will update the URL of the Progress.
	Add(prog Progress)

	// Handler for HTTP requests for Progress updates.
	Handler(w http.ResponseWriter, r *http.Request)
}

// cacheDuration is how long to cache a Progress after it completes, regardless of success.
const cacheDuration = 5 * time.Minute

// cacheUpdatePeriod is how often we scan the cache for finished or exired entries.
const cacheUpdatePeriod = time.Minute

// cacheSize is the size of the lru cache.
const cacheSize = 1000

// tracker implements Tracker.
type tracker struct {
	cache    *lru.Cache
	basePath string

	// metrics
	numEntriesInCache metrics2.Int64Metric
}

// cacheEntry is a single entry in the tracker lru cache.
type cacheEntry struct {
	Progress Progress
	Finished time.Time
}

// NewTracker returns a new Tracker instance.
//
// The basePath is the base of the URL path that Progress results will be served
// from. It must end in a '/' and will have the Progress id appended to it for
// each Progress. The tracker.Handler() must be set up to receive all requests
// for that basePath.
//
// Example:
//
//    // During init:
//    singleTrackerInstance := progress.NewTracker("/_/status/")
//    router.HandleFunc("/_/status/{id:.+}", t.Handler).Methods("GET")
//
// Then in any http handler that starts a long running progress:
//
//    prog := StartNewLongRunningProcess()
//    singleTrackerInstance.Add(prog)
//    if err := prog.JSON(w); err != nil {
//      sklog.Error(err)
//    }
//
// The serialized Progress contains the URL to make requests back to the app to
// query the status of the long running process, which will contain the final
// result when the long running process completes.
func NewTracker(basePath string) (*tracker, error) {
	cache, err := lru.New(cacheSize)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create tracker cache.")
	}
	if !strings.HasSuffix(basePath, "/") {
		return nil, skerr.Fmt("basePath %q must end with a '/'", basePath)
	}

	ret := &tracker{
		cache:             cache,
		basePath:          basePath,
		numEntriesInCache: metrics2.GetInt64Metric("perf_progress_tracker_num_entries_in_cache"),
	}

	// Start the periodic cleanup task.
	go func() {
		for range time.Tick(cacheUpdatePeriod) {
			ret.singleStep()
		}
	}()

	return ret, nil
}

func (t *tracker) get(key string) (*cacheEntry, bool) {
	iCacheEntry, ok := t.cache.Get(key)
	if !ok {
		return nil, false
	}
	ret, ok := iCacheEntry.(*cacheEntry)
	return ret, ok
}

//  singleStep does a single step in the cache cleanup progress.
func (t *tracker) singleStep() {
	now := timeNow()
	for _, key := range t.cache.Keys() {
		entry, ok := t.get(key.(string))
		if !ok {
			continue
		}
		// Remove cache entries that are old enough.
		if !entry.Finished.IsZero() && entry.Finished.Add(cacheDuration).Before(now) {
			t.cache.Remove(key)
			continue
		}
		// Record when a Progress has finished.
		if entry.Finished.IsZero() && entry.Progress.Status() != Running {
			entry.Finished = now
		}
	}
	t.numEntriesInCache.Update(int64(len(t.cache.Keys())))
}

// Handler implements Tracker.
func (t *tracker) Add(prog Progress) {
	id := uuid.Must(uuid.NewRandom()).String()
	prog.URL(t.basePath + id)
	t.cache.Add(id, &cacheEntry{
		prog,
		time.Time{},
	})
}

// Handler implements Tracker.
func (t *tracker) Handler(w http.ResponseWriter, r *http.Request) {
	// The id is always the last part of the path.
	id := path.Base(r.URL.Path)

	entry, ok := t.get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := entry.Progress.JSON(w); err != nil {
		http.Error(w, "Failed to serialize JSON", http.StatusInternalServerError)
		sklog.Errorf("Failed to encode Progress results: %s", err)
	}
}

// Assert that *tracker implements the Tracker interface.
var _ Tracker = (*tracker)(nil)
