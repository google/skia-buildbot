/* ingester loads JSON data from Google Storage and uses it to update the TileStore.

The output from nanobench looks like this:

  {
    "gitHash": "d1830323662ae8ae06908b97f15180fd25808894",
    "key": {
      "arch": "x86",
      "gpu": "GTX660",
      "os": "Ubuntu12",
      "model": "ShuttleA",
    },
    "options":{
        "system":"UNIX"
    },
    "results":{
        "DeferredSurfaceCopy_discardable_640_480":{
            "gpu":{
                "max_ms":7.9920480,
                "mean_ms":7.9920480,
                "median_ms":7.9920480,
                "min_ms":7.9920480,
                "options":{
                    "GL_RENDERER":"Quadro K600/PCIe/SSE2",
                    "GL_SHADING_LANGUAGE_VERSION":"4.40 NVIDIA via Cg compiler",
                    "GL_VENDOR":"NVIDIA Corporation",
                    "GL_VERSION":"4.4.0 NVIDIA 331.38"
                }
            },
            "nvprmsaa4":{
                "max_ms":16.7961230,
                "mean_ms":16.7961230,
                "median_ms":16.7961230,
                "min_ms":16.7961230,
                "options":{
                    "GL_RENDERER":"Quadro K600/PCIe/SSE2",
                    "GL_SHADING_LANGUAGE_VERSION":"4.40 NVIDIA via Cg compiler",
                    "GL_VENDOR":"NVIDIA Corporation",
                    "GL_VERSION":"4.4.0 NVIDIA 331.38"
                }
            }
        },
        ...

   Ingester converts that structure into Traces in Tiles.

   The key for a Trace is constructed from the "key" dictionary, along with the
   test name, the configuration name and the value being store. So, for
   example, the first value above will be store in the Trace with a key of:

     "x86:GTX660:Ubuntu12:ShuttleA:DeferredSurfaceCopy_discardable_640_480:gpu"

   Note that since we only record one value (min_ms for now) then we don't need
   to add that to the key.

   The Params for such a Trace will be the union of the "key" and all the
   related "options" dictionaries. Again, for the first value:

     "params": {
       "arch": "x86",
       "gpu": "GTX660",
       "os": "Ubuntu12",
       "model": "ShuttleA",
       "system":"UNIX"
       "GL_RENDERER":"Quadro K600/PCIe/SSE2",
       "GL_SHADING_LANGUAGE_VERSION":"4.40 NVIDIA via Cg compiler",
       "GL_VENDOR":"NVIDIA Corporation",
       "GL_VERSION":"4.4.0 NVIDIA 331.38"
     }

   If in the future we wanted to have Traces for both min_ms and median_ms
   then the keys would become:

     "x86:GTX660:Ubuntu12:ShuttleA:DeferredSurfaceCopy_discardable_640_480:gpu:min_ms"
     "x86:GTX660:Ubuntu12:ShuttleA:DeferredSurfaceCopy_discardable_640_480:gpu:median_ms"

   N.B. That would also require adding a synthetic option
   "value_type": ("min"|"median") to the Params for each Trace, so you could
   select out those different type of Traces in the UI.
*/
package ingester

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

import (
	"code.google.com/p/google-api-go-client/storage/v1"
	"github.com/golang/glog"
	"github.com/rcrowley/go-metrics"
	"skia.googlesource.com/buildbot.git/perf/go/config"
	"skia.googlesource.com/buildbot.git/perf/go/filetilestore"
	"skia.googlesource.com/buildbot.git/perf/go/gitinfo"
	"skia.googlesource.com/buildbot.git/perf/go/gs"
	"skia.googlesource.com/buildbot.git/perf/go/types"
	"skia.googlesource.com/buildbot.git/perf/go/util"
)

var (
	elapsedTimePerUpdate metrics.Timer
	perfMetricsProcessed metrics.Counter
	client               *http.Client
)

func Init() {
	elapsedTimePerUpdate = metrics.NewRegisteredTimer("ingester.nano.update", metrics.DefaultRegistry)
	perfMetricsProcessed = metrics.NewRegisteredCounter("ingester.nano.processed", metrics.DefaultRegistry)
	client = util.NewTimeoutClient()
}

// Ingester does the work of loading JSON files from Google Storage and putting
// the data into the TileStore.
type Ingester struct {
	git            *gitinfo.GitInfo
	tileStore      types.TileStore
	storage        *storage.Service
	hashToNumber   map[string]int
	lastIngestTime time.Time
}

// NewIngester creates an Ingester given the repo and tilestore specified.
//
// If pull is true then a Git pull will be done on the repo before doing updates.
// If the timestampFile is "" then no timestampFile will be used, otherwise the
// last ingestion time will be checkpointed into that file at the end of every
// upate.
func NewIngester(gitRepoDir, tileStoreDir string, pull bool, timestampFile string) (*Ingester, error) {
	git, err := gitinfo.NewGitInfo(gitRepoDir, pull)
	if err != nil {
		return nil, fmt.Errorf("Failed loading Git info: %s\n", err)
	}
	storage, err := storage.New(http.DefaultClient)
	if err != nil {
		return nil, fmt.Errorf("Failed to create interace to Google Storage: %s\n", err)
	}

	i := &Ingester{
		git:          git,
		tileStore:    filetilestore.NewFileTileStore(tileStoreDir, "nano", -1),
		storage:      storage,
		hashToNumber: map[string]int{},
	}
	return i, nil
}

// lastCommitTimeInTile looks backward in the list of Commits and finds the most recent.
func (i *Ingester) lastCommitTimeInTile(tile *types.Tile) time.Time {
	t := tile.Commits[0].CommitTime
	for i := (len(tile.Commits) - 1); i >= 0; i-- {
		if tile.Commits[i].CommitTime != 0 {
			t = tile.Commits[i].CommitTime
			break
		}
	}
	if time.Unix(t, 0).Before(time.Time(config.BEGINNING_OF_TIME)) {
		t = config.BEGINNING_OF_TIME.Unix()
	}
	return time.Unix(t, 0)
}

// TileTracker keeps track of which Tile we are on, and allows moving to new
// Tiles, writing out the current Tile when tiles are changed, and creating new
// Tiles if they don't exist.
type TileTracker struct {
	lastTileNum  int
	currentTile  *types.Tile
	tileStore    types.TileStore
	hashToNumber map[string]int
}

func NewTileTracker(tileStore types.TileStore, hashToNumber map[string]int) *TileTracker {
	return &TileTracker{
		lastTileNum:  -1,
		currentTile:  nil,
		tileStore:    tileStore,
		hashToNumber: hashToNumber,
	}
}

// Move changes the current Tile to the one that contains the given Git hash.
func (tt *TileTracker) Move(hash string) error {
	if _, ok := tt.hashToNumber[hash]; !ok {
		return fmt.Errorf("Commit does not exist in table: %s", hash)
	}
	hashNumber := tt.hashToNumber[hash]
	tileNum := hashNumber / config.TILE_SIZE
	if tileNum != tt.lastTileNum {
		glog.Infof("Moving from tile %d to %d", tt.lastTileNum, tileNum)
		if tt.lastTileNum != -1 {
			if err := tt.tileStore.Put(0, tt.lastTileNum, tt.currentTile); err != nil {
				return fmt.Errorf("TileTracker.Move() failed to flush old tile: %s", err)
			}
		}
		tt.lastTileNum = tileNum
		var err error
		tt.currentTile, err = tt.tileStore.GetModifiable(0, tileNum)
		if err != nil {
			return fmt.Errorf("UpdateCommitInfo: Failed to get modifiable tile %d: %s", tileNum, err)
		}
		if tt.currentTile == nil {
			tt.currentTile = types.NewTile()
			tt.currentTile.Scale = 0
			tt.currentTile.TileIndex = tileNum
		}
	}
	return nil
}

// Flush writes the current Tile out, should be called once all updates are
// done. Note that Move() writes out the former Tile as it moves to a new Tile,
// so this only needs to be called at the end of looping over a set of work.
func (tt TileTracker) Flush() {
	glog.Info("Flushing Tile.")
	if tt.lastTileNum != -1 {
		if err := tt.tileStore.Put(0, tt.lastTileNum, tt.currentTile); err != nil {
			glog.Error("Failed to write Tile: %s", err)
		}
	}
}

// Tile returns the current Tile.
func (tt TileTracker) Tile() *types.Tile {
	return tt.currentTile
}

// Offset returns the Value offset of a commit in a Trace.
func (tt TileTracker) Offset(hash string) int {
	return tt.hashToNumber[hash] % config.TILE_SIZE
}

// UpdateCommitInfo finds all the new commits since the last time we ran and
// adds them to the tiles, creating new tiles if necessary.
func (i *Ingester) UpdateCommitInfo(pull bool) error {
	if err := i.git.Update(pull); err != nil {
		return fmt.Errorf("Failed git pull during UpdateCommitInfo: %s", err)
	}

	// Compute Git CL number for each Git hash.
	allHashes := i.git.From(time.Time(config.BEGINNING_OF_TIME))
	hashToNumber := map[string]int{}
	for i, h := range allHashes {
		hashToNumber[h] = i
	}
	i.hashToNumber = hashToNumber

	// Find the time of the last Commit seen.
	ts := time.Time(config.BEGINNING_OF_TIME)
	lastTile, err := i.tileStore.Get(0, -1)
	if err == nil && lastTile != nil {
		ts = i.lastCommitTimeInTile(lastTile)
	} else {
		// Boundary condition; just started making Tiles and none exist.
		newTile := types.NewTile()
		newTile.Scale = 0
		newTile.TileIndex = 0
		if err := i.tileStore.Put(0, 0, newTile); err != nil {
			return fmt.Errorf("UpdateCommitInfo: Failed to write new tile: %s", err)
		}
	}
	glog.Infof("UpdateCommitInfo: Last commit timestamp: %s", ts)

	// Find all the Git hashes that are new to us.
	newHashes := i.git.From(ts)

	// Add Commit info to the Tiles for each new hash.
	tt := NewTileTracker(i.tileStore, i.hashToNumber)
	for _, hash := range newHashes {
		if err := tt.Move(hash); err != nil {
			glog.Errorf("UpdateCommitInfo Move(%s) failed with: %s", hash, err)
			continue
		}
		author, _, ts, err := i.git.Details(hash)
		if err != nil {
			glog.Errorf("Failed to get details for hash: %s: %s", hash, err)
			continue
		}
		tt.Tile().Commits[tt.Offset(hash)] = &types.Commit{
			CommitTime: ts.Unix(),
			Hash:       hash,
			Author:     author,
		}
	}
	tt.Flush()
	return nil
}

// equalMaps checks if the two maps are equal.
func equalMaps(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	// Since they are the same size we only need to check from one side, i.e.
	// compare a's values to b's values.
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

// addBenchDataToTile adds BenchData to a Tile.
//
// See the description at the top of this file for how the mapping works.
func addBenchDataToTile(benchData *BenchData, tile *types.Tile, offset int) {
	keyPrefix := benchData.KeyPrefix()
	for testName, allConfigs := range benchData.Results {
		for configName, result := range *allConfigs {
			key := fmt.Sprintf("%s:%s:%s", keyPrefix, testName, configName)

			// Construct the Traces params from all the options.
			params := map[string]string{
				"test":   testName,
				"config": configName,
			}
			for k, v := range benchData.Key {
				params[k] = v
			}
			for k, v := range benchData.Options {
				params[k] = v
			}
			for k, v := range result.Options {
				params[k] = v
			}

			var trace *types.Trace
			var ok bool
			needsUpdate := false
			if trace, ok = tile.Traces[key]; !ok {
				trace = types.NewTrace()
				tile.Traces[key] = trace
				needsUpdate = true
			} else if !equalMaps(params, tile.Traces[key].Params) {
				needsUpdate = true
			}
			tile.Traces[key].Params = params

			if needsUpdate {
				// Update the Tile's ParamSet with any new keys or values we see.
				//
				// TODO(jcgregorio) Maybe defer this until we are about to Put the Tile
				// back to disk and rebuild ParamSet from scratch over all the Traces.
				for k, v := range params {
					if _, ok = tile.ParamSet[k]; !ok {
						tile.ParamSet[k] = []string{v}
					} else if !util.In(v, tile.ParamSet[k]) {
						tile.ParamSet[k] = append(tile.ParamSet[k], v)
					}
				}
			}
			if trace.Values[offset] != config.MISSING_DATA_SENTINEL {
				glog.Infof("Duplicate entry found for %s, hash %s", key, benchData.Hash)
			}
			trace.Values[offset] = result.Min
			perfMetricsProcessed.Inc(1)
		}
	}
}

// Update does a single full update, first updating the commits and creating
// new tiles if necessary, and then pulling in new data from Google Storage to
// populate the traces.
func (i *Ingester) Update(pull bool, lastIngestTime int64) error {
	glog.Info("Beginning ingest.")
	begin := time.Now()
	if err := i.UpdateCommitInfo(pull); err != nil {
		glog.Errorf("Update: Failed to update commit info: %s", err)
		return err
	}
	if err := i.UpdateTiles(lastIngestTime); err != nil {
		glog.Errorf("Update: Failed to update tiles: %s", err)
		return err
	}
	elapsedTimePerUpdate.UpdateSince(begin)
	glog.Info("Finished ingest.")
	return nil
}

// UpdateTiles reads the latest JSON files from Google Storage and converts
// them into Traces stored in Tiles.
func (i *Ingester) UpdateTiles(lastIngestTime int64) error {
	tt := NewTileTracker(i.tileStore, i.hashToNumber)
	benchFiles, err := GetBenchFiles(lastIngestTime, i.storage, "nano-json-v1")
	if err != nil {
		return fmt.Errorf("Failed to update tiles: %s", err)
	}
	for _, b := range benchFiles {
		// Load and parse the JSON.
		benchData, err := b.FetchAndParse()
		if err != nil {
			// Don't fall over for a single corrupt file.
			continue
		}
		// Move to the correct Tile for the Git hash.
		hash := benchData.Hash
		if err := tt.Move(hash); err != nil {
			glog.Errorf("UpdateCommitInfo Move(%s) failed with: %s", hash, err)
			continue
		}
		// Add the parsed data to the Tile.
		addBenchDataToTile(benchData, tt.Tile(), tt.Offset(hash))
	}
	tt.Flush()
	return nil
}

// BenchResult represents a single test result.
//
// Used in BenchData.
type BenchResult struct {
	Min     float64           `json:"min_ms"`
	Options map[string]string `json:"options"`
}

// BenchResults is the dictionary of individual BenchResult structs.
//
// Used in BenchData.
type BenchResults map[string]*BenchResult

// BenchData is the top level struct for decoding the nanobench JSON format.
type BenchData struct {
	Hash    string                   `json:"gitHash"`
	Key     map[string]string        `json:"key"`
	Options map[string]string        `json:"options"`
	Results map[string]*BenchResults `json:"results"`
}

// KeyPrefix makes the first part of a Trace key by joining the parts of the
// BenchData Key value in sort order, i.e.
//
//   {"arch":"x86","model":"ShuttleA","gpu":"GTX660","os":"Ubuntu12"}
//
// should return:
//
//   "x86:GTX660:ShuttleA:Ubuntu12"
//
func (b BenchData) KeyPrefix() string {
	keys := make([]string, 0, len(b.Key))
	retval := make([]string, 0, len(b.Key))

	for k, _ := range b.Key {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		retval = append(retval, b.Key[k])
	}
	return strings.Join(retval, ":")
}

// BenchFile is the URI of a single JSON file with results in it.
type BenchFile struct {
	URI      string // Absolute URI used to fetch the file.
	Name     string // Complete path, w/o the gs:// prefix.
	filename string // The name of the just the file.
}

func NewBenchFile(uri, name string) *BenchFile {
	return &BenchFile{
		URI:      uri,
		Name:     name,
		filename: filepath.Base(uri),
	}
}

func (b BenchFile) parseFromReader(r io.Reader) (*BenchData, error) {
	dec := json.NewDecoder(r)
	benchData := &BenchData{}
	if err := dec.Decode(benchData); err != nil {
		glog.Warningf("Failed to decode JSON of %s: %s", b.URI, err)
		return nil, err
	}
	return benchData, nil
}

// FetchAndParse retrieves the JSON and parses it into a BenchData instance.
func (b BenchFile) FetchAndParse() (*BenchData, error) {
	for i := 0; i < config.MAX_URI_GET_TRIES; i++ {
		request, err := gs.RequestForStorageURL(b.URI)
		if err != nil {
			glog.Warningf("Unable to create Storage MediaURI request: %s\n", err)
			continue
		}
		resp, err := client.Do(request)
		if err != nil {
			glog.Warningf("Unable to retrieve URI while creating file iterator: %s", err)
			continue
		}
		defer resp.Body.Close()
		benchData, err := b.parseFromReader(resp.Body)
		if err != nil {
			glog.Warningf("Failed to decode JSON of %s: %s", b.URI, err)
			continue
		}
		return benchData, nil
	}
	return nil, fmt.Errorf("Failed fetching JSON after %d attempts", config.MAX_URI_GET_TRIES)
}

// BenchFileSlice is used for sorting BenchFile's by filename.
type BenchFileSlice []*BenchFile

func (p BenchFileSlice) Len() int           { return len(p) }
func (p BenchFileSlice) Less(i, j int) bool { return p[i].filename < p[j].filename }
func (p BenchFileSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// GetFilesFromGSDir returns a list of URIs to get of the JSON files in the
// given bucket and directory made after the given timestamp.
func GetFilesFromGSDir(dir string, earliestTimestamp int64, storage *storage.Service) ([]*BenchFile, error) {
	results := []*BenchFile{}
	glog.Infoln("Opening directory", dir)

	req := storage.Objects.List(gs.GS_PROJECT_BUCKET).Prefix(dir)
	for req != nil {
		resp, err := req.Do()
		if err != nil {
			return nil, fmt.Errorf("Error occurred while listing JSON files: %s", err)
		}
		for _, result := range resp.Items {
			updateDate, _ := time.Parse(time.RFC3339, result.Updated)
			updateTimestamp := updateDate.Unix()
			if updateTimestamp > earliestTimestamp {
				results = append(results, NewBenchFile(result.MediaLink, result.Name))
			}
		}
		if len(resp.NextPageToken) > 0 {
			req.PageToken(resp.NextPageToken)
		} else {
			req = nil
		}
	}
	return results, nil
}

// GetBenchFiles retrieves a list of BenchFiles from Cloud Storage, each one
// corresponding to a single JSON file.
func GetBenchFiles(last int64, storage *storage.Service, dir string) ([]*BenchFile, error) {
	dirs := gs.GetLatestGSDirs(last, time.Now().Unix(), dir)
	glog.Infoln("GetBenchFiles: Looking in dirs: ", dirs)

	retval := []*BenchFile{}
	for _, dir := range dirs {
		files, err := GetFilesFromGSDir(dir, last, storage)
		if err != nil {
			return nil, err
		}
		retval = append(retval, files...)
	}
	sort.Sort(BenchFileSlice(retval))
	return retval, nil
}
