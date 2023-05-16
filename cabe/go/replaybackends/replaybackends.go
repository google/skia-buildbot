// Package replaybackends provides in-memory implementations of backend dependencies
// for testing. These implementations re-play backend responses recorded during calls
// to live production services.
package replaybackends

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"reflect"
	"strings"

	"go.skia.org/infra/cabe/go/analyzer"
)

// ReplayBackends implements the backend interfaces required for testing package etl and rpcservice.
type ReplayBackends struct {
	parsedPerfResults map[string]analyzer.PerfResults
	CASResultReader   analyzer.CASResultReader
}

// FromZipFile opens a zip archive of pre-recorded backend responses and populates a
// ReplayBackends instance with it suitable for testing purposes.
//
// Replay zip files have the following internal directory structure:
// ./task_requests.textproto - text proto bigquery responses for swarming task requests
// ./task_results.textproto - test proto bigquery responses for swarming task results
// ./cas/<digest hash> - (multiple) the individual swarming task measurement output files
//
// benchmarkName is typically something determined by the thing *executing* the benchmarks, not
// by the benchmark code itself. Thus, when reconstructing the artifacts for a benchmark run we
// need to have that name provided a priori since it can't be determined automatically from task
// output files alone.
func FromZipFile(replayZipfile string, benchmarkName string) *ReplayBackends {

	archive, err := zip.OpenReader(replayZipfile)
	if err != nil {
		panic(err)
	}
	defer archive.Close()

	ret := &ReplayBackends{}
	ret.parsedPerfResults = make(map[string]analyzer.PerfResults)

	for _, file := range archive.File {
		dirName, fileName := path.Split(file.Name)
		if dirName == "cas/" {
			fileReader, err := file.Open()
			if err != nil {
				panic(err)
			}
			defer fileReader.Close()

			fileBytes := make([]byte, file.UncompressedSize64)
			if _, err = io.ReadFull(fileReader, fileBytes); err != nil {
				panic(err)
			}
			res := analyzer.PerfResults{}

			// if a task had no CAS output (e.g. task failed entirely) then the json can be zero length.
			if len(fileBytes) > 0 {
				// But if there are bytes, they need to actually parse or it's fatal for the test setup.
				err = json.Unmarshal(fileBytes, &res)
				if err != nil {
					fmt.Printf("failed to parse %q of type %v",
						file.Name,
						reflect.TypeOf(fileBytes))
					panic(err)
				}
			}

			ret.parsedPerfResults[fileName] = res
		}

	}

	// returns a map of benchmark name to parsed PerfResults.
	ret.CASResultReader = func(c context.Context, instance, digest string) (map[string]analyzer.PerfResults, error) {
		df := strings.Split(digest, "/")[0]
		res, ok := ret.parsedPerfResults[df]
		if !ok {
			return nil, fmt.Errorf("couldn't find a CAS blob for %q (%q)", digest, df)
		}
		return map[string]analyzer.PerfResults{benchmarkName: res}, nil
	}

	return ret
}
