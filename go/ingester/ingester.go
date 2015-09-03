package ingester

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	metrics "github.com/rcrowley/go-metrics"
	"github.com/skia-dev/glog"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"go.skia.org/infra/go/filetilestore"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/gitinfo"
	"go.skia.org/infra/go/gs"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/util"
	storage "google.golang.org/api/storage/v1"
)

const (
	// Limit the number of times the ingester tries to get a file before giving up.
	MAX_URI_GET_TRIES = 4
)

var (
	client *http.Client
)

var (
	// TODO(jcgregorio) Make into a flag.
	BEGINNING_OF_TIME = QuerySince(time.Date(2014, time.June, 18, 0, 0, 0, 0, time.UTC))
)

var (
	// Sync all writes to the leveldb file.
	SYNC_WRITE = &opt.WriteOptions{Sync: true}
)

// Init initializes the module, the optional http.Client is used to make HTTP
// requests to Google Storage. If nil is supplied then a default client is
// used.
func Init(cl *http.Client) {
	if cl != nil {
		client = cl
	} else {
		client = &http.Client{
			Transport: util.NewBackOffTransport(),
		}
	}
}

// QuerySince holds the start time we have data since.
// Don't consider data before this time. May be due to schema changes, etc.
// Note that the limit is exclusive, this date does not contain good data.
type QuerySince time.Time

// Date returns QuerySince in the YearMonDay format.
func (b QuerySince) Date() string {
	return time.Time(b).Format("20060102")
}

// Unix returns the unix timestamp.
func (b QuerySince) Unix() int64 {
	return time.Time(b).Unix()
}

func NewQuerySince(t time.Time) QuerySince {
	return QuerySince(t)
}

// constructor records the constructor function for each known ingester.
// New ingesters should register their constructors by calling the RegisterIngester
// function.
type constructor struct {
	name string
	f    func() ResultIngester
}

// ingesters is the list of registered ingesters.
var constructors []constructor

// RegisterIngester registers an ingester for use.
// Name is the name of the ingester to use; this need not match the name
//   of the dataset being ingested.  This way a single dataset can have
//   more than one possible ingester.
// Constructor is the function to return the specific type of ingester.

func Register(name string, f func() ResultIngester) {
	constructors = append(constructors, constructor{name, f})
}

// IngesterConstructor searches all registered ingesters for one that matches
// the given name, and returns its associated construction function.

func Constructor(name string) func() ResultIngester {
	for _, reg := range constructors {
		if reg.name == name {
			return reg.f
		}
	}
	glog.Fatalf("Not a registered ingester name: %s", name)
	return func() ResultIngester { return nil }
}

type Opener func() (io.ReadCloser, error)

// ResultIngester needs to be implemented to ingest files. It allows to
// ingest files individually or in a batch mode.
type ResultIngester interface {
	// The provided opener allows to open an input stream, parse it and add it
	// to the tile. The ingestion can also be done when BatchFinished is
	// called. In that case the openers should be cached and opened then.
	Ingest(tt *TileTracker, opener Opener, fileInfo *ResultsFileLocation, counter metrics.Counter) error

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
	tileStore      tiling.TileStore
	storage        *storage.Service
	hashToNumber   map[string]int
	lastIngestTime time.Time
	resultIngester ResultIngester
	config         map[string]string
	datasetName    string
	nCommits       int
	minDuration    time.Duration

	// Keeps track of processed files so we avoid duplicate downloads.
	processedFiles *leveldb.DB

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
func NewIngester(git *gitinfo.GitInfo, tileStoreDir string, datasetName string, ri ResultIngester, nCommits int, minDuration time.Duration, config map[string]string, statusDir, metricName string) (*Ingester, error) {
	var storageService *storage.Service = nil
	var err error = nil
	// check if the ingestion source is coming from Google Storage
	if config["GSDir"] != "" {
		storageService, err = storage.New(client)
		if err != nil {
			return nil, fmt.Errorf("Failed to create interace to Google Storage: %s\n", err)
		}
	}
	var processedFiles *leveldb.DB = nil
	if statusDir != "" {
		statusDir = fileutil.Must(fileutil.EnsureDirExists(filepath.Join(statusDir, datasetName)))
		processedFiles, err = leveldb.OpenFile(filepath.Join(statusDir, "processed_files.ldb"), nil)
		if err != nil {
			glog.Fatalf("Unable to open status db at %s: %s", filepath.Join(statusDir, "processed_files.ldb"), err)
		}
	}

	i := &Ingester{
		git:                            git,
		tileStore:                      filetilestore.NewFileTileStore(tileStoreDir, datasetName, -1),
		storage:                        storageService,
		hashToNumber:                   map[string]int{},
		resultIngester:                 ri,
		config:                         config,
		datasetName:                    datasetName,
		elapsedTimePerUpdate:           newGauge(metricName, "update"),
		metricsProcessed:               newCounter(metricName, "processed"),
		lastSuccessfulUpdate:           time.Now(),
		timeSinceLastSucceessfulUpdate: newGauge(metricName, "time-since-last-successful-update"),
		nCommits:                       nCommits,
		minDuration:                    minDuration,
		processedFiles:                 processedFiles,
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
func (i *Ingester) lastCommitTimeInTile(tile *tiling.Tile) time.Time {
	t := tile.Commits[0].CommitTime
	for i := (len(tile.Commits) - 1); i >= 0; i-- {
		if tile.Commits[i].CommitTime != 0 {
			t = tile.Commits[i].CommitTime
			break
		}
	}
	if time.Unix(t, 0).Before(time.Time(BEGINNING_OF_TIME)) {
		t = BEGINNING_OF_TIME.Unix()
	}
	return time.Unix(t, 0)
}

// TileTracker keeps track of which Tile we are on, and allows moving to new
// Tiles, writing out the current Tile when tiles are changed, and creating new
// Tiles if they don't exist.
type TileTracker struct {
	lastTileNum  int
	currentTile  *tiling.Tile
	tileStore    tiling.TileStore
	hashToNumber map[string]int
}

func NewTileTracker(tileStore tiling.TileStore, hashToNumber map[string]int) *TileTracker {
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
	tileNum := hashNumber / tiling.TILE_SIZE
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
			tt.currentTile = tiling.NewTile()
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
func (tt TileTracker) Tile() *tiling.Tile {
	return tt.currentTile
}

// Offset returns the Value offset of a commit in a Trace.
func (tt TileTracker) Offset(hash string) int {
	return tt.hashToNumber[hash] % tiling.TILE_SIZE
}

// LastTileNum is solely used for testing. See the perfingester package.
func (tt TileTracker) LastTileNum() int { return tt.lastTileNum }

// UpdateCommitInfo finds all the new commits since the last time we ran and
// adds them to the tiles, creating new tiles if necessary.
func (i *Ingester) UpdateCommitInfo(pull bool) error {
	glog.Infof("Ingest %s: Starting UpdateCommitInfo", i.datasetName)
	if err := i.git.Update(pull, false); err != nil {
		return fmt.Errorf("Ingest %s: Failed git pull for during UpdateCommitInfo: %s", i.datasetName, err)
	}

	// Compute Git CL number for each Git hash.
	allHashes := i.git.From(time.Time(BEGINNING_OF_TIME))
	hashToNumber := map[string]int{}
	for i, h := range allHashes {
		hashToNumber[h] = i
	}
	i.hashToNumber = hashToNumber

	// Find the time of the last Commit seen.
	ts := time.Time(BEGINNING_OF_TIME)
	lastTile, err := i.tileStore.Get(0, -1)
	if err == nil && lastTile != nil {
		ts = i.lastCommitTimeInTile(lastTile)
	} else {
		// Boundary condition; just started making Tiles and none exist.
		newTile := tiling.NewTile()
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
		tt.Tile().Commits[tt.Offset(hash)] = &tiling.Commit{
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
// the target time range in every run and therefore considers a large
// number of input files. That is somewhat mitigated by only ingesting
// files we have not seen before, but a future version should clever about
// picking a better timerange.
func (i *Ingester) UpdateTiles() error {
	var err error = nil
	startTS, endTS, err := i.getCommitRangeOfInterest()

	if err != nil {
		return err
	}
	glog.Infof("StartTime (%s): %s", i.datasetName, time.Unix(startTS, 0))
	glog.Infof("EndTime   (%s): %s", i.datasetName, time.Unix(endTS, 0))

	glog.Infof("Ingest %s: Starting UpdateTiles", i.datasetName)

	tt := NewTileTracker(i.tileStore, i.hashToNumber)

	var resultsFiles []*ResultsFileLocation

	if i.config["GSDir"] != "" {
		storageBucket := i.config["GSBucket"]
		storageBaseDir := i.config["GSDir"]

		resultsFiles, err = getGSResultsFileLocations(startTS, endTS, i.storage, storageBucket, storageBaseDir)
	} else if i.config["LocalDir"] != "" {
		resultsFiles, err = getLocalResultsFileLocations(startTS, endTS, i.config["LocalDir"])
	} else {
		return fmt.Errorf("No location for results file specified (missing both GSDir and LocalDir from config)")
	}
	if err != nil {
		return fmt.Errorf("Failed to update tiles: %s", err)
	}

	glog.Infof("Ingest %s: Found %d resultsFiles", i.datasetName, len(resultsFiles))

	processedMD5s := make([]string, 0, len(resultsFiles))
	for _, resultLocation := range resultsFiles {
		if !i.inProcessedFiles(resultLocation.MD5Hash) {
			opener := func() (io.ReadCloser, error) {
				r, err := resultLocation.Fetch()
				if err != nil {
					return nil, fmt.Errorf("Failed to fetch: %s: %s", resultLocation.Name, err)
				}
				return r, nil
			}

			if err := i.resultIngester.Ingest(tt, opener, resultLocation, i.metricsProcessed); err != nil {
				glog.Errorf("Failed to ingest %s: %s", resultLocation.Name, err)
				continue
			}
			// Gather all successfully processed MD5s
			processedMD5s = append(processedMD5s, resultLocation.MD5Hash)
		} else {
			glog.Infof("Skipped duplicate: %s (%s)", resultLocation.Name, resultLocation.MD5Hash)
		}
	}

	// Notify the ingester that the batch has finished and cause it to reset its
	// state and do any pending ingestion.
	if err := i.resultIngester.BatchFinished(i.metricsProcessed); err != nil {
		glog.Errorf("Batchfinished failed (%s): %s", i.datasetName, err)
	} else {
		i.addToProcessedFiles(processedMD5s)
	}

	tt.Flush()

	glog.Infof("Ingest %s: Finished UpdateTiles", i.datasetName)
	return nil
}

// inProcessedFiles returns true if the provided MD5 hash is recorded list of
// processed files.
func (i *Ingester) inProcessedFiles(md5Hash string) bool {
	if i.processedFiles == nil {
		return false
	}

	ret, err := i.processedFiles.Has([]byte(md5Hash), nil)
	if err != nil {
		glog.Errorf("Unable to read processedFiles DB: %s", err)
		return false
	}
	return ret
}

// addToProcessedFiles marks the provided MD5 hashes as processed and stores
// them in the persistent database.
func (i *Ingester) addToProcessedFiles(md5Hashes []string) {
	if i.processedFiles == nil {
		return
	}

	// Only consider the files ingested if Batchfinished succeeded.
	batch := &leveldb.Batch{}
	for _, h := range md5Hashes {
		batch.Put([]byte(h), []byte{})
	}
	err := i.processedFiles.Write(batch, SYNC_WRITE)
	if err != nil {
		glog.Errorf("Error writing processed files db %s", err)
	}
}

// getCommitRangeOfInterest returns the time range (start, end) that
// we are interested in. This method assumes that UpdateCommitInfo
// has been called and therefore reading the tile should not fail.
func (i *Ingester) getCommitRangeOfInterest() (int64, int64, error) {
	// Get the index of the last tile.
	tile, err := i.tileStore.Get(0, -1)
	if err != nil {
		return 0, 0, err
	}

	commitCounter := 1
	lastCommitIdx := tile.LastCommitIndex()
	startCommitTS := tile.Commits[lastCommitIdx].CommitTime
	now := time.Now()

Loop:
	for tileIndex := tile.TileIndex; tileIndex > 0; {
		for cidx := lastCommitIdx; cidx >= 0; cidx-- {
			timeDiff := now.Sub(time.Unix(tile.Commits[cidx].CommitTime, 0))
			startCommitTS = tile.Commits[cidx].CommitTime
			if (commitCounter >= i.nCommits) && (timeDiff >= i.minDuration) {
				break Loop
			}
			commitCounter++
		}

		// Get the next tile.
		tileIndex--
		if tileIndex > 0 {
			tile, err = i.tileStore.Get(0, tileIndex)
			if err != nil {
				return 0, 0, err
			}
			lastCommitIdx = tile.LastCommitIndex()
		}
	}

	return startCommitTS, now.Unix(), nil
}

// HashToNumber is solely implemented for testing. See the perfingester package.
func (i *Ingester) HashToNumber() map[string]int { return i.hashToNumber }

// ResultsFileLocation is the URI of a single JSON file with results in it.
type ResultsFileLocation struct {
	URI         string // Absolute URI used to fetch the file.
	Name        string // Complete path, w/o the gs:// prefix.
	LastUpdated int64  // Timestamp when this was last updated.
	MD5Hash     string // MD5 hash of the content.
}

func NewResultsFileLocation(uri, name string, lastUpdated int64, md5Hash string) *ResultsFileLocation {
	return &ResultsFileLocation{
		URI:         uri,
		Name:        name,
		LastUpdated: lastUpdated,
		MD5Hash:     md5Hash,
	}
}

// Fetch retrieves the file contents.
//
// Callers must call Close() on the returned io.ReadCloser.
func (b ResultsFileLocation) Fetch() (io.ReadCloser, error) {
	if strings.HasPrefix(b.URI, "file://") {
		return os.Open(b.URI[6:])
	} else {
		for i := 0; i < MAX_URI_GET_TRIES; i++ {
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
		return nil, fmt.Errorf("Failed fetching JSON after %d attempts", MAX_URI_GET_TRIES)
	}
}

// getFilesFromGSDir returns a list of URIs to get of the JSON files in the
// given bucket and directory made after the given timestamp.
func getFilesFromGSDir(bucket, dir string, earliestTimestamp int64, storage *storage.Service) ([]*ResultsFileLocation, error) {
	results := []*ResultsFileLocation{}
	glog.Infof("Opening bucket/directory: %s/%s", bucket, dir)

	req := storage.Objects.List(bucket).Prefix(dir).Fields("nextPageToken", "items/updated", "items/md5Hash", "items/mediaLink", "items/name")
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
				results = append(results, NewResultsFileLocation(result.MediaLink, result.Name, updateTimestamp, hex.EncodeToString(md5Bytes)))
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

// GetGSResultsFileLocations retrieves a list of ResultsFileLocations from Cloud Storage, each one
// corresponding to a single JSON file.
func getGSResultsFileLocations(startTS int64, endTS int64, storage *storage.Service, bucket, dir string) ([]*ResultsFileLocation, error) {
	dirs := gs.GetLatestGSDirs(startTS, endTS, dir)
	glog.Infof("getGSResultsFileLocations: Looking in bucket %s and dirs: %v ", bucket, dirs)

	retval := []*ResultsFileLocation{}
	for _, dir := range dirs {
		files, err := getFilesFromGSDir(bucket, dir, startTS, storage)
		if err != nil {
			return nil, err
		}
		retval = append(retval, files...)
	}
	return retval, nil
}

func computeMd5(path string) (string, error) {
	var result []byte
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer util.Close(file)

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(result)), nil
}

func getFilesFromLocalDir(dir string, earliestTimestamp int64) ([]*ResultsFileLocation, error) {
	results := []*ResultsFileLocation{}
	glog.Infof("Looking in local directory %s", dir)

	visit := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		updateDate := info.ModTime()
		updateTimestamp := updateDate.Unix()
		if updateTimestamp > earliestTimestamp {
			hash, err := computeMd5(path)
			if err != nil {
				return fmt.Errorf("Couldn't compute MD5 hash of %s: %v", path, err)
			}
			results = append(results, NewResultsFileLocation(fmt.Sprintf("file://%s", path), info.Name(), updateTimestamp, hash))
		}
		return nil
	}
	if err := filepath.Walk(dir, visit); err != nil {
		return nil, fmt.Errorf("Unable to read the local dir %s: %s", dir, err)
	}

	return results, nil
}

func getLocalResultsFileLocations(startTS, endTS int64, localDir string) ([]*ResultsFileLocation, error) {
	retval := []*ResultsFileLocation{}
	glog.Infof("getLocalResultsFileLocations: Looking in local directory %s", localDir)

	// although GetLatestGSDirs is in the "gs" package, there's nothing specific about
	// its operation that makes it not re-usable here.
	dirs := gs.GetLatestGSDirs(startTS, endTS, localDir)

	for _, dir := range dirs {
		files, err := getFilesFromLocalDir(dir, startTS)
		if err != nil {
			return nil, err
		}
		retval = append(retval, files...)
	}

	return retval, nil
}
