package perfingestion

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/sharedconfig"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/types"
	"google.golang.org/grpc"
)

const (
	// Configuration option that identifies the address of the traceDB service.
	CONFIG_TRACESERVICE = "TraceService"
)

// Register the processor with the ingestion framework.
func init() {
	ingestion.Register(config.CONSTRUCTOR_NANO, newPerfProcessor)
}

// BenchResult represents a single test result.
//
// Used in BenchData.
//
// Expected to be a map of strings to float64s, with the
// exception of the "options" entry which should be a
// map[string]string.
type BenchResult map[string]interface{}

// BenchResults is the dictionary of individual BenchResult structs.
//
// Used in BenchData.
type BenchResults map[string]BenchResult

// BenchData is the top level struct for decoding the nanobench JSON format.
type BenchData struct {
	Hash    string                  `json:"gitHash"`
	Key     map[string]string       `json:"key"`
	Options map[string]string       `json:"options"`
	Results map[string]BenchResults `json:"results"`
}

// keyPrefix returns the prefix that is common to all trace ids in a single
// instance of BenchData.
func (b *BenchData) keyPrefix() string {
	keys := make([]string, 0, len(b.Key))
	retval := make([]string, 0, len(b.Key))

	for k, _ := range b.Key {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		retval = append(retval, b.Key[k])
	}
	return strings.Join(retval, ":")
}

// getTraceDBEntries returns a map of tracedb.Entry instances.
func (b *BenchData) getTraceDBEntries() map[string]*tracedb.Entry {
	ret := make(map[string]*tracedb.Entry, len(b.Results))
	keyPrefix := b.keyPrefix()
	for testName, allConfigs := range b.Results {
		for configName, result := range allConfigs {
			key := fmt.Sprintf("%s:%s:%s", keyPrefix, testName, configName)

			// Construct the Traces params from all the options.
			params := util.CopyStringMap(b.Key)
			params["test"] = testName
			params["config"] = configName
			util.AddParams(params, b.Options)

			// If there is an options map inside the result add it to the params.
			if resultOptions, ok := result["options"]; ok {
				if opts, ok := resultOptions.(map[string]interface{}); ok {
					for k, vi := range opts {
						if s, ok := vi.(string); ok {
							params[k] = s
						}
					}
				}
			}

			// We used to just pick out only "min_ms" as the only result of a bunch
			// of key, value pairs in the result, such as max_ms, mean_ms, etc. Now
			// nanobench uploads only the metrics we are interested in, so we need to
			// use all the values there, except 'options'.
			for k, vi := range result {
				if k == "options" {
					continue
				}

				floatVal, ok := vi.(float64)
				if !ok {
					glog.Errorf("Found a non-float64 in %s", key)
					continue
				}

				params["sub_result"] = k
				perResultKey := key
				if k != "min_ms" {
					perResultKey = fmt.Sprintf("%s:%s", perResultKey, k)
				}
				paramsCopy := util.CopyStringMap(params)
				ret[perResultKey] = &tracedb.Entry{
					Params: paramsCopy,
					Value:  types.BytesFromFloat64(floatVal),
				}
			}
		}
	}
	return ret
}

// perfProcessor implements the ingestion.Processor interface for perf.
type perfProcessor struct {
	traceDB tracedb.DB
	vcs     vcsinfo.VCS
}

// implements the ingestion.Constructor signature.
func newPerfProcessor(vcs vcsinfo.VCS, config *sharedconfig.IngesterConfig) (ingestion.Processor, error) {
	traceDB, err := getTraceDB(config.ExtraParams[CONFIG_TRACESERVICE])
	if err != nil {
		return nil, err
	}

	return &perfProcessor{
		traceDB: traceDB,
		vcs:     vcs,
	}, nil
}

// See ingestion.Processor interface.
func (p *perfProcessor) Process(resultsFile ingestion.ResultFileLocation) error {
	r, err := resultsFile.Open()
	if err != nil {
		return err
	}

	benchData, err := parseBenchDataFromReader(r)
	if err != nil {
		return err
	}

	commit, err := p.vcs.Details(benchData.Hash)
	if err != nil {
		return err
	}

	cid := &tracedb.CommitID{
		Timestamp: commit.Timestamp,
		ID:        commit.Hash,
		Source:    "master",
	}

	// Add the column to the trace db.
	return p.traceDB.Add(cid, benchData.getTraceDBEntries())
}

// See ingestion.Processor interface.
func (p *perfProcessor) BatchFinished() error { return nil }

// getTraceDB is given the address of the traceService implementation and
// returns an instance of the traceDB (the higher level wrapper on top of
// trace service).
func getTraceDB(traceServiceAddr string) (tracedb.DB, error) {
	if traceServiceAddr == "" {
		return nil, fmt.Errorf("No value for '%s' specified in config.", CONFIG_TRACESERVICE)
	}

	conn, err := grpc.Dial(traceServiceAddr, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("Unable to connnect to trace service at %s. Got error: %s", traceServiceAddr, err)
	}

	return tracedb.NewTraceServiceDB(conn, types.PerfTraceBuilder)
}

// parseBenchDataFromReader parses the stream out of the io.ReadCloser
// into BenchData and closes the reader.
func parseBenchDataFromReader(r io.ReadCloser) (*BenchData, error) {
	defer util.Close(r)

	dec := json.NewDecoder(r)
	benchData := &BenchData{}
	if err := dec.Decode(benchData); err != nil {
		return nil, fmt.Errorf("Failed to decode JSON: %s", err)
	}
	return benchData, nil
}
