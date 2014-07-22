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
        "gs"
        "types"
)

var (
        // commitToTile describes the mapping from git hash to scale 0 tile number.
        commitToTile map[string]int
        // hashRegex describes the regex used to capture git commit hashes.
        hashRegex = regexp.MustCompile("[0-9a-f]+")
        // timestampToCount is a map from the git hash to the number of commits
        // after FIRST_COMMIT, starting with FIRST_COMMIT until now.
        // Since each tile contains a set number of commits, the tile number of a
        // commit hash can be found by dividing this number by the number of
        // commits per tile.
        timestampToCounter = make(map[CommitHash]int)
        // TODO: Will need a mutex around this once timestampToCounter is actually used.
        timestampPath = flag.String("timestamp_path", "./timestamp.json", "Path where timestamp data for ingester runs will be stored.")
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

// updateTimestampMap updates timestampToTileID, starting at FIRST_COMMIT if it's empty.
// TODO: Save hash info to disk.
func updateTimestampMap() {
        count := -1
        lastCommit := BEFORE_FIRST_COMMIT
        // Get the largest count currently in the latest map, if one exists
        for key, counter := range timestampToCounter {
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
                timestampToCounter[commit] = count
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
                if tileNum, exists := commitToTile[fragment.TileCoordinate().Commit]; !exists {
                        glog.Errorf("Commit does not exist in table: %s", fragment.TileCoordinate().Commit)
                        continue
                } else {
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
    updateTimestampMap()
    fmt.Println(timestampToCounter)
    time.Sleep(30*time.Minute)
    updateTimestampMap()
    fmt.Println(timestampToCounter)
}
