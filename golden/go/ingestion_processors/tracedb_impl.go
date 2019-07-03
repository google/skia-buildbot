package ingestion_processors

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/jsonio"
	"go.skia.org/infra/golden/go/types"
)

const (
	// Configuration option that identifies the address of the traceDB service.
	tracedbServiceConfig = "TraceService"
)

// Register the processor with the ingestion framework.
func init() {
	ingestion.Register(config.CONSTRUCTOR_GOLD, newDeprecatedTraceDBProcessor)
}

// traceDBProcessor implements the ingestion.Processor interface for gold.
type traceDBProcessor struct {
	traceDB tracedb.DB
	vcs     vcsinfo.VCS
}

// implements the ingestion.Constructor signature.
func newDeprecatedTraceDBProcessor(vcs vcsinfo.VCS, config *sharedconfig.IngesterConfig, _ *http.Client, _ eventbus.EventBus) (ingestion.Processor, error) {
	traceDB, err := tracedb.NewTraceServiceDBFromAddress(config.ExtraParams[tracedbServiceConfig], types.GoldenTraceBuilder)
	if err != nil {
		return nil, err
	}

	ret := &traceDBProcessor{
		traceDB: traceDB,
		vcs:     vcs,
	}
	return ret, nil
}

// See ingestion.Processor interface.
func (g *traceDBProcessor) Process(ctx context.Context, resultsFile ingestion.ResultFileLocation) error {
	dmResults, err := processDMResults(resultsFile)
	if err != nil {
		return skerr.Fmt("could not process results file: %s", err)
	}

	if len(dmResults.Results) == 0 {
		sklog.Infof("ignoring file %s because it has no results", resultsFile.Name())
		return ingestion.IgnoreResultsFileErr
	}

	var commit *vcsinfo.LongCommit = nil
	// If the target commit is not in the primary repository we look it up
	// in the secondary that has the primary as a dependency.
	targetHash, err := getCanonicalCommitHash(ctx, g.vcs, dmResults.GitHash)
	if err != nil {
		if err == ingestion.IgnoreResultsFileErr {
			return ingestion.IgnoreResultsFileErr
		}
		return skerr.Fmt("could not identify canonical commit from %q: %s", dmResults.GitHash, err)
	}

	commit, err = g.vcs.Details(ctx, targetHash, true)
	if err != nil {
		return skerr.Fmt("could not get details for git commit %q: %s", targetHash, err)
	}

	if !commit.Branches["master"] {
		sklog.Warningf("Commit %s is not in master branch. Got branches: %v", commit.Hash, commit.Branches)
		return ingestion.IgnoreResultsFileErr
	}

	// Add the column to the trace db.
	cid, err := g.getCommitID(commit)
	if err != nil {
		return skerr.Fmt("could not get trace db id: %s", err)
	}

	// Get the entries that should be added to the tracedb.
	entries, err := extractTraceDBEntries(dmResults)
	if err != nil {
		return skerr.Fmt("could not create entries for results: %s", err)
	}

	// Write the result to the tracedb.
	err = g.traceDB.Add(cid, entries)
	if err != nil {
		return skerr.Fmt("could not add to tracedb: %s", err)
	}
	return nil
}

// See ingestion.Processor interface.
func (g *traceDBProcessor) BatchFinished() error { return nil }

// getCommitID extracts the commitID from the given commit.
func (g *traceDBProcessor) getCommitID(commit *vcsinfo.LongCommit) (*tracedb.CommitID, error) {
	return &tracedb.CommitID{
		Timestamp: commit.Timestamp.Unix(),
		ID:        commit.Hash,
		Source:    "master",
	}, nil
}

// extractTraceDBEntries returns the traceDB entries to be inserted into the data store.
func extractTraceDBEntries(dm *dmResults) (map[tiling.TraceId]*tracedb.Entry, error) {
	ret := make(map[tiling.TraceId]*tracedb.Entry, len(dm.Results))
	for _, result := range dm.Results {
		traceId, params := idAndParams(dm, result)
		if ignoreResult(dm, params) {
			continue
		}

		ret[traceId] = &tracedb.Entry{
			Params: params,
			Value:  []byte(result.Digest),
		}
	}

	// If all results were ignored then we return an error.
	if len(ret) == 0 {
		return nil, fmt.Errorf("No valid results in file %s.", dm.name)
	}

	return ret, nil
}

// idAndParams constructs the Trace ID and the Trace params from the keys and options.
// It returns the id as a string of all the values, in the alphabetic order of the
// keys, separated by a colon. The trace params returned are a single map of
// key-> values. "Options" are omitted from the trace id, as per design.
func idAndParams(dm *dmResults, r *jsonio.Result) (tiling.TraceId, map[string]string) {
	combinedLen := len(dm.Key) + len(r.Key)
	traceIdParts := make(map[string]string, combinedLen)
	params := make(map[string]string, combinedLen+len(r.Options))
	for k, v := range dm.Key {
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
	for k := range traceIdParts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	values := []string{}
	for _, k := range keys {
		values = append(values, traceIdParts[k])
	}
	return tiling.TraceId(strings.Join(values, ":")), params
}

// ignoreResult returns true if the result with the given parameters should be
// ignored.
func ignoreResult(dm *dmResults, params map[string]string) bool {
	// Ignore anything that is not a png. In the early days (pre-2015), ext was omitted
	// but implied to be "png". Thus if ext is not provided, it will be ingested.
	// New entries (created by goldctl) will always have ext set.
	if ext, ok := params["ext"]; ok && (ext != "png") {
		return true
	}

	// Make sure the test name meets basic requirements.
	testName := params[types.PRIMARY_KEY_FIELD]

	// Ignore results that don't have a test given and log an error since that
	// should not happen. But we want to keep other results in the same input file.
	if testName == "" {
		sklog.Errorf("Missing test name in %s", dm.name)
		return true
	}

	// Make sure the test name does not exceed the allowed length.
	if len(testName) > types.MAXIMUM_NAME_LENGTH {
		sklog.Errorf("Received test name which is longer than the allowed %d bytes: %s", types.MAXIMUM_NAME_LENGTH, testName)
		return true
	}

	return false
}
