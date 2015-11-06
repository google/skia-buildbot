package goldingestion

// The JSON output from DM looks like this:
//
//  {
//     "build_number" : "20",
//     "gitHash" : "abcd",
//     "key" : {
//        "arch" : "x86",
//        "configuration" : "Debug",
//        "gpu" : "nvidia",
//        "model" : "z620",
//        "os" : "Ubuntu13.10"
//     },
//     "results" : [
//        {
//           "key" : {
//              "config" : "565",
//              "name" : "ninepatch-stretch",
//              "source_type" : "gm"
//           },
//           "md5" : "f78cfafcbabaf815f3dfcf61fb59acc7",
//           "options" : {
//              "ext" : "png"
//           }
//        },
//        {
//           "key" : {
//              "config" : "8888",
//              "name" : "ninepatch-stretch",
//              "source_type" : "gm"
//           },
//           "md5" : "3e8a42f35a1e76f00caa191e6310d789",
//           "options" : {
//              "ext" : "png"
//           }

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/sharedconfig"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/types"
)

const (
	// Configuration option that identifies the address of the traceDB service.
	CONFIG_TRACESERVICE = "TraceService"
)

// Register the processor with the ingestion framework.
func init() {
	ingestion.Register(config.CONSTRUCTOR_GOLD, newGoldProcessor)
}

// Result is used by DMResults hand holds the invidual result of one test.
type Result struct {
	Key     map[string]string `json:"key"`
	Options map[string]string `json:"options"`
	Digest  string            `json:"md5"`
}

// DMResults is the top level structure for decoding DM JSON output.
type DMResults struct {
	BuildNumber string            `json:"build_number"`
	GitHash     string            `json:"gitHash"`
	Key         map[string]string `json:"key"`
	Issue       string            `json:"issue"`
	Patchset    int64             `json:"patchset,string"`
	Results     []*Result         `json:"results"`
}

// idAndParams constructs the Trace ID and the Trace params from the keys and options.
func (d *DMResults) idAndParams(r *Result) (string, map[string]string) {
	combinedLen := len(d.Key) + len(r.Key)
	traceIdParts := make(map[string]string, combinedLen)
	params := make(map[string]string, combinedLen)
	for k, v := range d.Key {
		traceIdParts[k] = v
		params[k] = v
	}
	for k, v := range r.Key {
		traceIdParts[k] = v
		params[k] = v
	}
	for k, v := range r.Options {
		params[k] = v
	}

	keys := []string{}
	for k, _ := range traceIdParts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	values := []string{}
	for _, k := range keys {
		values = append(values, traceIdParts[k])
	}
	return strings.Join(values, ":"), params
}

// getTraceDBEntries returns the traceDB entries to be inserted into the data store.
func (d *DMResults) getTraceDBEntries() map[string]*tracedb.Entry {
	ret := make(map[string]*tracedb.Entry, len(d.Results))
	for _, result := range d.Results {
		traceId, params := d.idAndParams(result)
		if ignoreResult(params) {
			continue
		}

		ret[traceId] = &tracedb.Entry{
			Params: params,
			Value:  []byte(result.Digest),
		}
	}
	return ret
}

// ignoreResult returns true if the result with the given parameters should be
// ignored.
func ignoreResult(params map[string]string) bool {
	// Ignore anything that is not a png.
	ext, ok := params["ext"]
	return !ok || (ext != "png")
}

// goldProcessor implements the ingestion.Processor interface for gold.
type goldProcessor struct {
	traceDB tracedb.DB
	vcs     vcsinfo.VCS
}

// implements the ingestion.Constructor signature.
func newGoldProcessor(vcs vcsinfo.VCS, config *sharedconfig.IngesterConfig) (ingestion.Processor, error) {
	traceDB, err := tracedb.NewTraceServiceDBFromAddress(config.ExtraParams[CONFIG_TRACESERVICE], types.GoldenTraceBuilder)
	if err != nil {
		return nil, err
	}

	return &goldProcessor{
		traceDB: traceDB,
		vcs:     vcs,
	}, nil
}

// See ingestion.Processor interface.
func (g *goldProcessor) Process(resultsFile ingestion.ResultFileLocation) error {
	r, err := resultsFile.Open()
	if err != nil {
		return err
	}

	dmResults, err := parseDMResultsFromReader(r)
	if err != nil {
		return err
	}

	commit, err := g.vcs.Details(dmResults.GitHash)
	if err != nil {
		return err
	}

	cid := &tracedb.CommitID{
		Timestamp: commit.Timestamp,
		ID:        commit.Hash,
		Source:    "master",
	}

	// Add the column to the trace db.
	return g.traceDB.Add(cid, dmResults.getTraceDBEntries())
}

// See ingestion.Processor interface.
func (g *goldProcessor) BatchFinished() error { return nil }

// parseBenchDataFromReader parses the stream out of the io.ReadCloser
// into BenchData and closes the reader.
func parseDMResultsFromReader(r io.ReadCloser) (*DMResults, error) {
	defer util.Close(r)

	dec := json.NewDecoder(r)
	dmResults := &DMResults{}
	if err := dec.Decode(dmResults); err != nil {
		return nil, fmt.Errorf("Failed to decode JSON: %s", err)
	}
	return dmResults, nil
}
