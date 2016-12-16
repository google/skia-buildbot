// Package main provides Store, that keeps tiles in memory and periodically
// flushes them to disk, and processes queries against them.
package store

import (
	"fmt"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/boltdb/bolt"
	lru "github.com/hashicorp/golang-lru"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/ragemon/go/ts"
)

const (
	// TILE_FLUSH_PERIOD controls how frequently tiles are flushed to disk.
	TILE_FLUSH_PERIOD = 15 * time.Minute

	// MAX_CACHED_TILES is how many BoltBD tiles to keep open in the cache.
	MAX_CACHED_TILES = 5

	// TILE_SIZE_IN_SECONDS is the duration that each tile covers.
	TILE_SIZE_IN_SECONDS = 2 * 60 * 60

	// BUCKET_NAME is the name of the BoltDB bucket that timeseries are stored in.
	BUCKET_NAME = "measurements"

	// WRITE_BATCH_SIZE is the number of timeseries we write at any one time to
	// BoltDB while holding the mutex.
	WRITE_BATCH_SIZE = 100
)

// Measurement is a Point and the structured key that identifies it.
type Measurement struct {
	Key   string
	Point ts.Point
}

// MeasurementSlice is a helper for sorting Measurements.
type MeasurementSlice []Measurement

func (p MeasurementSlice) Len() int { return len(p) }
func (p MeasurementSlice) Less(i, j int) bool {
	return p[i].Point.Timestamp < p[j].Point.Timestamp
}
func (p MeasurementSlice) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

// Store is the interface for our Measurement store.
type Store interface {
	// Add the slice of Measurements to the store.
	//
	// Note that the passed in Measurement slice will be changed, it will be
	// sorted by time.
	Add([]Measurement) error

	// Returns the timeseries values between [begin, end) that match the given
	// query.
	Match(begin, end time.Time, q *query.Query) ts.TimeSeriesSet

	// Returns the calculated ParamSet for the tiles in memory.
	ParamSet() paramtools.ParamSet
}

// TileInfo is a single tile of timeseries held in memory.
//
// It represents all the data on disk in BoltDB, plus any new data that's
// arrived.
type TileInfo struct {
	set         ts.TimeSeriesSet
	updatedKeys map[string]bool // Keys updated since the last flush.
	lastNewKey  time.Time       // Used to control query cache.
}

func newTileInfo() *TileInfo {
	return &TileInfo{
		set:         ts.TimeSeriesSet{},
		updatedKeys: map[string]bool{},
	}
}

// StoreImpl implements Store.
//
// The important idea here is that all of time, or at least all of Unix time,
// is broken up into tiles. I.e. All of time from Jan 1, 1970 forward
// is broken up into 2 hour long tiles, with each BoltDB filename, and
// each key in 'tiles', being the index number of one of those tiles.
//
type StoreImpl struct {
	// mutex protects access to tiles, paramSet, and cache.
	mutex sync.Mutex

	// tiles contains three tiles.
	tiles map[int64]*TileInfo

	// paramSet is upadated as keys arrive that have never been seen before.
	paramSet paramtools.ParamSet

	// A cache of recent BoltDB tiles.
	cache *lru.Cache

	// The dir that tiles are written into.
	dir string
}

// boltNameFromIndex returns the BoltDB filename for the given tile index.
func boltNameFromIndex(index int64) string {
	return fmt.Sprintf("%06d.db", index)
}

func getBoltDBNoCache(dir string, index int64) (*bolt.DB, error) {
	filename := filepath.Join(dir, boltNameFromIndex(index))
	return bolt.Open(filename, 0600, &bolt.Options{Timeout: 1 * time.Second})
}

// getBoltDB returns a new/existing bolt.DB. Already opened db's are cached.
func (s *StoreImpl) getBoltDB(index int64, getLock bool) (*bolt.DB, error) {
	if getLock {
		s.mutex.Lock()
		defer s.mutex.Unlock()
	}
	name := boltNameFromIndex(index)
	if idb, ok := s.cache.Get(name); ok {
		if db, ok := idb.(*bolt.DB); ok {
			return db, nil
		}
	}
	db, err := getBoltDBNoCache(s.dir, index)
	if err != nil {
		return nil, fmt.Errorf("Unable to open boltdb %d: %s", index, err)
	}
	s.cache.Add(name, db)
	return db, nil
}

// closer is a callback we pass to the lru cache to close bolt.DBs once they've
// been evicted from the cache.
func closer(key, value interface{}) {
	if db, ok := value.(*bolt.DB); ok {
		util.Close(db)
	} else {
		sklog.Errorf("Found a non-bolt.DB in the cache at key %q", key)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// tileIndices returns all the keys of s.tiles.
func (s *StoreImpl) tileIndices() []int64 {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	ret := []int64{}
	for i, _ := range s.tiles {
		ret = append(ret, i)
	}
	return ret
}

// getAndClearUpdatedKeys gets all the timeseries keys of the tile at tile
// 'index' and resets the 'updatedKeys' of said tile.
func (s *StoreImpl) getAndClearUpdatedKeys(index int64) []string {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	ret := []string{}
	for key, _ := range s.tiles[index].updatedKeys {
		ret = append(ret, key)
	}
	s.tiles[index].updatedKeys = map[string]bool{}
	return ret
}

// oneStep does a single step of flushing all the tiles to disk.
func (s *StoreImpl) oneStep(now time.Time) []error {
	ret := []error{}
	tileIndices := s.tileIndices()

	// Grab each tile and write updated info to disk.
	for _, i := range tileIndices {
		keys := s.getAndClearUpdatedKeys(i)
		db, err := s.getBoltDB(i, true)
		if err != nil {
			ret = append(ret, err)
			sklog.Errorf("Failed to get boltdb for index %d: %s ", i, err)
			continue
		}

		for j := 0; j < len(keys); j += WRITE_BATCH_SIZE {
			batch := keys[j:min(j+WRITE_BATCH_SIZE, len(keys))]

			add := func(tx *bolt.Tx) error {
				m, err := tx.CreateBucketIfNotExists([]byte(BUCKET_NAME))
				if m == nil {
					return fmt.Errorf("Failed to get bucket %q: %s", BUCKET_NAME, err)
				}
				s.mutex.Lock()
				defer s.mutex.Unlock()

				t := s.tiles[i]
				for _, key := range batch {
					b, err := t.set[key].Bytes()
					if err != nil {
						return fmt.Errorf("Failed to convert to bytes while writing in the background for key=%q: %s", key, err)
					}
					if err := m.Put([]byte(key), b); err != nil {
						return fmt.Errorf("Failed writing in the background for key=%q: %s", key, err)
					}
				}
				return nil
			}

			if err := db.Update(add); err != nil {
				ret = append(ret, err)
				sklog.Errorf("Failed to save tile %d: %s", i, err)
			}
		}
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	currentTileIndex := timeToIndex(now)
	for i := currentTileIndex - 1; i <= currentTileIndex+1; i++ {
		if _, ok := s.tiles[i]; !ok {
			s.tiles[i] = newTileInfo()
		}
	}
	for k, _ := range s.tiles {
		if k < currentTileIndex-1 || k > currentTileIndex+1 {
			delete(s.tiles, k)
		}
	}

	// TODO(jcgregorio) If any tiles were removed then rebuild paramSet, maybe.
	return ret
}

// background runs in the background and periodically flushes tiles to disk.
func (s *StoreImpl) background() {
	for _ = range time.Tick(15 * time.Minute) {
		if errors := s.oneStep(time.Now()); len(errors) != 0 {
			sklog.Errorf("Errors occured while writing: %v", errors)
		}
	}
}

func tileInfoFromBolt(db *bolt.DB) *TileInfo {
	ret := newTileInfo()

	get := func(tx *bolt.Tx) error {
		m := tx.Bucket([]byte(BUCKET_NAME))
		if m == nil {
			// db or bucket might not exist, which is valid.
			return nil
		}

		v := m.Cursor()
		for bkey, rawValue := v.First(); bkey != nil; bkey, rawValue = v.Next() {
			key := string(dup(bkey))
			t, err := ts.NewFromData(rawValue)
			if err != nil {
				sklog.Errorf("Failed to load points %q: %s", key, err)
				continue
			}
			ret.set[key] = t
		}
		return nil
	}

	if err := db.View(get); err != nil {
		sklog.Errorf("Failed to load matches from BoltDB: %s", err)
	}
	return ret
}

// New returns a *StoreImpl that reads/writes BoltBD files stored
// in the given directory 'dir'.
func New(dir string) (*StoreImpl, error) {
	cache, err := lru.NewWithEvict(MAX_CACHED_TILES, closer)
	if err != nil {
		return nil, fmt.Errorf("Couldn't create boltdb cache: %s", err)
	}

	currentTileIndex := timeToIndex(time.Now())
	paramSet := paramtools.ParamSet{}
	tiles := map[int64]*TileInfo{}
	for i := currentTileIndex - 1; i <= currentTileIndex+1; i++ {
		db, err := getBoltDBNoCache(dir, i)
		if err != nil {
			sklog.Warningf("Failed to open tile %d: %s", i, err)
			tiles[i] = newTileInfo()
			continue
		} else {
			cache.Add(boltNameFromIndex(i), db)
			tiles[i] = tileInfoFromBolt(db)
		}
		for key, _ := range tiles[i].set {
			paramSet.AddParamsFromKey(key)
		}
	}

	impl := &StoreImpl{
		dir:      dir,
		cache:    cache,
		tiles:    tiles,
		paramSet: paramSet,
	}

	go impl.background()

	return impl, nil
}

// timeToIndex returns the tile index that contains the given time 't'.
func timeToIndex(t time.Time) int64 {
	return t.Unix() / TILE_SIZE_IN_SECONDS
}

func (s *StoreImpl) Add(points []Measurement) error {
	if len(points) == 0 {
		return nil
	}
	// Sort points by time so we don't thrash on which tile we are accessing.
	sort.Sort(MeasurementSlice(points))

	s.mutex.Lock()
	defer s.mutex.Unlock()

	tileIndex := points[0].Point.Timestamp / TILE_SIZE_IN_SECONDS
	tile, ok := s.tiles[tileIndex]
	if !ok {
		return fmt.Errorf("Got point that is out of range: tile index %d for point %v", tileIndex, points[0])
	}
	for _, pt := range points {
		// Have we moved into a new tile?
		newTileIndex := pt.Point.Timestamp / TILE_SIZE_IN_SECONDS
		if newTileIndex != tileIndex {
			if tile, ok = s.tiles[newTileIndex]; !ok {
				return fmt.Errorf("Got point that is out of range: tile index %d for point %v", newTileIndex, pt)
			}
			tileIndex = newTileIndex
		}
		if series, ok := tile.set[pt.Key]; ok {
			series.Add(pt.Point)
		} else {
			tile.lastNewKey = time.Now()

			if !query.ValidateKey(pt.Key) {
				sklog.Errorf("Got an invalid key: %q", pt.Key)
				continue
			}
			tile.set[pt.Key] = ts.New(pt.Point)
			s.paramSet.AddParamsFromKey(pt.Key)
		}
		tile.updatedKeys[pt.Key] = true
	}
	return nil
}

// dup makes a copy of a byte slice.
//
// Needed since values returned from BoltDB are only valid
// for the life of the transaction.
func dup(b []byte) []byte {
	ret := make([]byte, len(b))
	copy(ret, b)
	return ret
}

func addMatchesToTimeSeriesSet(tss ts.TimeSeriesSet, key string, matches []ts.Point) {
	if len(matches) == 0 {
		return
	}
	series, ok := tss[key]
	if !ok {
		series = ts.New(matches[0])
		matches = matches[1:]
		tss[key] = series
	}
	for _, p := range matches {
		series.Add(p)
	}
}

// Match
//
// TODO(jcgregorio) Keep a cache of recent queries and the keys that matched them
//  to avoid doing full scans for each call to Match.
func (s *StoreImpl) Match(begin, end time.Time, q *query.Query) ts.TimeSeriesSet {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	ret := ts.TimeSeriesSet{}
	// Need to search through both BoltDBs and TimeseriesSets.
	for i := timeToIndex(begin); i <= timeToIndex(end); i++ {
		if tileInfo, ok := s.tiles[i]; ok {
			for key, value := range tileInfo.set {
				if q.Matches(key) {
					matches := value.PointsInRange(begin.Unix(), end.Unix())
					addMatchesToTimeSeriesSet(ret, key, matches)
				}
			}
		} else {
			db, err := s.getBoltDB(i, false)
			if err != nil {
				sklog.Errorf("Failed to open BoltDB: %s", err)
				continue
			}
			get := func(tx *bolt.Tx) error {
				m := tx.Bucket([]byte(BUCKET_NAME))
				if m == nil {
					return fmt.Errorf("Failed to get bucket: %s", BUCKET_NAME)
				}

				v := m.Cursor()
				for bkey, rawValue := v.First(); bkey != nil; bkey, rawValue = v.Next() {

					if !q.Matches(string(bkey)) {
						continue
					}
					// Don't make the copy until we know we are going to need it.
					key := string(dup(bkey))
					matches, err := ts.PointsInRange(begin.Unix(), end.Unix(), rawValue)
					if err != nil {
						sklog.Errorf("Failed to load matched points %q: %s", key, err)
					} else {
						addMatchesToTimeSeriesSet(ret, key, matches)
					}
				}
				return nil
			}

			if err := db.View(get); err != nil {
				sklog.Errorf("Failed to load match from BoltDB: %s", err)
			}
		}
	}
	return ret
}

func (s *StoreImpl) ParamSet() paramtools.ParamSet {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.paramSet.Copy()
}

var _ Store = &StoreImpl{}
