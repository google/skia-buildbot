package perfingestion

import (
	"fmt"
	"net/http"

	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/sharedconfig"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/types"
)

// Register the processor with the ingestion framework.
func init() {
	ingestion.Register(config.CONSTRUCTOR_NANO, newPerfProcessor)
}

// perfProcessor implements the ingestion.Processor interface for perf.
type perfProcessor struct {
	traceDB tracedb.DB
	vcs     vcsinfo.VCS
}

// newPerfProcessor implements the ingestion.Constructor signature.
func newPerfProcessor(vcs vcsinfo.VCS, config *sharedconfig.IngesterConfig, client *http.Client) (ingestion.Processor, error) {
	traceDB, err := tracedb.NewTraceServiceDBFromAddress(config.ExtraParams[CONFIG_TRACESERVICE], types.PerfTraceBuilder)
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

	if !commit.Branches["master"] {
		return fmt.Errorf("Commit %s is not in master branch.", commit.Hash)
	}

	cid := &tracedb.CommitID{
		Timestamp: commit.Timestamp.Unix(),
		ID:        commit.Hash,
		Source:    "master",
	}

	// Add the column to the trace db.
	return p.traceDB.Add(cid, benchData.getTraceDBEntries())
}

// See ingestion.Processor interface.
func (p *perfProcessor) BatchFinished() error { return nil }
