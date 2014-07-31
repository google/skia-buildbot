package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)
import (
	"code.google.com/p/google-api-go-client/storage/v1"
	"github.com/golang/glog"

	"github.com/rcrowley/go-metrics"
)

import (
	"skia.googlesource.com/buildbot.git/perf/go/config"
	"skia.googlesource.com/buildbot.git/perf/go/filetilestore"
	"skia.googlesource.com/buildbot.git/perf/go/gs"
	"skia.googlesource.com/buildbot.git/perf/go/types"
)

var (
	// hashRegex describes the regex used to capture git commit hashes.
	hashRegex = regexp.MustCompile("[0-9a-f]+")
	// hashToCounter is a map from the git hash to the number of commits
	// after FIRST_COMMIT, starting with FIRST_COMMIT until now.
	// Since each tile contains a set number of commits, the tile number of a
	// commit hash can be found by dividing this number by the number of
	// commits per tile.
	hashToCounter = make(map[CommitHash]int)
	// Not going to mutex because I'll just ensure it's only updated while
	// it's not being used.

	timestampPath = flag.String("timestamp_path", "./timestamp.json", "Path where timestamp data for ingester runs will be stored.")
	tileDir       = flag.String("tiles_dir", "/tmp/test/", "Path where tiles will be placed.")
)

// DatasetMetrics stores all the dataset metrics for a single dataset
type DatasetMetrics struct {
	// Time spent processing a single fragment
	elapsedTimePerFragment metrics.Timer
	// Time spent processing a single JSON record
	elapsedTimePerJSONRecord metrics.Timer
	// Time spent performing a tilestore.Put()
	elapsedTimePerTileFlush metrics.Timer
}

func NewDatasetMetrics(dataset config.DatasetName) *DatasetMetrics {
	datasetStr := string(dataset)
	return &DatasetMetrics{
		elapsedTimePerFragment:   metrics.NewRegisteredTimer(fmt.Sprintf("ingester.%s.time_per_fragment", datasetStr), metrics.DefaultRegistry),
		elapsedTimePerJSONRecord: metrics.NewRegisteredTimer(fmt.Sprintf("ingester.%s.time_per_json", datasetStr), metrics.DefaultRegistry),
		elapsedTimePerTileFlush:  metrics.NewRegisteredTimer(fmt.Sprintf("ingester.%s.time_per_write", datasetStr), metrics.DefaultRegistry),
	}
}

var (
	datasetMetrics = make(map[config.DatasetName]*DatasetMetrics)
)

const (
	_BQ_PROJECT_NAME  = "google.com:chrome-skia"
	BEGINNING_OF_TIME = 1401840000
	//FIRST_COMMIT = "4962140c9e6623b29417a2fb9ad903641fb0159c"
	// One commit before FIRST_COMMIT, used to avoid some one-off errors.
	//BEFORE_FIRST_COMMIT = "df1640d413c16abf4527960642aca41581808699"
	MAX_INGEST_FRAGMENT = 32768
)

func Init() {
	metrics.RegisterRuntimeMemStats(metrics.DefaultRegistry)
	go metrics.CaptureRuntimeMemStats(metrics.DefaultRegistry, 1*time.Minute)
	addr, _ := net.ResolveTCPAddr("tcp", "jcgregorio.cnc:2003")
	go metrics.Graphite(metrics.DefaultRegistry, 1*time.Minute, "ingest", addr)

	for _, dataset := range config.ALL_DATASET_NAMES {
		datasetMetrics[dataset] = NewDatasetMetrics(dataset)
	}
}

type CommitHash string

type SkiaCommitAuthor struct {
	Name string `json:"name"`
	Time string `json:"time"`
}

type SkiaCommitEntry struct {
	// We're getting a lot of nice data here. Should we use more of it?
	Commit  CommitHash       `json:"commit"`
	Author  SkiaCommitAuthor `json:"author"`
	Message string           `json:"message"`
}

type SkiaJSON struct {
	Log  []SkiaCommitEntry `json:"log"`
	Next string            `json:"next"`
}

func (s SkiaCommitEntry) UpdateTile(t *types.Tile) error {
	if count, exists := hashToCounter[s.Commit]; exists && count >= t.TileIndex*config.TILE_SIZE && count < (1+t.TileIndex)*config.TILE_SIZE {
		pos := count % config.TILE_SIZE
		if len(t.Commits) < config.TILE_SIZE {
			curLen := len(t.Commits)
			t.Commits = append(t.Commits, make([]*types.Commit, config.TILE_SIZE-len(t.Commits))...)
			for i := curLen; i < len(t.Commits); i++ {
				t.Commits[i] = types.NewCommit()
			}
		}
		commitTime, err := time.Parse("Mon Jan 2 15:04:05 2006 -0700", s.Author.Time)
		if err != nil {
			return fmt.Errorf("Unable to convert git time to Unix time: %s", err)
		}
		t.Commits[pos] = &types.Commit{
			CommitTime:    commitTime.Unix(),
			Hash:          string(s.Commit),
			GitNumber:     -1,
			Author:        s.Author.Name,
			CommitMessage: s.Message,
			TailCommits:   make([]*types.Commit, 0),
		}
		return nil
	}
	glog.Warningln("Commit entry fragment called on wrong tile.")
	return nil
}

func (s SkiaCommitEntry) TileCoordinate() types.TileCoordinate {
	return types.TileCoordinate{
		Scale:  0,
		Commit: string(s.Commit),
	}
}

// getCommitPage gets a single page of commits from GoogleSource.
func getCommitPage(start string) (*SkiaJSON, error) {
	uriName := "https://skia.googlesource.com/skia/+log/" + start + "?format=json"
	glog.Infoln("Looking at commits: " + uriName)
	resp, err := http.Get(uriName)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve Skia JSON starting with hash %s: %s\n", start, err)
	}
	defer resp.Body.Close()
	result := new(SkiaJSON)
	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	rawJSON := buf.Bytes()
	// The JSON has some garbage on the first line that stops it from parsing.
	// This removes that.
	maybeStrip := bytes.IndexAny(rawJSON, "\n")
	if maybeStrip >= 0 {
		rawJSON = rawJSON[maybeStrip:]
	}
	err = json.Unmarshal(rawJSON, result)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse Skia JSON: %s\n", err)
	}
	return result, nil
}

// getCommits looks up the commits starting with start, and returns an array
// for all the commit hashes, along with an array of all the TileFragments
// representing the data for the commits.
func getCommits(startTimestamp int64) ([]CommitHash, []types.TileFragment, error) {
	// Unfortunately, skia.googlesource.com only supports going backwards
	// from a given commit, so this will be a little tricky. Basically
	// this will go backwards from HEAD until it finds the page with the
	// desired commit, then append all of the pages together in the right
	// order and return that.
	glog.Infoln("Getting commits...")
	hashPages := make([]*SkiaJSON, 0, 1)

	indexOfStart := func(s *SkiaJSON) int {
		startTime := time.Unix(startTimestamp, 0)
		for i, commit := range s.Log {
			commitTime, err := time.Parse("Mon Jan 2 15:04:05 2006 -0700", commit.Author.Time)
			if err == nil && !commitTime.After(startTime) {
				return i
			}
		}
		return -1
	}
	curPage, err := getCommitPage("HEAD")
	if err != nil {
		return []CommitHash{}, []types.TileFragment{}, fmt.Errorf("Failed to get first set of commits: %s")
	}
	for indexOfStart(curPage) == -1 {
		hashPages = append(hashPages, curPage)
		curPage, err = getCommitPage(curPage.Next)
		if err != nil {
			return []CommitHash{}, []types.TileFragment{}, fmt.Errorf("Failed to get commits for %s: %s", curPage.Next, err)
		}
	}

	// Now copy and return the appropriate commits.
	result := make([]CommitHash, 0)
	fragments := make([]types.TileFragment, 0)
	for i := indexOfStart(curPage); i >= 0; i-- {
		result = append(result, curPage.Log[i].Commit)
		fragments = append(fragments, curPage.Log[i])
	}
	if len(hashPages) <= 0 {
		return result, fragments, nil
	}

	// Now copy for all the remaining pages. In reverse!
	for len(hashPages) > 0 {
		curPage = hashPages[len(hashPages)-1]
		for i := len(curPage.Log) - 1; i >= 0; i-- {
			result = append(result, CommitHash(curPage.Log[i].Commit))
			fragments = append(fragments, curPage.Log[i])
		}
		hashPages = hashPages[:len(hashPages)-1]
	}
	return result, fragments, nil
}

// updateHashCounterMap updates hashToCounter, starting at FIRST_COMMIT if it's empty.
// It returns a list of tile fragments that will store the commit data.
// TODO: Save hash info to disk.
func updateHashCounterMap() []types.TileFragment {

	count := -1

	// Get all the commits
	lastTimestamp, _ := readTimestamp("lastHashCounterUpdate")
	commits, fragments, err := getCommits(lastTimestamp)
	if err != nil {
		glog.Errorf("Unable to get new commits: %s\n", err)
		return []types.TileFragment{}
	}
	if len(commits) <= 1 {
		glog.Info("No new commits")
		return []types.TileFragment{}
	}
	// getCommits includes lastCommit in response, so the first element
	// needs to be sliced off.
	newCommits := commits[1:]
	count++

	for _, commit := range newCommits {
		hashToCounter[commit] = count
		count++
	}

	return fragments
}

// readTimestamp reads the local timestamp file and returns the entry it was asked for.
// This file is used to keep record of the last time the ingester was run, so the
// next run over looks for files that occurred after this run.
// If an entry doesn't exist it returns BEGINNING_OF_TIME, and an error
// and BEGINNING_OF_TIME if something else fails in the process.

// Timestamp files look something like:
// {
//      "micro":1445363563,
//      "skps":1445363453
// }
func readTimestamp(name string) (int64, error) {
	timestampFile, err := os.Open(*timestampPath)
	if err != nil {
		return BEGINNING_OF_TIME, fmt.Errorf("Failed to read file %s: %s", *timestampPath, err)
	}
	defer timestampFile.Close()
	var timestamps map[string]int64
	err = json.NewDecoder(timestampFile).Decode(&timestamps)
	if err != nil {
		return BEGINNING_OF_TIME, fmt.Errorf("Failed to parse file %s: %s", *timestampPath, err)
	}
	if result, ok := timestamps[string(name)]; !ok {
		return BEGINNING_OF_TIME, nil
	} else {
		return result, nil
	}
}

// writeTimestamp reads the local timestamp file, adds an entry with the given name and value,
// and writes the file back to disk.
func writeTimestamp(name string, newTimestamp int64) error {
	var timestamps map[string]int64
	timestampFile, err := os.Open(*timestampPath)
	if err != nil {
		// File probably doesn't exist, so we'll use an empty dictionary.
		glog.Warningf("Failed to read file %s: %s", *timestampPath, err)
		timestamps = make(map[string]int64)
	} else {
		err = json.NewDecoder(timestampFile).Decode(&timestamps)
		if err != nil {
			return fmt.Errorf("Failed to parse file %s: %s", *timestampPath, err)
		}
	}
	timestampFile.Close()

	timestamps[string(name)] = newTimestamp
	writeTimestampFile, err := os.Create(*timestampPath)
	if err != nil {
		return fmt.Errorf("Failed to open file %s for writing: %s", *timestampPath, err)
	}
	err = json.NewEncoder(writeTimestampFile).Encode(&timestamps)
	writeTimestampFile.Close()
	if err != nil {
		return fmt.Errorf("Failed to write to file %s: %s", *timestampPath, err)
	}
	return nil
}

// JSONv2Record stores data from a single record in a JSON v2 file. This is the
// format for the JSON stored in {stats,pictures}-json-v2 Google Storage files.
type JSONv2Record struct {
	Params map[string]interface{} `json:"params"`
	// There are two trybot flags because the JSON format
	// changed from using one of these to the other one. Thus, we now have
	// two fields to make sure we capture at least one of them.
	// This should stop being a problem when we migrate over to JSON v3.
	IsTrybot  bool    `json:"isTrybot"`
	IsTrybot2 bool    `json:"is_trybot"`
	Value     float64 `json:"value"`
	Hash      string  `json:"gitHash"`

	// This is used to determine which set of parameters it should have/use.
	Dataset config.DatasetName
}

func NewJSONv2Record(in string, dataset config.DatasetName) (*JSONv2Record, error) {
	newRecord := &JSONv2Record{
		Params:    make(map[string]interface{}),
		IsTrybot:  false,
		IsTrybot2: false,
		Value:     1e+100,
		Hash:      "",
	}
	err := json.Unmarshal([]byte(in), newRecord)
	newRecord.Dataset = dataset
	return newRecord, err
}

// Implementation of TileFragment for JSONv2Record.
func (r *JSONv2Record) UpdateTile(t *types.Tile) error {
	// There should be no trybot data in the tile.
	if r.IsTrybot || r.IsTrybot2 {
		return nil
	}
	counter, exists := hashToCounter[CommitHash(r.Hash)]
	if !exists {
		return fmt.Errorf("Unable to look up commit position for %s", r.Hash)
	}
	if counter < config.TILE_SIZE*t.TileIndex || counter >= config.TILE_SIZE*(t.TileIndex+1) {
		return fmt.Errorf("UpdateTile called on wrong tile.")
	}

	fragmentKey := types.MakeTraceKey(r.Params, r.Dataset)

	var match *types.Trace
	if match, exists = t.Traces[fragmentKey]; !exists {
		match = types.NewTrace()
		t.Traces[fragmentKey] = match
		// See if it uses a new parameter that needs to be added to the tile.ParamSet.
		for _, param := range config.KEY_PARAM_ORDER[r.Dataset] {
			var fragVal string
			if rawFragVal, exists := r.Params[param]; !exists {
				fragVal = ""
			} else {
				fragVal = fmt.Sprint(rawFragVal)
			}
			if _, exists = t.ParamSet[param]; !exists {
				t.ParamSet[param] = make([]string, 0, 1)
			}
			alreadyExists := false
			for _, paramVal := range t.ParamSet[param] {
				if paramVal == fragVal {
					alreadyExists = true
					break
				}
			}
			if !alreadyExists {
				glog.Info("Adding new param..")
				t.ParamSet[param] = append(t.ParamSet[param], fragVal)
			}
		}
	}
	index := counter % config.TILE_SIZE
	if match.Values[index] == config.MISSING_DATA_SENTINEL {
		match.Values[index] = r.Value
	} else {
		glog.Infof("Duplicate entry found for %s, hash %s", string(fragmentKey), r.Hash)
	}

	return nil
}

func (r *JSONv2Record) TileCoordinate() types.TileCoordinate {
	return types.TileCoordinate{
		Scale:  0,
		Commit: r.Hash,
	}
}

// JSONv2FileIter iterates over all the records within a single file.
type JSONv2FileIter struct {
	// record is a slice that contains all the remaining records in the
	// file that still need to be iterated over.
	records []string
	// currentRecord stores the current iteratee(?)
	currentRecord *JSONv2Record

	//dataset stores the name of the dataset this iterator belongs to
	dataset config.DatasetName
}

// NewJSONv2FileIter retrieve a file from the passed in URI, and splits into separate JSON records.
// NOTE: The passed in URI is assumed to be in public access Google Storage
func NewJSONv2FileIter(uri string, dataset config.DatasetName) *JSONv2FileIter {
	glog.Infof("Creating new JSONv2FileIter for %s\n", uri)
	newIter := &JSONv2FileIter{
		records:       make([]string, 0),
		currentRecord: nil,
		dataset:       dataset,
	}
	request, err := gs.RequestForStorageURL(uri)
	if err != nil {
		glog.Errorf("Unable to create Storage MediaURI request: %s\n", err)
		return newIter
	}
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		glog.Errorf("Unable to retrieve URI while creating file iterator: %s", err)
		return newIter
	}
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		glog.Errorf("Unable to read from response body while creating file iterator: %s", err)
		return newIter
	}
	newIter.records = strings.Split(buf.String(), "\n")
	return newIter
}

// Implementation of TileFragmentIter for JSONv2FileIter.
// There really isn't a need for this to use this interface here, since for now
// it'll always be wrapped in a JSONv2FilesIter, but it does work out to be
// pretty convenient to do so anyways.
func (fi *JSONv2FileIter) Next() bool {
	if len(fi.records) <= 0 {
		return false
	}
	curJSON := fi.records[0]
	fi.records = fi.records[1:]
	// Grab the next one if the current one's blank.
	if curJSON == "" {
		return fi.Next()
	}
	newRecord, err := NewJSONv2Record(curJSON, fi.dataset)
	fi.currentRecord = newRecord
	if err != nil {
		glog.Infof("Error while parsing record: %s\n", err)
		glog.Infoln("JSON dump:")
		glog.Infoln(curJSON)
		return fi.Next()
		// See note in JSVONv2FilesIter.Next(); same deal.
	}
	return true
}

func (fi *JSONv2FileIter) TileFragment() types.TileFragment {
	return fi.currentRecord
}

// JSONv2FilesIter iterates over all the records within a group of files.
// It basically wraps around JSONv2FileIter to allow for seamless iteration
// over a group of GS files.
type JSONv2FilesIter struct {
	// currentIter contains the current iterator that is being iterated over.
	currentIter *JSONv2FileIter
	// dataset stores the dataset name this iterator belongs to.
	dataset config.DatasetName
	// uris stores the URIs left to retrieve and parse
	uris []string
}

// NewJSONv2FilesIter creates a new TileFragmentIter that will iterate over all
// the records in those files.
func NewJSONv2FilesIter(uris []string, dataset config.DatasetName) JSONv2FilesIter {
	return JSONv2FilesIter{
		uris:        uris,
		currentIter: nil,
		dataset:     dataset,
	}
}

// Implementation of TileFragmentIter for JSONv2FilesIter.
func (fsi *JSONv2FilesIter) Next() bool {
	if fsi.currentIter != nil && fsi.currentIter.Next() == true {
		return true
	}
	if len(fsi.uris) <= 0 {
		return false
	}
	fsi.currentIter = NewJSONv2FileIter(fsi.uris[0], fsi.dataset)
	fsi.uris = fsi.uris[1:]
	// NOTE: May cause a stack overflow (do those exist in Go?) if there
	// are a LOT of files queued, and all of them are somehow bad.
	// This could be fixed by adding a for loop here to keep trying
	// to get an iterator from each URI off the list, but I really like
	// the simplicity of using a recursive tail call instead.
	return fsi.Next()
}

func (fsi *JSONv2FilesIter) TileFragment() types.TileFragment {
	if fsi.currentIter != nil {
		return fsi.currentIter.TileFragment()
	}
	return nil
}

// FragmentArrayIter wraps an iterator around a slice of TileFragments.
type FragmentArrayIter struct {
	fragments []types.TileFragment
	curPos    int
}

func NewFragmentArrayIter(fragments []types.TileFragment) *FragmentArrayIter {
	return &FragmentArrayIter{
		fragments: fragments,
		curPos:    -1,
	}
}

func (fai *FragmentArrayIter) Next() bool {
	fai.curPos++
	return fai.curPos < len(fai.fragments)
}

func (fai *FragmentArrayIter) TileFragment() types.TileFragment {
	if fai.curPos >= 0 && fai.curPos < len(fai.fragments) {
		return fai.fragments[fai.curPos]
	}
	return nil
}

// getStorageService returns a Cloud Storage service.
func getStorageService() (*storage.Service, error) {
	return storage.New(http.DefaultClient)
}

// getFileHash returns the hash if it locates it in the URI, or an empty string and an error if it doesn't.
func getFileHash(uri string) (string, error) {
	dirParts := strings.Split(uri, "/")
	fileName := dirParts[len(dirParts)-1]
	fileNameParts := strings.Split(uri, "_")
	for _, part := range fileNameParts {
		maybeHash := hashRegex.FindString(part)
		if len(maybeHash) > 0 {
			return maybeHash, nil
		}
	}
	return "", fmt.Errorf("Failed to find git hash in file name %s\n", fileName)
}

// getFilesFromGSDir returns a list of URIs to get of the JSON files in the
// given bucket and directory made after the given timestamp.
func getFilesFromGSDir(service *storage.Service, directory string, bucket string, lowestTimestamp int64) []string {
	results := make([]string, 0)
	glog.Infoln("Opening directory", directory, "of bucket", bucket)
	req := service.Objects.List(bucket).Prefix(directory)

	for req != nil {
		resp, err := req.Do()
		if err != nil {
			glog.Errorln("Error occurred while getting files: ", err)
			break
		}
		for _, result := range resp.Items {
			updateDate, _ := time.Parse(time.RFC3339, result.Updated)
			updateTimestamp := updateDate.Unix()
			if updateTimestamp > lowestTimestamp {
				results = append(results, result.MediaLink)
			}
		}
		if len(resp.NextPageToken) > 0 {
			req.PageToken(resp.NextPageToken)
		} else {
			req = nil
		}
	}
	return results
}

// getFiles retrieves a list of files from Cloud Storage, returning them as a map keyed by git hash.
func getFiles(cs *storage.Service, prefix, sourceBucketSubdir string, timestamp int64) (map[string][]string, error) {
	var realTimestamp int64
	if timestamp < BEGINNING_OF_TIME {
		realTimestamp = BEGINNING_OF_TIME
	} else {
		realTimestamp = timestamp
	}
	glog.Infoln("Start of getFiles, subdir = ", sourceBucketSubdir)
	glog.Infoln("Using timestamp: ", realTimestamp)
	// Get all the JSON files with a timestamp after that
	dirs := gs.GetLatestGSDirs(realTimestamp, time.Now().Unix(), sourceBucketSubdir)
	glog.Infoln("Looking in dirs: ", dirs)

	jsonUris := make(map[string][]string)
	for _, dir := range dirs {
		files := getFilesFromGSDir(cs, dir, gs.GS_PROJECT_BUCKET, realTimestamp)
		for _, fileName := range files {
			fileHash, err := getFileHash(fileName)
			if err != nil {
				glog.Errorf("Unable to extract hash from %s: %s", fileName, err)
			}
			if _, ok := jsonUris[fileHash]; !ok {
				jsonUris[fileHash] = make([]string, 0, 1)
			}
			jsonUris[fileHash] = append(jsonUris[fileHash], fileName)
			glog.Infoln(fileName, "added to list")
		}
	}
	return jsonUris, nil
}

// submitFragments takes a list of fragments and applies them to the appropriate tiles in TileStore.
func submitFragments(t types.TileStore, iter types.TileFragmentIter, dataset config.DatasetName) {
	// TODO: Add support for different scales. Currently assumes scale is always zero.
	tileMap := make(map[int][]types.TileFragment)
	count := 0

	startTime := time.Now()
	startJSONTime := time.Now()
	for iter.Next() {
		// Time how long it takes to get process the new JSON fragmnet.
		datasetMetrics[dataset].elapsedTimePerJSONRecord.Update(time.Since(startJSONTime))

		fragment := iter.TileFragment()
		if hashCount, exists := hashToCounter[CommitHash(fragment.TileCoordinate().Commit)]; !exists {
			glog.Errorf("Commit does not exist in table: %s", fragment.TileCoordinate().Commit)
			continue
		} else {
			tileNum := hashCount / config.TILE_SIZE
			if _, exists := tileMap[tileNum]; !exists {
				tileMap[tileNum] = make([]types.TileFragment, 0, 1)
			}
			tileMap[tileNum] = append(tileMap[tileNum], fragment)
		}
		count += 1

		// Flush the current fragments to the tiles when there's too many.
		if count >= MAX_INGEST_FRAGMENT {
			glog.Infoln("Submitting fragments..")
			for i, fragments := range tileMap {
				glog.Infof("Writing to tile %d\n", i)
				tile, err := t.GetModifiable(0, i)
				if err != nil {
					glog.Errorf("Failed to get tile number %d: %s", i, err)
					// TODO: Keep track of failed fragments
					continue
				}
				// If the tile didn't exist before, make a new one.
				if tile == nil {
					tile = types.NewTile()
					tile.Scale = 0
					tile.TileIndex = i
				}
				for _, fragment := range fragments {
					startFragmentTime := time.Now()
					fragment.UpdateTile(tile)
					datasetMetrics[dataset].elapsedTimePerJSONRecord.Update(time.Since(startFragmentTime))
				}

				// Measure how long it takes to Put() a tile.
				startTileTime := time.Now()
				t.Put(0, i, tile)
				datasetMetrics[dataset].elapsedTimePerTileFlush.Update(time.Since(startTileTime))

				// TODO: writeTimestamp, so that it'll restart at roughly the right
				// point on sudden failure.
			}
			count = 0
			tileMap = make(map[int][]types.TileFragment)
		}

		startJSONTime = time.Now()
	}
	glog.Infoln("Submitting remaining fragments..")
	// Flush any remaining fragments.
	for i, fragments := range tileMap {
		tile, err := t.GetModifiable(0, i)
		glog.Infof("Writing to tile %d\n", i)
		if err != nil {
			glog.Errorf("Failed to get tile number %d: %s", i, err)
			// TODO: Keep track of failed fragments
			continue
		}
		for _, fragment := range fragments {
			startFragmentTime := time.Now()
			fragment.UpdateTile(tile)
			datasetMetrics[dataset].elapsedTimePerJSONRecord.Update(time.Since(startFragmentTime))
		}
		startTileTime := time.Now()
		t.Put(0, i, tile)
		if err != nil {
			glog.Errorf("Failed to store tile %d: %s", i, err)
		}
		datasetMetrics[dataset].elapsedTimePerTileFlush.Update(time.Since(startTileTime))
	}
	writeTimestamp(string(dataset), startTime.Unix())
}

// IngestForDataset runs the ingestion pipeline for the given dataset, using JSON files from gs_subdir,
// accessing the JSON via cs, and updating the tiles with the fragments passed in otherFragments.
// It uses readTimestamp and writeTimestamp to keep track of how much of the data
// has already been written.
func IngestForDataset(dataset config.DatasetName, gs_subdir string, otherFragments types.TileFragmentIter, cs *storage.Service) {
	timestamp, err := readTimestamp(string(dataset))
	if err != nil {
		glog.Infof("Error while reading timestamp: %s", err)
	}
	newTimestamp := time.Now().Unix()
	fileMap, err := getFiles(cs, string(dataset), gs_subdir, timestamp)
	if err != nil {
		glog.Errorf("getFiles failed with error: %s\n", err)
	}
	// Flatten the map. There's probably some benefit to sorting it by tile,
	// but for now we'll just flatten it.
	uriListSize := 0
	for _, uris := range fileMap {
		uriListSize += len(uris)
	}
	allUris := make([]string, 0, uriListSize)
	for _, uris := range fileMap {
		allUris = append(allUris, uris...)
	}
	//glog.Infoln("Final files list: ", allUris)
	filesIter := NewJSONv2FilesIter(allUris, dataset)
	datasetTilestore := filetilestore.NewFileTileStore(*tileDir, string(dataset), -1)
	submitFragments(datasetTilestore, otherFragments, dataset)
	submitFragments(datasetTilestore, &filesIter, dataset)
	glog.Infoln("Fragment submission finished. Writing timestamp..")
	writeTimestamp(string(dataset), newTimestamp)
}

// RunIngester runs a single run of the ingestion cycle (or at least will shortly).
// prefixMapping maps from the dataset name to the GS directory where that data
// is stored.
func RunIngester(prefixMappings map[config.DatasetName]string) {
	glog.Infoln("Ingestion run started.")
	fragments := updateHashCounterMap()
	//glog.Infoln(hashToCounter)
	cs, err := getStorageService()
	if err != nil {
		glog.Errorf("getFiles failed to create storage service: %s\n", err)
	}
	for dataset, gsSubdir := range prefixMappings {
		// Make sure the hash counter map is as close to up to date as it can be.
		fragments = append(fragments, updateHashCounterMap()...)
		IngestForDataset(dataset, gsSubdir, NewFragmentArrayIter(fragments), cs)
	}
	glog.Infoln("Ingestion run ended.")
}
