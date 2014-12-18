package ingester

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"

	"time"
)

import (
	storage "code.google.com/p/google-api-go-client/storage/v1"
	"github.com/golang/glog"
	metrics "github.com/rcrowley/go-metrics"
	"skia.googlesource.com/buildbot.git/go/gitinfo"
	"skia.googlesource.com/buildbot.git/go/gs"
	"skia.googlesource.com/buildbot.git/go/util"
	"skia.googlesource.com/buildbot.git/perf/go/config"
	"skia.googlesource.com/buildbot.git/perf/go/filetilestore"
	"skia.googlesource.com/buildbot.git/perf/go/types"
)

var (
	client *http.Client
)

// Init initializes the module, the optional http.Client is used to make HTTP
// requests to Google Storage. If nil is supplied then a default client is
// used.
func Init(cl *http.Client) {
	if cl != nil {
		client = cl
	} else {
		client = util.NewTimeoutClient()
	}
}

type Opener func() (io.ReadCloser, error)

// ResultIngester needs to be implemented to ingest files. It allows to
// ingest files individually or in a batch mode.
type ResultIngester interface {
	// The provided opener allows to open an input stream, parse it and add it
	// to the tile. The ingestion can also be done when BatchFinished is
	// called. In that case the openers should be cached and opened then.
	Ingest(tt *TileTracker, opener Opener, fname string, counter metrics.Counter) error

	// BatchFinished is called when the current batch is finished. This is
	// to cover the case when ingestion is better done for the whole batch or
	// a processing is necessary for the batch. This should reset the internal
	// state of the Ingester.
	BatchFinished(counter metrics.Counter) error
}

// Ingester does the work of loading JSON files from Google Storage and putting
// the data into the TileStore. The time range it ingests is controlled by
// minDuration and nCommits. It aims to cover all commits within minDuration
// from the last commit and cover at least nCommits.

// TODO(stephana): Currently this relies on a client to call the Update()
// method in intervals. Instead we should make the ingester self contained
// in that it runs a goroutine internally to drive ingestion.
type Ingester struct {
	git            *gitinfo.GitInfo
	tileStore      types.TileStore
	storage        *storage.Service
	hashToNumber   map[string]int
	lastIngestTime time.Time
	resultIngester ResultIngester
	storageBaseDir string
	datasetName    string
	nCommits       int
	minDuration    time.Duration

	// Keeps track of processed files so we avoid duplicate downloads.
	processedFiles map[string]bool

	// Metrics about the ingestion process.
	elapsedTimePerUpdate           metrics.Gauge
	metricsProcessed               metrics.Counter
	lastSuccessfulUpdate           time.Time
	timeSinceLastSucceessfulUpdate metrics.Gauge
}

func newGauge(name, suffix string) metrics.Gauge {
	return metrics.NewRegisteredGauge("ingester."+name+".gauge."+suffix, metrics.DefaultRegistry)
}

func newCounter(name, suffix string) metrics.Counter {
	return metrics.NewRegisteredCounter("ingester."+name+".gauge."+suffix, metrics.DefaultRegistry)
}

// NewIngester creates an Ingester given the repo and tilestore specified.
func NewIngester(git *gitinfo.GitInfo, tileStoreDir string, datasetName string, ri ResultIngester, nCommits int, minDuration time.Duration, storageBaseDir, metricName string) (*Ingester, error) {
	storage, err := storage.New(http.DefaultClient)
	if err != nil {
		return nil, fmt.Errorf("Failed to create interace to Google Storage: %s\n", err)
	}

	i := &Ingester{
		git:                            git,
		tileStore:                      filetilestore.NewFileTileStore(tileStoreDir, datasetName, -1),
		storage:                        storage,
		hashToNumber:                   map[string]int{},
		resultIngester:                 ri,
		storageBaseDir:                 storageBaseDir,
		datasetName:                    datasetName,
		elapsedTimePerUpdate:           newGauge(metricName, "update"),
		metricsProcessed:               newCounter(metricName, "processed"),
		lastSuccessfulUpdate:           time.Now(),
		timeSinceLastSucceessfulUpdate: newGauge(metricName, "time-since-last-successful-update"),
		nCommits:                       nCommits,
		minDuration:                    minDuration,
		processedFiles:                 map[string]bool{},
	}

	i.timeSinceLastSucceessfulUpdate.Update(int64(time.Since(i.lastSuccessfulUpdate).Seconds()))
	go func() {
		for _ = range time.Tick(time.Minute) {
			i.timeSinceLastSucceessfulUpdate.Update(int64(time.Since(i.lastSuccessfulUpdate).Seconds()))
		}
	}()
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
			glog.Errorf("Failed to write Tile: %s", err)
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
	glog.Infof("Ingest %s: Starting UpdateCommitInfo", i.datasetName)
	if err := i.git.Update(pull, false); err != nil {
		return fmt.Errorf("Ingest %s: Failed git pull for during UpdateCommitInfo: %s", i.datasetName, err)
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
			return fmt.Errorf("Ingest %s: UpdateCommitInfo: Failed to write new tile: %s", i.datasetName, err)
		}
	}
	glog.Infof("Ingest %s: UpdateCommitInfo: Last commit timestamp: %s", i.datasetName, ts)

	// Find all the Git hashes that are new to us.
	newHashes := i.git.From(ts)

	glog.Infof("Ingest %s: len(newHashes): from %d", i.datasetName, len(newHashes))

	// Add Commit info to the Tiles for each new hash.
	tt := NewTileTracker(i.tileStore, i.hashToNumber)
	for _, hash := range newHashes {
		if err := tt.Move(hash); err != nil {
			glog.Errorf("UpdateCommitInfo Move(%s) failed with: %s", hash, err)
			continue
		}
		details, err := i.git.Details(hash)
		if err != nil {
			glog.Errorf("Failed to get details for hash: %s: %s", hash, err)
			continue
		}
		tt.Tile().Commits[tt.Offset(hash)] = &types.Commit{
			CommitTime: details.Timestamp.Unix(),
			Hash:       hash,
			Author:     details.Author,
		}
	}
	glog.Infof("Ingest %s: Starting to flush tile.", i.datasetName)
	tt.Flush()

	glog.Infof("Ingest %s: Finished UpdateCommitInfo", i.datasetName)
	return nil
}

// Update does a single full update, first updating the commits and creating
// new tiles if necessary, and then pulling in new data from Google Storage to
// populate the traces.
func (i *Ingester) Update() error {
	glog.Info("Beginning ingest.")
	begin := time.Now()
	if err := i.UpdateCommitInfo(true); err != nil {
		glog.Errorf("Update: Failed to update commit info: %s", err)
		return err
	}
	if err := i.UpdateTiles(); err != nil {
		glog.Errorf("Update: Failed to update tiles: %s", err)
		return err
	}
	i.lastSuccessfulUpdate = time.Now()
	i.elapsedTimePerUpdate.Update(int64(time.Since(begin).Seconds()))
	glog.Info("Finished ingest.")
	return nil
}

// UpdateTiles reads the latest JSON files from Google Storage and converts
// them into Traces stored in Tiles.

// TODO(stephana): Currently this is very coarse in that it determines
// the target time range in every run and therfore considers a large
// number of input files. That is somewhat mitigated by only ingesting
// files we have not seen before, but a future version should clever about
// picking a better timerange.
func (i *Ingester) UpdateTiles() error {
	startTS, endTS, err := i.getCommitRangeOfInterest()
	if err != nil {
		return err
	}
	glog.Infof("StartTime (%s): %s", i.datasetName, time.Unix(startTS, 0))
	glog.Infof("EndTime   (%s): %s", i.datasetName, time.Unix(endTS, 0))

	glog.Infof("Ingest %s: Starting UpdateTiles", i.datasetName)

	tt := NewTileTracker(i.tileStore, i.hashToNumber)
	resultsFiles, err := GetResultsFileLocations(startTS, endTS, i.storage, i.storageBaseDir)
	if err != nil {
		return fmt.Errorf("Failed to update tiles: %s", err)
	}

	glog.Infof("Ingest %s: Found %d resultsFiles", i.datasetName, len(resultsFiles))

	// Lookup table to trim the cache at the end.
	inCurrentBatch := make(map[string]bool, len(resultsFiles))
	processedMD5s := make([]string, 0, len(resultsFiles))
	for _, resultLocation := range resultsFiles {
		if !i.processedFiles[resultLocation.MD5Hash] {
			opener := func() (io.ReadCloser, error) {
				r, err := resultLocation.Fetch()
				if err != nil {
					return nil, fmt.Errorf("Failed to fetch: %s: %s", resultLocation.Name, err)
				}
				return r, nil
			}

			if err := i.resultIngester.Ingest(tt, opener, resultLocation.Name, i.metricsProcessed); err != nil {
				glog.Errorf("Failed to ingest %s: %s", resultLocation.Name, err)
				continue
			}
			// Gather all successfully processed MD5s
			processedMD5s = append(processedMD5s, resultLocation.MD5Hash)
		} else {
			glog.Infof("Skipped duplicate: %s (%s)", resultLocation.Name, resultLocation.MD5Hash)
		}
		inCurrentBatch[resultLocation.MD5Hash] = true
	}

	// Notify the ingester that the batch has finished and cause it to reset its
	// state and do any pending ingestion.
	if err := i.resultIngester.BatchFinished(i.metricsProcessed); err != nil {
		glog.Errorf("Batchfinished failed (%s): %s", i.datasetName, err)
	} else {
		// Only consider the files ingested if Batchfinished succeeded.
		for _, h := range processedMD5s {
			i.processedFiles[h] = true
		}

		// Trim the cache.
		for k := range i.processedFiles {
			if !inCurrentBatch[k] {
				delete(i.processedFiles, k)
			}
		}
	}

	tt.Flush()

	glog.Infof("Ingest %s: Finished UpdateTiles", i.datasetName)
	return nil
}

// getCommitRangeOfInterest returns the time range (start, end) that
// we are interested in. This method assumes that UpdateCommitInfo
// has been called and therefore reading the tile should not fail.
func (i *Ingester) getCommitRangeOfInterest() (int64, int64, error) {
	// Get the first commit of interest.
	tile, err := i.tileStore.Get(0, -1)
	if err != nil {
		return 0, 0, err
	}

	counter := 2
	lastCommitIdx := tile.LastCommitIndex()
	startCommitTS := tile.Commits[lastCommitIdx].CommitTime
	now := time.Now()
	for cidx := lastCommitIdx - 1; cidx >= 0; cidx-- {
		timeDiff := now.Sub(time.Unix(tile.Commits[cidx].CommitTime, 0))
		if (counter >= i.nCommits) && (timeDiff >= i.minDuration) {
			startCommitTS = tile.Commits[cidx].CommitTime
			break
		}
		counter++
	}

	return startCommitTS, now.Unix(), nil
}

// ResultsFileLocation is the URI of a single JSON file with results in it.
type ResultsFileLocation struct {
	URI     string // Absolute URI used to fetch the file.
	Name    string // Complete path, w/o the gs:// prefix.
	MD5Hash string // MD5 hash of the content.
}

func NewResultsFileLocation(uri, name string, md5Hash string) *ResultsFileLocation {
	return &ResultsFileLocation{
		URI:     uri,
		Name:    name,
		MD5Hash: md5Hash,
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
		glog.Infof("GS FETCH %s", b.URI)
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
				md5Bytes, err := base64.StdEncoding.DecodeString(result.Md5Hash)
				if err != nil {
					glog.Errorf("Unable to decode base64-encoded MD5: %s", err)
				}
				results = append(results, NewResultsFileLocation(result.MediaLink, result.Name, hex.EncodeToString(md5Bytes)))
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
func GetResultsFileLocations(startTS int64, endTS int64, storage *storage.Service, dir string) ([]*ResultsFileLocation, error) {
	dirs := gs.GetLatestGSDirs(startTS, endTS, dir)
	glog.Infoln("GetResultsFileLocations: Looking in dirs: ", dirs)

	retval := []*ResultsFileLocation{}
	for _, dir := range dirs {
		files, err := getFilesFromGSDir(dir, startTS, storage)
		if err != nil {
			return nil, err
		}
		retval = append(retval, files...)
	}
	return retval, nil
}
