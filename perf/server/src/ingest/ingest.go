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
        "config"
        "gs"
        "types"
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
        // TODO: Will need a mutex around this once hashToCounter is actually used.
        timestampPath = flag.String("timestamp_path", "./timestamp.json", "Path where timestamp data for ingester runs will be stored.")

        importantParameters = map[string][]string{
            // TODO
        }
)

const (
	_BQ_PROJECT_NAME   = "google.com:chrome-skia"
	BEGINNING_OF_TIME  = 1401840000
        FIRST_COMMIT = "3f73e8c8d589e0d5a1f75327b4aa22c1e745732d"
        // One commit before FIRST_COMMIT, used to avoid some one-off errors.
        BEFORE_FIRST_COMMIT = "373dd9b52f88158edd1e24419e6d937efaf59d55"
        MAX_INGEST_FRAGMENT = 4096
)

func Init() {
	metrics.RegisterRuntimeMemStats(metrics.DefaultRegistry)
	go metrics.CaptureRuntimeMemStats(metrics.DefaultRegistry, 1*time.Minute)
	addr, _ := net.ResolveTCPAddr("tcp", "jcgregorio.cnc:2003")
	go metrics.Graphite(metrics.DefaultRegistry, 1*time.Minute, "ingest", addr)
}

type CommitHash string

type SkiaCommitEntry struct {
        // We're getting a lot of nice data here. Should we use more of it?
        Commit      CommitHash `json:"commit"`
}

type SkiaJSON struct {
        Log        []SkiaCommitEntry `json:"log"`
        Next        string          `json:"next"`
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
// for all the commits since that first one.
func getCommits(start string) ([]CommitHash, error) {
        // Unfortunately, skia.googlesource.com only supports going backwards
        // from a given commit, so this will be a little tricky. Basically
        // this will go backwards from HEAD until it finds the page with the
        // desired commit, then append all of the pages together in the right
        // order and return that.
        hashPages := make([]*SkiaJSON, 0, 1)

        indexOfStart := func(s *SkiaJSON) int {
                for i, commit := range s.Log {
                        if commit.Commit == CommitHash(start) {
                                return i
                        }
                }
                return -1
        }
        curPage, err := getCommitPage("HEAD")
        if err != nil {
                return []CommitHash{}, fmt.Errorf("Failed to get first set of commits: %s")
        }
        for indexOfStart(curPage) == -1 {
                hashPages = append(hashPages, curPage)
                curPage, err = getCommitPage(curPage.Next)
                if err != nil {
                        return []CommitHash{}, fmt.Errorf("Failed to get commits for %s: %s", curPage.Next, err)
                }
        }

        // Now copy and return the appropriate commits.
        result := make([]CommitHash, 0)
        for i := indexOfStart(curPage); i >= 0; i-- {
                result = append(result, curPage.Log[i].Commit)
        }
        if len(hashPages) <= 1 {
                return result, nil
        }
        hashPages = hashPages[:len(hashPages)-1]

        // Now copy for all the remaining pages. In reverse!
        for len(hashPages) > 0 {
                curPage = hashPages[len(hashPages)-1]
                for i := len(curPage.Log)-1; i >= 0; i-- {
                        result = append(result, CommitHash(curPage.Log[i].Commit))
                }
                hashPages = hashPages[:len(hashPages)-1]
        }
        return result, nil
}

// updateHashCounterMap updates hashToCounter, starting at FIRST_COMMIT if it's empty.
// TODO: Save hash info to disk.
func updateHashCounterMap() {
        count := -1
        lastCommit := BEFORE_FIRST_COMMIT
        // Get the largest count currently in the latest map, if one exists
        for key, counter := range hashToCounter {
                if counter > count {
                        count = counter
                        lastCommit = string(key)
                }
        }

        // Get all the commits
        commits, err := getCommits(lastCommit)
        if err != nil {
                glog.Errorf("Unable to get new commits: %s\n", err)
                return
        }
        if len(commits) <= 1 {
                glog.Info("No new commits")
                return
        }
        // getCommits includes lastCommit in response, so the first element 
        // needs to be sliced off.
        newCommits := commits[1:]
        count++

        for _, commit := range newCommits {
                hashToCounter[commit] = count
                count++
        }
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
        if result, ok := timestamps[name]; !ok {
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

        timestamps[name] = newTimestamp
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
        Params      map[string]string       `json:"params"`
        IsTrybot    bool                    `json:"isTrybot"`
        Value       float64                 `json:"value"`
        Hash        string                  `json:"gitHash"`

        // This is used to determine which set of parameters it should have/use.
        Dataset     string
}

func NewJSONv2Record(in, dataset string) (*JSONv2Record, error) {
        newRecord := &JSONv2Record {
                Params: make(map[string]string),
                IsTrybot : false,
                Value: 1e+100,
                Hash: "",
        }
        err := json.Unmarshal([]byte(in), newRecord)
        newRecord.Dataset = dataset
        return newRecord, err
}

// Implementation of TileFragment for JSONv2Record.
func (r *JSONv2Record) UpdateTile(t *types.Tile) error {
        // There should be no trybot data in the tile.
        if r.IsTrybot {
                return nil
        }
        counter, exists := hashToCounter[CommitHash(r.Hash)]
        if !exists {
                return fmt.Errorf("Unable to look up commit position for %s", r.Hash)
        }
        if counter < config.TILE_SIZE * t.TileIndex || counter >= config.TILE_SIZE * (t.TileIndex + 1) {
                return fmt.Errorf("UpdateTile called on wrong tile.")
        }
        // Find the trace whose important parameters match those of this fragment.
        var match *types.Trace
        // Again, counting duplicate entries.
        count := 0
        for _, trace := range t.Traces {
                // This for loop will break on nonmatch.
                for _, param := range importantParameters[r.Dataset] {
                        var fragVal string
                        var tileVal string
                        if fragVal, exists = r.Params[param]; !exists {
                                fragVal = ""
                        }
                        if tileVal, exists = trace.Params[param]; !exists {
                                tileVal = ""
                        }
                        if fragVal != tileVal {
                                break
                        }
                }
                // Match!
                count += 1
                // Multiple matches (should be eliminated eventually!)
                if count > 1 {
                        glog.Infoln("Fragment matches multiple entries")
                }
                match = trace
        }
        if match == nil {
                t.Traces = append(t.Traces, types.NewTrace(config.TILE_SIZE))
                match = t.Traces[len(t.Traces) - 1]
                // Populate match.Params, and also see if it uses a new parameter that needs to be added to the tile.ParamSet.
                for _, param := range importantParameters[r.Dataset] {
                        var fragVal string
                        if fragVal, exists := r.Params[param]; exists {
                                match.Params[param] = fragVal
                        } else {
                                match.Params[param] = ""
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
                                t.ParamSet[param] = append(t.ParamSet[param], fragVal)
                        }
                }
        }
        index := counter % config.TILE_SIZE
        match.Values[index] = r.Value

        return nil
}

func (r *JSONv2Record) TileCoordinate() types.TileCoordinate {
        return types.TileCoordinate{
                Scale: 0,
                Commit: r.Hash,
        }
}


// JSONv2FileIter iterates over all the records within a single file.
type JSONv2FileIter struct {
        // record is a slice that contains all the remaining records in the
        // file that still need to be iterated over.
        records     []string
        // currentRecord stores the current iteratee(?)
        currentRecord *JSONv2Record

        //dataset stores the name of the dataset this iterator belongs to
        dataset string
}

// NewJSONv2FileIter retrieve a file from the passed in URI, and splits into separate JSON records.
func NewJSONv2FileIter(uri, dataset string) *JSONv2FileIter {
        newIter := &JSONv2FileIter{
                records: make([]string, 0),
                currentRecord: nil,
                dataset: dataset,
        }
        resp, err := http.Get(uri)
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
        newRecord, err := NewJSONv2Record(fi.records[0], fi.dataset)
        fi.currentRecord = newRecord
        fi.records = fi.records[1:]
        if err != nil {
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
        // uris is a slice that contains all the remaining URIs that need to
        // be retrieved and iterated over.
        uris            []string
        // currentIter contains the current iterator that is being iterated over.
        currentIter     *JSONv2FileIter
        // dataset stores the dataset name this iterator belongs to.
        dataset string
}

// NewJSONv2FilesIter creates a new TileFragmentIter that will iterate over all
// the records in those files.
func NewJSONv2FilesIter(uris []string, dataset string) JSONv2FilesIter {
        return JSONv2FilesIter{
                uris: uris,
                currentIter: nil,
                dataset: dataset,
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


// getStorageService returns a Cloud Storage service.
func getStorageService() (*storage.Service, error) {
	return storage.New(http.DefaultClient)
}

// getFileHash returns the hash if it locates it in the URI, or an empty string and an error if it doesn't.
func getFileHash(uri string) (string, error) {
        dirParts := strings.Split(uri, "/")
        fileName := dirParts[len(dirParts) - 1]
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

// getFiles loads the new files from Cloud Storage into the BigQuery, returning them as a map keyed by git hash.
func getFiles(cs *storage.Service, prefix, sourceBucketSubdir string, timestamp int64) (map[string][]string , error) {
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
func submitFragments(t types.TileStore, iter types.TileFragmentIter) {
        // TODO: Add support for different scales. Currently assumes scale is always zero.
        tileMap := make(map[int][]types.TileFragment)
        count := 0
        for iter.Next() {
                fragment := iter.TileFragment()
                if hashCount, exists := hashToCounter[CommitHash(fragment.TileCoordinate().Commit)]; !exists {
                        glog.Errorf("Commit does not exist in table: %s", fragment.TileCoordinate().Commit)
                        continue
                } else {
                        tileNum := hashCount/config.TILE_SIZE
                        if _, exists := tileMap[tileNum]; !exists {
                                tileMap[tileNum] = make([]types.TileFragment, 0, 1)
                        }
                        tileMap[tileNum] = append(tileMap[tileNum], fragment)
                }
                count += 1
                // Flush the current fragments to the tiles when there's too many.
                if count >= MAX_INGEST_FRAGMENT {
                        for i, fragments := range tileMap {
                                tile, err := t.GetModifiable(0, i)
                                if err != nil {
                                        glog.Errorf("Failed to get tile number %i: %s", i, err)
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
                                        fragment.UpdateTile(tile)
                                }
                                t.Put(0, i, tile)
                                // TODO: writeTimestamp, so that it'll restart at roughly the right
                                // point on sudden failure.
                        }
                        count = 0
                        tileMap = make(map[int][]types.TileFragment)
                }
        }
        // Flush any remaining fragments.
        for i, fragments := range tileMap {
                // NOTE: Same problem as above.
                tile, err := t.Get(0, i)
                if err != nil {
                        glog.Errorf("Failed to get tile number %i: %s", i, err)
                        // TODO: Keep track of failed fragments
                        continue
                }
                for _, fragment := range fragments {
                        fragment.UpdateTile(tile)
                }
                t.Put(0, i, tile)
                // TODO: writeTimestamp, so that it'll restart at roughly the right
                // point on sudden failure.
        }
}

// RunIngester runs a single run of the ingestion cycle (or at least will shortly).
func RunIngester() {
    /*
    cs, err := getStorageService()
    if err != nil {
            glog.Errorf("getFiles failed to create storage service: %s\n", err)
    }
    fileMap, err := getFiles(cs, "micro", "stats-json-v2", BEGINNING_OF_TIME)
    if err != nil {
            glog.Errorf("getFiles failed with error: %s\n", err)
    }
    glog.Infoln(fileMap)
    */
    updateHashCounterMap()
    fmt.Println(hashToCounter)
    time.Sleep(30*time.Minute)
    updateHashCounterMap()
    fmt.Println(hashToCounter)
}
