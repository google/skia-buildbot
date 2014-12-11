// tiletool is a command line application to validate a tile store.
package main

import (
	"bytes"
	"crypto/md5"
	"encoding/gob"
	"flag"
	"fmt"
	"time"

	"sort"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"skia.googlesource.com/buildbot.git/go/common"
	"skia.googlesource.com/buildbot.git/go/util"
	"skia.googlesource.com/buildbot.git/perf/go/config"
	"skia.googlesource.com/buildbot.git/perf/go/filetilestore"
	"skia.googlesource.com/buildbot.git/perf/go/types"
	"skia.googlesource.com/buildbot.git/perf/go/validator"
)

// Commands
const (
	VALIDATE     = "validate"
	DUMP_COMMITS = "dump"
	MD5          = "md5"
)

// Command line flags.
var (
	tileDir    = flag.String("tile_dir", "/tmp/tileStore", "What directory to look for tiles in.")
	verbose    = flag.Bool("verbose", false, "Verbose.")
	echoHashes = flag.Bool("echo_hashes", false, "Echo Git hashes during validation.")
	dataset    = flag.String("dataset", config.DATASET_NANO, fmt.Sprintf("Choose from the valid datasets: %v", config.VALID_DATASETS))
)

func dumpCommits(tile *types.Tile, n int) {
	tileLen := tile.LastCommitIndex() + 1
	commits := tile.Commits[:tileLen]

	if n <= 0 {
		n = tileLen
	}
	startIdx := tileLen - n

	// Keep track of empty traces.
	notEmpty := map[string]bool{}

	for i := startIdx; i < tileLen; i++ {
		count := 0
		for traceKey, v := range tile.Traces {
			gTrace := v.(*types.GoldenTrace)
			if gTrace.Values[i] != types.MISSING_DIGEST {
				count++
				notEmpty[traceKey] = true
			}
		}
		commit := commits[i]

		// This works because a hash is always ascii.
		outHash := commit.Hash[:20]
		fmt.Printf("%v: %5d/%5d : %s : %s \n", time.Unix(commit.CommitTime, 0), count, len(tile.Traces), outHash, commit.Author)
	}

	fmt.Printf("Total Commits   : %d\n", tileLen)
	fmt.Printf("Non-empty traces: %d\n", len(notEmpty))
}

func getBytes(key interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(key)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func md5Commits(store types.TileStore, targetHash string, nCommits int) {
	tile, err := store.Get(0, -1)
	if err != nil {
		glog.Fatal("Could not read tile: " + err.Error())
	}

	tileLen := tile.LastCommitIndex() + 1
	commits := tile.Commits[:tileLen]

	// Find the target index.
	endIdx := -1
	for i, v := range commits {
		if strings.HasPrefix(v.Hash, targetHash) {
			endIdx = i
			break
		}
	}
	if endIdx == -1 {
		glog.Fatalf("Unable to find commit %s", targetHash)
	}

	endIdx++
	startIdx := endIdx - nCommits

	traceKeys := make([]string, 0, len(tile.Traces))
	for k := range tile.Traces {
		gTrace := tile.Traces[k].(*types.GoldenTrace)
		for _, val := range gTrace.Values[startIdx:endIdx] {
			// Only consider traces that are not empty
			if val != types.MISSING_DIGEST {
				traceKeys = append(traceKeys, k)
				break
			}
		}
	}
	sort.Strings(traceKeys)

	result := make([][]string, len(traceKeys))
	for i, k := range traceKeys {
		gTrace := tile.Traces[k].(*types.GoldenTrace)
		result[i] = gTrace.Values[startIdx:endIdx]
	}

	byteStr, err := getBytes(result)
	if err != nil {
		glog.Fatalf("Unable to serialize to bytes: %s", err.Error())
	}

	md5Hash := fmt.Sprintf("%x", md5.Sum(byteStr))

	fmt.Printf("Commit Range    : %s - %s\n", commits[startIdx].Hash, commits[endIdx-1].Hash)
	fmt.Printf("Hash            : %s\n", md5Hash)
	fmt.Printf("Total     traces: %d\n", len(tile.Traces))
	fmt.Printf("Non-empty traces: %d\n", len(traceKeys))
}

func parseInt(nStr string) int {
	ret, err := strconv.ParseInt(nStr, 10, 0)
	if err != nil {
		glog.Fatalf("ERROR: %s", err.Error())
	}
	return int(ret)
}

func main() {
	common.Init()
	if !util.In(*dataset, config.VALID_DATASETS) {
		glog.Fatalf("Not a valid dataset: %s", *dataset)
	}
	store := filetilestore.NewFileTileStore(*tileDir, *dataset, 0)

	args := flag.Args()

	switch args[0] {
	case VALIDATE:
		if !validator.ValidateDataset(store, *verbose, *echoHashes) {
			glog.Fatal("FAILED Validation.")
		}
	case DUMP_COMMITS:
		nCommits := parseInt(args[1])
		tile, err := store.Get(0, -1)
		if err != nil {
			glog.Fatal("Could not read tile: " + err.Error())
		}
		dumpCommits(tile, nCommits)
	case MD5:
		hash := args[1]
		nCommits := parseInt(args[2])
		md5Commits(store, hash, nCommits)
	default:
		glog.Fatalf("Unknow command: %s", args[0])
	}
}
