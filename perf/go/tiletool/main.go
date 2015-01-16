// tiletool is a command line application to validate a tile store.
package main

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/gob"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"sort"
	"strconv"
	"strings"

	"github.com/skia-dev/glog"
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
	JSON         = "json"
)

// Command line flags.
var (
	tileDir    = flag.String("tile_dir", "/tmp/tileStore", "What directory to look for tiles in.")
	verbose    = flag.Bool("verbose", false, "Verbose.")
	echoHashes = flag.Bool("echo_hashes", false, "Echo Git hashes during validation.")
	dataset    = flag.String("dataset", config.DATASET_NANO, fmt.Sprintf("Choose from the valid datasets: %v", config.VALID_DATASETS))
)

func dumpCommits(store types.TileStore, n int) {
	tile, err := store.Get(0, -1)
	if err != nil {
		glog.Fatal("Could not read tile: " + err.Error())
	}

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
			if !v.IsMissing(i) {
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

func dumpTileToJSON(store types.TileStore, nCommits int, nTraces int, fname string) {
	tile, err := store.Get(0, -1)
	if err != nil {
		glog.Fatal("Could not read tile: " + err.Error())
	}

	newTile := tile
	if (nCommits > 0) || (nTraces > 0) {
		lastIdx := tile.LastCommitIndex()
		if nCommits <= 0 {
			nCommits = lastIdx + 1
		}

		if nTraces <= 0 {
			nTraces = len(tile.Traces)
		}

		commitLen := util.MinInt(nCommits, lastIdx+1)
		startCommit := lastIdx + 1 - commitLen
		newTraces := map[string]types.Trace{}
		for key, trace := range tile.Traces {
			for i := startCommit; i <= lastIdx; i++ {
				if !trace.IsMissing(i) {
					newTraces[key] = trace
					break
				}
			}
			if len(newTraces) >= nTraces {
				break
			}
		}

		newCommits := tile.Commits[startCommit:]
		newParamSet := map[string][]string{}
		types.GetParamSet(newTraces, newParamSet)

		newTile = &types.Tile{
			Traces:    newTraces,
			ParamSet:  newParamSet,
			Commits:   newCommits,
			Scale:     tile.Scale,
			TileIndex: tile.TileIndex,
		}
	}

	result, err := json.Marshal(newTile)
	if err != nil {
		glog.Fatalf("Could not marshal to JSON: %s", err)
	}

	err = ioutil.WriteFile(fname, result, 0644)
	if err != nil {
		glog.Fatalf("Could not write output file %s", err)
	}

	fmt.Printf("Commits included: %d\n", len(newTile.Commits))
	fmt.Printf("Traces included:  %d\n", len(newTile.Traces))
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
	for k, v := range tile.Traces {
		for i := startIdx; i < endIdx; i++ {
			if !v.IsMissing(i) {
				traceKeys = append(traceKeys, k)
				break
			}
		}
	}
	sort.Strings(traceKeys)

	result := make([][]string, len(traceKeys))
	for i, k := range traceKeys {
		switch trace := tile.Traces[k].(type) {
		case *types.GoldenTrace:
			result[i] = trace.Values[startIdx:endIdx]
		case *types.PerfTrace:
			result[i] = asStringSlice(trace.Values[startIdx:endIdx])
		}
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

func asStringSlice(fVals []float64) []string {
	result := make([]string, len(fVals))
	for idx, val := range fVals {
		var buf bytes.Buffer
		if err := binary.Write(&buf, binary.LittleEndian, val); err != nil {
			glog.Fatalf("Unable to convert float to bytes: %f", val)
		}
		result[idx] = string(buf.Bytes())
	}
	return result
}

func parseInt(nStr string) int {
	ret, err := strconv.ParseInt(nStr, 10, 0)
	if err != nil {
		glog.Fatalf("ERROR: %s", err.Error())
	}
	return int(ret)
}

func printUsage() {
	fmt.Printf("Usage: %s [flags] command [parameters]\n\n", os.Args[0])
	fmt.Println("Valid commands are:")

	fmt.Printf("   %s \n", VALIDATE)
	fmt.Printf("      Validates the tile.\n")
	fmt.Printf("   %s n \n", DUMP_COMMITS)
	fmt.Printf("      Dumps the last n commits in the tile.\n")
	fmt.Printf("   %s githash n\n", MD5)
	fmt.Printf("      Returns the MD5 hash of n commits up to the commit identified by githash.\n")
	fmt.Printf("   %s commits traces outputfile\n", JSON)
	fmt.Printf("      Dumps a tile to JSON that consists has the given number of commits and traces.\n")
	fmt.Println("\n\nFlags:")
	flag.PrintDefaults()
}

func checkArgs(args []string, command string, requiredArgs int) {
	if len(args) != (requiredArgs + 1) {
		fmt.Printf("ERROR: The %s command requires exactly %d arguments.\n\n", command, requiredArgs)
		printUsage()
		os.Exit(1)
	}
}

func main() {
	flag.Usage = printUsage
	common.Init()
	if !util.In(*dataset, config.VALID_DATASETS) {
		glog.Fatalf("Not a valid dataset: %s", *dataset)
	}

	args := flag.Args()
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	store := filetilestore.NewFileTileStore(*tileDir, *dataset, 0)

	switch args[0] {
	case VALIDATE:
		if !validator.ValidateDataset(store, *verbose, *echoHashes) {
			glog.Fatal("FAILED Validation.")
		}
	case DUMP_COMMITS:
		checkArgs(args, DUMP_COMMITS, 1)
		nCommits := parseInt(args[1])
		dumpCommits(store, nCommits)
	case MD5:
		checkArgs(args, MD5, 2)
		hash := args[1]
		nCommits := parseInt(args[2])
		md5Commits(store, hash, nCommits)
	case JSON:
		checkArgs(args, JSON, 3)
		nCommits := parseInt(args[1])
		nTraces := parseInt(args[2])
		fname := args[3]
		dumpTileToJSON(store, nCommits, nTraces, fname)
	default:
		glog.Fatalf("Unknow command: %s", args[0])
	}
}
