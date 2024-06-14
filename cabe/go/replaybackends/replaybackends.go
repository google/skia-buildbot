// Package replaybackends provides in-memory implementations of backend dependencies
// for testing. These implementations re-play backend responses recorded during calls
// to live production services.
// To update the files containing replay data for cabe unit tests, see the
// instructions here: go/cabe-skia-assets
package replaybackends

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	rbeclient "github.com/bazelbuild/remote-apis-sdks/go/pkg/client"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.skia.org/infra/cabe/go/backends"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	swarmingv2 "go.skia.org/infra/go/swarming/v2"
	"go.skia.org/infra/perf/go/perfresults"
)

const (
	swarmingTasksFileName = "swarming-tasks.json"
	casDirName            = "cas"
)

// ReplayBackends implements the backend interfaces required for testing package etl and rpcservice.
type ReplayBackends struct {
	ParsedPerfResults   map[string]perfresults.PerfResults
	ParsedSwarmingTasks []*apipb.TaskRequestMetadataResponse

	rbeClients map[string]*rbeclient.Client

	CASResultReader    backends.CASResultReader
	SwarmingTaskReader backends.SwarmingTaskReader

	replayZipFile string

	// zipBuf is an in-memory buffer for data to be written out to a zip file.
	zipBuf *bytes.Buffer

	// muZip protects zipWriter
	muZip     sync.Mutex
	zipWriter *zip.Writer
}

// FromZipFile opens a zip archive of pre-recorded backend responses and populates a
// ReplayBackends instance with it suitable for testing purposes.
//
// Replay zip files have the following internal directory structure:
// ./swarming-tasks.json - json serialized array of [apipb.TaskRequestMetadataResponse]
// ./cas/<digest hash> - (multiple) the individual swarming task measurement output files
//
// benchmarkName is typically something determined by the thing *executing* the benchmarks, not
// by the benchmark code itself. Thus, when reconstructing the artifacts for a benchmark run we
// need to have that name provided a priori since it can't be determined automatically from task
// output files alone.
func FromZipFile(replayZipFile string, benchmarkName string) *ReplayBackends {
	archive, err := zip.OpenReader(replayZipFile)
	if err != nil {
		panic(err)
	}
	defer archive.Close()

	ret := &ReplayBackends{
		replayZipFile: replayZipFile,
	}
	ret.ParsedPerfResults = make(map[string]perfresults.PerfResults)

	for _, file := range archive.File {
		dirName, fileName := path.Split(file.Name)
		if fileName == swarmingTasksFileName {
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

		if dirName == casDirName+"/" {
			fileReader, err := file.Open()
			if err != nil {
				panic(err)
			}
			defer fileReader.Close()

			res, err := perfresults.NewResults(fileReader)
			if err != nil {
				fmt.Printf("failed to parse %q", file.Name)
				panic(err)
			}
			ret.ParsedPerfResults[fileName] = *res
		}
	}

	ret.SwarmingTaskReader = func(ctx context.Context, pinpointJobID string) ([]*apipb.TaskRequestMetadataResponse, error) {
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

// ToZipFile is the inverse of FromZipFile. Given a filename to record to, and RBE and Swarming API
// clients to intercept, this returns a [*ReplayBackends] instance with SwarmingTaskReader and
// CasResultReader properties set to functions that will record responses from live services.
// Be sure to call [ReplayBackends.Close()] when you are done making calls to RBE and Swarming
// in order to complete the recording process and save the replay data to replayZipFile.
func ToZipFile(replayZipFile string,
	rbeClients map[string]*rbeclient.Client,
	swarmingClient swarmingv2.SwarmingV2Client) *ReplayBackends {
	ret := &ReplayBackends{
		replayZipFile: replayZipFile,
	}
	ret.SwarmingTaskReader = func(ctx context.Context, pinpointJobID string) ([]*apipb.TaskRequestMetadataResponse, error) {
		sklog.Infof("getting task metadata from swarming service, pinpoint_job_id: %v", pinpointJobID)
		rmd, err := swarmingv2.ListTaskRequestMetadataHelper(ctx, swarmingClient, &apipb.TasksWithPerfRequest{
			Start: timestamppb.New(time.Now().Add(-time.Hour * 24 * 52)), // past 52 days
			State: apipb.StateQuery_QUERY_ALL,
			Tags:  []string{"pinpoint_job_id:" + pinpointJobID},
		})
		if err != nil {
			sklog.Errorf("getting swarming tasks: %v", err)
			return nil, err
		}
		raw, err := json.Marshal(rmd)
		if err != nil {
			return nil, err
		}
		if err := ret.recordData(raw, swarmingTasksFileName); err != nil {
			return nil, err
		}
		return rmd, err
	}
	ret.CASResultReader = func(ctx context.Context, instance, digest string) (map[string]perfresults.PerfResults, error) {
		rbeClient, ok := rbeClients[instance]
		if !ok {
			return nil, fmt.Errorf("no RBE client for instance %s", instance)
		}

		res, err := backends.FetchBenchmarkJSONRaw(ctx, rbeClient, digest)
		if err != nil {
			return nil, err
		}
		if len(res) > 1 {
			return nil, fmt.Errorf("recording doesn't work for outputs with multiple benchmarks")
		}
		d := strings.Split(digest, "/")[0]

		for _, raw := range res {
			if err := ret.recordData(raw, casDirName, d); err != nil {
				return nil, err
			}
		}
		ret := make(map[string]perfresults.PerfResults)
		for benchmark, blob := range res {
			res, err := perfresults.NewResults(bytes.NewReader(blob))
			if err != nil {
				return nil, skerr.Wrapf(err, "unmarshaling benchmark json")
			}
			ret[benchmark] = *res
		}
		return ret, nil
	}
	ret.zipBuf = new(bytes.Buffer)
	ret.zipWriter = zip.NewWriter(ret.zipBuf)
	return ret
}

func (r *ReplayBackends) Close() error {
	if r.zipWriter == nil {
		return nil
	}
	if err := r.zipWriter.Close(); err != nil {
		return err
	}
	sklog.Infof("writing replay data to %s", r.replayZipFile)

	return os.WriteFile(r.replayZipFile, r.zipBuf.Bytes(), 0755)
}

func (r *ReplayBackends) recordData(raw []byte, path ...string) error {
	r.muZip.Lock()
	defer r.muZip.Unlock()
	sklog.Infof("about to write %d bytes to %s in replay zip", len(raw), filepath.Join(path...))
	f, err := r.zipWriter.Create(filepath.Join(path...))
	if err != nil {
		sklog.Fatal(err)
	}
	_, err = f.Write(raw)
	if err != nil {
		sklog.Fatal(err)
	}

	sklog.Infof("successfully wrote %d bytes to %s in replay zip", len(raw), filepath.Join(path...))
	return nil
}
