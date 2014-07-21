package main

import (
	"fmt"
	"net"
	"net/http"
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
)

const (
	_BQ_PROJECT_NAME   = "google.com:chrome-skia"
	BEGINNING_OF_TIME  = 1401840000
        MAX_INGEST_FRAGMENT = 4096
)

func Init() {
	metrics.RegisterRuntimeMemStats(metrics.DefaultRegistry)
	go metrics.CaptureRuntimeMemStats(metrics.DefaultRegistry, 1*time.Minute)
	addr, _ := net.ResolveTCPAddr("tcp", "jcgregorio.cnc:2003")
	go metrics.Graphite(metrics.DefaultRegistry, 1*time.Minute, "ingest", addr)
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

// getLatestGSDirs gets the appropriate directory names in which data
// would be stored between the given timestamp and now.
func getLatestGSDirs(timestamp int64, bsSubdir string) []string {
	oldTime := time.Unix(timestamp, 0).UTC()
	glog.Infoln("Old time: ", oldTime)
	newTime := time.Now().UTC()
	lastAddedTime := oldTime
	results := make([]string, 0)
	newYear, newMonth, newDay := newTime.Date()
	newHour := newTime.Hour()
	lastYear, lastMonth, _ := lastAddedTime.Date()
	if lastYear != newYear {
		for i := lastMonth; i < 12; i++ {
			results = append(results, fmt.Sprintf("%04d/%02d", lastYear, lastMonth))
		}
		for i := lastYear + 1; i < newYear; i++ {
			results = append(results, fmt.Sprintf("%04d", i))
		}
		lastAddedTime = time.Date(newYear, 0, 1, 0, 0, 0, 0, time.UTC)
	}
	lastYear, lastMonth, _ = lastAddedTime.Date()
	if lastMonth != newMonth {
		for i := lastMonth; i < newMonth; i++ {
			results = append(results, fmt.Sprintf("%04d/%02d", lastYear, i))
		}
		lastAddedTime = time.Date(newYear, newMonth, 1, 0, 0, 0, 0, time.UTC)
	}
	lastYear, lastMonth, lastDay := lastAddedTime.Date()
	if lastDay != newDay {
		for i := lastDay; i < newDay; i++ {
			results = append(results, fmt.Sprintf("%04d/%02d/%02d", lastYear, lastMonth, i))
		}
		lastAddedTime = time.Date(newYear, newMonth, newDay, 0, 0, 0, 0, time.UTC)
	}
	lastYear, lastMonth, lastDay = lastAddedTime.Date()
	lastHour := lastAddedTime.Hour()
	for i := lastHour; i < newHour+1; i++ {
		results = append(results, fmt.Sprintf("%04d/%02d/%02d/%02d", lastYear, lastMonth, lastDay, i))
	}
	for i := range results {
		results[i] = fmt.Sprintf("%s/%s", bsSubdir, results[i])
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
                                // NOTE: This seems wrong. The version of tile that's being modified
                                // is the version stored in cache. We're breaking thread safety
                                // without any hint in the code that we are, because it seems
                                // like Get() assumes the tile it sends will not be modified.
                                // Should Get() return a deep copy of the Tile, or should we
                                // make a deep copy here?
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
    cs, err := getStorageService()
    if err != nil {
            glog.Errorf("getFiles failed to create storage service: %s\n", err)
    }
    fileMap, err := getFiles(cs, "micro", "stats-json-v2", BEGINNING_OF_TIME)
    if err != nil {
            glog.Errorf("getFiles failed with error: %s\n", err)
    }
    glog.Infoln(fileMap)
}
