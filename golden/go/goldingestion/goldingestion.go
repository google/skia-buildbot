package goldingestion

import (
	"fmt"
	"net/http"

	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/sharedconfig"
	tracedb "go.skia.org/infra/go/trace/db"
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

// goldProcessor implements the ingestion.Processor interface for gold.
type goldProcessor struct {
	traceDB   tracedb.DB
	vcs       vcsinfo.VCS
	extractID extractIDFn
}

type extractIDFn func(*vcsinfo.LongCommit, *DMResults) (*tracedb.CommitID, error)

// implements the ingestion.Constructor signature.
func newGoldProcessor(vcs vcsinfo.VCS, config *sharedconfig.IngesterConfig, client *http.Client) (ingestion.Processor, error) {
	traceDB, err := tracedb.NewTraceServiceDBFromAddress(config.ExtraParams[CONFIG_TRACESERVICE], types.GoldenTraceBuilder)
	if err != nil {
		return nil, err
	}

	ret := &goldProcessor{
		traceDB: traceDB,
		vcs:     vcs,
	}
	ret.extractID = ret.getCommitID
	return ret, nil
}

// See ingestion.Processor interface.
func (g *goldProcessor) Process(resultsFile ingestion.ResultFileLocation) error {
	r, err := resultsFile.Open()
	if err != nil {
		return err
	}

	dmResults, err := ParseDMResultsFromReader(r)
	if err != nil {
		return err
	}

	commit, err := g.vcs.Details(dmResults.GitHash, true)
	if err != nil {
		return err
	}

	if !commit.Branches["master"] {
		return fmt.Errorf("Commit %s is not in master branch.", commit.Hash)
	}

	// Add the column to the trace db.
	cid, err := g.extractID(commit, dmResults)
	if err != nil {
		return err
	}

	return g.traceDB.Add(cid, dmResults.getTraceDBEntries())
}

// See ingestion.Processor interface.
func (g *goldProcessor) BatchFinished() error { return nil }

// getCommitID extracts the commitID from the given commit and dm results.
func (g *goldProcessor) getCommitID(commit *vcsinfo.LongCommit, dmResults *DMResults) (*tracedb.CommitID, error) {
	return &tracedb.CommitID{
		Timestamp: commit.Timestamp.Unix(),
		ID:        commit.Hash,
		Source:    "master",
	}, nil
}
