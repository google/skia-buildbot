package ingester

import (
	"fmt"
	"io"
	"net/http"

	"time"
)

import (
	"code.google.com/p/google-api-go-client/storage/v1"
	"github.com/golang/glog"
	"github.com/rcrowley/go-metrics"
	"skia.googlesource.com/buildbot.git/go/gs"
	"skia.googlesource.com/buildbot.git/go/util"
	"skia.googlesource.com/buildbot.git/perf/go/config"
	"skia.googlesource.com/buildbot.git/perf/go/filetilestore"
	"skia.googlesource.com/buildbot.git/perf/go/gitinfo"
	"skia.googlesource.com/buildbot.git/perf/go/types"
)

var (
	elapsedTimePerUpdate metrics.Timer
	perfMetricsProcessed metrics.Counter
	client               *http.Client
)

// Init initializes the module, the optional http.Client is used to make HTTP
// requests to Google Storage. If nil is supplied then a default client is
// used.
func Init(cl *http.Client) {
	elapsedTimePerUpdate = metrics.NewRegisteredTimer("ingester.nano.update", metrics.DefaultRegistry)
	perfMetricsProcessed = metrics.NewRegisteredCounter("ingester.nano.processed", metrics.DefaultRegistry)
	if cl != nil {
		client = cl
	} else {
		client = util.NewTimeoutClient()
	}
}

// IngestResultsFiles is passed to NewIngester, it does the actual work of mapping the resultsFiles into the Tiles.
type IngestResultsFiles func(tt *TileTracker, resultsFiles []*ResultsFileLocation) error

// Ingester does the work of loading JSON files from Google Storage and putting
// the data into the TileStore.
//
// TODO(jcgregorio) This needs a refactor since we also use it to drive the ingestion
// of trybot data. It needs to be broken into two pieces, one which feeds ResultsFileLocations,
// and a second optional piece tha builds a TileTracker if necessary.
type Ingester struct {
	git            *gitinfo.GitInfo
	tileStore      types.TileStore
	storage        *storage.Service
	hashToNumber   map[string]int
	lastIngestTime time.Time
	ingestResults  IngestResultsFiles
	storageBaseDir string
}

// NewIngester creates an Ingester given the repo and tilestore specified.
func NewIngester(git *gitinfo.GitInfo, tileStoreDir string, datasetName string, f IngestResultsFiles, storageBaseDir string) (*Ingester, error) {
	storage, err := storage.New(http.DefaultClient)
	if err != nil {
		return nil, fmt.Errorf("Failed to create interace to Google Storage: %s\n", err)
	}

	i := &Ingester{
		git:            git,
		tileStore:      filetilestore.NewFileTileStore(tileStoreDir, datasetName, -1),
		storage:        storage,
		hashToNumber:   map[string]int{},
		ingestResults:  f,
		storageBaseDir: storageBaseDir,
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
	resultsFiles, err := GetResultsFileLocations(lastIngestTime, i.storage, i.storageBaseDir)
	if err != nil {
		return fmt.Errorf("Failed to update tiles: %s", err)
	}
	i.ingestResults(tt, resultsFiles)
	tt.Flush()
	return nil
}

// ResultsFileLocation is the URI of a single JSON file with results in it.
type ResultsFileLocation struct {
	URI  string // Absolute URI used to fetch the file.
	Name string // Complete path, w/o the gs:// prefix.
}

func NewResultsFileLocation(uri, name string) *ResultsFileLocation {
	return &ResultsFileLocation{
		URI:  uri,
		Name: name,
	}
}

// Fetch retrieves the file contents.
//
// Callers must call Close() on the returned io.ReadCloser.
func (b ResultsFileLocation) Fetch() (io.ReadCloser, error) {
	for i := 0; i < config.MAX_URI_GET_TRIES; i++ {
		glog.Infof("Fetching: %s", b.Name)
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
		if resp.StatusCode != 200 {
			glog.Errorf("Failed to retrieve: %d  %s", resp.StatusCode, resp.Status)
		}
		return resp.Body, nil
	}
	return nil, fmt.Errorf("Failed fetching JSON after %d attempts", config.MAX_URI_GET_TRIES)
}

// getFilesFromGSDir returns a list of URIs to get of the JSON files in the
// given bucket and directory made after the given timestamp.
func getFilesFromGSDir(dir string, earliestTimestamp int64, storage *storage.Service) ([]*ResultsFileLocation, error) {
	results := []*ResultsFileLocation{}
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
				results = append(results, NewResultsFileLocation(result.MediaLink, result.Name))
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

// GetResultsFileLocations retrieves a list of ResultsFileLocations from Cloud Storage, each one
// corresponding to a single JSON file.
func GetResultsFileLocations(last int64, storage *storage.Service, dir string) ([]*ResultsFileLocation, error) {
	dirs := gs.GetLatestGSDirs(last, time.Now().Unix(), dir)
	glog.Infoln("GetResultsFileLocations: Looking in dirs: ", dirs)

	retval := []*ResultsFileLocation{}
	for _, dir := range dirs {
		files, err := getFilesFromGSDir(dir, last, storage)
		if err != nil {
			return nil, err
		}
		retval = append(retval, files...)
	}
	return retval, nil
}
