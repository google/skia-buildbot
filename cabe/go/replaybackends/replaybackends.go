// Package replaybackends provides in-memory implementations of backend dependencies
// for testing. These implementations re-play backend responses recorded during calls
// to live production services.
// To update the files containing replay data for cabe unit tests, see the
// instructions here: go/cabe-skia-assets
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

	swarmingapi "go.chromium.org/luci/common/api/swarming/swarming/v1"

	"go.skia.org/infra/cabe/go/backends"
	"go.skia.org/infra/cabe/go/perfresults"
)

// ReplayBackends implements the backend interfaces required for testing package etl and rpcservice.
type ReplayBackends struct {
	ParsedPerfResults   map[string]perfresults.PerfResults
	ParsedSwarmingTasks []*swarmingapi.SwarmingRpcsTaskRequestMetadata

	CASResultReader    backends.CASResultReader
	SwarmingTaskReader backends.SwarmingTaskReader
}

// FromZipFile opens a zip archive of pre-recorded backend responses and populates a
// ReplayBackends instance with it suitable for testing purposes.
//
// Replay zip files have the following internal directory structure:
// ./swarming-tasks.json - json serialized array of [swarmingapi.SwarmingRpcsTaskRequestMetadata]
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
	ret.ParsedPerfResults = make(map[string]perfresults.PerfResults)

	for _, file := range archive.File {
		dirName, fileName := path.Split(file.Name)
		if fileName == "swarming-tasks.json" {
			fileReader, err := file.Open()
			if err != nil {
				panic(err)
			}
			defer fileReader.Close()

			tasksBytes := make([]byte, file.UncompressedSize64)
			if _, err = io.ReadFull(fileReader, tasksBytes); err != nil {
				panic(err)
			}

			if err = json.Unmarshal([]byte(tasksBytes), &ret.ParsedSwarmingTasks); err != nil {
				panic(err)
			}
		}

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
			res := perfresults.PerfResults{}

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

			ret.ParsedPerfResults[fileName] = res
		}
	}

	ret.SwarmingTaskReader = func(ctx context.Context) ([]*swarmingapi.SwarmingRpcsTaskRequestMetadata, error) {
		return ret.ParsedSwarmingTasks, nil
	}

	// returns a map of benchmark name to parsed PerfResults.
	ret.CASResultReader = func(c context.Context, instance, digest string) (map[string]perfresults.PerfResults, error) {
		df := strings.Split(digest, "/")[0]
		res, ok := ret.ParsedPerfResults[df]
		if !ok {
			return nil, fmt.Errorf("couldn't find a CAS blob for %q (%q)", digest, df)
		}
		return map[string]perfresults.PerfResults{benchmarkName: res}, nil
	}

	return ret
}
